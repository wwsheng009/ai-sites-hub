// Package sub2api 数据同步：keys / groups / quota / affiliate（FR-4、FR-10）。
package sub2api

import (
	"context"
	"fmt"
	"net/http"

	"aiclient/internal/adapter"
)

// keyDTO GET /api/v1/keys 列表项（额度字段在 key 对象内）。
type keyDTO struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Group       string  `json:"group"`
	Status      string  `json:"status"`
	QuotaLimit  float64 `json:"quota_limit"`
	QuotaUsed   float64 `json:"quota_used"`
	QuotaRemain float64 `json:"quota_remaining"`
	Unlimited   bool    `json:"unlimited_quota"`
	ExpiresAt   string  `json:"expired_at"`
	CreatedAt   string  `json:"created_at"`
}

// keysPage 分页结构（data 内 list/total，结构以实际部署为准，M2 宽松解析）。
type keysPage struct {
	Items    []keyDTO `json:"items"`
	Total    int64    `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}

// groupDTO 可绑分组。
type groupDTO struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Ratio     float64 `json:"ratio"`
	Available bool    `json:"available"`
	Desc      string  `json:"description"`
}

// ListKeys 分页拉取 key 列表（M2 单页版：一次拉取，封顶 500）。
func (a *Adapter) ListKeys(ctx context.Context, atx adapter.AuthCtx, page adapter.Page) (adapter.KeyPage, error) {
	if atx.AccessToken == "" {
		return adapter.KeyPage{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少 access token", nil)
	}
	var out keysPage
	url := fmt.Sprintf("%s/api/v1/keys?page=%d&page_size=%d", a.BaseURL(), maxInt(page.Page, 1), clampSize(page.Size))
	status, _, err := a.hc.DoJSON(ctx, "GET", url, nil, &out)
	if err != nil {
		return adapter.KeyPage{}, adapter.NewErr(adapter.CodeUpstreamError, "拉取 key 列表失败", err)
	}
	if status != http.StatusOK {
		return adapter.KeyPage{}, statusErr(status, "拉取 key 列表")
	}
	items := make([]adapter.SiteKey, 0, len(out.Items))
	for _, k := range out.Items {
		limit := k.QuotaLimit
		used := k.QuotaUsed
		rem := k.QuotaRemain
		items = append(items, adapter.SiteKey{
			RemoteKeyID:    k.ID,
			Name:           k.Name,
			Group:          k.Group,
			Status:         k.Status,
			QuotaLimit:     ptrF(limit),
			QuotaUsed:      ptrF(used),
			QuotaRemaining: ptrF(rem),
			Unlimited:      k.Unlimited,
			KeySource:      "plaintext",
		})
	}
	return adapter.KeyPage{Items: items, Total: int(out.Total), Page: out.Page, HasNext: false}, nil
}

// ListGroups 用户可绑分组。
func (a *Adapter) ListGroups(ctx context.Context, atx adapter.AuthCtx) ([]adapter.Group, error) {
	if atx.AccessToken == "" {
		return nil, adapter.NewErr(adapter.CodeUnauthorized, "缺少 access token", nil)
	}
	var out []groupDTO
	status, _, err := a.hc.DoJSON(ctx, "GET", a.endpoint("/api/v1/groups/available"), nil, &out)
	if err != nil {
		return nil, adapter.NewErr(adapter.CodeUpstreamError, "拉取分组失败", err)
	}
	if status != http.StatusOK {
		return nil, statusErr(status, "拉取分组")
	}
	groups := make([]adapter.Group, 0, len(out))
	for _, g := range out {
		groups = append(groups, adapter.Group{
			RemoteGroupID: g.ID,
			Name:          g.Name,
			Ratio:         g.Ratio,
			Available:     g.Available,
			Desc:          g.Desc,
		})
	}
	return groups, nil
}

// Quota 账号级额度（M2：无直接端点契约，返回 has_*=false 占位，作业层按能力跳过展示）。
func (a *Adapter) Quota(ctx context.Context, atx adapter.AuthCtx) (adapter.AccountQuota, error) {
	if atx.AccessToken == "" {
		return adapter.AccountQuota{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少 access token", nil)
	}
	return adapter.AccountQuota{Currency: "USD", UnitNote: "sub2api 账号额度端点待 M4 接入"}, nil
}

// affResp GET /api/v1/user/aff 响应（字段宽松解析，缺失记 NULL）。
type affResp struct {
	AffCode      string   `json:"aff_code"`
	RebateRate   *float64 `json:"rebate_rate"`
	Available    *float64 `json:"available"`
	Frozen       *float64 `json:"frozen"`
	History      *float64 `json:"history"`
	InviteeCount *int     `json:"invitee_count"`
}

// AffiliateInfo 返利概览（FR-10.1）。
func (a *Adapter) AffiliateInfo(ctx context.Context, atx adapter.AuthCtx) (adapter.AffiliateInfo, error) {
	if atx.AccessToken == "" {
		return adapter.AffiliateInfo{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少 access token", nil)
	}
	var out affResp
	status, _, err := a.hc.DoJSON(ctx, "GET", a.endpoint("/api/v1/user/aff"), nil, &out)
	if err != nil {
		return adapter.AffiliateInfo{}, adapter.NewErr(adapter.CodeUpstreamError, "拉取返利信息失败", err)
	}
	if status != http.StatusOK {
		return adapter.AffiliateInfo{}, statusErr(status, "拉取返利信息")
	}
	return adapter.AffiliateInfo{
		AffCode:      out.AffCode,
		RebateRate:   out.RebateRate,
		Available:    out.Available,
		Frozen:       out.Frozen,
		History:      out.History,
		InviteeCount: out.InviteeCount,
		Currency:     "USD",
		UnitNote:     "站点余额单位",
	}, nil
}

// transferReq POST /api/v1/user/aff/transfer 请求体。
type transferReq struct {
	Amount float64 `json:"amount"`
}

// TransferAffiliate 手动/自动划转（不可逆；调用方负责 dry_run 与频控）。
func (a *Adapter) TransferAffiliate(ctx context.Context, atx adapter.AuthCtx, amount adapter.Amount) (adapter.TransferResult, error) {
	if atx.AccessToken == "" {
		return adapter.TransferResult{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少 access token", nil)
	}
	if amount.Value <= 0 {
		return adapter.TransferResult{}, adapter.NewErr(adapter.CodeUpstreamError, "划转金额必须大于 0", nil)
	}
	var out map[string]any
	status, _, err := a.hc.DoJSON(ctx, "POST", a.endpoint("/api/v1/user/aff/transfer"),
		transferReq{Amount: amount.Value}, &out)
	if err != nil {
		return adapter.TransferResult{}, adapter.NewErr(adapter.CodeUpstreamError, "划转请求失败", err)
	}
	if status != http.StatusOK {
		return adapter.TransferResult{}, statusErr(status, "划转")
	}
	msg, _ := out["message"].(string)
	return adapter.TransferResult{Amount: amount.Value, Currency: "USD", UnitNote: "站点余额单位", Message: msg}, nil
}

// CheckinStatus / Checkin：sub2api 不支持签到（Capabilities.Checkin=unsupported）。
func (a *Adapter) CheckinStatus(ctx context.Context, atx adapter.AuthCtx) (adapter.CheckinStatusResult, error) {
	return adapter.CheckinStatusResult{State: adapter.CheckinUnsupported,
		Message: "sub2api 无签到能力"}, &adapter.ErrUnsupported{Capability: "checkin"}
}

func (a *Adapter) Checkin(ctx context.Context, atx adapter.AuthCtx) (adapter.CheckinResult, error) {
	return adapter.CheckinResult{State: adapter.CheckinUnsupported,
		Message: "sub2api 无签到能力"}, &adapter.ErrUnsupported{Capability: "checkin"}
}

// ---- 共用小工具 ----

func ptrF(f float64) *float64 { return &f }

func clampSize(size int) int {
	if size <= 0 {
		return 100
	}
	if size > 500 {
		return 500
	}
	return size
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func statusErr(status int, action string) error {
	switch status {
	case http.StatusUnauthorized:
		return adapter.NewErr(adapter.CodeUnauthorized, fmt.Sprintf("%s：未授权（401）", action), nil)
	case http.StatusForbidden:
		return adapter.NewErr(adapter.CodeForbidden, fmt.Sprintf("%s：无权限（403）", action), nil)
	case http.StatusTooManyRequests:
		return adapter.NewErr(adapter.CodeRateLimited, fmt.Sprintf("%s：限流（429）", action), nil)
	default:
		return adapter.NewErr(adapter.CodeUpstreamError, fmt.Sprintf("%s：状态码 %d", action, status), nil)
	}
}
