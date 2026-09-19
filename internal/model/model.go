// Package model GORM 实体（列名与 migrations/0001_init.sql 对齐，禁止 AutoMigrate）。
package model

import "time"

// Site 站点。
type Site struct {
	ID      string `gorm:"column:id;primaryKey;size:36" json:"id"`
	Name    string `gorm:"column:name;not null" json:"name"`
	BaseURL string `gorm:"column:base_url;not null;uniqueIndex" json:"base_url"`
	// ProxyURL 站点级出站代理（http/https/socks5；空=使用全局 proxy.url，两者皆空则直连）。
	ProxyURL     string     `gorm:"column:proxy_url;not null;default:''" json:"proxy_url"`
	APIBaseURL   string     `gorm:"column:api_base_url;not null;default:''" json:"api_base_url"`
	SiteType     string     `gorm:"column:site_type;not null;default:'unknown'" json:"site_type"`
	Status       string     `gorm:"column:status;not null;default:'active'" json:"status"`
	DetectScore  int        `gorm:"column:detect_score;not null;default:0" json:"detect_score"`
	DetectReport string     `gorm:"column:detect_report;not null;default:'{}'" json:"detect_report"`
	CheckinCfg   string     `gorm:"column:checkin_cfg;not null;default:'{}'" json:"checkin_cfg"`
	AffCfg       string     `gorm:"column:aff_cfg;not null;default:'{}'" json:"aff_cfg"`
	SyncCfg      string     `gorm:"column:sync_cfg;not null;default:'{}'" json:"sync_cfg"`
	LastProbeAt  *time.Time `gorm:"column:last_probe_at" json:"last_probe_at"`
	LatencyMs    *int64     `gorm:"column:latency_ms" json:"latency_ms"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 表名。
func (Site) TableName() string { return "sites" }

// SiteCredential 凭据（cipher 列为密文；明文永不落库）。
type SiteCredential struct {
	ID                 string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	SiteID             string     `gorm:"column:site_id;not null;uniqueIndex" json:"site_id"`
	AuthMode           string     `gorm:"column:auth_mode;not null;default:'username_password'" json:"auth_mode"`
	Username           string     `gorm:"column:username;not null;default:''" json:"username"`
	PasswordCipher     []byte     `gorm:"column:password_cipher" json:"-"`
	TOTPSecretCipher   []byte     `gorm:"column:totp_secret_cipher" json:"-"`
	AccessTokenCipher  []byte     `gorm:"column:access_token_cipher" json:"-"`
	RefreshTokenCipher []byte     `gorm:"column:refresh_token_cipher" json:"-"`
	PATCipher          []byte     `gorm:"column:pat_cipher" json:"-"`
	AuthState          string     `gorm:"column:auth_state;not null;default:'none'" json:"auth_state"`
	AuthStateMsg       string     `gorm:"column:auth_state_msg;not null;default:''" json:"auth_state_msg"`
	LastLoginAt        *time.Time `gorm:"column:last_login_at" json:"last_login_at"`
	TokenExpireHint    *time.Time `gorm:"column:token_expire_hint" json:"token_expire_hint"`
	UpdatedAt          time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 表名。
func (SiteCredential) TableName() string { return "site_credentials" }

// SiteKey key 归一化投影。
type SiteKey struct {
	ID             string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	SiteID         string     `gorm:"column:site_id;not null" json:"site_id"`
	RemoteKeyID    string     `gorm:"column:remote_key_id;not null" json:"remote_key_id"`
	Name           string     `gorm:"column:name;not null;default:''" json:"name"`
	Group          string     `gorm:"column:grp;not null;default:''" json:"group"`
	Status         string     `gorm:"column:status;not null;default:'active'" json:"status"`
	QuotaLimit     *float64   `gorm:"column:quota_limit" json:"quota_limit"`
	QuotaUsed      *float64   `gorm:"column:quota_used" json:"quota_used"`
	QuotaRemaining *float64   `gorm:"column:quota_remaining" json:"quota_remaining"`
	Unlimited      bool       `gorm:"column:unlimited;not null;default:0" json:"unlimited"`
	ExpiresAt      *time.Time `gorm:"column:expires_at" json:"expires_at"`
	CreatedAt      *time.Time `gorm:"column:created_at" json:"created_at"`
	KeySource      string     `gorm:"column:key_source;not null;default:'masked'" json:"key_source"`
	RawSnapshotID  *string    `gorm:"column:raw_snapshot_id" json:"raw_snapshot_id"`
	Freshness      string     `gorm:"column:freshness;not null;default:'missing'" json:"freshness"`
	LastSyncAt     *time.Time `gorm:"column:last_sync_at" json:"last_sync_at"`
}

// TableName 表名。
func (SiteKey) TableName() string { return "site_keys" }

// SiteGroup 分组倍率投影。
type SiteGroup struct {
	ID            string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	SiteID        string     `gorm:"column:site_id;not null" json:"site_id"`
	RemoteGroupID string     `gorm:"column:remote_group_id;not null" json:"remote_group_id"`
	Name          string     `gorm:"column:name;not null;default:''" json:"name"`
	Ratio         float64    `gorm:"column:ratio;not null;default:1.0" json:"ratio"`
	Available     bool       `gorm:"column:available;not null;default:1" json:"available"`
	Desc          string     `gorm:"column:desc;not null;default:''" json:"desc"`
	Freshness     string     `gorm:"column:freshness;not null;default:'missing'" json:"freshness"`
	LastSyncAt    *time.Time `gorm:"column:last_sync_at" json:"last_sync_at"`
}

// TableName 表名。
func (SiteGroup) TableName() string { return "site_groups" }

// SiteAffiliateInvitee 受邀用户条目（投影 sub2api AffiliateDetail.invitees；
// email 上游服务层已脱敏，这里原样落库）。
type SiteAffiliateInvitee struct {
	UserID      int64      `json:"user_id"`
	Email       string     `json:"email"`
	Username    string     `json:"username"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	TotalRebate float64    `json:"total_rebate"`
}

