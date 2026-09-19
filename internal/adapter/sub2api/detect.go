// Package sub2api 站点识别（公开端点特征，architecture §4.4 配合打分器）。
package sub2api

import (
	"context"
	"encoding/json"

	"aiclient/internal/adapter"
)

// detectSetupStatus GET /setup/status 特征。
func (a *Adapter) detectSetupStatus(ctx context.Context) (bool, map[string]any) {
	var out map[string]any
	status, _, err := a.hc.DoJSON(ctx, "GET", a.endpoint("/setup/status"), nil, &out)
	if err != nil || status != 200 {
		return false, nil
	}
	return true, out
}

// detectSettingsPublic GET /api/v1/settings/public 特征。
func (a *Adapter) detectSettingsPublic(ctx context.Context) (bool, map[string]any) {
	var out map[string]any
	status, _, err := a.hc.DoJSON(ctx, "GET", a.endpoint("/api/v1/settings/public"), nil, &out)
	if err != nil || status != 200 {
		return false, nil
	}
	return true, out
}

// Detect 实现接口：任一特征命中即给出证据（打分器综合判定）。
func (a *Adapter) Detect(ctx context.Context, baseURL string) (adapter.DetectResult, error) {
	ad, err := New(baseURL, a.proxyURL, a.log) // 探测目标以传入 baseURL 为准
	if err != nil {
		return adapter.DetectResult{}, err
	}
	res := adapter.DetectResult{Type: adapter.TypeSub2API, Evidence: map[string]any{}}

	if ok, body := ad.detectSetupStatus(ctx); ok {
		res.Score += 4
		res.Evidence["setup_status"] = true
		res.Evidence["setup_status_body"] = maskKeys(body)
	}
	if ok, body := ad.detectSettingsPublic(ctx); ok {
		res.Score += 4
		res.Evidence["settings_public"] = true
		res.Evidence["settings_public_body"] = maskKeys(body)
	}
	if res.Score > 0 {
		res.FinalURL = ad.BaseURL()
	}
	return res, nil
}

// maskKeys 截断证据体积（快照脱敏存储）。
func maskKeys(body map[string]any) map[string]any {
	if body == nil {
		return nil
	}
	b, _ := json.Marshal(body)
	if len(b) > 2048 {
		return map[string]any{"truncated": true, "head": string(b[:2048])}
	}
	return body
}
