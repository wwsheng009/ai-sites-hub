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

// usageItemDTO GET /api/v1/usage data.data[] 单项。
type usageItemDTO struct {
	ModelName        string  `json:"model_name"`
	ApiKeyID         int64   `json:"api_key_id"`
	ApiKeyMask       string  `json:"key"` // 打码
	RequestID        string  `json:"request_id"`
	CreatedAt        string  `json:"created_at"` // ISO8601
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	Amount           float64 `json:"amount"` // 站点币
	Status           string  `json:"status"` // success/failed/aborted
	ErrorCode        string  `json:"error_code"`
}

// usageData GET /api/v1/usage data 包装（分页）。
type usageData struct {
	Items []usageItemDTO `json:"data"`
	Total int64          `json:"total"`
}

// UsageLogs 游标增量：GET /api/v1/usage?start_date=<date>（sub2api 按日）。
func (a *Adapter) UsageLogs(ctx context.Context, atx adapter.AuthCtx, since time.Time) ([]adapter.UsageLog, error) {
	if atx.AccessToken == "" {
		return nil, adapter.NewErr(adapter.CodeUnauthorized, "缺少 access token", nil)
	}
	start := since.UTC().Format("2006-01-02")
	url := fmt.Sprintf("%s/api/v1/usage?start_date=%s&end_date=%s&page=1&page_size=100",
		a.BaseURL(), start, time.Now().UTC().Format("2006-01-02"))
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
		_ = json.Unmarshal(env.Data, &ud)
	}
	logs := make([]adapter.UsageLog, 0, len(ud.Items))
	for _, l := range ud.Items {
		status := l.Status
		if status == "" || status == "success" {
			status = "ok"
		}
		ts, _ := time.Parse(time.RFC3339, l.CreatedAt)
		if ts.IsZero() {
			ts = since
		}
		logs = append(logs, adapter.UsageLog{
			RemoteRef:        l.RequestID,
			Timestamp:        ts,
			ModelName:        l.ModelName,
			ApiKeyID:         strconv.FormatInt(l.ApiKeyID, 10),
			ApiKeyMask:       l.ApiKeyMask,
			PromptTokens:     l.PromptTokens,
			CompletionTokens: l.CompletionTokens,
			TotalTokens:      l.TotalTokens,
			Amount:           l.Amount,
			Currency:         "quota",
			Status:           status,
			ErrCode:          l.ErrorCode,
		})
	}
	return logs, nil
}
