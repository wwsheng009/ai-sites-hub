// Package sub2api 数据同步：keys / groups / quota / affiliate（FR-4、FR-10）。
package sub2api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"aiclient/internal/adapter"
)

// keyDTO GET /api/v1/keys 列表项（真实部署结构：额度在 key 上，分组是嵌套对象）。
type keyDTO struct {
	ID        int64          `json:"id"`
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	GroupID   int64          `json:"group_id"`
	Group     *keyGroupInner `json:"group"`
	Quota     *float64       `json:"quota"`
	QuotaUsed *float64       `json:"quota_used"`
	ExpiresAt *string        `json:"expires_at"`
	CreatedAt string         `json:"created_at"`
}

// keyGroupInner key 内嵌分组摘要。
type keyGroupInner struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// keysData GET /api/v1/keys 响应 data（分页）。
type keysData struct {
	Items    []keyDTO `json:"items"`
	Total    int64    `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}

// groupDTO 可绑分组（真实字段：rate_multiplier 倍率，status 判可用）。
type groupDTO struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	RateMultiplier float64 `json:"rate_multiplier"`
	Status         string  `json:"status"`
}

// affData GET /api/v1/user/aff 响应 data。
type affData struct {
	AffCode        string   `json:"aff_code"`
	AffCount       int      `json:"aff_count"`
	AffFrozenQuota *float64 `json:"aff_frozen_quota"`
	AffHistory     *float64 `json:"aff_history_quota"`
	RebatePercent  *float64 `json:"effective_rebate_rate_percent"`
}

// ListKeys 分页拉取 key 列表（单页封顶 500；Bearer 认证）。
func (a *Adapter) ListKeys(ctx context.Context, atx adapter.AuthCtx, page adapter.Page) (adapter.KeyPage, error) {
	if atx.AccessToken == "" {
		return adapter.KeyPage{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少 access token", nil)
	}
	var env apiEnvelope
	url := fmt.Sprintf("%s/api/v1/keys?page=%d&page_size=%d", a.BaseURL(), maxInt(page.Page, 1), clampSize(page.Size))
	status, _, err := a.hc.DoJSONWithHeader(ctx, "GET", url, nil, &env, authHeaders(atx.AccessToken))
	if err != nil {
		return adapter.KeyPage{}, adapter.NewErr(adapter.CodeUpstreamError, "拉取 key 列表失败", err)
	}
	if status != http.StatusOK || env.Code != 0 {
		return adapter.KeyPage{}, statusErr(status, "拉取 key 列表")
	}
	var out keysData
	_ = json.Unmarshal(env.Data, &out)
	items := make([]adapter.SiteKey, 0, len(out.Items))
	for _, k := range out.Items {
		groupName := ""
		if k.Group != nil {
			groupName = k.Group.Name
		}
		var expiresAt *time.Time
		if k.ExpiresAt != nil && *k.ExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, *k.ExpiresAt); err == nil {
				expiresAt = &t
			}
		}
		items = append(items, adapter.SiteKey{
			RemoteKeyID:    strconv.FormatInt(k.ID, 10),
			Name:           k.Name,
			Group:          groupName,
			Status:         k.Status,
			QuotaLimit:     k.Quota,
			QuotaUsed:      k.QuotaUsed,
			QuotaRemaining: subF(k.Quota, k.QuotaUsed),
			Unlimited:      k.Quota == nil || *k.Quota == 0,
			ExpiresAt:      expiresAt,
			KeySource:      "plaintext",
		})
	}
	return adapter.KeyPage{Items: items, Total: int(out.Total), Page: out.Page, HasNext: false}, nil
}

// ListGroups 用户可绑分组（Bearer 认证；available=status==active）。
func (a *Adapter) ListGroups(ctx context.Context, atx adapter.AuthCtx) ([]adapter.Group, error) {
	if atx.AccessToken == "" {
		return nil, adapter.NewErr(adapter.CodeUnauthorized, "缺少 access token", nil)
	}
	var env apiEnvelope
	status, _, err := a.hc.DoJSONWithHeader(ctx, "GET", a.endpoint("/api/v1/groups/available"), nil, &env, authHeaders(atx.AccessToken))
	if err != nil {
		return nil, adapter.NewErr(adapter.CodeUpstreamError, "拉取分组失败", err)
	}
	if status != http.StatusOK || env.Code != 0 {
		return nil, statusErr(status, "拉取分组")
	}
	var out []groupDTO
	_ = json.Unmarshal(env.Data, &out)
	groups := make([]adapter.Group, 0, len(out))
	for _, g := range out {
		groups = append(groups, adapter.Group{
			RemoteGroupID: strconv.FormatInt(g.ID, 10),
			Name:          g.Name,
			Ratio:         g.RateMultiplier,
			Available:     g.Status == "active",
			Desc:          g.Description,
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

// AffiliateInfo 返利概览（FR-10.1；rebate_rate 以小数存储，available 暂无对应端点记 NULL）。
func (a *Adapter) AffiliateInfo(ctx context.Context, atx adapter.AuthCtx) (adapter.AffiliateInfo, error) {
	if atx.AccessToken == "" {
		return adapter.AffiliateInfo{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少 access token", nil)
	}
	var env apiEnvelope
	status, _, err := a.hc.DoJSONWithHeader(ctx, "GET", a.endpoint("/api/v1/user/aff"), nil, &env, authHeaders(atx.AccessToken))
	if err != nil {
		return adapter.AffiliateInfo{}, adapter.NewErr(adapter.CodeUpstreamError, "拉取返利信息失败", err)
	}
	if status != http.StatusOK || env.Code != 0 {
		return adapter.AffiliateInfo{}, statusErr(status, "拉取返利信息")
	}
	var out affData
	_ = json.Unmarshal(env.Data, &out)
	var rebate *float64
	if out.RebatePercent != nil {
		v := *out.RebatePercent / 100
		rebate = &v
	}
	count := out.AffCount
	return adapter.AffiliateInfo{
		AffCode:      out.AffCode,
		RebateRate:   rebate,
		Frozen:       out.AffFrozenQuota,
		History:      out.AffHistory,
		InviteeCount: &count,
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
	var env apiEnvelope
	status, _, err := a.hc.DoJSONWithHeader(ctx, "POST", a.endpoint("/api/v1/user/aff/transfer"),
		transferReq{Amount: amount.Value}, &env, authHeaders(atx.AccessToken))
	if err != nil {
		return adapter.TransferResult{}, adapter.NewErr(adapter.CodeUpstreamError, "划转请求失败", err)
	}
	if status != http.StatusOK || env.Code != 0 {
		return adapter.TransferResult{}, statusErr(status, "划转")
	}
	return adapter.TransferResult{Amount: amount.Value, Currency: "USD", UnitNote: "站点余额单位", Message: env.Message}, nil
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

// subF a-b（nil 透传 nil，用于 剩余=quota-quota_used）。
func subF(a, b *float64) *float64 {
	if a == nil {
		return nil
	}
	v := *a
	if b != nil {
		v -= *b
	}
	return &v
}

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
