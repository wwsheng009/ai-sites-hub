// Package newapi 返利（FR-10）与签到（FR-3）。
package newapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"aiclient/internal/adapter"
)

// ---- Affiliate（FR-10.1/10.3/10.4）----

// AffiliateInfo GET /api/user/aff（邀请码）+ GET /api/user/self（aff_* 字段）。
// 归一化：quota 原始单位 → USD（500000=1USD）；rate 上游缺失记 NULL。
func (a *Adapter) AffiliateInfo(ctx context.Context, atx adapter.AuthCtx) (adapter.AffiliateInfo, error) {
	if atx.PAT == "" && atx.AccessToken == "" {
		return adapter.AffiliateInfo{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少 PAT/token", nil)
	}

	var info adapter.AffiliateInfo
	info.Currency = "USD"
	info.UnitNote = "new-api quota 原始单位 500000=1USD，已换算展示"

	// 1) /api/user/self 提供 aff_quota / aff_history_quota / aff_count
	self, err := a.fetchSelf(ctx, atx)
	if err != nil {
		return adapter.AffiliateInfo{}, err
	}
	avail := quotaToUSD(self.AffQuota)
	hist := quotaToUSD(self.AffHistory)
	info.Available = &avail
	info.History = &hist
	info.InviteeCount = &self.AffCount

	// 2) /api/user/aff 提供邀请码（结构与版本相关，宽松解析）
	env, code, err := a.apiGet(ctx, atx, a.endpoint("/api/user/aff"))
	if err == nil && code == http.StatusOK && env.Success && len(env.Data) > 0 {
		var aff struct {
			AffCode   string `json:"aff_code"`
			InviteURL string `json:"invite_url"`
		}
		if err := json.Unmarshal(env.Data, &aff); err == nil && aff.AffCode != "" {
			info.AffCode = aff.AffCode
		} else if err == nil && aff.InviteURL != "" {
			info.AffCode = aff.InviteURL
		} else {
			// data 可能是纯字符串邀请码
			var code2 string
			if json.Unmarshal(env.Data, &code2) == nil {
				info.AffCode = code2
			}
		}
	}
	// aff 端点失败不阻塞概览（self 已有余额数据）；rate 上游无契约 → 不填
	return info, nil
}

// TransferAffiliate POST /api/user/aff_transfer，body {quota}（上游原始 quota 单位）。
// 不可逆；调用方负责 dry_run 与频控。
func (a *Adapter) TransferAffiliate(ctx context.Context, atx adapter.AuthCtx, amount adapter.Amount) (adapter.TransferResult, error) {
	if atx.PAT == "" && atx.AccessToken == "" {
		return adapter.TransferResult{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少 PAT/token", nil)
	}
	if amount.Value <= 0 {
		return adapter.TransferResult{}, adapter.NewErr(adapter.CodeUpstreamError, "划转金额必须大于 0", nil)
	}

	// 统一入参 USD → 原始 quota（quasi 精度；上游整数 quota）
	quota := usdToQuota(amount.Value)
	if quota <= 0 {
		return adapter.TransferResult{}, adapter.NewErr(adapter.CodeUpstreamError, "金额过小，换算后 quota 为 0", nil)
	}

	// 划转前余额（留证 amount_before）
	var before float64
	if self, err := a.fetchSelf(ctx, atx); err == nil {
		before = quotaToUSD(self.AffQuota)
	}

	var env apiEnvelope
	code, _, err := a.hc.DoJSONWithHeader(ctx, "POST", a.endpoint("/api/user/aff_transfer"),
		map[string]int64{"quota": quota}, &env, map[string]string{
			"Authorization": "Bearer " + pickToken(atx),
		})
	if err != nil {
		return adapter.TransferResult{}, adapter.NewErr(adapter.CodeUpstreamError, "划转请求失败", err)
	}
	if code != http.StatusOK || !env.Success {
		return adapter.TransferResult{}, apiErrResp(code, env.Message, "划转")
	}
	return adapter.TransferResult{
		Amount:       amount.Value,
		AmountBefore: before,
		Currency:     "USD",
		UnitNote:     "new-api quota 原始单位 500000=1USD，已换算展示",
		Message:      env.Message,
	}, nil
}

// ---- Checkin（FR-3）----

// checkinStats GET /api/user/checkin 响应。
type checkinStats struct {
	Enabled  bool           `json:"enabled"`
	MinQuota int64          `json:"min_quota"`
	MaxQuota int64          `json:"max_quota"`
	Stats    map[string]any `json:"stats"`
}

// CheckinStatus 查询签到能力/今日状态。
func (a *Adapter) CheckinStatus(ctx context.Context, atx adapter.AuthCtx) (adapter.CheckinStatusResult, error) {
	if atx.PAT == "" && atx.AccessToken == "" {
		return adapter.CheckinStatusResult{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少 PAT/token", nil)
	}
	env, code, err := a.apiGet(ctx, atx, a.endpoint("/api/user/checkin"))
	if err != nil {
		return adapter.CheckinStatusResult{}, adapter.NewErr(adapter.CodeUpstreamError, "查询签到状态失败", err)
	}
	if code != http.StatusOK || !env.Success {
		return adapter.CheckinStatusResult{}, apiErrResp(code, env.Message, "查询签到状态")
	}
	var stats checkinStats
	if len(env.Data) > 0 {
		_ = json.Unmarshal(env.Data, &stats)
	}
	if !stats.Enabled {
		return adapter.CheckinStatusResult{State: adapter.CheckinUnsupported, Message: "站点未开启签到"}, nil
	}
	return adapter.CheckinStatusResult{State: adapter.CheckinSuccess, Message: "签到功能已开启"}, nil
}

// Checkin POST /api/user/checkin 执行签到（Turnstile 场景转人工）。
func (a *Adapter) Checkin(ctx context.Context, atx adapter.AuthCtx) (adapter.CheckinResult, error) {
	if atx.PAT == "" && atx.AccessToken == "" {
		return adapter.CheckinResult{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少 PAT/token", nil)
	}
	var env apiEnvelope
	code, _, err := a.hc.DoJSONWithHeader(ctx, "POST", a.endpoint("/api/user/checkin"), nil, &env,
		map[string]string{"Authorization": "Bearer " + pickToken(atx)})
	if err != nil {
		return adapter.CheckinResult{}, adapter.NewErr(adapter.CodeUpstreamError, "签到请求失败", err)
	}
	if containsAny(env.Message, "turnstile", "验证码", "captcha") {
		return adapter.CheckinResult{State: adapter.CheckinBlockedByTurnstile, Message: env.Message}, nil
	}
	if code != http.StatusOK || !env.Success {
		if containsAny(env.Message, "已签到", "already") {
			return adapter.CheckinResult{State: adapter.CheckinAlready, Message: env.Message}, nil
		}
		return adapter.CheckinResult{}, apiErrResp(code, env.Message, "签到")
	}
	var data struct {
		QuotaAwarded int64  `json:"quota_awarded"`
		CheckinDate  string `json:"checkin_date"`
	}
	if len(env.Data) > 0 {
		_ = json.Unmarshal(env.Data, &data)
	}
	return adapter.CheckinResult{
		State:        adapter.CheckinSuccess,
		QuotaAwarded: quotaToUSD(data.QuotaAwarded),
		Message:      fmt.Sprintf("签到成功 date=%s", data.CheckinDate),
	}, nil
}

// pickToken PAT 优先。
func pickToken(atx adapter.AuthCtx) string {
	if atx.PAT != "" {
		return atx.PAT
	}
	return atx.AccessToken
}

func apiErrResp(code int, msg, action string) error {
	if msg != "" {
		return adapter.NewErr(adapter.CodeUpstreamError, fmt.Sprintf("%s：%s", action, msg), nil)
	}
	return adapter.NewErr(adapter.CodeUpstreamError, fmt.Sprintf("%s：状态码 %d", action, code), nil)
}
