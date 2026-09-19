// Package sub2api 用量日志同步（S2）。
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

// usageItemDTO GET /api/v1/usage data.items[] 单项。
type usageItemDTO struct {
	ModelName        string  `json:"model"`
	ApiKeyID         int64   `json:"api_key_id"`
	ApiKeyMask       string  `json:"key"` // 打码（来自 api_key.key 子对象）
	RequestID        string  `json:"request_id"`
	CreatedAt        string  `json:"created_at"` // ISO8601
	PromptTokens     int64   `json:"input_tokens"`
	CompletionTokens int64   `json:"output_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	ActualCost       float64 `json:"actual_cost"` // 站点币（含费率折扣）
	Status           string  `json:"status"`      // success/failed/aborted
	ErrorCode        string  `json:"error_code"`
	ApiKey           *struct {
		Key string `json:"key"`
	} `json:"api_key,omitempty"`
}

// usageData GET /api/v1/usage data 包装（分页）。
type usageData struct {
	Items    []usageItemDTO `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Pages    int            `json:"pages"`
}

// UsageLogs 游标增量：GET /api/v1/usage?start_date=<date>（sub2api 按日）。
// 分页获取（ai-gateway lessons：pageSize=200/maxPages=100/total 一致性校验）。
func (a *Adapter) UsageLogs(ctx context.Context, atx adapter.AuthCtx, since time.Time) ([]adapter.UsageLog, error) {
	if atx.AccessToken == "" {
		return nil, adapter.NewErr(adapter.CodeUnauthorized, "缺少 access token", nil)
	}
	start := since.UTC().Format("2006-01-02")
	const pageSize = 200
	const maxPages = 100
	var allLogs []adapter.UsageLog
	for page := 1; page <= maxPages; page++ {
		url := fmt.Sprintf("%s/api/v1/usage?start_date=%s&end_date=%s&page=%d&page_size=%d",
			a.BaseURL(), start, time.Now().UTC().Format("2006-01-02"), page, pageSize)
		var env apiEnvelope
		status, _, err := a.hc.DoJSONWithHeader(ctx, "GET", url, nil, &env, authHeaders(atx.AccessToken))
		if err != nil {
			return nil, adapter.NewErr(adapter.CodeUpstreamError, "拉取用量日志失败", err)
		}
		if status != http.StatusOK || env.Code != 0 {
			return nil, statusErr(status, "拉取用量日志")
		}
		var ud usageData
		if len(env.Data) > 0 {
			if err := json.Unmarshal(env.Data, &ud); err != nil {
				return nil, adapter.NewErr(adapter.CodeUpstreamError, "解析用量日志响应失败", err)
			}
		}
		for _, l := range ud.Items {
			status := l.Status
			if status == "" || status == "success" {
				status = "ok"
			}
			ts, _ := time.Parse(time.RFC3339, l.CreatedAt)
			if ts.IsZero() {
				ts = since
			}
			mask := l.ApiKeyMask
			if mask == "" && l.ApiKey != nil {
				mask = maskKey(l.ApiKey.Key)
			}
			totalTokens := l.PromptTokens + l.CompletionTokens + l.CacheReadTokens
			allLogs = append(allLogs, adapter.UsageLog{
				RemoteRef:        l.RequestID,
				Timestamp:        ts,
				ModelName:        l.ModelName,
				ApiKeyID:         strconv.FormatInt(l.ApiKeyID, 10),
				ApiKeyMask:       mask,
				PromptTokens:     l.PromptTokens,
				CompletionTokens: l.CompletionTokens,
				TotalTokens:      totalTokens,
				Amount:           l.ActualCost,
				Currency:         "quota",
				Status:           status,
				ErrCode:          l.ErrorCode,
			})
		}
		// 分页终止条件：无更多数据或到达最后一页
		if len(ud.Items) == 0 || page >= ud.Pages {
			break
		}
	}
	return allLogs, nil
}

// maskKey 脱敏 API key：保留前 8 位 + 后 4 位，中间用 * 替代。
func maskKey(key string) string {
	if len(key) <= 12 {
		return key
	}
	return key[:8] + "****" + key[len(key)-4:]
}
