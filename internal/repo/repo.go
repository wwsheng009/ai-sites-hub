// Package repo GORM 数据访问（M2 骨架：sites/credentials/keys/groups/affiliates/events）。
package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"aiclient/internal/model"
)

// ErrNotFound 记录不存在。
var ErrNotFound = errors.New("repo: 记录不存在")

// Repo 数据访问集合。
type Repo struct {
	db *gorm.DB
}

// New 构建 Repo。
func New(db *gorm.DB) *Repo { return &Repo{db: db} }

// DB 暴露底层句柄（事务场景用）。
func (r *Repo) DB() *gorm.DB { return r.db }

func newID() string { return uuid.NewString() }

// ---- Site ----

// CreateSiteInput 新建站点入参。
type CreateSiteInput struct {
	Name     string
	BaseURL  string
	ProxyURL string // 站点级出站代理（可空；空=回落全局 proxy.url）
	SiteType string // 可空：由识别流程回填
}

// CreateSite 创建站点（base_url 唯一）。
func (r *Repo) CreateSite(ctx context.Context, in CreateSiteInput) (*model.Site, error) {
	s := &model.Site{
		ID:       newID(),
		Name:     in.Name,
		BaseURL:  in.BaseURL,
		ProxyURL: in.ProxyURL,
		SiteType: in.SiteType,
		Status:   "active",
	}
	if err := r.db.WithContext(ctx).Create(s).Error; err != nil {
		return nil, fmt.Errorf("repo: 创建站点: %w", err)
	}
	return s, nil
}

// GetSite 按 id 查询。
func (r *Repo) GetSite(ctx context.Context, id string) (*model.Site, error) {
	var s model.Site
	if err := r.db.WithContext(ctx).First(&s, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repo: 查询站点: %w", err)
	}
	return &s, nil
}

// ListSites 站点列表（status 过滤）。
func (r *Repo) ListSites(ctx context.Context, status string) ([]model.Site, error) {
	q := r.db.WithContext(ctx).Model(&model.Site{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var out []model.Site
	if err := q.Order("created_at DESC").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 站点列表: %w", err)
	}
	return out, nil
}

// SiteFilter 站点列表过滤。
type SiteFilter struct {
	Status   string
	SiteType string
	Q        string // name/base_url 模糊
}

// ListSitesByFilter 带过滤的站点列表。
func (r *Repo) ListSitesByFilter(ctx context.Context, f SiteFilter) ([]model.Site, error) {
	q := r.db.WithContext(ctx).Model(&model.Site{})
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.SiteType != "" {
		q = q.Where("site_type = ?", f.SiteType)
	}
	if f.Q != "" {
		like := "%" + f.Q + "%"
		q = q.Where("name LIKE ? OR base_url LIKE ?", like, like)
	}
	var out []model.Site
	if err := q.Order("created_at DESC").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 站点列表: %w", err)
	}
	return out, nil
}

// UpdateSite 更新站点（nil 字段跳过）。
func (r *Repo) UpdateSite(ctx context.Context, id string, fn func(*model.Site)) (*model.Site, error) {
	var s model.Site
	if err := r.db.WithContext(ctx).First(&s, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repo: 查询站点: %w", err)
	}
	fn(&s)
	if err := r.db.WithContext(ctx).Save(&s).Error; err != nil {
		return nil, fmt.Errorf("repo: 保存站点: %w", err)
	}
	return &s, nil
}

