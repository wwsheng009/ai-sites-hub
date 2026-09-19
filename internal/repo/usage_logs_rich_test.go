package repo

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"aiclient/internal/model"
	"aiclient/internal/store"
)

// TestUsageLogsRichFieldsRoundtrip 校验 0009 + 0010 迁移可应用（幂等）且
// 「拉取 → 落库 → 回读」全字段无损，含 upsert 覆盖新列与可空延迟。
func TestUsageLogsRichFieldsRoundtrip(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "verify.db"))
	if err != nil {
		t.Fatalf("打开临时库: %v", err)
	}
	defer db.Close()

	migDir := filepath.Join("..", "..", "migrations")
	applied, err := db.Migrate(ctx, migDir)
	if err != nil {
		t.Fatalf("首次迁移: %v", err)
	}
	t.Logf("首次迁移应用 %d 个（含 0010=%v）", len(applied), contains(applied, "0010_usage_logs_full_fields.sql"))
	again, err := db.Migrate(ctx, migDir)
	if err != nil {
		t.Fatalf("二次迁移: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("迁移应幂等，二次应用了 %v", again)
	}

	r := New(db.GORM)
	ms := func(v int64) *int64 { return &v }
	ts := time.Date(2026, 9, 19, 10, 30, 0, 0, time.UTC)

	src := model.UsageLog{
		RemoteRef: "req-001", Timestamp: ts, ModelName: "claude-sonnet-4-5",
		ApiKeyID: "7", ApiKeyMask: "sk-12345678****abcd",
		PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 3350,
		Amount: 9.99, Currency: "quota", Status: "ok", ErrCode: "rate_limit",
		CacheReadTokens: 3000, CacheCreationTokens: 50, CacheCreation5mTokens: 30, CacheCreation1hTokens: 20,
		InputCost: 0.1, OutputCost: 0.2, CacheCreationCost: 0.3, CacheReadCost: 0.4, TotalCost: 2.0,
		RateMultiplier: 0.8, FirstTokenMs: ms(123), DurationMs: ms(4567),
		RequestType: "stream", Stream: 1, BillingMode: "token", ServiceTier: "flex",
		ReasoningEffort: "high", InboundEndpoint: "/v1/messages", GroupID: "42",
		UserID: "1001", AccountID: "2002", SubscriptionID: "3003",
		UpstreamEndpoint: "/v1/messages", BillingType: 2,
		LongContextBillingApplied: 1, CacheTTLOverridden: 1, OpenAIWSMode: 1, NativeCompactionV2: 1,
		ImageCount: 3, ImageSize: "1024x1024", ImageInputSize: "512x512", ImageOutputSize: "1024x1024",
		ImageInputTokens: 111, ImageInputCost: 1.11, ImageOutputTokens: 222, ImageOutputCost: 2.22,
		ImageSizeSource: "request", ImageSizeBreakdown: `{"1024x1024":2,"512x512":1}`, MediaType: "image/png",
		UserAgent: "claude-cli/1.0", IPAddress: "1.2.3.4", SessionID: "sess-9",
	}
	if _, err := r.InsertUsageLogs(ctx, "site-1", []model.UsageLog{src}); err != nil {
		t.Fatalf("写入: %v", err)
	}

	// 同 remote_ref 再写一次：验证 DoUpdates 覆盖新列（不是只插入）。
	upd := src
	upd.ServiceTier, upd.Amount, upd.BillingType, upd.SessionID = "priority", 8.88, 1, "sess-10"
	upd.ImageSizeBreakdown = `{"2048x2048":4}`
	if _, err := r.InsertUsageLogs(ctx, "site-1", []model.UsageLog{upd}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := r.ListUsageLogs(ctx, "site-1", time.Time{}, time.Time{}, "", 10)
	if err != nil {
		t.Fatalf("查询: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("upsert 后应 1 行，实际 %d", len(got))
	}
	g := got[0]

	eqS := func(name, want, have string) {
		t.Helper()
		if want != have {
			t.Errorf("%s: want %q, got %q", name, want, have)
		}
	}
	eqI := func(name string, want, have int64) {
		t.Helper()
		if want != have {
			t.Errorf("%s: want %d, got %d", name, want, have)
		}
	}
	eqF := func(name string, want, have float64) {
		t.Helper()
		if want != have {
			t.Errorf("%s: want %v, got %v", name, want, have)
		}
	}

	eqS("model_name", upd.ModelName, g.ModelName)
	eqS("api_key_mask", upd.ApiKeyMask, g.ApiKeyMask)
	eqS("status", upd.Status, g.Status)
	eqS("err_code", upd.ErrCode, g.ErrCode)
	eqI("prompt_tokens", upd.PromptTokens, g.PromptTokens)
	eqI("completion_tokens", upd.CompletionTokens, g.CompletionTokens)
	eqI("total_tokens", upd.TotalTokens, g.TotalTokens)
	eqI("cache_read_tokens", upd.CacheReadTokens, g.CacheReadTokens)
	eqI("cache_creation_tokens", upd.CacheCreationTokens, g.CacheCreationTokens)
	eqI("cache_creation_5m_tokens", upd.CacheCreation5mTokens, g.CacheCreation5mTokens)
	eqI("cache_creation_1h_tokens", upd.CacheCreation1hTokens, g.CacheCreation1hTokens)
	eqF("input_cost", upd.InputCost, g.InputCost)
	eqF("output_cost", upd.OutputCost, g.OutputCost)
	eqF("cache_creation_cost", upd.CacheCreationCost, g.CacheCreationCost)
	eqF("cache_read_cost", upd.CacheReadCost, g.CacheReadCost)
	eqF("total_cost", upd.TotalCost, g.TotalCost)
	eqF("amount", upd.Amount, g.Amount)
	eqF("rate_multiplier", upd.RateMultiplier, g.RateMultiplier)
	eqS("currency", upd.Currency, g.Currency)
	eqS("request_type", upd.RequestType, g.RequestType)
	eqI("stream", upd.Stream, g.Stream)
	eqS("billing_mode", upd.BillingMode, g.BillingMode)
	eqS("service_tier", upd.ServiceTier, g.ServiceTier)
	eqS("reasoning_effort", upd.ReasoningEffort, g.ReasoningEffort)
	eqS("inbound_endpoint", upd.InboundEndpoint, g.InboundEndpoint)
	eqS("group_id", upd.GroupID, g.GroupID)
	// 0010 新增字段
	eqS("user_id", upd.UserID, g.UserID)
	eqS("account_id", upd.AccountID, g.AccountID)
	eqS("subscription_id", upd.SubscriptionID, g.SubscriptionID)
	eqS("upstream_endpoint", upd.UpstreamEndpoint, g.UpstreamEndpoint)
	eqI("billing_type", upd.BillingType, g.BillingType)
	eqI("long_context_billing_applied", upd.LongContextBillingApplied, g.LongContextBillingApplied)
	eqI("cache_ttl_overridden", upd.CacheTTLOverridden, g.CacheTTLOverridden)
	eqI("openai_ws_mode", upd.OpenAIWSMode, g.OpenAIWSMode)
	eqI("native_compaction_v2", upd.NativeCompactionV2, g.NativeCompactionV2)
	eqI("image_count", upd.ImageCount, g.ImageCount)
	eqS("image_size", upd.ImageSize, g.ImageSize)
	eqS("image_input_size", upd.ImageInputSize, g.ImageInputSize)
	eqS("image_output_size", upd.ImageOutputSize, g.ImageOutputSize)
	eqI("image_input_tokens", upd.ImageInputTokens, g.ImageInputTokens)
	eqF("image_input_cost", upd.ImageInputCost, g.ImageInputCost)
	eqI("image_output_tokens", upd.ImageOutputTokens, g.ImageOutputTokens)
	eqF("image_output_cost", upd.ImageOutputCost, g.ImageOutputCost)
	eqS("image_size_source", upd.ImageSizeSource, g.ImageSizeSource)
	eqS("image_size_breakdown", upd.ImageSizeBreakdown, g.ImageSizeBreakdown)
	eqS("media_type", upd.MediaType, g.MediaType)
	eqS("user_agent", upd.UserAgent, g.UserAgent)
	eqS("ip_address", upd.IPAddress, g.IPAddress)
	eqS("session_id", upd.SessionID, g.SessionID)
	if g.FirstTokenMs == nil || *g.FirstTokenMs != 123 {
		t.Errorf("first_token_ms: want 123, got %v", g.FirstTokenMs)
	}
	if g.DurationMs == nil || *g.DurationMs != 4567 {
		t.Errorf("duration_ms: want 4567, got %v", g.DurationMs)
	}

	// 可空延迟：NULL 回读仍为 nil（不能被写成 0）。
	nul := src
	nul.RemoteRef, nul.FirstTokenMs, nul.DurationMs = "req-002", nil, nil
	if _, err := r.InsertUsageLogs(ctx, "site-1", []model.UsageLog{nul}); err != nil {
		t.Fatalf("写入空延迟行: %v", err)
	}
	rows, err := r.ListUsageLogs(ctx, "site-1", time.Time{}, time.Time{}, "", 10)
	if err != nil {
		t.Fatalf("查询: %v", err)
	}
	for _, row := range rows {
		if row.RemoteRef == "req-002" && (row.FirstTokenMs != nil || row.DurationMs != nil) {
			t.Errorf("空延迟应为 NULL，got first=%v dur=%v", row.FirstTokenMs, row.DurationMs)
		}
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
