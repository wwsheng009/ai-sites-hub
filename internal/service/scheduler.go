// Package service 同步调度器（site_sync_state 驱动 + singleflight 防重入）。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"

	"aiclient/internal/adapter"
	"aiclient/internal/model"
	"aiclient/internal/repo"
)

// 默认调度参数。
const (
	defaultInterval = 300 // 5min
	freshnessWindow = 2 * time.Minute
)

// SyncScheduler 站点同步调度器（architecture §3）。
type SyncScheduler struct {
	reg     *adapter.Registry
	repo    *repo.Repo
	svc     *Services
	sem     chan struct{}
	stop    chan struct{}
	wg      sync.WaitGroup
	budgets map[string]*rateLimiter // siteID -> 简单令牌桶
	mu      sync.Mutex
	sf      map[string]*sync.Mutex // singleflight key -> mutex
	sfMu    sync.Mutex
	log     *slog.Logger
}

// rateLimiter 极简令牌桶（每站点 1 rps，可配置）。
type rateLimiter struct {
	mu     sync.Mutex
	tokens float64
	rate   float64
	last   time.Time
}

func newRateLimiter(rps float64) *rateLimiter {
	return &rateLimiter{tokens: rps, rate: rps, last: time.Now()}
}

func (l *rateLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(l.last).Seconds()
	l.tokens = min(l.tokens+elapsed*l.rate, l.rate)
	l.last = now
	if l.tokens >= 1 {
		l.tokens--
		return true
	}
	return false
}

// NewSyncScheduler 构建调度器（未启动）。
func NewSyncScheduler(reg *adapter.Registry, r *repo.Repo, svc *Services, log *slog.Logger) *SyncScheduler {
	return &SyncScheduler{
		reg:     reg,
		repo:    r,
		svc:     svc,
		sem:     make(chan struct{}, 4),
		stop:    make(chan struct{}),
		budgets: map[string]*rateLimiter{},
		log:     log,
	}
}

// Start 启动调度循环（每个 tick 扫描到期域）。
func (s *SyncScheduler) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stop:
				return
			case <-ticker.C:
				s.tick(ctx)
			}
		}
	}()
	s.log.Info("scheduler: started")
}

// Stop 停止调度循环并等待退出。
func (s *SyncScheduler) Stop() {
	close(s.stop)
	s.wg.Wait()
	s.log.Info("scheduler: stopped")
}

// tick 扫描待调度域并并发执行（每站点 ≤4 并发）。
func (s *SyncScheduler) tick(ctx context.Context) {
	sites, err := s.repo.ListSites(ctx, "active")
	if err != nil {
		s.log.Warn("scheduler: 读取站点列表", "err", err)
		return
	}
	now := time.Now()
	for _, site := range sites {
		if err := s.ensureSiteDomains(ctx, &site); err != nil {
			s.log.Warn("scheduler: 补全站点域状态", "site_id", site.ID, "err", err)
		}
		due, err := s.repo.ListSyncStateDue(ctx, now)
		if err != nil {
			continue
		}
		for _, row := range due {
			if row.SiteID != site.ID || row.Enabled == 0 {
				continue
			}
			s.sem <- struct{}{}
			s.wg.Add(1)
			go func(r repo.SyncStateRow) {
				defer func() {
					<-s.sem
					s.wg.Done()
				}()
				s.runDomain(ctx, site, r)
			}(row)
		}
	}
}

// domainInterval 各域默认调度间隔（s）；usage_daily 离线聚合每日一次。
func domainInterval(d string) int {
	if d == "usage_daily" {
		return 86400
	}
	return defaultInterval
}

// ensureSiteDomains 为站点补全默认域行（幂等）。
func (s *SyncScheduler) ensureSiteDomains(ctx context.Context, site *model.Site) error {
	domains := []string{"keys", "groups", "account", "affiliate", "usage_log", "usage_daily", "announcements", "checkin"}
	now := time.Now()
	for _, d := range domains {
		if err := s.repo.UpsertSyncState(ctx, &model.SiteSyncState{
			SiteID:    site.ID,
			Domain:    d,
			Enabled:   1,
			IntervalS: domainInterval(d),
			NextRunAt: &now,
		}); err != nil {
			return err
		}
	}
	return nil
}