// DeleteSite 删除站点（级联由 FK 处理）。
func (r *Repo) DeleteSite(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Delete(&model.Site{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("repo: 删除站点: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Credential ----

// UpsertCredentialInput 凭据写入入参（cipher 由 service 层加密后传入）。
type UpsertCredentialInput struct {
	SiteID             string
	AuthMode           string
	Username           string
	PasswordCipher     []byte
	TOTPSecretCipher   []byte
	AccessTokenCipher  []byte
	RefreshTokenCipher []byte
	PATCipher          []byte
}

// UpsertCredential 创建或更新凭据（site_id 唯一）。
func (r *Repo) UpsertCredential(ctx context.Context, in UpsertCredentialInput) (*model.SiteCredential, error) {
	var c model.SiteCredential
	err := r.db.WithContext(ctx).First(&c, "site_id = ?", in.SiteID).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c = model.SiteCredential{ID: newID(), SiteID: in.SiteID}
	case err != nil:
		return nil, fmt.Errorf("repo: 查询凭据: %w", err)
	}
	c.AuthMode = in.AuthMode
	if in.Username != "" {
		c.Username = in.Username
	}
	if in.PasswordCipher != nil {
		c.PasswordCipher = in.PasswordCipher
	}
	if in.TOTPSecretCipher != nil {
		c.TOTPSecretCipher = in.TOTPSecretCipher
	}
	if in.AccessTokenCipher != nil {
		c.AccessTokenCipher = in.AccessTokenCipher
	}
	if in.RefreshTokenCipher != nil {
		c.RefreshTokenCipher = in.RefreshTokenCipher
	}
	if in.PATCipher != nil {
		c.PATCipher = in.PATCipher
	}
	if err := r.db.WithContext(ctx).Save(&c).Error; err != nil {
		return nil, fmt.Errorf("repo: 保存凭据: %w", err)
	}
	return &c, nil
}

// GetCredential 按 site_id 查询凭据。
func (r *Repo) GetCredential(ctx context.Context, siteID string) (*model.SiteCredential, error) {
	var c model.SiteCredential
	if err := r.db.WithContext(ctx).First(&c, "site_id = ?", siteID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repo: 查询凭据: %w", err)
	}
	return &c, nil
}

// UpdateCredentialState 更新认证状态机字段。
func (r *Repo) UpdateCredentialState(ctx context.Context, siteID, state, msg string, extra func(*model.SiteCredential)) error {
	q := r.db.WithContext(ctx).Model(&model.SiteCredential{}).
		Where("site_id = ?", siteID).
		Updates(map[string]any{
			"auth_state":     state,
			"auth_state_msg": msg,
			"updated_at":     time.Now(),
		})
	if extra != nil {
		var c model.SiteCredential
		if err := r.db.WithContext(ctx).First(&c, "site_id = ?", siteID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("repo: 查询凭据: %w", err)
		}
		extra(&c)
		if err := r.db.WithContext(ctx).Save(&c).Error; err != nil {
			return fmt.Errorf("repo: 保存凭据: %w", err)
		}
		return nil
	}
	if q.Error != nil {
		return fmt.Errorf("repo: 更新凭据状态: %w", q.Error)
	}
	return nil
}

// ---- SiteKey ----

// UpsertKeys 批量 upsert key 投影（UNIQUE(site_id, remote_key_id)）。
func (r *Repo) UpsertKeys(ctx context.Context, siteID string, keys []model.SiteKey) error {
	if len(keys) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range keys {
			keys[i].ID = newID()
			keys[i].SiteID = siteID
			now := time.Now()
			keys[i].LastSyncAt = &now
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "site_id"}, {Name: "remote_key_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"name", "grp", "status", "quota_limit", "quota_used", "quota_remaining", "unlimited", "expires_at", "key_source", "freshness", "last_sync_at"}),
			}).Create(&keys[i]).Error; err != nil {
				return fmt.Errorf("repo: upsert key %s: %w", keys[i].RemoteKeyID, err)
			}
		}
		return nil
	})
}

// ListKeys 查询 key 投影。
func (r *Repo) ListKeys(ctx context.Context, siteID string) ([]model.SiteKey, error) {
	var out []model.SiteKey
	q := r.db.WithContext(ctx)
	if siteID != "" {
		q = q.Where("site_id = ?", siteID)
	}
	if err := q.Order("site_id, remote_key_id").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: key 列表: %w", err)
	}
	return out, nil
}

// ---- SiteGroup ----

// UpsertGroups 批量 upsert 分组投影。
func (r *Repo) UpsertGroups(ctx context.Context, siteID string, groups []model.SiteGroup) error {
	if len(groups) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range groups {
			groups[i].ID = newID()
			groups[i].SiteID = siteID
			now := time.Now()
			groups[i].LastSyncAt = &now
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "site_id"}, {Name: "remote_group_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"name", "ratio", "available", "desc", "freshness", "last_sync_at"}),
			}).Create(&groups[i]).Error; err != nil {
				return fmt.Errorf("repo: upsert group %s: %w", groups[i].RemoteGroupID, err)
			}
		}
		return nil
	})
}

