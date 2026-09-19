// Package service 业务编排层：站点/凭据/识别/同步/返利。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"aiclient/internal/adapter"
	"aiclient/internal/model"
	"aiclient/internal/repo"
	"aiclient/internal/secret"
	"aiclient/internal/service/sitedetect"
)

// Services 业务服务集合。
type Services struct {
	Repo   *repo.Repo
	Sec    *secret.Cipher
	Reg    *adapter.Registry
	Detect *sitedetect.Detector
	Log    *slog.Logger

	// GlobalProxy 全局出站代理（config proxy.url；站点级 proxy_url 为空时回落）。
	GlobalProxy string
}

// ErrNotFound 透传 repo.ErrNotFound。
var ErrNotFound = repo.ErrNotFound

// ---- 站点 CRUD（FR-1）----

// CreateSiteInput 新建站点入参。
type CreateSiteInput struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	// ProxyURL 站点级出站代理（http/https/socks5；空=回落全局 proxy.url）
	ProxyURL string `json:"proxy_url,omitempty"`
	// ManualType 手动指定站点类型（非空时跳过自动识别；FR-1 人工覆盖优先）
	ManualType string `json:"site_type,omitempty"`
	// DetectNow 创建后立即识别（默认 true）
	DetectNow *bool `json:"detect_now,omitempty"`
}

// normalizeBaseURL 归一化用户输入的站点地址。
func normalizeBaseURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("base_url 不能为空")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("base_url 无效: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("不支持的 scheme %q", u.Scheme)
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	u.RawQuery, u.Fragment = "", ""
	return u.String(), nil
}

// normalizeProxyURL 校验出站代理 URL（空合法=未设置；支持 http/https/socks5/socks5h）。
func normalizeProxyURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("proxy_url 无效: %w", err)
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return "", fmt.Errorf("不支持的代理协议 %q（仅 http/https/socks5）", u.Scheme)
	}
	if u.Host == "" {
		return "", errors.New("proxy_url 缺少主机（如 socks5://127.0.0.1:1080）")
	}
	return s, nil
}

// CreateSite 新建站点 + 触发识别（新增向导第 1 步，architecture §9.1）。
func (s *Services) CreateSite(ctx context.Context, in CreateSiteInput) (*model.Site, error) {
	base, err := normalizeBaseURL(in.BaseURL)
	if err != nil {
		return nil, err
	}
	if in.Name == "" {
		if u, err := url.Parse(base); err == nil {
			in.Name = u.Host
		} else {
			in.Name = base
		}
	}

	proxyURL, err := normalizeProxyURL(in.ProxyURL)
	if err != nil {
		return nil, err
	}

	siteType := ""
	if t := adapter.Type(in.ManualType); t == adapter.TypeSub2API || t == adapter.TypeNewAPI {
		siteType = string(t)
	}
	site, err := s.Repo.CreateSite(ctx, repo.CreateSiteInput{Name: in.Name, BaseURL: base, ProxyURL: proxyURL, SiteType: siteType})
	if err != nil {
		return nil, err
	}

	if in.DetectNow == nil || *in.DetectNow {
		if _, err := s.DetectSite(ctx, site.ID); err != nil {
			// 识别失败不阻塞建站（unknown 可手动指定）
			s.Log.Warn("站点识别失败", "site_id", site.ID, "err", err)
		}
	}
	return s.Repo.GetSite(ctx, site.ID)
}

// DetectSite 触发识别并回填 site_type/detect_report + 快照留证（FR-1）。
func (s *Services) DetectSite(ctx context.Context, siteID string) (*model.Site, error) {
	site, err := s.Repo.GetSite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	res, err := s.Detect.Detect(ctx, site.BaseURL, s.proxyFor(site))
	if err != nil {
		return nil, err
	}
	updated, err := s.Repo.UpdateSite(ctx, siteID, func(s2 *model.Site) {
		if res.SiteType != string(adapter.TypeUnknown) {
			s2.SiteType = res.SiteType
		}
		s2.DetectScore = res.Score
		s2.DetectReport = res.ReportJSON
		now := time.Now()
		s2.LastProbeAt = &now
	})
	if err != nil {
		return nil, err
	}
	_ = s.Repo.InsertSnapshot(ctx, siteID, "detect", res.ReportJSON, nil)
	return updated, nil
}

