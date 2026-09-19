package newapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"aiclient/internal/adapter"
)

// statusQuotaResp GET /api/status data 内的配额系数字段。
type statusQuotaResp struct {
	QuotaPerUnit *int64 `json:"quota_per_unit"`
}

// quotaPerUnit 返回当前适配器的 quota→USD 换算系数（默认 500000）。
// 首次调用时并发安全地读取 /api/status 动态值；失败则回退 500000。
func (a *Adapter) quotaPerUnit(ctx context.Context, atx adapter.AuthCtx) int64 {
	if v := a.quotaCache.Load(); v != 0 {
		return v
	}
	a._puMu.Lock()
	defer a._puMu.Unlock()
	if v := a.quotaCache.Load(); v != 0 {
		return v
	}
	unit := int64(500000)
	if atx.PAT != "" || atx.AccessToken != "" {
		var env apiEnvelope
		env, code, err := a.apiGet(ctx, atx, a.endpoint("/api/status"))
		if err == nil && code == http.StatusOK && env.Success && len(env.Data) > 0 {
			var st struct {
				QuotaPerUnit *int64 `json:"quota_per_unit"`
			}
			if err := json.Unmarshal(env.Data, &st); err == nil && st.QuotaPerUnit != nil && *st.QuotaPerUnit > 0 {
				unit = *st.QuotaPerUnit
			}
		}
	}
	a.quotaCache.Store(unit)
	return unit
}

// QuotaPerUnit 公开读取当前系数（供外部测试/日志）。
func (a *Adapter) QuotaPerUnit() int64 {
	if v := a.quotaCache.Load(); v != 0 {
		return v
	}
	return 500000
}

// SetQuotaPerUnit 显式设置系数（测试/运维覆盖）。
func (a *Adapter) SetQuotaPerUnit(v int64) {
	if v > 0 {
		a.quotaCache.Store(v)
	}
}

// quotaToUSD 按当前系数换算。
func (a *Adapter) quotaToUSD(q int64) float64 {
	return float64(q) / float64(a.quotaPerUnit(backgroundCtx, adapter.AuthCtx{}))
}

// usdToQuota 按当前系数反向换算。
func (a *Adapter) usdToQuota(usd float64) int64 {
	return int64(usd*float64(a.quotaPerUnit(backgroundCtx, adapter.AuthCtx{})) + 0.5)
}

var backgroundCtx = context.Background()

// refreshQuotaPerUnit 强制刷新系数（返回旧值）。
func (a *Adapter) refreshQuotaPerUnit(ctx context.Context, atx adapter.AuthCtx) int64 {
	a.quotaCache.Store(0)
	return a.quotaPerUnit(ctx, atx)
}

// ---- usage log ----

// logItemDTO GET /api/log/self data.logs[] 单项。
type logItemDTO struct {
	Type             string  `json:"type"` // chat/completion/embeddings/...
	ModelName        string  `json:"model_name"`
	ApiKeyID         int64   `json:"api_key_id"`
	ApiKeyMask       string  `json:"key"` // 打码
	RequestID        string  `json:"request_id"`
	CreateTime       int64   `json:"create_time"` // unix seconds
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	Amount           float64 `json:"amount"` // 站点币
	Status           string  `json:"status"` // success/failed/aborted
	ErrorCode        string  `json:"error_code"`
}

// UsageLogs 游标增量：GET /api/log/self?start_timestamp=<unix>（newapi 秒级）。
func (a *Adapter) UsageLogs(ctx context.Context, atx adapter.AuthCtx, since time.Time) ([]adapter.UsageLog, error) {
	if !hasAuth(atx) {
		return nil, adapter.NewErr(adapter.CodeUnauthorized, "缺少 PAT/token/cookie", nil)
	}
	url := fmt.Sprintf("%s/api/log/self?start_timestamp=%d", a.BaseURL(), since.Unix())
	env, code, err := a.apiGet(ctx, atx, url)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK || !env.Success {
		return nil, apiErr(env, code, "拉取用量日志")
	}
	var out []logItemDTO
	if len(env.Data) > 0 {
		var wrap struct {
			Logs []logItemDTO `json:"logs"`
		}
		_ = json.Unmarshal(env.Data, &wrap)
		out = wrap.Logs
		if out == nil {
			_ = json.Unmarshal(env.Data, &out) // data 直接是数组
		}
	}
	logs := make([]adapter.UsageLog, 0, len(out))
	for _, l := range out {
		status := l.Status
		if status == "" {
			status = "ok"
		}
		if status == "success" {
			status = "ok"
		}
		logs = append(logs, adapter.UsageLog{
			RemoteRef:        l.RequestID,
			Timestamp:        tsToTime(l.CreateTime),
			ModelName:        l.ModelName,
			ApiKeyID:         fmt.Sprintf("%d", l.ApiKeyID),
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
