// Package model GORM 实体（列名与 migrations/0001_init.sql 对齐，禁止 AutoMigrate）。
package model

import "time"

// Site 站点。
type Site struct {
	ID           string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	Name         string     `gorm:"column:name;not null" json:"name"`
	BaseURL      string     `gorm:"column:base_url;not null;uniqueIndex" json:"base_url"`
	// ProxyURL 站点级出站代理（http/https/socks5；空=使用全局 proxy.url，两者皆空则直连）。
	ProxyURL    string     `gorm:"column:proxy_url;not null;default:''" json:"proxy_url"`
	APIBaseURL  string     `gorm:"column:api_base_url;not null;default:''" json:"api_base_url"`
	SiteType     string     `gorm:"column:site_type;not null;default:'unknown'" json:"site_type"`
	Status       string     `gorm:"column:status;not null;default:'active'" json:"status"`
	DetectScore  int        `gorm:"column:detect_score;not null;default:0" json:"detect_score"`
	DetectReport string     `gorm:"column:detect_report;not null;default:'{}'" json:"detect_report"`
	CheckinCfg   string     `gorm:"column:checkin_cfg;not null;default:'{}'" json:"checkin_cfg"`
	AffCfg       string     `gorm:"column:aff_cfg;not null;default:'{}'" json:"aff_cfg"`
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