// GetSite / ListSites 透传。
func (s *Services) GetSite(ctx context.Context, id string) (*model.Site, error) {
	return s.Repo.GetSite(ctx, id)
}

// ListSites 站点列表。
func (s *Services) ListSites(ctx context.Context, status, siteType, q string) ([]model.Site, error) {
	return s.Repo.ListSitesByFilter(ctx, repo.SiteFilter{Status: status, SiteType: siteType, Q: q})
}

// UpdateSiteInput 更新入参。
type UpdateSiteInput struct {
	Name     *string `json:"name"`
	Status   *string `json:"status"`
	SiteType *string `json:"site_type"`
	// ProxyURL 出站代理；nil=不修改，空串=清除（回落全局 proxy.url）。
	ProxyURL *string `json:"proxy_url"`
	// SyncCfg 同步配置 JSON（FR-4.3；含 checkin.enable 等）。nil=不修改。
	SyncCfg *string `json:"sync_cfg"`
}

// UpdateSite 更新站点（人工覆盖 site_type 后识别流程跳过自动判定）。
func (s *Services) UpdateSite(ctx context.Context, id string, in UpdateSiteInput) (*model.Site, error) {
	if in.SiteType != nil {
		t := adapter.Type(*in.SiteType)
		if t != adapter.TypeSub2API && t != adapter.TypeNewAPI && t != adapter.TypeUnknown {
			return nil, fmt.Errorf("无效 site_type %q", *in.SiteType)
		}
	}
	if in.Status != nil {
		switch *in.Status {
		case "active", "disabled", "archived":
		default:
			return nil, fmt.Errorf("无效 status %q", *in.Status)
		}
	}
	proxyURL := ""
	if in.ProxyURL != nil {
		p, err := normalizeProxyURL(*in.ProxyURL)
		if err != nil {
			return nil, err
		}
		proxyURL = p
	}
	return s.Repo.UpdateSite(ctx, id, func(s2 *model.Site) {
		if in.Name != nil {
			s2.Name = *in.Name
		}
		if in.Status != nil {
			s2.Status = *in.Status
		}
		if in.SiteType != nil {
			s2.SiteType = *in.SiteType
		}
		if in.ProxyURL != nil {
			s2.ProxyURL = proxyURL
		}
		if in.SyncCfg != nil {
			s2.SyncCfg = *in.SyncCfg
		}
	})
}

// DeleteSite 删除站点。
func (s *Services) DeleteSite(ctx context.Context, id string) error {
	return s.Repo.DeleteSite(ctx, id)
}

// ---- 凭据（FR-2；密文落库，NFR-3）----

