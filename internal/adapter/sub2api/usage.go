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

// usageItemDTO GET /api/v1/usage data.items[] 单项（对齐上游 dto.UsageLog）。
type usageItemDTO struct {
	ModelName             string  `json:"model"`
	ApiKeyID              int64   `json:"api_key_id"`
	ApiKeyMask            string  `json:"key"` // 打码（来自 api_key.key 子对象）
	RequestID             string  `json:"request_id"`
	CreatedAt             string  `json:"created_at"` // ISO8601
	PromptTokens          int64   `json:"input_tokens"`
	CompletionTokens      int64   `json:"output_tokens"`
	CacheReadTokens       int64   `json:"cache_read_tokens"`
	CacheCreationTokens   int64   `json:"cache_creation_tokens"`
	CacheCreation5mTokens int64   `json:"cache_creation_5m_tokens"`
	CacheCreation1hTokens int64   `json:"cache_creation_1h_tokens"`
	InputCost             float64 `json:"input_cost"`
	OutputCost            float64 `json:"output_cost"`
	CacheCreationCost     float64 `json:"cache_creation_cost"`
	CacheReadCost         float64 `json:"cache_read_cost"`
	TotalCost             float64 `json:"total_cost"`  // 原价
	ActualCost            float64 `json:"actual_cost"` // 实付（站点币，含费率折扣）
	RateMultiplier        float64 `json:"rate_multiplier"`
	DurationMs            *int64  `json:"duration_ms"`    // 未知 = null
	FirstTokenMs          *int64  `json:"first_token_ms"` // 未知 = null
	RequestType           string  `json:"request_type"`
	Stream                bool    `json:"stream"`
	BillingMode           *string `json:"billing_mode"`
	ServiceTier           *string `json:"service_tier"`
	ReasoningEffort       *string `json:"reasoning_effort"`
	InboundEndpoint       *string `json:"inbound_endpoint"`
	GroupID               *int64  `json:"group_id"`
	Status                string  `json:"status"` // success/failed/aborted
	ErrorCode             string  `json:"error_code"`
	// 全量字段（0010）：账号归属 / 端点 / 计费标记 / 图片 / 客户端信息
	UserID                    int64          `json:"user_id"`
	AccountID                 int64          `json:"account_id"`
	SubscriptionID            *int64         `json:"subscription_id"`
	UpstreamEndpoint          *string        `json:"upstream_endpoint"`
	BillingType               int8           `json:"billing_type"`
	LongContextBillingApplied bool           `json:"long_context_billing_applied"`
	CacheTTLOverridden        bool           `json:"cache_ttl_overridden"`
	OpenAIWSMode              bool           `json:"openai_ws_mode"`
	NativeCompactionV2        bool           `json:"native_compaction_v2"`
	ImageCount                int            `json:"image_count"`
	ImageSize                 *string        `json:"image_size"`
	ImageInputSize            *string        `json:"image_input_size"`
	ImageOutputSize           *string        `json:"image_output_size"`
	ImageInputTokens          int            `json:"image_input_tokens"`
	ImageInputCost            float64        `json:"image_input_cost"`
	ImageOutputTokens         int            `json:"image_output_tokens"`
	ImageOutputCost           float64        `json:"image_output_cost"`
	ImageSizeSource           *string        `json:"image_size_source"`
	ImageSizeBreakdown        map[string]int `json:"image_size_breakdown"`
	MediaType                 *string        `json:"media_type"`
	UserAgent                 *string        `json:"user_agent"`
	IPAddress                 *string        `json:"ip_address,omitempty"`
	SessionID                 *string        `json:"session_id,omitempty"`
	ApiKey                    *struct {
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
			// rate_multiplier：上游返回 0 视为无折扣（1），避免前端显示 ×0。
			multiplier := l.RateMultiplier
			if multiplier == 0 {
				multiplier = 1
			}
			// group_id 上游为 *int64，nil/0 → ""。
			groupID := ""
			if l.GroupID != nil && *l.GroupID != 0 {
				groupID = strconv.FormatInt(*l.GroupID, 10)
			}
			stream := int64(0)
			if l.Stream {
				stream = 1
			}
			// 账号归属：上游 0 / nil 视为未知 → ""。
			userID := ""
			if l.UserID != 0 {
				userID = strconv.FormatInt(l.UserID, 10)
			}
			accountID := ""
			if l.AccountID != 0 {
				accountID = strconv.FormatInt(l.AccountID, 10)
			}
			subscriptionID := ""
			if l.SubscriptionID != nil && *l.SubscriptionID != 0 {
				subscriptionID = strconv.FormatInt(*l.SubscriptionID, 10)
			}
			// TotalTokens = 输入 + 输出 + 缓存读取 + 缓存写入（对齐上游 usage 口径）。
			totalTokens := l.PromptTokens + l.CompletionTokens + l.CacheReadTokens + l.CacheCreationTokens
			allLogs = append(allLogs, adapter.UsageLog{
				RemoteRef:                 l.RequestID,
				Timestamp:                 ts,
				ModelName:                 l.ModelName,
				ApiKeyID:                  strconv.FormatInt(l.ApiKeyID, 10),
				ApiKeyMask:                mask,
				PromptTokens:              l.PromptTokens,
				CompletionTokens:          l.CompletionTokens,
				TotalTokens:               totalTokens,
				Amount:                    l.ActualCost, // 实付
				Currency:                  "quota",
				Status:                    status,
				ErrCode:                   l.ErrorCode,
				CacheReadTokens:           l.CacheReadTokens,
				CacheCreationTokens:       l.CacheCreationTokens,
				CacheCreation5mTokens:     l.CacheCreation5mTokens,
				CacheCreation1hTokens:     l.CacheCreation1hTokens,
				InputCost:                 l.InputCost,
				OutputCost:                l.OutputCost,
				CacheCreationCost:         l.CacheCreationCost,
				CacheReadCost:             l.CacheReadCost,
				TotalCost:                 l.TotalCost,
				RateMultiplier:            multiplier,
				FirstTokenMs:              l.FirstTokenMs,
				DurationMs:                l.DurationMs,
				RequestType:               l.RequestType,
				Stream:                    stream,
				BillingMode:               derefString(l.BillingMode),
				ServiceTier:               derefString(l.ServiceTier),
				ReasoningEffort:           derefString(l.ReasoningEffort),
				InboundEndpoint:           derefString(l.InboundEndpoint),
				GroupID:                   groupID,
				UserID:                    userID,
				AccountID:                 accountID,
				SubscriptionID:            subscriptionID,
				UpstreamEndpoint:          derefString(l.UpstreamEndpoint),
				BillingType:               int64(l.BillingType),
				LongContextBillingApplied: boolToInt(l.LongContextBillingApplied),
				CacheTTLOverridden:        boolToInt(l.CacheTTLOverridden),
				OpenAIWSMode:              boolToInt(l.OpenAIWSMode),
				NativeCompactionV2:        boolToInt(l.NativeCompactionV2),
				ImageCount:                int64(l.ImageCount),
				ImageSize:                 derefString(l.ImageSize),
				ImageInputSize:            derefString(l.ImageInputSize),
				ImageOutputSize:           derefString(l.ImageOutputSize),
				ImageInputTokens:          int64(l.ImageInputTokens),
				ImageInputCost:            l.ImageInputCost,
				ImageOutputTokens:         int64(l.ImageOutputTokens),
				ImageOutputCost:           l.ImageOutputCost,
				ImageSizeSource:           derefString(l.ImageSizeSource),
				ImageSizeBreakdown:        intMapToInt64(l.ImageSizeBreakdown),
				MediaType:                 derefString(l.MediaType),
				UserAgent:                 derefString(l.UserAgent),
				IPAddress:                 derefString(l.IPAddress),
				SessionID:                 derefString(l.SessionID),
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

// derefString 安全解引用 *string（nil → ""）。
func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// boolToInt 布尔 → 0/1（DB 以 INTEGER 存储，出参再转 bool）。
func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// intMapToInt64 图片尺寸分布 map（int → int64）；空 map → nil（存 NULL 语义即空字符串）。
func intMapToInt64(m map[string]int) map[string]int64 {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]int64, len(m))
	for k, v := range m {
		out[k] = int64(v)
	}
	return out
}
