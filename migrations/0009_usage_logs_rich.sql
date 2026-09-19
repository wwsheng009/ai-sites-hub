-- 0009_usage_logs_rich.sql — 补齐 usage_logs / usage_daily 富字段（sub2api /api/v1/usage）
-- 上游权威结构：sub2api backend/internal/handler/dto/types.go 的 UsageLog（40+ 字段），
-- 此前仅解析/落库 9 个字段，这里补齐 token 明细、费用分解、延迟与请求形态等。
-- 幂等：迁移执行器按 schema_migrations 记录版本、已应用即跳过；首次对旧库执行 ADD COLUMN。
-- 注意：first_token_ms / duration_ms 可空，上游未知时存 NULL（不用 0 冒充）。

-- 1. usage_logs：缓存 token 明细（Anthropic prompt caching 读取/写入/5m/1h）
ALTER TABLE usage_logs ADD COLUMN cache_read_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN cache_creation_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN cache_creation_5m_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN cache_creation_1h_tokens INTEGER NOT NULL DEFAULT 0;

-- 2. usage_logs：费用分解（站点币，仅标明不折算；total_cost 为原价，amount 为实付 actual_cost）
ALTER TABLE usage_logs ADD COLUMN input_cost REAL NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN output_cost REAL NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN cache_creation_cost REAL NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN cache_read_cost REAL NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN total_cost REAL NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN rate_multiplier REAL NOT NULL DEFAULT 1;

-- 3. usage_logs：延迟（可空：上游 duration_ms / first_token_ms 为 *int，未知 = NULL）
ALTER TABLE usage_logs ADD COLUMN first_token_ms INTEGER;
ALTER TABLE usage_logs ADD COLUMN duration_ms INTEGER;

-- 4. usage_logs：请求形态与计费元信息
ALTER TABLE usage_logs ADD COLUMN request_type TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN stream INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN billing_mode TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN service_tier TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN reasoning_effort TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN inbound_endpoint TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN group_id TEXT NOT NULL DEFAULT '';

-- 5. usage_daily：日聚合同步补齐缓存 token 列（AggregateUsageDaily 汇总）
ALTER TABLE usage_daily ADD COLUMN cache_read_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_daily ADD COLUMN cache_creation_tokens INTEGER NOT NULL DEFAULT 0;