// PutCredentialsInput 凭据写入入参（明文仅存在于请求体，落库前加密）。
type PutCredentialsInput struct {
	AuthMode     string `json:"auth_mode"` // username_password | token | hybrid
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
	TOTPSecret   string `json:"totp_secret,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	PAT          string `json:"pat,omitempty"`
}

// PutCredentials 加密并保存凭据。
func (s *Services) PutCredentials(ctx context.Context, siteID string, in PutCredentialsInput) error {
	if _, err := s.Repo.GetSite(ctx, siteID); err != nil {
		return err
	}
	switch in.AuthMode {
	case "username_password", "token", "hybrid":
	default:
		return fmt.Errorf("无效 auth_mode %q", in.AuthMode)
	}
	if s.Sec == nil {
		return errors.New("未配置主密钥，凭据写入被拒绝（见 doctor）")
	}

	enc := func(plain string) ([]byte, error) {
		if plain == "" {
			return nil, nil
		}
		return s.Sec.Encrypt([]byte(plain))
	}
	var (
		pc, tc, ac, rc, patc []byte
		err                  error
	)
	if pc, err = enc(in.Password); err != nil {
		return err
	}
	if tc, err = enc(in.TOTPSecret); err != nil {
		return err
	}
	if ac, err = enc(in.AccessToken); err != nil {
		return err
	}
	if rc, err = enc(in.RefreshToken); err != nil {
		return err
	}
	if patc, err = enc(in.PAT); err != nil {
		return err
	}

	_, err = s.Repo.UpsertCredential(ctx, repo.UpsertCredentialInput{
		SiteID: siteID, AuthMode: in.AuthMode, Username: in.Username,
		PasswordCipher: pc, TOTPSecretCipher: tc,
		AccessTokenCipher: ac, RefreshTokenCipher: rc, PATCipher: patc,
	})
	if err != nil {
		return err
	}
	return s.Repo.UpdateCredentialState(ctx, siteID, adapter.StateNone, "凭据已更新，待测试", nil)
}

// AuthStateOut 认证状态投影（不含任何密文）。
type AuthStateOut struct {
	AuthMode        string     `json:"auth_mode"`
	Username        string     `json:"username"`
	AuthState       string     `json:"auth_state"`
	AuthStateMsg    string     `json:"auth_state_msg"`
	LastLoginAt     *time.Time `json:"last_login_at"`
	TokenExpireHint *time.Time `json:"token_expire_hint"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// GetAuthState 读取认证状态机。
func (s *Services) GetAuthState(ctx context.Context, siteID string) (*AuthStateOut, error) {
	c, err := s.Repo.GetCredential(ctx, siteID)
	if err != nil {
		return nil, err
	}
	return &AuthStateOut{
		AuthMode: c.AuthMode, Username: c.Username,
		AuthState: c.AuthState, AuthStateMsg: c.AuthStateMsg,
		LastLoginAt: c.LastLoginAt, TokenExpireHint: c.TokenExpireHint, UpdatedAt: c.UpdatedAt,
	}, nil
}

// decryptField 解密凭据列（空列返回空串）。
func (s *Services) decryptField(cipherBytes []byte) string {
	if len(cipherBytes) == 0 || s.Sec == nil {
		return ""
	}
	plain, err := s.Sec.Decrypt(cipherBytes)
	if err != nil {
		s.Log.Error("凭据解密失败（密钥不匹配？）", "err", err)
		return ""
	}
	return string(plain)
}

// authContext 组装 adapter.AuthCtx。
func (s *Services) authContext(c *model.SiteCredential) adapter.AuthCtx {
	return adapter.AuthCtx{
		AccessToken:  s.decryptField(c.AccessTokenCipher),
		RefreshToken: s.decryptField(c.RefreshTokenCipher),
		PAT:          s.decryptField(c.PATCipher),
	}
}

// AuthContext 公开 authContext（供调度器复用）。
func (s *Services) AuthContext(c *model.SiteCredential) adapter.AuthCtx {
	return s.authContext(c)
}

// credentials 组装 adapter.Credentials。
func (s *Services) credentials(c *model.SiteCredential) adapter.Credentials {
	return adapter.Credentials{
		AuthMode:     c.AuthMode,
		Username:     c.Username,
		Password:     s.decryptField(c.PasswordCipher),
		TOTPSecret:   s.decryptField(c.TOTPSecretCipher),
		AccessToken:  s.decryptField(c.AccessTokenCipher),
		RefreshToken: s.decryptField(c.RefreshTokenCipher),
		PAT:          s.decryptField(c.PATCipher),
	}
}

// proxyFor 站点出站代理：站点级优先，空回落全局 proxy.url（均空=直连）。
func (s *Services) proxyFor(site *model.Site) string {
	if site.ProxyURL != "" {
		return site.ProxyURL
	}
	return s.GlobalProxy
}

// adapterFor 按站点构建 adapter（工厂绑定站点基址与出站代理，同步方法可用）。
func (s *Services) adapterFor(site *model.Site) (adapter.SiteAdapter, error) {
	return s.Reg.Get(adapter.Type(site.SiteType), site.BaseURL, s.proxyFor(site), s.Log)
}

// AuthTest 登录测试（architecture §9.1 第 3 步；结果写认证状态机）。
func (s *Services) AuthTest(ctx context.Context, siteID string) (*AuthStateOut, error) {
	site, err := s.Repo.GetSite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	cred, err := s.Repo.GetCredential(ctx, siteID)
	if err != nil {
		return nil, err
	}
	ad, err := s.adapterFor(site)
	if err != nil {
		return nil, err
	}

	st, err := ad.Login(ctx, s.credentials(cred))
	state, msg := st.State, st.Message
	if err != nil {
		var ae *adapter.AdapterError
		if errors.As(err, &ae) && ae.Code == adapter.CodeUnsupported {
			state, msg = adapter.StateLoginFailed, "站点类型不支持用户名密码登录"
		} else if state == "" {
			state, msg = adapter.StateLoginFailed, err.Error()
		}
	}
	now := time.Now()
	updates := func(c *model.SiteCredential) {
		c.LastLoginAt = &now
		if state == adapter.StateOK {
			if st.AccessToken != "" {
				if b, err := s.Sec.Encrypt([]byte(st.AccessToken)); err == nil {
					c.AccessTokenCipher = b
				}
			}
			if st.RefreshToken != "" {
				if b, err := s.Sec.Encrypt([]byte(st.RefreshToken)); err == nil {
					c.RefreshTokenCipher = b
				}
			}
			if st.PAT != "" {
				if b, err := s.Sec.Encrypt([]byte(st.PAT)); err == nil {
					c.PATCipher = b
				}
			}
			c.TokenExpireHint = st.ExpiresHint
		}
	}
	if err := s.Repo.UpdateCredentialState(ctx, siteID, state, msg, updates); err != nil {
		return nil, err
	}
	_ = s.emitEvent(ctx, siteID, "auth.test", stateOKLevel(state), msg)

	return s.GetAuthState(ctx, siteID)
}

func stateOKLevel(state string) string {
	if state == adapter.StateOK {
		return "info"
	}
	return "warn"
}

// emitEvent 记平台事件。
func (s *Services) emitEvent(ctx context.Context, siteID, typ, level, msg string) error {
	sid := siteID
	return s.Repo.InsertEvent(ctx, &model.Event{SiteID: &sid, Type: typ, Level: level, Message: msg, Data: "{}"})
}

// ---- 数据同步（FR-4；M2 手动触发版）----

// SyncResult 同步结果摘要。
type SyncResult struct {
	SiteID        string   `json:"site_id"`
	KeysSync      int      `json:"keys_synced"`
	GroupsSync    int      `json:"groups_synced"`
	Affiliate     bool     `json:"affiliate_synced"`
	UsageLogsSync int      `json:"usage_logs_synced"`
	Errors        []string `json:"errors,omitempty"`
}

// SyncSite 拉取 keys/groups/quota/affiliate → 投影落库 + 快照（FR-4、FR-10.1）。
func (s *Services) SyncSite(ctx context.Context, siteID string) (*SyncResult, error) {
	site, err := s.Repo.GetSite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	cred, err := s.Repo.GetCredential(ctx, siteID)
	if err != nil {
		return nil, err
	}
	ad, err := s.adapterFor(site)
	if err != nil {
		return nil, err
	}
	atx := s.authContext(cred)

	// 登录态校验（auth.refresh 作业的 M2 手动入口）
	if err := ad.Verify(ctx, atx); err != nil {
		var ae *adapter.AdapterError
		if errors.As(err, &ae) && ae.Code == adapter.CodeUnauthorized {
			_ = s.Repo.UpdateCredentialState(ctx, siteID, adapter.StateTokenExpired, "登录态失效，请重新测试", nil)
		}
		return nil, fmt.Errorf("登录态校验失败: %w", err)
	}

	res := &SyncResult{SiteID: siteID}
	caps := ad.Capabilities()

	if caps.ListKeys {
		kp, err := ad.ListKeys(ctx, atx, adapter.Page{Page: 1, Size: 500})
		if err != nil {
			res.Errors = append(res.Errors, "keys: "+err.Error())
		} else {
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
			if err := s.Repo.UpsertKeys(ctx, siteID, keys); err != nil {
				res.Errors = append(res.Errors, "keys 落库: "+err.Error())
			} else {
				res.KeysSync = len(keys)
			}
			if b, jerr := json.Marshal(kp); jerr == nil {
				_ = s.Repo.InsertSnapshot(ctx, siteID, "keys", redactJSON(string(b)), nil)
			}
		}
	}

	if caps.ListGroups {
		groups, err := ad.ListGroups(ctx, atx)
		if err != nil {
			res.Errors = append(res.Errors, "groups: "+err.Error())
		} else {
			gs := make([]model.SiteGroup, 0, len(groups))
			for _, g := range groups {
				gs = append(gs, model.SiteGroup{
					RemoteGroupID: g.RemoteGroupID, Name: g.Name,
					Ratio: g.Ratio, Available: g.Available, Desc: g.Desc, Freshness: "fresh",
				})
			}
			if err := s.Repo.UpsertGroups(ctx, siteID, gs); err != nil {
				res.Errors = append(res.Errors, "groups 落库: "+err.Error())
			} else {
				res.GroupsSync = len(gs)
			}
			if b, jerr := json.Marshal(groups); jerr == nil {
				_ = s.Repo.InsertSnapshot(ctx, siteID, "groups", redactJSON(string(b)), nil)
			}
		}
	}

	// 返利概览（FR-10.1）：能力 gate，unsupported 跳过不重试
	if caps.Affiliate == adapter.CapSupported {
		if err := s.SyncAffiliate(ctx, ad, atx, siteID); err != nil {
			res.Errors = append(res.Errors, "affiliate: "+err.Error())
		} else {
			res.Affiliate = true
		}
	}

	// 账号余额（account 域）
	if caps.Quota {
		if err := s.SyncAccount(ctx, ad, atx, siteID); err != nil {
			res.Errors = append(res.Errors, "account: "+err.Error())
		}
	}

	// 用量日志（usage_log 域）：游标增量
	if caps.UsageLogs {
		if n, err := s.SyncUsageLog(ctx, ad, atx, siteID); err != nil {
			res.Errors = append(res.Errors, "usage_log: "+err.Error())
		} else {
			res.UsageLogsSync = n
		}
	}

	level := "info"
	if len(res.Errors) > 0 {
		level = "warn"
	}
	_ = s.emitEvent(ctx, siteID, "sync.site", level,
		fmt.Sprintf("同步完成 keys=%d groups=%d affiliate=%v", res.KeysSync, res.GroupsSync, res.Affiliate))
	return res, nil
}

// SyncAffiliate 拉取返利概览 → site_affiliates 投影（FR-10.1/10.2）。
func (s *Services) SyncAffiliate(ctx context.Context, ad adapter.SiteAdapter, atx adapter.AuthCtx, siteID string) error {
	info, err := ad.AffiliateInfo(ctx, atx)
	if err != nil {
		return err
	}
	row := &model.SiteAffiliate{
		SiteID:       siteID,
		AffCode:      info.AffCode,
		RebateRate:   info.RebateRate,
		Available:    info.Available,
		Frozen:       info.Frozen,
		History:      info.History,
		InviteeCount: info.InviteeCount,
		Freshness:    "fresh",
	}
	if len(info.Invitees) > 0 {
		if b, jerr := json.Marshal(info.Invitees); jerr == nil {
			row.Invitees = string(b)
		}
	}
	if err := s.Repo.UpsertAffiliate(ctx, row); err != nil {
		return err
	}
	if b, jerr := json.Marshal(info); jerr == nil {
		_ = s.Repo.InsertSnapshot(ctx, siteID, "affiliates", redactJSON(string(b)), nil)
	}
	return nil
}

// SyncAccount 拉取账号余额 → site_account 投影（FR-4.1；currency 不折算）。
func (s *Services) SyncAccount(ctx context.Context, ad adapter.SiteAdapter, atx adapter.AuthCtx, siteID string) error {
	q, err := ad.Quota(ctx, atx)
	if err != nil {
		return err
	}
	bal := q.Balance
	used := q.Used
	row := &model.SiteAccount{
		SiteID:            siteID,
		Balance:           &bal,
		Used:              &used,
		Currency:          q.Currency,
		UnitNote:          q.UnitNote,
		SubscriptionState: "",
		Freshness:         "fresh",
	}
	if err := s.Repo.UpsertAccount(ctx, row); err != nil {
		return err
	}
	if b, jerr := json.Marshal(q); jerr == nil {
		_ = s.Repo.InsertSnapshot(ctx, siteID, "account", redactJSON(string(b)), nil)
	}
	return nil
}

// AggregateUsageDaily 汇总指定日期的 usage_logs → usage_daily（S3）。
func (s *Services) AggregateUsageDaily(ctx context.Context, siteID, day string) (int, error) {
	return s.Repo.AggregateUsageDaily(ctx, siteID, day)
}

// ListUsageDaily 跨站日聚合查询（用于 Dashboard）。
func (s *Services) ListUsageDaily(ctx context.Context, siteID, since, until, modelName string, limit int) ([]model.UsageDaily, error) {
	return s.Repo.ListUsageDaily(ctx, siteID, since, until, modelName, limit)
}

// ListSiteModels 模型广场聚合（S4）：跨站已用模型 + 余额。
func (s *Services) ListSiteModels(ctx context.Context, limit int) ([]model.SiteModel, error) {
	return s.Repo.ListSiteModels(ctx, limit)
}

// ListAnnouncements 跨站公告聚合（S5；FR-4.4）。
func (s *Services) ListAnnouncements(ctx context.Context, limit int) ([]model.SiteAnnouncement, error) {
	return s.Repo.ListAnnouncements(ctx, limit)
}

// SyncCheckin 执行单日签到（FR-3）：查询状态 → 必要时执行 → 记录 checkins。
func (s *Services) SyncCheckin(ctx context.Context, ad adapter.SiteAdapter, atx adapter.AuthCtx, siteID string) error {
	if ad.Capabilities().Checkin != adapter.CapSupported {
		return nil
	}
	st, err := ad.CheckinStatus(ctx, atx)
	if err != nil {
		return err
	}
	if st.State == adapter.CheckinAlready {
		_ = s.Repo.UpsertCheckin(ctx, &model.SiteCheckin{
			SiteID: siteID, Date: time.Now().UTC().Format("2006-01-02"),
			State: string(st.State), Message: st.Message,
		})
		return nil
	}
	res, err := ad.Checkin(ctx, atx)
	if err != nil {
		return err
	}
	var qa *float64
	if res.QuotaAwarded > 0 {
		v := res.QuotaAwarded
		qa = &v
	}
	return s.Repo.UpsertCheckin(ctx, &model.SiteCheckin{
		SiteID: siteID, Date: time.Now().UTC().Format("2006-01-02"),
		State: string(res.State), QuotaAwarded: qa, Message: res.Message,
	})
}

// ListUsageLogs 站点用量日志查询（Dashboard 明细）。
func (s *Services) ListUsageLogs(ctx context.Context, siteID string, since, until time.Time, modelName string, limit int) ([]model.UsageLog, error) {
	return s.Repo.ListUsageLogs(ctx, siteID, since, until, modelName, limit)
}

// SyncUsageLog 游标增量拉取用量日志 → usage_logs 投影（S2）。
// cursor 存于 site_sync_state.cursor（JSON {"since":"<RFC3339>"}），newapi 秒级 / sub2api 按日。
func (s *Services) SyncUsageLog(ctx context.Context, ad adapter.SiteAdapter, atx adapter.AuthCtx, siteID string) (int, error) {
	since := time.Now().Add(-24 * time.Hour).UTC()
	if st, err := s.Repo.GetSyncState(ctx, siteID, "usage_log"); err == nil {
		var cur struct {
			Since string `json:"since"`
		}
		if err := json.Unmarshal([]byte(st.Cursor), &cur); err == nil && cur.Since != "" {
			if t, perr := time.Parse(time.RFC3339, cur.Since); perr == nil {
				since = t
			}
		}
	}
	logs, err := ad.UsageLogs(ctx, atx, since)
	if err != nil {
		return 0, err
	}
	rows := make([]model.UsageLog, 0, len(logs))
	for _, l := range logs {
		rows = append(rows, model.UsageLog{
			RemoteRef:        l.RemoteRef,
			Timestamp:        l.Timestamp,
			ModelName:        l.ModelName,
			ApiKeyID:         l.ApiKeyID,
			ApiKeyMask:       l.ApiKeyMask,
			PromptTokens:     l.PromptTokens,
			CompletionTokens: l.CompletionTokens,
			TotalTokens:      l.TotalTokens,
			Amount:           l.Amount,
			Currency:         l.Currency,
			Status:           l.Status,
			ErrCode:          l.ErrCode,
		})
	}
	n, err := s.Repo.InsertUsageLogs(ctx, siteID, rows)
	if err != nil {
		return 0, err
	}
	// 更新游标为本批最晚时间
	if len(logs) > 0 {
		last := since
		for _, l := range logs {
			if l.Timestamp.After(last) {
				last = l.Timestamp
			}
		}
		cur, _ := json.Marshal(map[string]string{"since": last.UTC().Format(time.RFC3339)})
		_ = s.Repo.MarkSyncStateRun(ctx, siteID, "usage_log", true, "", strPtr(string(cur)), nil)
	}
	return n, nil
}

// redactJSON 快照脱敏：对疑似 token/password 字段打码（NFR-3）。
func redactJSON(s string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return "{}"
	}
	redactMap(m)
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func redactMap(m map[string]any) {
	sensitive := map[string]bool{
		"access_token": true, "refresh_token": true, "password": true,
		"token": true, "secret": true, "pat": true, "key": true,
	}
	for k, v := range m {
		if sensitive[strings.ToLower(k)] {
			m[k] = "***"
			continue
		}
		if sub, ok := v.(map[string]any); ok {
			redactMap(sub)
		}
	}
}

// ---- 返利（FR-10.3/10.4）----

// AffiliateOut 返利概览出参（含站点名便于跨站汇总）。
type AffiliateOut struct {
	model.SiteAffiliate
	SiteName string `json:"site_name"`
	SiteURL  string `json:"site_url"`
	SiteType string `json:"site_type"`
}

// ListAffiliates 跨站返利汇总。
func (s *Services) ListAffiliates(ctx context.Context, siteID string) ([]AffiliateOut, error) {
	var ids []string
	if siteID != "" {
		ids = []string{siteID}
	}
	rows, err := s.Repo.ListAffiliates(ctx, ids)
	if err != nil {
		return nil, err
	}
	sites, err := s.Repo.ListSites(ctx, "")
	if err != nil {
		return nil, err
	}
	byID := make(map[string]model.Site, len(sites))
	for _, st := range sites {
		byID[st.ID] = st
	}
	out := make([]AffiliateOut, 0, len(rows))
	for _, r := range rows {
		o := AffiliateOut{SiteAffiliate: r}
		if st, ok := byID[r.SiteID]; ok {
			o.SiteName = st.Name
			o.SiteURL = st.BaseURL
			o.SiteType = st.SiteType
		}
		out = append(out, o)
	}
	return out, nil
}

// AffRule 自动划转规则（sites.aff_cfg / site_affiliates.aff_cfg JSON）。
type AffRule struct {
	Enabled          bool    `json:"enabled"`
	MinAmount        float64 `json:"min_amount"`
	MaxPerTransfer   float64 `json:"max_per_transfer"`
	DailyLimit       int     `json:"daily_limit"`
	LastTransferDate string  `json:"last_transfer_date,omitempty"`
}

// defaultAffRule 规则默认值（enabled=false，architecture §3）。
func defaultAffRule() AffRule {
	return AffRule{Enabled: false, MinAmount: 0, MaxPerTransfer: 0, DailyLimit: 1}
}

// GetAffiliate 单站返利 + 规则。
func (s *Services) GetAffiliate(ctx context.Context, siteID string) (*AffiliateOut, error) {
	row, err := s.Repo.GetAffiliate(ctx, siteID)
	if err != nil {
		return nil, err
	}
	out := &AffiliateOut{SiteAffiliate: *row}
	if st, err := s.Repo.GetSite(ctx, siteID); err == nil {
		out.SiteName = st.Name
		out.SiteURL = st.BaseURL
		out.SiteType = st.SiteType
	}
	return out, nil
}

// PutAffiliateRule 更新自动划转规则（FR-10.4 配置）。
func (s *Services) PutAffiliateRule(ctx context.Context, siteID string, rule AffRule) error {
	// 先校验站点存在，避免为不存在的站点创建孤儿规则投影
	if _, err := s.Repo.GetSite(ctx, siteID); err != nil {
		return err
	}
	if rule.MinAmount < 0 || rule.MaxPerTransfer < 0 || rule.DailyLimit < 0 {
		return errors.New("规则数值不能为负")
	}
	if rule.MaxPerTransfer > 0 && rule.MaxPerTransfer < rule.MinAmount {
		return errors.New("max_per_transfer 不能小于 min_amount")
	}
	b, err := json.Marshal(rule)
	if err != nil {
		return err
	}
	return s.Repo.UpdateAffiliateCfg(ctx, siteID, string(b))
}

// SiteAffiliateInvitees 站点受邀用户列表（投影，参考 sub2api AffiliateDetail.invitees）。
func (s *Services) SiteAffiliateInvitees(ctx context.Context, siteID string) ([]model.SiteAffiliateInvitee, error) {
	return s.Repo.AffiliateInvitees(ctx, siteID)
}

// GetAffiliateRule 读取规则（无记录返回默认）。
func (s *Services) GetAffiliateRule(ctx context.Context, siteID string) (AffRule, error) {
	row, err := s.Repo.GetAffiliate(ctx, siteID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return defaultAffRule(), nil
		}
		return AffRule{}, err
	}
	var rule AffRule
	if err := json.Unmarshal([]byte(row.AffCfg), &rule); err != nil {
		return defaultAffRule(), nil
	}
	return rule, nil
}