// runDomain 执行单域同步（singleflight 防重入 + 新鲜度窗口）。
func (s *SyncScheduler) runDomain(ctx context.Context, site model.Site, row repo.SyncStateRow) {
	key := site.ID + ":" + row.Domain
	// singleflight 防重入：获得 key 的互斥锁否则跳过（手动触发 + 定时同时到账）。
	m := s.lockOf(key)
	if !m.TryLock() {
		s.log.Debug("scheduler: 域同步并发去重", "site_id", site.ID, "domain", row.Domain)
		return
	}
	defer m.Unlock()
	if err := s.syncDomain(ctx, site, row); err != nil {
		s.log.Warn("scheduler: 域同步失败", "site_id", site.ID, "domain", row.Domain, "err", err)
	}
}

// lockOf 获得 key 对应的互斥锁（懒创建）。
func (s *SyncScheduler) lockOf(key string) *sync.Mutex {
	s.sfMu.Lock()
	defer s.sfMu.Unlock()
	if s.sf == nil {
		s.sf = map[string]*sync.Mutex{}
	}
	m, ok := s.sf[key]
	if !ok {
		m = &sync.Mutex{}
		s.sf[key] = m
	}
	return m
}

// syncDomain 单域同步 + 状态落库（含退避）。
func (s *SyncScheduler) syncDomain(ctx context.Context, site model.Site, row repo.SyncStateRow) error {
	// 新鲕度窗口：距上次成功 < 2min → 跳过（force 可越过）
	if row.LastRunAt != nil && time.Since(*row.LastRunAt) < freshnessWindow {
		s.log.Debug("scheduler: 域在新鲕度窗口内跳过", "site_id", site.ID, "domain", row.Domain)
		return nil
	}
	// 限流
	s.mu.Lock()
	rl := s.budgets[site.ID]
	if rl == nil {
		rl = newRateLimiter(1)
		s.budgets[site.ID] = rl
	}
	s.mu.Unlock()
	if !rl.allow() {
		s.log.Debug("scheduler: 限流跳过", "site_id", site.ID, "domain", row.Domain)
		return nil
	}

	ad, err := s.reg.Get(adapter.Type(site.SiteType), site.BaseURL, site.ProxyURL, s.log)
	if err != nil {
		return s.markErr(ctx, site.ID, row.Domain, "upstream_error", err)
	}
	cred, err := s.repo.GetCredential(ctx, site.ID)
	if err != nil {
		return s.markErr(ctx, site.ID, row.Domain, "upstream_error", err)
	}
	atx := s.svc.AuthContext(cred)

	var cursor, fingerprint *string
	var succ bool
	switch row.Domain {
	case "keys":
		succ, cursor, fingerprint = s.syncKeys(ctx, ad, atx, site.ID)
	case "groups":
		succ, cursor, fingerprint = s.syncGroups(ctx, ad, atx, site.ID)
	case "account":
		succ, cursor, fingerprint = s.syncAccount(ctx, ad, atx, site.ID)
	case "affiliate":
		succ, cursor, fingerprint = s.syncAffiliate(ctx, ad, atx, site.ID)
	case "usage_log":
		succ, cursor, fingerprint = s.syncUsageLog(ctx, ad, atx, site.ID)
	case "usage_daily":
		succ, cursor, fingerprint = s.syncUsageDaily(ctx, ad, atx, site.ID)
	case "announcements":
		succ, cursor, fingerprint = s.syncAnnouncements(ctx, ad, atx, site.ID)
	case "checkin":
		succ, cursor, fingerprint = s.syncCheckin(ctx, ad, atx, site.ID)
	default:
		succ = true
	}
	if err := s.repo.MarkSyncStateRun(ctx, site.ID, row.Domain, succ, "", cursor, fingerprint); err != nil {
		s.log.Warn("scheduler: 更新调度状态失败", "site_id", site.ID, "domain", row.Domain, "err", err)
	}
	// §6 事件：emit sync.{domain}.{success|error}，data 含 duration/counts/cursor（不含敏感字段）。
	s.emitSyncEvent(ctx, site.ID, row.Domain, succ, cursor)
	return nil
}

