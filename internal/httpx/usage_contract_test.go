package httpx

import (
	"encoding/json"
	"testing"
)

// TestUsageLogOutContract 锁定前端 frontend/src/types.ts 的 UsageLog 契约：
// 出参必须包含下列全部键，且布尔/可空类型与前端类型一致（stream: boolean、
// first_token_ms/duration_ms/image_size_breakdown 无数据为 null）。
func TestUsageLogOutContract(t *testing.T) {
	want := []string{
		"id", "remote_ref", "ts", "model_name", "api_key_id", "api_key_mask",
		"prompt_tokens", "completion_tokens", "total_tokens",
		"cache_read_tokens", "cache_creation_tokens", "cache_creation_5m_tokens", "cache_creation_1h_tokens",
		"input_cost", "output_cost", "cache_creation_cost", "cache_read_cost", "total_cost",
		"amount", "rate_multiplier", "currency", "status", "err_code",
		"request_type", "stream", "billing_mode", "service_tier", "reasoning_effort",
		"inbound_endpoint", "group_id", "first_token_ms", "duration_ms",
		"user_id", "account_id", "subscription_id", "upstream_endpoint", "billing_type",
		"long_context_billing_applied", "cache_ttl_overridden", "openai_ws_mode", "native_compaction_v2",
		"image_count", "image_size", "image_input_size", "image_output_size",
		"image_input_tokens", "image_input_cost", "image_output_tokens", "image_output_cost",
		"image_size_source", "image_size_breakdown", "media_type",
		"user_agent", "ip_address", "session_id",
	}

	b, err := json.Marshal(UsageLogOut{})
	if err != nil {
		t.Fatalf("序列化: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("反序列化: %v", err)
	}
	for _, k := range want {
		if _, ok := m[k]; !ok {
			t.Errorf("出参缺少前端契约字段: %s", k)
		}
	}
	t.Logf("出参字段数=%d，前端契约 %d 项全覆盖", len(m), len(want))

	if _, ok := m["stream"].(bool); !ok {
		t.Errorf("stream 应为 bool，实际 %T", m["stream"])
	}
	for _, k := range []string{"first_token_ms", "duration_ms", "image_size_breakdown"} {
		if v, ok := m[k]; !ok || v != nil {
			t.Errorf("%s 无数据应为 null，实际 %v", k, v)
		}
	}
	for _, k := range []string{"long_context_billing_applied", "cache_ttl_overridden", "openai_ws_mode", "native_compaction_v2"} {
		if _, ok := m[k].(bool); !ok {
			t.Errorf("%s 应为 bool，实际 %T", k, m[k])
		}
	}
}

// TestParseInt64Map 校验 JSON 文本 ↔ map 转换（空串/损坏 → nil）。
func TestParseInt64Map(t *testing.T) {
	if got := parseInt64Map(""); got != nil {
		t.Errorf("空串应为 nil，got %v", got)
	}
	if got := parseInt64Map("{bad json"); got != nil {
		t.Errorf("损坏 JSON 应为 nil，got %v", got)
	}
	got := parseInt64Map(`{"1024x1024":3,"512x512":1}`)
	if len(got) != 2 || got["1024x1024"] != 3 || got["512x512"] != 1 {
		t.Errorf("解析结果错误: %v", got)
	}
}
