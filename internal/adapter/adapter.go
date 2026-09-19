// Package adapter 站点类型适配层：SiteAdapter 统一接口 + 注册表（architecture §4.1）。
package adapter

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Type 站点类型。
type Type string

const (
	TypeSub2API Type = "sub2api"
	TypeNewAPI  Type = "new-api"
	TypeUnknown Type = "unknown"
)

// Page 分页请求。
type Page struct {
	Page    int `json:"page"`
	Size    int `json:"size"`
	MaxPage int `json:"-"` // adapter 内部封顶
}

// KeyPage 分页结果。
type KeyPage struct {
	Items   []SiteKey
	Total   int
	Page    int
	HasNext bool
}

// SiteKey 归一化 key 投影。
type SiteKey struct {
	RemoteKeyID    string     `json:"remote_key_id"`
	Name           string     `json:"name"`
	Group          string     `json:"group"`
	Status         string     `json:"status"`
	QuotaLimit     *float64   `json:"quota_limit,omitempty"`
	QuotaUsed      *float64   `json:"quota_used,omitempty"`
	QuotaRemaining *float64   `json:"quota_remaining,omitempty"`
	Unlimited      bool       `json:"unlimited"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      *time.Time `json:"created_at,omitempty"`
	KeySource      string     `json:"key_source"` // masked | plaintext
}

// Group 归一化分组。
type Group struct {
	RemoteGroupID string  `json:"remote_group_id"`
	Name          string  `json:"name"`
	Ratio         float64 `json:"ratio"`
	Available     bool    `json:"available"`
	Desc          string  `json:"desc"`
}

// AccountQuota 账号级额度（统一站点余额单位）。
type AccountQuota struct {
	Balance    float64 `json:"balance"`
	Used       float64 `json:"used"`
	Currency   string  `json:"currency"` // USD | quota
	UnitNote   string  `json:"unit_note"`
	HasBalance bool    `json:"has_balance"`
	HasUsed    bool    `json:"has_used"`
}

// AffiliateInfo 归一化站点返利信息（FR-10.1；余额单位为站点余额单位）。
type AffiliateInfo struct {
	AffCode      string   `json:"aff_code"`
	RebateRate   *float64 `json:"rebate_rate,omitempty"`
	Available    *float64 `json:"available,omitempty"`
	Frozen       *float64 `json:"frozen,omitempty"`
	History      *float64 `json:"history,omitempty"`
	InviteeCount *int     `json:"invitee_count,omitempty"`
	Currency     string   `json:"currency"` // USD | quota
	UnitNote     string   `json:"unit_note"`
}

// TransferResult 划转结果（不可逆操作留证）。
type TransferResult struct {
	RemoteRef    string  `json:"remote_ref,omitempty"`
	Amount       float64 `json:"amount"`
	AmountBefore float64 `json:"amount_before,omitempty"`
	Currency     string  `json:"currency"`
	UnitNote     string  `json:"unit_note"`
	Message      string  `json:"message"`
}

// CheckinState 签到状态。
type CheckinState string

const (
	CheckinSuccess            CheckinState = "success"
	CheckinAlready            CheckinState = "already"
	CheckinFailed             CheckinState = "failed"
	CheckinSkipped            CheckinState = "skipped"
	CheckinUnsupported        CheckinState = "unsupported"
	CheckinBlockedByTurnstile CheckinState = "blocked_by_turnstile"
)

// CheckinStatusResult 签到状态查询结果。
type CheckinStatusResult struct {
	State     CheckinState
	Message   string
	TodayDone bool
}

// CheckinResult 签到执行结果。
type CheckinResult struct {
	State        CheckinState
	QuotaAwarded float64
	Message      string
}

// Capabilities adapter 自报能力（上层按能力跳过，unsupported 不重试）。
type Capabilities struct {
	Checkin      string `json:"checkin"`   // supported | unsupported | blocked_by_turnstile
	Affiliate    string `json:"affiliate"` // supported | unsupported
	Refresh      bool   `json:"refresh"`   // 是否支持 token 刷新
	ListKeys     bool   `json:"list_keys"`
	ListGroups   bool   `json:"list_groups"`
	Quota        bool   `json:"quota"`
	PlaintextKey bool   `json:"plaintext_key"` // 是否能取明文 key
}

// Credentials 认证凭据（调用方保证明文不落日志）。
type Credentials struct {
	AuthMode     string // username_password | token | hybrid
	Username     string
	Password     string
	TOTPSecret   string
	AccessToken  string
	RefreshToken string
	PAT          string
}

// AuthCtx 已认证上下文（token/PAT + cookies）。
type AuthCtx struct {
	AccessToken  string
	RefreshToken string
	PAT          string
	Cookies      []*http.Cookie
	Extra        map[string]string
}

// AuthState 认证结果（对齐 site_credentials.auth_state；密文字段由调用方加密落库）。
type AuthState struct {
	State        string `json:"state"` // none|ok|login_failed|totp_required|turnstile_blocked|cf_blocked|token_expired|refresh_failed
	Message      string `json:"message"`
	AccessToken  string `json:"-"`
	RefreshToken string `json:"-"`
	PAT          string `json:"-"`
	Cookies      []*http.Cookie
	ExpiresHint  *time.Time `json:"-"`
}

// Amount 划转金额（携带单位约定；Value>0）。
type Amount struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"` // USD | quota
	Note     string  `json:"note,omitempty"`
}