// ListGroups 查询分组投影。
func (r *Repo) ListGroups(ctx context.Context, siteID string) ([]model.SiteGroup, error) {
	var out []model.SiteGroup
	q := r.db.WithContext(ctx)
	if siteID != "" {
		q = q.Where("site_id = ?", siteID)
	}
	if err := q.Order("site_id, remote_group_id").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 分组列表: %w", err)
	}
	return out, nil
}

// ---- SiteAffiliate（FR-10）----

// UpsertAffiliate 创建/更新返利投影（site_id 唯一）。
func (r *Repo) UpsertAffiliate(ctx context.Context, a *model.SiteAffiliate) error {
	var existing model.SiteAffiliate
	err := r.db.WithContext(ctx).First(&existing, "site_id = ?", a.SiteID).Error
	now := time.Now()
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		a.ID = newID()
		a.LastSyncAt = &now
		if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
			return fmt.Errorf("repo: 创建返利投影: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("repo: 查询返利投影: %w", err)
	}
	a.ID = existing.ID
	a.AffCfg = existing.AffCfg // 保留本地规则配置
	a.LastSyncAt = &now
	if err := r.db.WithContext(ctx).Model(&model.SiteAffiliate{}).
		Where("site_id = ?", a.SiteID).
		Updates(map[string]any{
			"aff_code":      a.AffCode,
			"rebate_rate":   a.RebateRate,
			"available":     a.Available,
			"frozen":        a.Frozen,
			"history":       a.History,
			"invitee_count": a.InviteeCount,
			"invitees":      a.Invitees,
			"freshness":     a.Freshness,
			"last_sync_at":  now,
			"updated_at":    now,
		}).Error; err != nil {
		return fmt.Errorf("repo: 更新返利投影: %w", err)
	}
	return nil
}

// GetAffiliate 按 site_id 查询返利投影。
func (r *Repo) GetAffiliate(ctx context.Context, siteID string) (*model.SiteAffiliate, error) {
	var a model.SiteAffiliate
	if err := r.db.WithContext(ctx).First(&a, "site_id = ?", siteID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repo: 查询返利投影: %w", err)
	}
	return &a, nil
}

// ListAffiliates 返利投影列表（可按 site_ids 过滤）。
func (r *Repo) ListAffiliates(ctx context.Context, siteIDs []string) ([]model.SiteAffiliate, error) {
	var out []model.SiteAffiliate
	q := r.db.WithContext(ctx).Model(&model.SiteAffiliate{})
	if len(siteIDs) > 0 {
		q = q.Where("site_id IN ?", siteIDs)
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 返利列表: %w", err)
	}
	return out, nil
}

// AffiliateInvitees 解析站点返利投影中的受邀用户列表（JSON 列）。
func (r *Repo) AffiliateInvitees(ctx context.Context, siteID string) ([]model.SiteAffiliateInvitee, error) {
	a, err := r.GetAffiliate(ctx, siteID)
	if err != nil {
		return nil, err
	}
	out := make([]model.SiteAffiliateInvitee, 0)
	if a.Invitees == "" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(a.Invitees), &out); err != nil {
		return out, nil // JSON 异常不阻断详情页，返回空列表
	}
	return out, nil
}

// UpdateAffiliateCfg 更新划转规则 JSON（FR-10.4 配置）；投影不存在时以
// freshness=missing 的最小行 upsert（单管理员场景，无并发竞争担忧）。
func (r *Repo) UpdateAffiliateCfg(ctx context.Context, siteID, cfgJSON string) error {
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&model.SiteAffiliate{}).
		Where("site_id = ?", siteID).
		Updates(map[string]any{"aff_cfg": cfgJSON, "updated_at": now})
	if res.Error != nil {
		return fmt.Errorf("repo: 更新划转规则: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		row := model.SiteAffiliate{
			ID: newID(), SiteID: siteID,
			AffCfg: cfgJSON, Freshness: "missing", UpdatedAt: now,
		}
		if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
			return fmt.Errorf("repo: 创建划转规则投影: %w", err)
		}
	}
	return nil
}