// SiteAffiliate 站点返利投影（FR-10）。
type SiteAffiliate struct {
	ID           string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	SiteID       string     `gorm:"column:site_id;not null;uniqueIndex" json:"site_id"`
	AffCode      string     `gorm:"column:aff_code;not null;default:''" json:"aff_code"`
	RebateRate   *float64   `gorm:"column:rebate_rate" json:"rebate_rate"`
	Available    *float64   `gorm:"column:available" json:"available"`
	Frozen       *float64   `gorm:"column:frozen" json:"frozen"`
	History      *float64   `gorm:"column:history" json:"history"`
	InviteeCount *int       `gorm:"column:invitee_count" json:"invitee_count"`
	Invitees     string     `gorm:"column:invitees;not null;default:'[]'" json:"invitees"`
	AffCfg       string     `gorm:"column:aff_cfg;not null;default:'{}'" json:"aff_cfg"`
	Freshness    string     `gorm:"column:freshness;not null;default:'missing'" json:"freshness"`
	LastSyncAt   *time.Time `gorm:"column:last_sync_at" json:"last_sync_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 表名。
func (SiteAffiliate) TableName() string { return "site_affiliates" }

// AffTransfer 划转记录（不可逆留证，FR-10.4）。
type AffTransfer struct {
	ID           string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	SiteID       string     `gorm:"column:site_id;not null" json:"site_id"`
	Amount       float64    `gorm:"column:amount;not null" json:"amount"`
	State        string     `gorm:"column:state;not null" json:"state"`               // pending|success|failed|dry_run
	TriggerType  string     `gorm:"column:trigger_type;not null" json:"trigger_type"` // manual|auto_rule
	RuleSnapshot string     `gorm:"column:rule_snapshot;not null;default:'{}'" json:"rule_snapshot"`
	AmountBefore *float64   `gorm:"column:amount_before" json:"amount_before"`
	ErrorClass   string     `gorm:"column:error_class;not null;default:''" json:"error_class"`
	Message      string     `gorm:"column:message;not null;default:''" json:"message"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	FinishedAt   *time.Time `gorm:"column:finished_at" json:"finished_at"`
}

// TableName 表名。
func (AffTransfer) TableName() string { return "aff_transfers" }

