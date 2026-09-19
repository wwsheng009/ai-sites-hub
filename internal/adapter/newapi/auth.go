// Package newapi 站点识别与认证（architecture §4.2 契约映射）。
package newapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aiclient/internal/adapter"
)

// ---- Detect ----

// statusResp GET /api/status 特征字段。
type statusResp struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	Version    string `json:"version"`
	SystemName string `json:"system_name"`
	StartTime  int64  `json:"start_time"`
	// Data is present on a few compatible/new-api deployments which wrap the
	// public status payload in the usual {success,message,data} envelope.
	Data json.RawMessage `json:"data"`
}

type loginData struct {
	AccessToken     string `json:"access_token"`
	RefreshToken    string `json:"refresh_token"`
	Token           string `json:"token"` // 兼容较早的 new-api 变体
	TokenType       string `json:"token_type"`
	AccessExpiresAt int64  `json:"access_expires_at"`
	ExpiresAt       int64  `json:"expires_at"`
	ExpiresIn       int64  `json:"expires_in"`
}

// Detect GET /api/status 特征：success + version/system_name 命中即给证据。
func (a *Adapter) Detect(ctx context.Context, baseURL string) (adapter.DetectResult, error) {
	ad, err := New(baseURL, a.proxyURL, a.log) // 探测目标以传入 baseURL 为准
	if err != nil {
		return adapter.DetectResult{}, err
	}
	res := adapter.DetectResult{Type: adapter.TypeNewAPI, Evidence: map[string]any{}}

	var st statusResp
	code, _, err := ad.hc.DoJSON(ctx, "GET", ad.endpoint("/api/status"), nil, &st)
	if err == nil && code == http.StatusOK {
		// The reference new-api currently returns version/system_name at the
		// top level.  Some forks put the same fields under data; accept both
		// shapes so a harmless envelope change does not turn a known site into
		// "unknown".
		if len(st.Data) > 0 && (st.Version == "" || st.SystemName == "") {
			var nested statusResp
			if json.Unmarshal(st.Data, &nested) == nil {
				if st.Version == "" {
					st.Version = nested.Version
				}
				if st.SystemName == "" {
					st.SystemName = nested.SystemName
				}
				if st.StartTime == 0 {
					st.StartTime = nested.StartTime
				}
			}
		}
	}
	if err == nil && code == http.StatusOK && (st.Version != "" || st.SystemName != "") {
		res.Score += 8
		res.Evidence["api_status"] = true
		res.Evidence["success"] = st.Success
		res.Evidence["version"] = st.Version
		res.Evidence["system_name"] = st.SystemName
		res.FinalURL = ad.BaseURL()
	}
	if err != nil {
		res.Evidence["api_status_error"] = err.Error()
	} else if code != http.StatusOK {
		res.Evidence["api_status_code"] = code
	}
	return res, nil
}

// ---- Auth ----

// loginReq POST /api/user/login。
type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// apiEnvelope new-api 统一响应 {success, message, data}。
type apiEnvelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// userSelf GET /api/user/self 用户 DTO（aff_* 字段见 affiliate.go）。
type userSelf struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        int    `json:"role"`
	Status      int    `json:"status"`
	Quota       int64  `json:"quota"`
	UsedQuota   int64  `json:"used_quota"`
	Group       string `json:"group"`
	AffQuota    int64  `json:"aff_quota"`
	AffHistory  int64  `json:"aff_history_quota"`
	AffCount    int    `json:"aff_count"`
	InviterID   int    `json:"inviter_id"`
}