// DetectResult 识别结果。
type DetectResult struct {
	Score    int            `json:"score"`
	Type     Type           `json:"type"`
	Evidence map[string]any `json:"evidence"`
	FinalURL string         `json:"final_url"`
}

// SiteAdapter 统一站点适配接口（architecture §4.1）。
type SiteAdapter interface {
	Type() Type

	// 识别与探测（公开端点，无需凭据）
	Detect(ctx context.Context, baseURL string) (DetectResult, error)

	// 认证（凭据阶梯 + 状态机，FR-2）
	Login(ctx context.Context, cred Credentials) (AuthState, error)
	Refresh(ctx context.Context, cred Credentials) (AuthState, error)
	Verify(ctx context.Context, a AuthCtx) error

	// 能力查询
	Capabilities() Capabilities

	// 数据同步
	ListKeys(ctx context.Context, a AuthCtx, page Page) (KeyPage, error)
	ListGroups(ctx context.Context, a AuthCtx) ([]Group, error)
	Quota(ctx context.Context, a AuthCtx) (AccountQuota, error)

	// 站点返利（FR-10；Capabilities.Affiliate gate，unsupported 返回错误不再重试）
	AffiliateInfo(ctx context.Context, a AuthCtx) (AffiliateInfo, error)
	TransferAffiliate(ctx context.Context, a AuthCtx, amount Amount) (TransferResult, error)

	// 签到（仅 new-api 实现；sub2api 返回 unsupported）
	CheckinStatus(ctx context.Context, a AuthCtx) (CheckinStatusResult, error)
	Checkin(ctx context.Context, a AuthCtx) (CheckinResult, error)
}

// ErrUnsupported 能力不支持（调用方应跳过且不重试）。
type ErrUnsupported struct{ Capability string }

func (e *ErrUnsupported) Error() string {
	return fmt.Sprintf("adapter: 站点类型不支持能力 %q", e.Capability)
}

// AdapterFactory adapter 工厂：按站点基址与日志构建实例
// （同步类方法依赖 adapter 内部绑定的 base，必须按站点实例化）。
type AdapterFactory func(baseURL string, log *slog.Logger) (SiteAdapter, error)

// Registry adapter 注册表（按站点类型构建实例）。
type Registry struct {
	mu        sync.RWMutex
	factories map[Type]AdapterFactory
}

// NewRegistry 空注册表。
func NewRegistry() *Registry {
	return &Registry{factories: make(map[Type]AdapterFactory)}
}

// Register 注册类型工厂（重复注册后者覆盖）。
func (r *Registry) Register(t Type, f AdapterFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[t] = f
}

// Get 按类型为指定站点构建 adapter。
func (r *Registry) Get(t Type, baseURL string, log *slog.Logger) (SiteAdapter, error) {
	r.mu.RLock()
	f, ok := r.factories[t]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("adapter: 未注册的站点类型 %q", t)
	}
	if log == nil {
		log = slog.Default()
	}
	return f(baseURL, log)
}
