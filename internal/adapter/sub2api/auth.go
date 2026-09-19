// Package sub2api 认证：login / refresh / verify（architecture §4.2）。
package sub2api

import (
	"context"
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

// tokenPair 响应 token 对。
type tokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// authMeResp GET /api/v1/auth/me 响应 data。
type authMeResp struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	RunMode  string `json:"run_mode"`
}

// Login 用户名密码登录（2FA/turnstile 场景返回对应状态，M2 先按 plain JSON 处理）。
func (a *Adapter) Login(ctx context.Context, cred adapter.Credentials) (adapter.AuthState, error) {
	if cred.Username == "" || cred.Password == "" {
		return adapter.AuthState{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少用户名或密码", nil)
	}
	var pair tokenPair
	status, _, err := a.hc.DoJSON(ctx, "POST", a.endpoint("/api/v1/auth/login"),
		loginReq{Email: cred.Username, Password: cred.Password}, &pair)
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
	var pair tokenPair
	status, _, err := a.hc.DoJSON(ctx, "POST", a.endpoint("/api/v1/auth/refresh"),
		map[string]string{"refresh_token": cred.RefreshToken}, &pair)
	if err != nil {
		return adapter.AuthState{}, adapter.NewErr(adapter.CodeUpstreamError, "刷新请求失败", err)
	}
	if status != http.StatusOK || pair.AccessToken == "" {
		return stateErr(adapter.StateRefreshFailed, fmt.Sprintf("刷新失败，状态码 %d", status))
	}
	st := adapter.AuthState{State: adapter.StateOK, AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken}
	if pair.ExpiresIn > 0 {
		hint := time.Now().Add(time.Duration(pair.ExpiresIn) * time.Second)
		st.ExpiresHint = &hint
	}
	return st, nil
}

// Verify GET /api/v1/auth/me 轻量校验。
func (a *Adapter) Verify(ctx context.Context, atx adapter.AuthCtx) error {
	if atx.AccessToken == "" {
		return adapter.NewErr(adapter.CodeUnauthorized, "缺少 access token", nil)
	}
	var me authMeResp
	status, _, err := a.hc.DoJSON(ctx, "GET", a.endpoint("/api/v1/auth/me"), nil, &me)
	if err != nil {
		return adapter.NewErr(adapter.CodeUpstreamError, "校验请求失败", err)
	}
	switch status {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return adapter.NewErr(adapter.CodeUnauthorized, "token 无效或过期", nil)
	case http.StatusForbidden:
		return adapter.NewErr(adapter.CodeForbidden, "无权限", nil)
	default:
		return adapter.NewErr(adapter.CodeUpstreamError, fmt.Sprintf("校验失败，状态码 %d", status), nil)
	}
}

// authHeader 为请求注入 Bearer。
func authHeader(h http.Header, token string) {
	h.Set("Authorization", "Bearer "+token)
}

// stateErr 构建带状态的消息错误（认证状态机落库用）。
func stateErr(state, msg string) (adapter.AuthState, error) {
	st := adapter.AuthState{State: state, Message: msg}
	return st, fmt.Errorf("sub2api: %s", msg)
}
