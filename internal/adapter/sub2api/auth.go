// Package sub2api 认证：login / refresh / verify（architecture §4.2）。
package sub2api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"aiclient/internal/adapter"
)

// loginReq POST /api/v1/auth/login 请求体。
type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// apiEnvelope sub2api 统一响应包装 {code, message, data}（真实部署均为信封结构）。
type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// tokenPair 登录/刷新响应 data 内的 token 对。
type tokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// authMeResp GET /api/v1/auth/me 响应 data。
type authMeResp struct {
	ID      int64  `json:"id"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	Status  string `json:"status"`
	RunMode string `json:"run_mode"`
}

// Login 用户名密码登录（成功后 data 内返回 access_token/refresh_token；2FA/turnstile 场景 M2 先按 plain JSON 处理）。
func (a *Adapter) Login(ctx context.Context, cred adapter.Credentials) (adapter.AuthState, error) {
	if cred.Username == "" || cred.Password == "" {
		return adapter.AuthState{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少用户名或密码", nil)
	}
	var env apiEnvelope
	status, _, err := a.hc.DoJSON(ctx, "POST", a.endpoint("/api/v1/auth/login"),
		loginReq{Email: cred.Username, Password: cred.Password}, &env)
	if err != nil {
		return adapter.AuthState{}, adapter.NewErr(adapter.CodeUpstreamError, "登录请求失败", err)
	}
	if status != http.StatusOK {
		st := adapter.StateLoginFailed
		if status == http.StatusTooManyRequests {
			st = adapter.StateCFBlocked
		}
		return stateErr(st, fmt.Sprintf("登录失败，状态码 %d", status))
	}
	if env.Code != 0 {
		return stateErr(adapter.StateLoginFailed, fmt.Sprintf("登录失败：%s", env.Message))
	}
	var pair tokenPair
	_ = json.Unmarshal(env.Data, &pair) // 宽松：data 可能为 null
	if pair.AccessToken == "" {
		return stateErr(adapter.StateLoginFailed, "响应缺少 access_token")
	}
	st := adapter.AuthState{State: adapter.StateOK, AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken}
	if pair.ExpiresIn > 0 {
		hint := time.Now().Add(time.Duration(pair.ExpiresIn) * time.Second)
		st.ExpiresHint = &hint
	}
	return st, nil
}

// Refresh 用 refresh token 轮换（RT 单次有效，轮换后调用方需重存密文）。
func (a *Adapter) Refresh(ctx context.Context, cred adapter.Credentials) (adapter.AuthState, error) {
	if cred.RefreshToken == "" {
		return adapter.AuthState{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少 refresh token", nil)
	}
	var env apiEnvelope
	status, _, err := a.hc.DoJSON(ctx, "POST", a.endpoint("/api/v1/auth/refresh"),
		map[string]string{"refresh_token": cred.RefreshToken}, &env)
	if err != nil {
		return adapter.AuthState{}, adapter.NewErr(adapter.CodeUpstreamError, "刷新请求失败", err)
	}
	if status != http.StatusOK || env.Code != 0 {
		return stateErr(adapter.StateRefreshFailed, fmt.Sprintf("刷新失败，状态码 %d code %d", status, env.Code))
	}
	var pair tokenPair
	_ = json.Unmarshal(env.Data, &pair)
	if pair.AccessToken == "" {
		return stateErr(adapter.StateRefreshFailed, "刷新响应缺少 access_token")
	}
	st := adapter.AuthState{State: adapter.StateOK, AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken}
	if pair.ExpiresIn > 0 {
		hint := time.Now().Add(time.Duration(pair.ExpiresIn) * time.Second)
		st.ExpiresHint = &hint
	}
	return st, nil
}

// Verify GET /api/v1/auth/me 轻量校验（需携带 Bearer token）。
func (a *Adapter) Verify(ctx context.Context, atx adapter.AuthCtx) error {
	if atx.AccessToken == "" {
		return adapter.NewErr(adapter.CodeUnauthorized, "缺少 access token", nil)
	}
	var env apiEnvelope
	status, _, err := a.hc.DoJSONWithHeader(ctx, "GET", a.endpoint("/api/v1/auth/me"), nil, &env, authHeaders(atx.AccessToken))
	if err != nil {
		return adapter.NewErr(adapter.CodeUpstreamError, "校验请求失败", err)
	}
	switch {
	case status == http.StatusUnauthorized:
		return adapter.NewErr(adapter.CodeUnauthorized, "token 无效或过期", nil)
	case status == http.StatusForbidden:
		return adapter.NewErr(adapter.CodeForbidden, "无权限", nil)
	case status != http.StatusOK || env.Code != 0:
		return adapter.NewErr(adapter.CodeUpstreamError,
			fmt.Sprintf("校验失败：HTTP %d code %d %s", status, env.Code, env.Message), nil)
	}
	return nil
}

// authHeaders 数据请求统一携带 Bearer token（sub2api 所有业务端点都需要认证）。
func authHeaders(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

// stateErr 构建带状态的消息错误（认证状态机落库用）。
func stateErr(state, msg string) (adapter.AuthState, error) {
	st := adapter.AuthState{State: state, Message: msg}
	return st, fmt.Errorf("sub2api: %s", msg)
}