// InsertAffTransfer 记录划转。
func (r *Repo) InsertAffTransfer(ctx context.Context, t *model.AffTransfer) error {
	if t.ID == "" {
		t.ID = newID()
	}
	if err := r.db.WithContext(ctx).Create(t).Error; err != nil {
		return fmt.Errorf("repo: 记录划转: %w", err)
	}
	return nil
}

// ListAffTransfers 划转记录（按站点）。
func (r *Repo) ListAffTransfers(ctx context.Context, siteID string, limit int) ([]model.AffTransfer, error) {
	var out []model.AffTransfer
	q := r.db.WithContext(ctx).Model(&model.AffTransfer{})
	if siteID != "" {
		q = q.Where("site_id = ?", siteID)
	}
	if limit <= 0 {
		limit = 100
	}
	if err := q.Order("created_at DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 划转记录: %w", err)
	}
	return out, nil
}

// ---- Snapshot / Event ----

// InsertSnapshot 存快照（body 已脱敏）。
func (r *Repo) InsertSnapshot(ctx context.Context, siteID, kind, body string, statusCode *int) error {
	s := &model.Snapshot{ID: newID(), SiteID: siteID, Kind: kind, Body: body, StatusCode: statusCode}
	if err := r.db.WithContext(ctx).Create(s).Error; err != nil {
		return fmt.Errorf("repo: 存快照: %w", err)
	}
	return nil
}

// InsertEvent 记平台事件。
func (r *Repo) InsertEvent(ctx context.Context, e *model.Event) error {
	if e.ID == "" {
		e.ID = newID()
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return fmt.Errorf("repo: 记事件: %w", err)
	}
	return nil
}

// ListEvents 事件流水（since/site_id 过滤）。
func (r *Repo) ListEvents(ctx context.Context, since *time.Time, siteID string, limit int) ([]model.Event, error) {
	q := r.db.WithContext(ctx).Model(&model.Event{})
	if since != nil {
		q = q.Where("created_at >= ?", *since)
	}
	if siteID != "" {
		q = q.Where("site_id = ?", siteID)
	}
	if limit <= 0 {
		limit = 200
	}
	var out []model.Event
	if err := q.Order("created_at DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 事件列表: %w", err)
	}
	return out, nil
}

// ---- SiteSyncState（调度器）----

// SyncStateRow site_sync_state 查询/写入行。
type SyncStateRow struct {
	ID                  string
	SiteID              string
	Domain              string
	Enabled             int
	IntervalS           int
	LastRunAt           *time.Time
	NextRunAt           *time.Time
	Cursor              string
	Fingerprint         string
	ConsecutiveFailures int
	LastErrorClass      string
}

// GetSyncState 按站点+域查询调度状态行。
func (r *Repo) GetSyncState(ctx context.Context, siteID, domain string) (*model.SiteSyncState, error) {
	var row model.SiteSyncState
	if err := r.db.WithContext(ctx).First(&row, "site_id = ? AND domain = ?", siteID, domain).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repo: 查询调度状态: %w", err)
	}
	return &row, nil
}

// ListSyncStateBySite 查询站点下所有域的调度状态（作业页用）。
func (r *Repo) ListSyncStateBySite(ctx context.Context, siteID string) ([]model.SiteSyncState, error) {
	var out []model.SiteSyncState
	if err := r.db.WithContext(ctx).Model(&model.SiteSyncState{}).
		Where("site_id = ?", siteID).
		Order("domain").
		Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 查询站点调度状态: %w", err)
	}
	return out, nil
}

// ListSyncStateDue 读取 next_run_at <= now 的所有域（调度器 tick 用）。
func (r *Repo) ListSyncStateDue(ctx context.Context, now time.Time) ([]SyncStateRow, error) {
	var out []SyncStateRow
	if err := r.db.WithContext(ctx).Model(&model.SiteSyncState{}).
		Where("next_run_at IS NULL OR next_run_at <= ?", now).
		Order("site_id, domain").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 读取待调度: %w", err)
	}
	return out, nil
}

