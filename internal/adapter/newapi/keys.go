// Package newapi 数据同步：keys / groups / quota（architecture §4.2 契约映射）。
package newapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"aiclient/internal/adapter"
)

// tokenDTO GET /api/token/ 列表项（key 打码）。
type tokenDTO struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Key            string `json:"key"` // 打码（sk-***）
	Status         int    `json:"status"`
	Group          string `json:"group"`
	RemainQuota    int64  `json:"remain_quota"`
	UsedQuota      int64  `json:"used_quota"`
	UnlimitedQuota bool   `json:"unlimited_quota"`
	ExpiredTime    int64  `json:"expired_time"` // -1 = 永不过期
	CreatedTime    int64  `json:"created_time"`
}

// tokenPage data 分页（宽松：items+total；老版本可能是数组）。
type tokenPage struct {
	Items []tokenDTO `json:"items"`
	Total int64      `json:"total"`
}

// statusName new-api token 状态映射。
func statusName(s int) string {
	switch s {
	case 1:
		return "active"
	case 2:
		return "disabled"
	case 3:
		return "expired"
	case 4:
		return "exhausted"
	default:
		return "unknown"
	}
}

// ListKeys 分页拉取 token 列表（M2 单页版，封顶 500）。
func (a *Adapter) ListKeys(ctx context.Context, atx adapter.AuthCtx, page adapter.Page) (adapter.KeyPage, error) {
	if atx.PAT == "" && atx.AccessToken == "" {
		return adapter.KeyPage{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少 PAT/token", nil)
	}
	url := fmt.Sprintf("%s/api/token/?p=%d&size=%d", a.BaseURL(), maxInt(page.Page, 1), clampSize(page.Size))
	env, code, err := a.apiGet(ctx, atx, url)
	if err != nil {
		return adapter.KeyPage{}, err
	}
	if code != http.StatusOK || !env.Success {
		return adapter.KeyPage{}, apiErr(env, code, "拉取 token 列表")
	}

	var items []tokenDTO
	var total int64
	if len(env.Data) > 0 {
		var tp tokenPage
		if err := json.Unmarshal(env.Data, &tp); err == nil && tp.Items != nil {
			items, total = tp.Items, tp.Total
		} else {
			_ = json.Unmarshal(env.Data, &items) // 老版本 data 直接是数组
		}
	}

	out := make([]adapter.SiteKey, 0, len(items))
	for _, t := range items {
		rem := float64(t.RemainQuota)
		used := float64(t.UsedQuota)
		item := adapter.SiteKey{
			RemoteKeyID: fmt.Sprintf("%d", t.ID),
			Name:        t.Name,
			Group:       t.Group,
			Status:      statusName(t.Status),
			QuotaUsed:   &used,
			Unlimited:   t.UnlimitedQuota,
			KeySource:   "masked",
		}
		if !t.UnlimitedQuota {
			item.QuotaRemaining = &rem
		}
		if t.ExpiredTime > 0 {
			exp := tsToTime(t.ExpiredTime)
			item.ExpiresAt = &exp
		}
		if t.CreatedTime > 0 {
			created := tsToTime(t.CreatedTime)
			item.CreatedAt = &created
		}
		out = append(out, item)
	}
	return adapter.KeyPage{Items: out, Total: int(total), Page: maxInt(page.Page, 1), HasNext: false}, nil
}

// ListGroups 用户可用分组 + 公开倍率。
func (a *Adapter) ListGroups(ctx context.Context, atx adapter.AuthCtx) ([]adapter.Group, error) {
	if atx.PAT == "" && atx.AccessToken == "" {
		return nil, adapter.NewErr(adapter.CodeUnauthorized, "缺少 PAT/token", nil)
	}

	// 1) 用户可用分组（宽松解析 map[string]float64 或 {"group": "..."} ）
	ratios := map[string]float64{}
	if env, code, err := a.apiGet(ctx, atx, a.endpoint("/api/user/self/groups")); err == nil &&
		code == http.StatusOK && env.Success && len(env.Data) > 0 {
		_ = json.Unmarshal(env.Data, &ratios) // 形如 {"default":1,"vip":0.8}
	}

	// 2) 公开倍率兜底
	if len(ratios) == 0 {
		if env, code, err := a.apiGet(ctx, atx, a.endpoint("/api/ratio_config")); err == nil &&
			code == http.StatusOK && env.Success && len(env.Data) > 0 {
			var wrapper struct {
				GroupRatio map[string]float64 `json:"group_ratio"`
			}
			if err := json.Unmarshal(env.Data, &wrapper); err == nil && wrapper.GroupRatio != nil {
				ratios = wrapper.GroupRatio
			} else {
				_ = json.Unmarshal(env.Data, &ratios)
			}
		}
	}

	groups := make([]adapter.Group, 0, len(ratios))
	for name, ratio := range ratios {
		groups = append(groups, adapter.Group{
			RemoteGroupID: name,
			Name:          name,
			Ratio:         ratio,
			Available:     true,
		})
	}
	return groups, nil
}

// QuotaNew 用户 DTO 额度（quota 原始单位，500000 = 1 USD）。
func (a *Adapter) Quota(ctx context.Context, atx adapter.AuthCtx) (adapter.AccountQuota, error) {
	self, err := a.fetchSelf(ctx, atx)
	if err != nil {
		return adapter.AccountQuota{}, err
	}
	return adapter.AccountQuota{
		Balance:    quotaToUSD(self.Quota),
		Used:       quotaToUSD(self.UsedQuota),
		Currency:   "USD",
		UnitNote:   "new-api quota 原始单位 500000=1USD，已换算展示",
		HasBalance: true,
		HasUsed:    true,
	}, nil
}

// fetchSelf GET /api/user/self。
func (a *Adapter) fetchSelf(ctx context.Context, atx adapter.AuthCtx) (userSelf, error) {
	if atx.PAT == "" && atx.AccessToken == "" {
		return userSelf{}, adapter.NewErr(adapter.CodeUnauthorized, "缺少 PAT/token", nil)
	}
	env, code, err := a.apiGet(ctx, atx, a.endpoint("/api/user/self"))
	if err != nil {
		return userSelf{}, err
	}
	if code == http.StatusUnauthorized {
		return userSelf{}, adapter.NewErr(adapter.CodeUnauthorized, "登录态无效", nil)
	}
	if code != http.StatusOK || !env.Success {
		return userSelf{}, apiErr(env, code, "拉取用户信息")
	}
	var self userSelf
	if err := json.Unmarshal(env.Data, &self); err != nil {
		return userSelf{}, adapter.NewErr(adapter.CodeUpstreamError, "解析用户信息失败", err)
	}
	return self, nil
}

// ---- 共用 ----

// apiGet 带 PAT/token 的 GET（M2 骨架：Authorization 头在请求层注入封装于此）。
func (a *Adapter) apiGet(ctx context.Context, atx adapter.AuthCtx, url string) (apiEnvelope, int, error) {
	token := atx.PAT
	if token == "" {
		token = atx.AccessToken
	}
	var env apiEnvelope
	code, _, err := a.hc.DoJSONWithHeader(ctx, "GET", url, nil, &env, map[string]string{
		"Authorization": "Bearer " + token,
	})
	return env, code, err
}

func apiErr(env apiEnvelope, code int, action string) error {
	if env.Message != "" {
		return adapter.NewErr(adapter.CodeUpstreamError, fmt.Sprintf("%s：%s", action, env.Message), nil)
	}
	return adapter.NewErr(adapter.CodeUpstreamError, fmt.Sprintf("%s：状态码 %d", action, code), nil)
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

// quotaToUSD new-api 原始 quota → USD（500000 quota = 1 USD）。
func quotaToUSD(q int64) float64 {
	return float64(q) / 500000.0
}

// usdToQuota USD → 原始 quota（划转入参用）。
func usdToQuota(usd float64) int64 {
	return int64(usd*500000.0 + 0.5)
}

func tsToTime(ts int64) time.Time {
	return time.Unix(ts, 0)
}
