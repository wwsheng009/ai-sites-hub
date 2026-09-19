// Package httpx 模型广场 API（S4）。
package httpx

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// SiteModelOut 模型广场出参（前端 Dashboard 模型卡）。
type SiteModelOut struct {
	SiteID      string   `json:"site_id"`
	SiteName    string   `json:"site_name"`
	ModelName   string   `json:"model_name"`
	TotalTokens int64    `json:"total_tokens"`
	Amount      float64  `json:"amount"`
	Currency    string   `json:"currency"` // 单位说明，不折算
	Balance     *float64 `json:"balance,omitempty"`
	Freshness   string   `json:"freshness"`
}

// ListSiteModels GET /api/v1/models?limit=
func (a *API) ListSiteModels(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	models, err := a.Svc.ListSiteModels(c.Request.Context(), limit)
	if err != nil {
		Fail(c, err)
		return
	}
	out := make([]SiteModelOut, 0, len(models))
	for _, m := range models {
		out = append(out, SiteModelOut{
			SiteID:      m.SiteID,
			SiteName:    m.SiteName,
			ModelName:   m.ModelName,
			TotalTokens: m.TotalTokens,
			Amount:      m.Amount,
			Currency:    m.Currency,
			Balance:     m.Balance,
			Freshness:   m.Freshness,
		})
	}
	OK(c, out)
}