// UpsertSyncState 写入/更新调度状态行。
func (r *Repo) UpsertSyncState(ctx context.Context, row *model.SiteSyncState) error {
	var existing model.SiteSyncState
	err := r.db.WithContext(ctx).First(&existing, "site_id = ? AND domain = ?", row.SiteID, row.Domain).Error
	now := time.Now()
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		row.ID = newID()
		row.LastRunAt = nil
		row.NextRunAt = &now
		if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
			return fmt.Errorf("repo: 创建调度状态: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("repo: 查询调度状态: %w", err)
	}
	row.ID = existing.ID
	if err := r.db.WithContext(ctx).Model(&model.SiteSyncState{}).
		Where("site_id = ? AND domain = ?", row.SiteID, row.Domain).
		Updates(map[string]any{
			"enabled":              row.Enabled,
			"interval_s":           row.IntervalS,
			"last_run_at":          row.LastRunAt,
			"next_run_at":          row.NextRunAt,
			"cursor":               row.Cursor,
			"fingerprint":          row.Fingerprint,
			"consecutive_failures": row.ConsecutiveFailures,
			"last_error_class":     row.LastErrorClass,
		}).Error; err != nil {
		return fmt.Errorf("repo: 更新调度状态: %w", err)
	}
	return nil
}

// MarkSyncStateRun 更新本次运行结果（成功/失败 + 退避）。
func (r *Repo) MarkSyncStateRun(ctx context.Context, siteID, domain string, success bool, errClass string, cursor, fingerprint *string) error {
	var row model.SiteSyncState
	if err := r.db.WithContext(ctx).First(&row, "site_id = ? AND domain = ?", siteID, domain).Error; err != nil {
		return fmt.Errorf("repo: 查询调度状态: %w", err)
	}
	now := time.Now()
	updates := map[string]any{
		"last_run_at": &now,
		"cursor":      coalesceStrPtr(cursor, row.Cursor),
		"fingerprint": coalesceStrPtr(fingerprint, row.Fingerprint),
	}
	if success {
		row.ConsecutiveFailures = 0
		row.LastErrorClass = ""
		updates["consecutive_failures"] = 0
		updates["last_error_class"] = ""
		// next_run = now + interval
		next := now.Add(time.Duration(row.IntervalS) * time.Second)
		updates["next_run_at"] = &next
	} else {
		row.ConsecutiveFailures++
		row.LastErrorClass = errClass
		updates["consecutive_failures"] = row.ConsecutiveFailures
		updates["last_error_class"] = errClass
		backoff := time.Duration(1<<min(row.ConsecutiveFailures, 12)) * time.Second
		if backoff > time.Hour {
			backoff = time.Hour
		}
		next := now.Add(backoff)
		updates["next_run_at"] = &next
	}
	return r.db.WithContext(ctx).Model(&row).Updates(updates).Error
}

func coalesceStrPtr(a *string, b string) string {
	if a != nil {
		return *a
	}
	return b
}

// ---- SiteAccount ----

// UpsertAccount 写入账号余额投影。
func (r *Repo) UpsertAccount(ctx context.Context, a *model.SiteAccount) error {
	var existing model.SiteAccount
	err := r.db.WithContext(ctx).First(&existing, "site_id = ?", a.SiteID).Error
	now := time.Now()
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		a.Freshness = "fresh"
		a.LastSyncAt = &now
		if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
			return fmt.Errorf("repo: 创建账号投影: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("repo: 查询账号投影: %w", err)
	}
	a.Freshness = "fresh"
	a.LastSyncAt = &now
	if err := r.db.WithContext(ctx).Model(&model.SiteAccount{}).
		Where("site_id = ?", a.SiteID).
		Updates(map[string]any{
			"balance":            a.Balance,
			"used":               a.Used,
			"currency":           a.Currency,
			"unit_note":          a.UnitNote,
			"subscription_state": a.SubscriptionState,
			"freshness":          a.Freshness,
			"last_sync_at":       now,
		}).Error; err != nil {
		return fmt.Errorf("repo: 更新账号投影: %w", err)
	}
	return nil
}