// Login 登录认证。
//
// New-api has two intentionally different credential paths:
//   - a system access token/PAT is sent as Authorization: Bearer <token>;
//   - a username/password login returns a short-lived access_token (and, on
//     current versions, a refresh cookie).
//
// The UI stores a token in Credentials.AccessToken, so token mode must not be
// routed through the username/password-only request.
func (a *Adapter) Login(ctx context.Context, cred adapter.Credentials) (adapter.AuthState, error) {
	if token := credentialToken(cred); token != "" {
		st, err := a.loginWithToken(ctx, token, cred)
		if err == nil || cred.AuthMode != "hybrid" || cred.Username == "" || cred.Password == "" {
			return st, err
		}
		// hybrid explicitly permits falling back to a password login when the
		// long-lived token has expired/revoked.
	}

	if cred.Username == "" || cred.Password == "" {
		return adapter.AuthState{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少用户名或密码", nil)
	}
	var env apiEnvelope
	code, _, err := a.hc.DoJSON(ctx, "POST", a.endpoint("/api/user/login"),
		loginReq{Username: cred.Username, Password: cred.Password}, &env)
	if err != nil {
		return adapter.AuthState{}, adapter.NewErr(adapter.CodeUpstreamError, "登录请求失败", err)
	}
	if code == http.StatusTooManyRequests {
		return stateErr(adapter.StateCFBlocked, "登录被限流（429）")
	}
	if code != http.StatusOK {
		return stateErr(adapter.StateLoginFailed, fmt.Sprintf("登录失败，状态码 %d", code))
	}
	if !env.Success {
		// Turnstile/2FA 场景：message 通常带提示，M2 统一归类 login_failed，turnstile 单列
		if containsAny(env.Message, "turnstile", "验证码", "captcha") {
			return stateErr(adapter.StateTurnstileBlocked, env.Message)
		}
		msg := env.Message
		if msg == "" {
			msg = "登录失败"
		}
		return stateErr(adapter.StateLoginFailed, msg)
	}

	// Current new-api returns data.access_token.  Keep parsing permissive for
	// older forks which returned data.token or a bare string.
	var ld loginData
	if len(env.Data) > 0 && string(env.Data) != "null" {
		if err := json.Unmarshal(env.Data, &ld); err != nil {
			var bare string
			if json.Unmarshal(env.Data, &bare) == nil {
				ld.AccessToken = bare
			}
		}
	}
	access := ld.AccessToken
	if access == "" {
		access = ld.Token
	}
	if access == "" {
		// A successful password response without a usable access token cannot
		// be maintained by this non-browser client (the refresh cookie is not
		// exposed through the adapter contract yet).
		return stateErr(adapter.StateLoginFailed, "登录成功但响应未返回 access_token")
	}
	st := adapter.AuthState{
		State:        adapter.StateOK,
		Message:      "登录成功",
		AccessToken:  access,
		RefreshToken: ld.RefreshToken,
		ExpiresHint:  loginExpiryHint(ld),
	}
	return st, nil
}

// Refresh new-api JWT 走 cookie 刷新（M2 骨架：PAT 首选，JWT refresh 留 M4）。
func (a *Adapter) Refresh(ctx context.Context, cred adapter.Credentials) (adapter.AuthState, error) {
	return adapter.AuthState{}, adapter.NewErr(adapter.CodeUnsupported,
		"new-api 首选 PAT 长期有效；JWT cookie 刷新 M4 接入", nil)
}

// Verify GET /api/user/self 轻量校验（PAT 或会话 cookie）。
func (a *Adapter) Verify(ctx context.Context, atx adapter.AuthCtx) error {
	token := authToken(atx)
	if token == "" && len(atx.Cookies) == 0 {
		return adapter.NewErr(adapter.CodeUnauthorized, "缺少 PAT/token/cookie", nil)
	}
	var env apiEnvelope
	code, _, err := a.hc.DoJSONWithHeader(ctx, "GET", a.endpoint("/api/user/self"), nil, &env, authHeaders(atx))
	if err != nil {
		return adapter.NewErr(adapter.CodeUpstreamError, "校验请求失败", err)
	}
	switch {
	case code == http.StatusOK && env.Success:
		return nil
	case code == http.StatusUnauthorized ||
		(code == http.StatusOK && !env.Success && containsAny(env.Message, "无权", "未登录", "登录", "unauthorized", "invalid token")):
		return adapter.NewErr(adapter.CodeUnauthorized, "登录态无效", nil)
	case code == http.StatusForbidden:
		return adapter.NewErr(adapter.CodeForbidden, "无权限", nil)
	default:
		msg := env.Message
		if msg == "" {
			msg = fmt.Sprintf("状态码 %d", code)
		}
		return adapter.NewErr(adapter.CodeUpstreamError, "校验失败："+msg, nil)
	}
}

func (a *Adapter) loginWithToken(ctx context.Context, token string, cred adapter.Credentials) (adapter.AuthState, error) {
	err := a.Verify(ctx, adapter.AuthCtx{PAT: token})
	if err != nil {
		state := adapter.StateLoginFailed
		if adapter.IsCode(err, adapter.CodeUnauthorized) {
			state = adapter.StateTokenExpired
		}
		return adapter.AuthState{State: state, Message: err.Error()}, err
	}
	st := adapter.AuthState{State: adapter.StateOK, Message: "Token 验证成功"}
	if cred.PAT != "" {
		st.PAT = token
	} else {
		st.AccessToken = token
	}
	return st, nil
}

func credentialToken(cred adapter.Credentials) string {
	if strings.TrimSpace(cred.PAT) != "" {
		return normalizeToken(cred.PAT)
	}
	return normalizeToken(cred.AccessToken)
}

func normalizeToken(token string) string {
	token = strings.TrimSpace(token)
	parts := strings.Fields(token)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return token
}

func authToken(atx adapter.AuthCtx) string {
	if atx.PAT != "" {
		return normalizeToken(atx.PAT)
	}
	return normalizeToken(atx.AccessToken)
}

func hasAuth(atx adapter.AuthCtx) bool {
	if authToken(atx) != "" {
		return true
	}
	for _, c := range atx.Cookies {
		if c != nil && strings.TrimSpace(c.Name) != "" {
			return true
		}
	}
	return false
}

func authHeaders(atx adapter.AuthCtx) map[string]string {
	headers := make(map[string]string, 2)
	if token := authToken(atx); token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	values := make([]string, 0, len(atx.Cookies))
	for _, c := range atx.Cookies {
		if c != nil && strings.TrimSpace(c.Name) != "" {
			values = append(values, c.Name+"="+c.Value)
		}
	}
	if len(values) > 0 {
		headers["Cookie"] = strings.Join(values, "; ")
	}
	return headers
}

func loginExpiryHint(ld loginData) *time.Time {
	var unix int64
	switch {
	case ld.AccessExpiresAt > 0:
		unix = ld.AccessExpiresAt
	case ld.ExpiresAt > 0:
		unix = ld.ExpiresAt
	}
	if unix > 0 {
		t := time.Unix(unix, 0)
		return &t
	}
	if ld.ExpiresIn > 0 {
		t := now().Add(time.Duration(ld.ExpiresIn) * time.Second)
		return &t
	}
	return nil
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if sub != "" && containsFold(s, sub) {
			return true
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	n := len(sub)
	if n == 0 {
		return true
	}
	for i := 0; i+n <= len(s); i++ {
		if toLowerByte(s[i]) == toLowerByte(sub[0]) && equalFold(s[i:i+n], sub) {
			return true
		}
	}
	return false
}

func toLowerByte(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if toLowerByte(a[i]) != toLowerByte(b[i]) {
			return false
		}
	}
	return true
}

func stateErr(state, msg string) (adapter.AuthState, error) {
	return adapter.AuthState{State: state, Message: msg}, fmt.Errorf("newapi: %s", msg)
}

// now 便于测试注入（M2 占位）。
var now = time.Now
