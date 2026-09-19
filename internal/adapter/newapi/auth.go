// Package newapi 站点识别与认证（architecture §4.2 契约映射）。
package newapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	Start_time int64  `json:"start_time"`
}

// Detect GET /api/status 特征：success + version/system_name 命中即给证据。
func (a *Adapter) Detect(ctx context.Context, baseURL string) (adapter.DetectResult, error) {
	ad, err := New(baseURL, a.log) // 探测目标以传入 baseURL 为准
	if err != nil {
		return adapter.DetectResult{}, err
	}
	res := adapter.DetectResult{Type: adapter.TypeNewAPI, Evidence: map[string]any{}}

	var st statusResp
	code, _, err := ad.hc.DoJSON(ctx, "GET", ad.endpoint("/api/status"), nil, &st)
	if err == nil && code == http.StatusOK && (st.Version != "" || st.SystemName != "") {
		res.Score += 8
		res.Evidence["api_status"] = true
		res.Evidence["version"] = st.Version
		res.Evidence["system_name"] = st.SystemName
		res.FinalURL = ad.BaseURL()
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

// Login 用户名密码登录（成功后 new-api 会种 new_api_* cookie；M2 记录 cookie 供后续请求）。
func (a *Adapter) Login(ctx context.Context, cred adapter.Credentials) (adapter.AuthState, error) {
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
		return stateErr(adapter.StateLoginFailed, env.Message)
	}
	var self userSelf
	_ = json.Unmarshal(env.Data, &self) // 宽松：data 可能为 null
	st := adapter.AuthState{State: adapter.StateOK, Message: fmt.Sprintf("登录成功：user_id=%d", self.ID)}
	return st, nil
}

// Refresh new-api JWT 走 cookie 刷新（M2 骨架：PAT 首选，JWT refresh 留 M4）。
func (a *Adapter) Refresh(ctx context.Context, cred adapter.Credentials) (adapter.AuthState, error) {
	return adapter.AuthState{}, adapter.NewErr(adapter.CodeUnsupported,
		"new-api 首选 PAT 长期有效；JWT cookie 刷新 M4 接入", nil)
}

// Verify GET /api/user/self 轻量校验（PAT 或会话 cookie）。
func (a *Adapter) Verify(ctx context.Context, atx adapter.AuthCtx) error {
	token := atx.PAT
	if token == "" {
		token = atx.AccessToken
	}
	if token == "" && len(atx.Cookies) == 0 {
		return adapter.NewErr(adapter.CodeUnauthorized, "缺少 PAT/token/cookie", nil)
	}
	var env apiEnvelope
	code, _, err := a.hc.DoJSON(ctx, "GET", a.endpoint("/api/user/self"), nil, &env)
	if err != nil {
		return adapter.NewErr(adapter.CodeUpstreamError, "校验请求失败", err)
	}
	switch {
	case code == http.StatusOK && env.Success:
		return nil
	case code == http.StatusUnauthorized || (code == http.StatusOK && !env.Success && containsAny(env.Message, "无权", "login")):
		return adapter.NewErr(adapter.CodeUnauthorized, "登录态无效", nil)
	case code == http.StatusForbidden:
		return adapter.NewErr(adapter.CodeForbidden, "无权限", nil)
	default:
		return adapter.NewErr(adapter.CodeUpstreamError, fmt.Sprintf("校验失败，状态码 %d", code), nil)
	}
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