// TransferAffiliateInput 手动划转入参。
type TransferAffiliateInput struct {
	Amount float64 `json:"amount"`
	// DryRun 默认 true（不可逆操作安全默认）；前端确认后显式传 false
	DryRun *bool `json:"dry_run,omitempty"`
}

// TransferAffiliate 手动划转（FR-10.3；留证 aff_transfers + 事件）。
func (s *Services) TransferAffiliate(ctx context.Context, siteID string, in TransferAffiliateInput) (*model.AffTransfer, error) {
	site, err := s.Repo.GetSite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	if in.Amount <= 0 {
		return nil, errors.New("amount 必须大于 0")
	}
	dryRun := in.DryRun == nil || *in.DryRun

	// 规则上限校验（手动划转也受单笔上限约束）
	rule, _ := s.GetAffiliateRule(ctx, siteID)
	if rule.MaxPerTransfer > 0 && in.Amount > rule.MaxPerTransfer {
		return nil, fmt.Errorf("超过单笔上限 %.2f", rule.MaxPerTransfer)
	}

	row, err := s.Repo.GetAffiliate(ctx, siteID)
	var before *float64
	if err == nil && row.Available != nil {
		before = row.Available
		if in.Amount > *before {
			return nil, fmt.Errorf("可用余额 %.2f 不足", *before)
		}
	}

	t := &model.AffTransfer{
		SiteID: siteID, Amount: in.Amount,
		TriggerType: "manual", State: "pending",
		AmountBefore: before,
		RuleSnapshot: mustJSON(rule),
	}
	if dryRun {
		t.State = "dry_run"
		t.Message = "dry_run 演练：未调用上游"
		if err := s.Repo.InsertAffTransfer(ctx, t); err != nil {
			return nil, err
		}
		_ = s.emitEvent(ctx, siteID, "aff.transfer", "info", fmt.Sprintf("划转演练 amount=%.2f（未执行）", in.Amount))
		return t, nil
	}

	cred, err := s.Repo.GetCredential(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("缺少凭据: %w", err)
	}
	ad, err := s.adapterFor(site)
	if err != nil {
		return nil, err
	}
	res, err := ad.TransferAffiliate(ctx, s.authContext(cred), adapter.Amount{Value: in.Amount, Currency: "USD"})
	now := time.Now()
	t.FinishedAt = &now
	if err != nil {
		t.State = "failed"
		t.ErrorClass = errClass(err)
		t.Message = err.Error()
	} else {
		t.State = "success"
		t.Message = res.Message
		if res.AmountBefore > 0 {
			b := res.AmountBefore
			t.AmountBefore = &b
		}
	}
	if ierr := s.Repo.InsertAffTransfer(ctx, t); ierr != nil {
		return nil, ierr
	}
	level := "info"
	if t.State == "failed" {
		level = "error"
	}
	_ = s.emitEvent(ctx, siteID, "aff.transfer", level, fmt.Sprintf("划转 %s amount=%.2f：%s", t.State, in.Amount, t.Message))

	// 划转后同步最新余额
	if t.State == "success" {
		_ = s.SyncAffiliate(ctx, ad, s.authContext(cred), siteID)
	}
	return t, nil
}

func errClass(err error) string {
	var ae *adapter.AdapterError
	if errors.As(err, &ae) {
		return string(ae.Code)
	}
	return "upstream_error"
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ---- keys / groups / events 查询 ----

// ListKeys 跨站 key 汇总。
func (s *Services) ListKeys(ctx context.Context, siteID string) ([]model.SiteKey, error) {
	return s.Repo.ListKeys(ctx, siteID)
}

// ListGroups 分组汇总。
func (s *Services) ListGroups(ctx context.Context, siteID string) ([]model.SiteGroup, error) {
	return s.Repo.ListGroups(ctx, siteID)
}

// GetAccount 单站账号余额投影。
func (s *Services) GetAccount(ctx context.Context, siteID string) (*model.SiteAccount, error) {
	return s.Repo.GetAccount(ctx, siteID)
}

// ListAccounts 跨站账号余额投影。
func (s *Services) ListAccounts(ctx context.Context) ([]model.SiteAccount, error) {
	return s.Repo.ListAccounts(ctx)
}

// ListEvents 事件流水。
func (s *Services) ListEvents(ctx context.Context, siteID string, since *time.Time, limit int) ([]model.Event, error) {
	return s.Repo.ListEvents(ctx, since, siteID, limit)
}