// GetAccount 按站点查询账号余额投影。
func (r *Repo) GetAccount(ctx context.Context, siteID string) (*model.SiteAccount, error) {
	var a model.SiteAccount
	if err := r.db.WithContext(ctx).First(&a, "site_id = ?", siteID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repo: 查询账号投影: %w", err)
	}
	return &a, nil
}

// ListAccounts 跨站账号余额投影。
func (r *Repo) ListAccounts(ctx context.Context) ([]model.SiteAccount, error) {
	var out []model.SiteAccount
	if err := r.db.WithContext(ctx).Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 账号列表: %w", err)
	}
	return out, nil
}

// ---- usage logs（S2）----

// InsertUsageLogs 批量写入用量日志（幂等 upsert；remote_ref 为空时回退哈希去重）。
func (r *Repo) InsertUsageLogs(ctx context.Context, siteID string, logs []model.UsageLog) (int, error) {
	if len(logs) == 0 {
		return 0, nil
	}
	now := time.Now().UTC()
	rows := make([]model.UsageLog, 0, len(logs))
	for _, l := range logs {
		if l.ID == "" {
			l.ID = uuid.NewString()
		}
		l.SiteID = siteID
		l.FetchedAt = now
		rows = append(rows, l)
	}
	// 分批 upsert（sqlite 单事务限制），batch=200
	const batch = 200
	affected := 0
	for i := 0; i < len(rows); i += batch {
		end := i + batch
		if end > len(rows) {
			end = len(rows)
		}
		tx := r.db.WithContext(ctx).Begin()
		batchRows := rows[i:end]
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "site_id"}, {Name: "remote_ref"}},
			DoUpdates: clause.AssignmentColumns([]string{"ts", "model_name", "api_key_id", "api_key_mask", "prompt_tokens", "completion_tokens", "total_tokens", "amount", "currency", "status", "err_code", "fetched_at"}),
		}).Create(&batchRows).Error; err != nil {
			_ = tx.Rollback()
			return affected, fmt.Errorf("repo: 写入 usage_logs: %w", err)
		}
		_ = tx.Commit()
		affected += end - i
	}
	return affected, nil
}

// ListUsageLogs 查询站点用量日志（按时间范围 + 模型过滤）。
func (r *Repo) ListUsageLogs(ctx context.Context, siteID string, since, until time.Time, modelName string, limit int) ([]model.UsageLog, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	tx := r.db.WithContext(ctx).Model(&model.UsageLog{}).Where("site_id = ?", siteID)
	if !since.IsZero() {
		tx = tx.Where("ts >= ?", since)
	}
	if !until.IsZero() {
		tx = tx.Where("ts <= ?", until)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	var out []model.UsageLog
	if err := tx.Order("ts DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 查询 usage_logs: %w", err)
	}
	return out, nil
}

// ListSiteModels 模型广场聚合（S4）：跨站已用模型 + 余额。
// 从 usage_daily 汇总 + site_account 余额左连接，仅含已同步站点。
func (r *Repo) ListSiteModels(ctx context.Context, limit int) ([]model.SiteModel, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}
	const sql = `SELECT s.id AS site_id, s.name AS site_name, u.model_name,
       COALESCE(SUM(u.total_tokens),0) AS total_tokens,
       COALESCE(SUM(u.amount),0) AS amount,
       COALESCE(u.currency,'quota') AS currency,
       a.balance AS balance,
       COALESCE(a.freshness,'missing') AS freshness
FROM sites s
JOIN usage_daily u ON u.site_id = s.id
LEFT JOIN site_account a ON a.site_id = s.id
GROUP BY s.id, s.name, u.model_name, u.currency, a.balance, a.freshness
ORDER BY total_tokens DESC
LIMIT ?`
	var out []model.SiteModel
	if err := r.db.WithContext(ctx).Raw(sql, limit).Scan(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 模型广场聚合: %w", err)
	}
	return out, nil
}

// ---- announcements（S5）----

