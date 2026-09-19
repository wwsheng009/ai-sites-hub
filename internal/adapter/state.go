// Package adapter 认证状态机常量（对齐 site_credentials.auth_state，FR-2）。
package adapter

// 认证状态（写入 site_credentials.auth_state）。
const (
	StateNone             = "none"
	StateOK               = "ok"
	StateLoginFailed      = "login_failed"
	StateTOTPRequired     = "totp_required"
	StateTurnstileBlocked = "turnstile_blocked"
	StateCFBlocked        = "cf_blocked"
	StateTokenExpired     = "token_expired"
	StateRefreshFailed    = "refresh_failed"
)

// 能力字符串常量。
const (
	CapSupported   = "supported"
	CapUnsupported = "unsupported"
	CapBlocked     = "blocked_by_turnstile"
)
