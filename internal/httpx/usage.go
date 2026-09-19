// Package httpx 用量日志 API（S2/S3）。
package httpx

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// UsageLogOut 用量日志出参（前端 Dashboard 明细）。
type UsageLogOut struct {
	RemoteRef        string  `json:"remote_ref"`
	Timestamp        string  `json:"ts"`
	ModelName        string  `json:"model_name"`
	ApiKeyMask       string  `json:"api_key_mask"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	Amount           float64 `json:"amount"`
	Currency         string  `json:"currency"` // 单位说明，不折算
	Status           string  `json:"status"`
	ErrCode          string  `json:"err_code"`
}

// ListUsageLogs GET /api/v1/sites/:id/usage/logs?start=&end=&model=&limit=
func (a *API) ListUsageLogs(c *gin.Context) {
	siteID := c.Param("id")
	since := parseDay(c.Query("start"))
	until := parseDay(c.Query("end"))
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
			RemoteRef:        l.RemoteRef,
			Timestamp:        l.Timestamp.Format(time.RFC3339),
			ModelName:        l.ModelName,
			ApiKeyMask:       l.ApiKeyMask,
			PromptTokens:     l.PromptTokens,
			CompletionTokens: l.CompletionTokens,
			TotalTokens:      l.TotalTokens,
			Amount:           l.Amount,
			Currency:         l.Currency,
			Status:           l.Status,
			ErrCode:          l.ErrCode,
		})
	}
	OK(c, out)
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

// parseDay 解析 YYYY-MM-DD，空则返回零值。
func parseDay(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse("2006-01-02", s)
	return t
}
