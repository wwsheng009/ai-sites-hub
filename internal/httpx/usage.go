// Package httpx 用量日志 API（S2/S3）。
package httpx

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// UsageLogOut 用量日志出参（前端 Dashboard 明细）。
type UsageLogOut struct {
	ID               string  `json:"id"` // 前端行标识（remote_ref 不下发，换 id）
	RemoteRef        string  `json:"remote_ref"`
	Timestamp        string  `json:"ts"`
	ModelName        string  `json:"model_name"`
	ApiKeyID         string  `json:"api_key_id"`
	ApiKeyMask       string  `json:"api_key_mask"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	Amount           float64 `json:"amount"`
	Currency         string  `json:"currency"` // 单位说明，不折算
	Status           string  `json:"status"`
	ErrCode          string  `json:"err_code"`
	// 富字段（对齐 sub2api /api/v1/usage；前端 UsageLog 契约）
	CacheReadTokens       int64   `json:"cache_read_tokens"`
	CacheCreationTokens   int64   `json:"cache_creation_tokens"`
	CacheCreation5mTokens int64   `json:"cache_creation_5m_tokens"`
	CacheCreation1hTokens int64   `json:"cache_creation_1h_tokens"`
	InputCost             float64 `json:"input_cost"`
	OutputCost            float64 `json:"output_cost"`
	CacheCreationCost     float64 `json:"cache_creation_cost"`
	CacheReadCost         float64 `json:"cache_read_cost"`
	TotalCost             float64 `json:"total_cost"` // 原价；amount 为实付
	RateMultiplier        float64 `json:"rate_multiplier"`
	FirstTokenMs          *int64  `json:"first_token_ms"` // 上游未记录 = null
	DurationMs            *int64  `json:"duration_ms"`    // 上游未记录 = null
	RequestType           string  `json:"request_type"`
	Stream                bool    `json:"stream"`
	BillingMode           string  `json:"billing_mode"`
	ServiceTier           string  `json:"service_tier"`
	ReasoningEffort       string  `json:"reasoning_effort"`
	InboundEndpoint       string  `json:"inbound_endpoint"`
	GroupID               string  `json:"group_id"`
	// 全量字段（0010）：账号归属 / 端点 / 计费标记 / 图片计费 / 客户端信息
	UserID                    string           `json:"user_id"`
	AccountID                 string           `json:"account_id"`
	SubscriptionID            string           `json:"subscription_id"`
	UpstreamEndpoint          string           `json:"upstream_endpoint"`
	BillingType               int64            `json:"billing_type"`
	LongContextBillingApplied bool             `json:"long_context_billing_applied"`
	CacheTTLOverridden        bool             `json:"cache_ttl_overridden"`
	OpenAIWSMode              bool             `json:"openai_ws_mode"`
	NativeCompactionV2        bool             `json:"native_compaction_v2"`
	ImageCount                int64            `json:"image_count"`
	ImageSize                 string           `json:"image_size"`
	ImageInputSize            string           `json:"image_input_size"`
	ImageOutputSize           string           `json:"image_output_size"`
	ImageInputTokens          int64            `json:"image_input_tokens"`
	ImageInputCost            float64          `json:"image_input_cost"`
	ImageOutputTokens         int64            `json:"image_output_tokens"`
	ImageOutputCost           float64          `json:"image_output_cost"`
	ImageSizeSource           string           `json:"image_size_source"`
	ImageSizeBreakdown        map[string]int64 `json:"image_size_breakdown"` // 无数据 = null
	MediaType                 string           `json:"media_type"`
	UserAgent                 string           `json:"user_agent"`
	IPAddress                 string           `json:"ip_address"`
	SessionID                 string           `json:"session_id"`
}

// ListUsageLogs GET /api/v1/sites/:id/usage/logs?start=&end=&model=&limit=
func (a *API) ListUsageLogs(c *gin.Context) {
	siteID := c.Param("id")
	since := parseDay(c.Query("start"))
	until := parseDayEnd(c.Query("end"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 {
		limit = 200
	}
	logs, err := a.Svc.ListUsageLogs(c.Request.Context(), siteID, since, until, c.Query("model"), limit)
	if err != nil {
		Fail(c, err)
		return
	}
	out := make([]UsageLogOut, 0, len(logs))
	for _, l := range logs {
		out = append(out, UsageLogOut{
			ID:                        l.ID,
			RemoteRef:                 l.RemoteRef,
			Timestamp:                 l.Timestamp.Format(time.RFC3339),
			ModelName:                 l.ModelName,
			ApiKeyID:                  l.ApiKeyID,
			ApiKeyMask:                l.ApiKeyMask,
			PromptTokens:              l.PromptTokens,
			CompletionTokens:          l.CompletionTokens,
			TotalTokens:               l.TotalTokens,
			Amount:                    l.Amount,
			Currency:                  l.Currency,
			Status:                    l.Status,
			ErrCode:                   l.ErrCode,
			CacheReadTokens:           l.CacheReadTokens,
			CacheCreationTokens:       l.CacheCreationTokens,
			CacheCreation5mTokens:     l.CacheCreation5mTokens,
			CacheCreation1hTokens:     l.CacheCreation1hTokens,
			InputCost:                 l.InputCost,
			OutputCost:                l.OutputCost,
			CacheCreationCost:         l.CacheCreationCost,
			CacheReadCost:             l.CacheReadCost,
			TotalCost:                 l.TotalCost,
			RateMultiplier:            l.RateMultiplier,
			FirstTokenMs:              l.FirstTokenMs,
			DurationMs:                l.DurationMs,
			RequestType:               l.RequestType,
			Stream:                    l.Stream != 0,
			BillingMode:               l.BillingMode,
			ServiceTier:               l.ServiceTier,
			ReasoningEffort:           l.ReasoningEffort,
			InboundEndpoint:           l.InboundEndpoint,
			GroupID:                   l.GroupID,
			UserID:                    l.UserID,
			AccountID:                 l.AccountID,
			SubscriptionID:            l.SubscriptionID,
			UpstreamEndpoint:          l.UpstreamEndpoint,
			BillingType:               l.BillingType,
			LongContextBillingApplied: l.LongContextBillingApplied != 0,
			CacheTTLOverridden:        l.CacheTTLOverridden != 0,
			OpenAIWSMode:              l.OpenAIWSMode != 0,
			NativeCompactionV2:        l.NativeCompactionV2 != 0,
			ImageCount:                l.ImageCount,
			ImageSize:                 l.ImageSize,
			ImageInputSize:            l.ImageInputSize,
			ImageOutputSize:           l.ImageOutputSize,
			ImageInputTokens:          l.ImageInputTokens,
			ImageInputCost:            l.ImageInputCost,
			ImageOutputTokens:         l.ImageOutputTokens,
			ImageOutputCost:           l.ImageOutputCost,
			ImageSizeSource:           l.ImageSizeSource,
			ImageSizeBreakdown:        parseInt64Map(l.ImageSizeBreakdown),
			MediaType:                 l.MediaType,
			UserAgent:                 l.UserAgent,
			IPAddress:                 l.IPAddress,
			SessionID:                 l.SessionID,
		})
	}
	OK(c, out)
}

// ListSyncStates GET /api/v1/sites/:id/sync-states — 作业页域级状态（S1）。
func (a *API) ListSyncStates(c *gin.Context) {
	siteID := c.Param("id")
	states, err := a.Svc.ListSyncStatesBySite(c.Request.Context(), siteID)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, states)
}

// RevealKey POST /api/v1/sites/:id/keys/:keyId/reveal — 按需获取完整 key（FR-4.2）。
func (a *API) RevealKey(c *gin.Context) {
	siteID := c.Param("id")
	keyID := c.Param("keyId")
	key, err := a.Svc.RevealKey(c.Request.Context(), siteID, keyID)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"key": key})
}

// ListUsageDaily GET /api/v1/sites/:id/usage/daily?start=&end=&model=&limit=
func (a *API) ListUsageDaily(c *gin.Context) {
	siteID := c.Param("id")
	since := c.Query("start")
	until := c.Query("end")
	limit, _ := strconv.Atoi(c.Query("limit"))
	logs, err := a.Svc.ListUsageDaily(c.Request.Context(), siteID, since, until, c.Query("model"), limit)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, logs)
}

// AggregateUsageDaily POST /api/v1/sites/:id/usage/daily?day=2026-09-19
func (a *API) AggregateUsageDaily(c *gin.Context) {
	siteID := c.Param("id")
	day := c.Query("day")
	if day == "" {
		day = time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	}
	n, err := a.Svc.AggregateUsageDaily(c.Request.Context(), siteID, day)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"aggregated": n, "day": day})
}

// parseInt64Map JSON 文本 → map（空串或损坏 → nil，出参为 null）。
func parseInt64Map(s string) map[string]int64 {
	if s == "" {
		return nil
	}
	var m map[string]int64
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

// parseDay 解析 YYYY-MM-DD，空则返回零值。
func parseDay(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse("2006-01-02", s)
	return t
}

// parseDayEnd 解析 YYYY-MM-DD 为当日结束（23:59:59），用于闭区间查询。
func parseDayEnd(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse("2006-01-02", s)
	if t.IsZero() {
		return time.Time{}
	}
	return t.Add(24*time.Hour - time.Second)
}