// emitSyncEvent 记录同步事件（sync-architecture §6）。
func (s *SyncScheduler) emitSyncEvent(ctx context.Context, siteID, domain string, succ bool, cursor *string) {
	etype := "sync." + domain + "."
	level := "info"
	if !succ {
		etype += "error"
		level = "warn"
	} else {
		etype += "success"
	}
	data := map[string]any{"domain": domain, "succ": succ}
	if cursor != nil {
		data["cursor"] = *cursor
	}
	dj, _ := json.Marshal(data)
	_ = s.repo.InsertEvent(ctx, &model.Event{
		SiteID:  &siteID,
		Type:    etype,
		Level:   level,
		Message: "同步完成",
		Data:    string(dj),
	})
}

func (s *SyncScheduler) markErr(ctx context.Context, siteID, domain, cls string, err error) error {
	s.log.Warn("scheduler: 域同步失败", "site_id", siteID, "domain", domain, "class", cls, "err", err)
	_ = s.repo.MarkSyncStateRun(ctx, siteID, domain, false, cls, nil, nil)
	// §6 事件：emit sync.{domain}.error
	dj, _ := json.Marshal(map[string]any{"domain": domain, "class": cls, "err": err.Error()})
	_ = s.repo.InsertEvent(ctx, &model.Event{
		SiteID:  &siteID,
		Type:    "sync." + domain + ".error",
		Level:   "warn",
		Message: "同步失败",
		Data:    string(dj),
	})
	return err
}

// ---- 域同步桥接（复用 service 现有逻辑）----

func (s *SyncScheduler) syncKeys(ctx context.Context, ad adapter.SiteAdapter, atx adapter.AuthCtx, siteID string) (bool, *string, *string) {
	if !ad.Capabilities().ListKeys {
		return true, nil, nil
	}
	kp, err := ad.ListKeys(ctx, atx, adapter.Page{Page: 1, Size: 200, MaxPage: 100})
	if err != nil {
		if code, _ := adapter.CodeOf(err); code == adapter.CodeUnauthorized {
			_ = s.repo.UpdateCredentialState(ctx, siteID, adapter.StateTokenExpired, "key 同步 401", nil)
		}
		return false, nil, nil
	}
	keys := make([]model.SiteKey, 0, len(kp.Items))
	for _, k := range kp.Items {
		keys = append(keys, model.SiteKey{
			RemoteKeyID: k.RemoteKeyID, Name: k.Name, Group: k.Group,
			Status: k.Status, QuotaLimit: k.QuotaLimit, QuotaUsed: k.QuotaUsed,
			QuotaRemaining: k.QuotaRemaining, Unlimited: k.Unlimited,
			ExpiresAt: k.ExpiresAt, CreatedAt: k.CreatedAt, KeySource: k.KeySource,
			Freshness: "fresh",
		})
	}
	if err := s.repo.UpsertKeys(ctx, siteID, keys); err != nil {
		return false, nil, nil
	}
	fp := fingerprintJSON(kp)
	return true, strPtr(""), &fp
}

func (s *SyncScheduler) syncGroups(ctx context.Context, ad adapter.SiteAdapter, atx adapter.AuthCtx, siteID string) (bool, *string, *string) {
	if !ad.Capabilities().ListGroups {
		return true, nil, nil
	}
	groups, err := ad.ListGroups(ctx, atx)
	if err != nil {
		return false, nil, nil
	}
	gs := make([]model.SiteGroup, 0, len(groups))
	for _, g := range groups {
		gs = append(gs, model.SiteGroup{
			RemoteGroupID: g.RemoteGroupID, Name: g.Name,
			Ratio: g.Ratio, Available: g.Available, Desc: g.Desc, Freshness: "fresh",
		})
	}
	if err := s.repo.UpsertGroups(ctx, siteID, gs); err != nil {
		return false, nil, nil
	}
	fp := fingerprintJSON(groups)
	return true, nil, &fp
}