// Snapshot 快照（上游原始响应，脱敏后存储）。
type Snapshot struct {
	ID         string    `gorm:"column:id;primaryKey;size:36" json:"id"`
	SiteID     string    `gorm:"column:site_id;not null" json:"site_id"`
	Kind       string    `gorm:"column:kind;not null" json:"kind"`
	StatusCode *int      `gorm:"column:status_code" json:"status_code"`
	Body       string    `gorm:"column:body;not null;default:'{}'" json:"body"`
	FetchedAt  time.Time `gorm:"column:fetched_at" json:"fetched_at"`
}

// TableName 表名。
func (Snapshot) TableName() string { return "snapshots" }

// Event 平台事件。
type Event struct {
	ID        string    `gorm:"column:id;primaryKey;size:36" json:"id"`
	SiteID    *string   `gorm:"column:site_id" json:"site_id"`
	Type      string    `gorm:"column:type;not null" json:"type"`
	Level     string    `gorm:"column:level;not null;default:'info'" json:"level"`
	Message   string    `gorm:"column:message;not null;default:''" json:"message"`
	Data      string    `gorm:"column:data;not null;default:'{}'" json:"data"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 表名。
func (Event) TableName() string { return "events" }

// SiteSyncState 调度与游标状态（每站点 × 每域一行）。
type SiteSyncState struct {
	ID                  string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	SiteID              string     `gorm:"column:site_id;not null" json:"site_id"`
	Domain              string     `gorm:"column:domain;not null" json:"domain"`
	Enabled             int        `gorm:"column:enabled;not null;default:1" json:"enabled"`
	IntervalS           int        `gorm:"column:interval_s;not null;default:300" json:"interval_s"`
	LastRunAt           *time.Time `gorm:"column:last_run_at" json:"last_run_at"`
	NextRunAt           *time.Time `gorm:"column:next_run_at" json:"next_run_at"`
	Cursor              string     `gorm:"column:cursor;not null;default:'{}'" json:"cursor"`
	Fingerprint         string     `gorm:"column:fingerprint;not null;default:''" json:"fingerprint"`
	ConsecutiveFailures int        `gorm:"column:consecutive_failures;not null;default:0" json:"consecutive_failures"`
	LastErrorClass      string     `gorm:"column:last_error_class;not null;default:''" json:"last_error_class"`
}

// TableName 表名。
func (SiteSyncState) TableName() string { return "site_sync_state" }

// SiteAccount 账号余额投影。
type SiteAccount struct {
	SiteID            string     `gorm:"column:site_id;primaryKey;size:36" json:"site_id"`
	Balance           *float64   `gorm:"column:balance" json:"balance"`
	Used              *float64   `gorm:"column:used" json:"used"`
	Currency          string     `gorm:"column:currency;not null;default:'quota'" json:"currency"`
	UnitNote          string     `gorm:"column:unit_note;not null;default:''" json:"unit_note"`
	SubscriptionState string     `gorm:"column:subscription_state;not null;default:''" json:"subscription_state"`
	Freshness         string     `gorm:"column:freshness;not null;default:'missing'" json:"freshness"`
	LastSyncAt        *time.Time `gorm:"column:last_sync_at" json:"last_sync_at"`
}

// TableName 表名。
func (SiteAccount) TableName() string { return "site_account" }

// UsageLog 单条调用日志（归一化；newapi /api/log/self，sub2api /api/v1/usage）。
// currency 仅标明，不折算。
type UsageLog struct {
	ID               string    `gorm:"column:id;primaryKey;size:36" json:"id"`
	SiteID           string    `gorm:"column:site_id;not null" json:"site_id"`
	RemoteRef        string    `gorm:"column:remote_ref;not null;default:''" json:"remote_ref"` // 上游日志 id / request_id
	Timestamp        time.Time `gorm:"column:ts;not null" json:"ts"`                            // 调用时间
	ModelName        string    `gorm:"column:model_name;not null;default:''" json:"model_name"`
	ApiKeyID         string    `gorm:"column:api_key_id;not null;default:''" json:"api_key_id"`
	ApiKeyMask       string    `gorm:"column:api_key_mask;not null;default:''" json:"api_key_mask"`
	PromptTokens     int64     `gorm:"column:prompt_tokens;not null;default:0" json:"prompt_tokens"`
	CompletionTokens int64     `gorm:"column:completion_tokens;not null;default:0" json:"completion_tokens"`
	TotalTokens      int64     `gorm:"column:total_tokens;not null;default:0" json:"total_tokens"`
	Amount           float64   `gorm:"column:amount;not null;default:0" json:"amount"`           // 计费金额（站点币）
	Currency         string    `gorm:"column:currency;not null;default:'quota'" json:"currency"` // 单位说明，不折算
	Status           string    `gorm:"column:status;not null;default:'ok'" json:"status"`        // ok|err|abort
	ErrCode          string    `gorm:"column:err_code;not null;default:''" json:"err_code"`
	FetchedAt        time.Time `gorm:"column:fetched_at;not null" json:"fetched_at"`
}

// TableName 表名。
func (UsageLog) TableName() string { return "usage_logs" }

// UsageDaily 用量日聚合（S3；按 day + model_name 汇总）。
type UsageDaily struct {
	ID               string  `gorm:"column:id;primaryKey;size:36" json:"id"`
	SiteID           string  `gorm:"column:site_id;not null" json:"site_id"`
	Day              string  `gorm:"column:day;not null" json:"day"` // 'YYYY-MM-DD'
	ModelName        string  `gorm:"column:model_name;not null;default:''" json:"model_name"`
	PromptTokens     int64   `gorm:"column:prompt_tokens;not null;default:0" json:"prompt_tokens"`
	CompletionTokens int64   `gorm:"column:completion_tokens;not null;default:0" json:"completion_tokens"`
	TotalTokens      int64   `gorm:"column:total_tokens;not null;default:0" json:"total_tokens"`
	Amount           float64 `gorm:"column:amount;not null;default:0" json:"amount"`
	Currency         string  `gorm:"column:currency;not null;default:'quota'" json:"currency"`
	RequestCount     int64   `gorm:"column:request_count;not null;default:0" json:"request_count"`
}

// TableName 表名。
func (UsageDaily) TableName() string { return "usage_daily" }

// SiteModel 模型广场聚合视图（S4；非物理表，仅用于跨站模型+余额查询）。
// currency 仅标明，不折算。
type SiteModel struct {
	SiteID      string   `gorm:"column:site_id" json:"site_id"`
	SiteName    string   `gorm:"column:site_name" json:"site_name"`
	ModelName   string   `gorm:"column:model_name" json:"model_name"`
	TotalTokens int64    `gorm:"column:total_tokens" json:"total_tokens"`
	Amount      float64  `gorm:"column:amount" json:"amount"`
	Currency    string   `gorm:"column:currency" json:"currency"`
	Balance     *float64 `gorm:"column:balance" json:"balance"`
	Freshness   string   `gorm:"column:freshness" json:"freshness"`
}

// SiteAnnouncement 站点公告（S5；FR-4.4）。
type SiteAnnouncement struct {
	ID          string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	SiteID      string     `gorm:"column:site_id;not null" json:"site_id"`
	RemoteRef   string     `gorm:"column:remote_ref;not null;default:''" json:"remote_ref"`
	Title       string     `gorm:"column:title;not null;default:''" json:"title"`
	Content     string     `gorm:"column:content;not null;default:''" json:"content"`
	ContentHash string     `gorm:"column:content_hash;not null;default:''" json:"content_hash"`
	PublishedAt *time.Time `gorm:"column:published_at" json:"published_at"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 表名。
func (SiteAnnouncement) TableName() string { return "site_announcements" }

// SiteCheckin 签到记录（FR-3）。
type SiteCheckin struct {
	ID           string    `gorm:"column:id;primaryKey;size:36" json:"id"`
	SiteID       string    `gorm:"column:site_id;not null" json:"site_id"`
	Date         string    `gorm:"column:date;not null" json:"date"`   // YYYY-MM-DD
	State        string    `gorm:"column:state;not null" json:"state"` // success|already|failed|skipped|unsupported|blocked_by_turnstile
	QuotaAwarded *float64  `gorm:"column:quota_awarded" json:"quota_awarded"`
	Message      string    `gorm:"column:message;not null;default:''" json:"message"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 表名。
func (SiteCheckin) TableName() string { return "checkins" }