// ListAnnouncements 跨站公告聚合（按发布时间倒序）。
func (r *Repo) ListAnnouncements(ctx context.Context, limit int) ([]model.SiteAnnouncement, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	var out []model.SiteAnnouncement
	if err := r.db.WithContext(ctx).Model(&model.SiteAnnouncement{}).
		Order("published_at DESC, created_at DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 查询公告: %w", err)
	}
	return out, nil
}

// UpsertAnnouncement 公告幂等写入（remote_ref 或 content_hash 去重）。
func (r *Repo) UpsertAnnouncement(ctx context.Context, a *model.SiteAnnouncement) error {
	a.ID = newID()
	a.UpdatedAt = time.Now()
	if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
		// 唯一键冲突 → 退化为更新已有行
		var set = map[string]any{
			"title":        a.Title,
			"content":      a.Content,
			"content_hash": a.ContentHash,
			"published_at": a.PublishedAt,
			"updated_at":   a.UpdatedAt,
		}
		if err := r.db.WithContext(ctx).Model(&model.SiteAnnouncement{}).
			Where("site_id = ? AND (remote_ref = ? OR content_hash = ?)", a.SiteID, a.RemoteRef, a.ContentHash).
			Updates(set).Error; err != nil {
			return fmt.Errorf("repo: upsert 公告: %w", err)
		}
	}
	return nil
}

// ---- checkin（FR-3）----

// UpsertCheckin 签到记录幂等写入（UNIQUE(site_id, date)）。
func (r *Repo) UpsertCheckin(ctx context.Context, c *model.SiteCheckin) error {
	c.ID = newID()
	c.CreatedAt = time.Now()
	if err := r.db.WithContext(ctx).Create(c).Error; err != nil {
		var set = map[string]any{
			"state":         c.State,
			"quota_awarded": c.QuotaAwarded,
			"message":       c.Message,
			"created_at":    c.CreatedAt,
		}
		if err := r.db.WithContext(ctx).Model(&model.SiteCheckin{}).
			Where("site_id = ? AND date = ?", c.SiteID, c.Date).
			Updates(set).Error; err != nil {
			return fmt.Errorf("repo: upsert 签到: %w", err)
		}
	}
	return nil
}

// AggregateUsageDaily 按 day + model_name 汇总 usage_logs → usage_daily（幂等 upsert）。
func (r *Repo) AggregateUsageDaily(ctx context.Context, siteID, day string) (int, error) {
	const aggSQL = `INSERT INTO usage_daily (id, site_id, day, model_name, prompt_tokens, completion_tokens, total_tokens, amount, currency)
SELECT ?, ?, ?, COALESCE(model_name,''),
       SUM(prompt_tokens), SUM(completion_tokens), SUM(total_tokens), SUM(amount), 'quota'
FROM usage_logs
WHERE site_id = ? AND date(ts) = ?
GROUP BY COALESCE(model_name,'')
ON CONFLICT(site_id, day, model_name) DO UPDATE SET
  prompt_tokens = excluded.prompt_tokens,
  completion_tokens = excluded.completion_tokens,
  total_tokens = excluded.total_tokens,
  amount = excluded.amount;`
	res := r.db.WithContext(ctx).Exec(aggSQL, newID(), siteID, day, siteID, day)
	if res.Error != nil {
		return 0, fmt.Errorf("repo: 汇总 usage_daily: %w", res.Error)
	}
	return int(res.RowsAffected), nil
}

// ListUsageDaily 查询站点日聚合（按日期范围 + 模型过滤）。
func (r *Repo) ListUsageDaily(ctx context.Context, siteID, since, until, modelName string, limit int) ([]model.UsageDaily, error) {
	if limit <= 0 {
		limit = 60
	}
	if limit > 366 {
		limit = 366
	}
	tx := r.db.WithContext(ctx).Model(&model.UsageDaily{}).Where("site_id = ?", siteID)
	if since != "" {
		tx = tx.Where("day >= ?", since)
	}
	if until != "" {
		tx = tx.Where("day <= ?", until)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	var out []model.UsageDaily
	if err := tx.Order("day DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 查询 usage_daily: %w", err)
	}
	return out, nil
}