func (s *SyncScheduler) syncAccount(ctx context.Context, ad adapter.SiteAdapter, atx adapter.AuthCtx, siteID string) (bool, *string, *string) {
	if !ad.Capabilities().Quota {
		return true, nil, nil
	}
	q, err := ad.Quota(ctx, atx)
	if err != nil {
		return false, nil, nil
	}
	bal := q.Balance
	used := q.Used
	row := &model.SiteAccount{
		SiteID:    siteID,
		Balance:   &bal,
		Used:      &used,
		Currency:  q.Currency,
		UnitNote:  q.UnitNote,
		Freshness: "fresh",
	}
	if err := s.repo.UpsertAccount(ctx, row); err != nil {
		return false, nil, nil
	}
	return true, nil, nil
}

func (s *SyncScheduler) syncAffiliate(ctx context.Context, ad adapter.SiteAdapter, atx adapter.AuthCtx, siteID string) (bool, *string, *string) {
	if ad.Capabilities().Affiliate != adapter.CapSupported {
		return true, nil, nil
	}
	if err := s.svc.SyncAffiliate(ctx, ad, atx, siteID); err != nil {
		return false, nil, nil
	}
	return true, nil, nil
}

func (s *SyncScheduler) syncUsageLog(ctx context.Context, ad adapter.SiteAdapter, atx adapter.AuthCtx, siteID string) (bool, *string, *string) {
	if !ad.Capabilities().UsageLogs {
		return true, nil, nil
	}
	n, err := s.svc.SyncUsageLog(ctx, ad, atx, siteID)
	if err != nil {
		if code, _ := adapter.CodeOf(err); code == adapter.CodeUnauthorized {
			_ = s.repo.UpdateCredentialState(ctx, siteID, adapter.StateTokenExpired, "usage_log 同步 401", nil)
		}
		return false, nil, nil
	}
	s.log.Debug("scheduler: usage_log 同步完成", "site_id", siteID, "rows", n)
	return true, nil, nil
}

func (s *SyncScheduler) syncUsageDaily(ctx context.Context, ad adapter.SiteAdapter, atx adapter.AuthCtx, siteID string) (bool, *string, *string) {
	// usage_daily 为离线聚合：无需 adapter，按昨日回补最近 2 天
	for _, d := range []time.Time{time.Now().AddDate(0, 0, -1).UTC(), time.Now().UTC()} {
		day := d.Format("2006-01-02")
		if _, err := s.svc.AggregateUsageDaily(ctx, siteID, day); err != nil {
			s.log.Warn("scheduler: usage_daily 汇总失败", "site_id", siteID, "day", day, "err", err)
			return false, nil, nil
		}
	}
	return true, nil, nil
}

// syncAnnouncements 公告同步（S5；FR-4.4）。
// 当前 adapter 尚无公告接口（api-inventory：❌ 待新增），能力门控跳过，domain 行占位。
func (s *SyncScheduler) syncAnnouncements(ctx context.Context, ad adapter.SiteAdapter, atx adapter.AuthCtx, siteID string) (bool, *string, *string) {
	// 未来对接 /api/notice：全量拉取 + content_hash 比对 → UpsertAnnouncement
	_ = ad
	_ = atx
	return true, nil, nil
}

// syncCheckin 签到同步（FR-3）：每日 08:05 抖动。
func (s *SyncScheduler) syncCheckin(ctx context.Context, ad adapter.SiteAdapter, atx adapter.AuthCtx, siteID string) (bool, *string, *string) {
	if ad.Capabilities().Checkin != adapter.CapSupported {
		return true, nil, nil
	}
	if err := s.svc.SyncCheckin(ctx, ad, atx, siteID); err != nil {
		if code, _ := adapter.CodeOf(err); code == adapter.CodeUnauthorized {
			_ = s.repo.UpdateCredentialState(ctx, siteID, adapter.StateTokenExpired, "checkin 同步 401", nil)
		}
		return false, nil, nil
	}
	return true, nil, nil
}

func fingerprintJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	h := fnv.New32a()
	_, _ = h.Write(b)
	return fmt.Sprintf("%x", h.Sum32())
}

func strPtr(s string) *string { return &s }
