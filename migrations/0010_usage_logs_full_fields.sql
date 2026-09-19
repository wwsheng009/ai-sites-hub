-- 0010_usage_logs_full_fields.sql — usage_logs 再补齐上游用户侧 DTO 余下字段
-- 依据：sub2api backend/internal/handler/dto/types.go 的 UsageLog（用户侧，/api/v1/usage 出参）
-- 0009 已补 token 明细/费用分解/延迟/请求形态；本迁移补齐账号归属、请求端点、计费标记、
-- 图片生成计费、客户端信息（UA/IP/session）等剩余标量字段，做到「拉取即全量落库」。
-- 幂等：迁移执行器按 schema_migrations 记录版本，已应用即跳过；首次对旧库执行 ADD COLUMN。
-- 约定：字符串未知 = ''，数值未知 = 0，布尔以 INTEGER 0/1 存储（出参转 bool）。

-- 1. 账号归属（上游 user / account / subscription；出参转字符串，未知 = ''）
ALTER TABLE usage_logs ADD COLUMN user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN account_id TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN subscription_id TEXT NOT NULL DEFAULT '';

-- 2. 请求端点与计费标记
ALTER TABLE usage_logs ADD COLUMN upstream_endpoint TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN billing_type INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN long_context_billing_applied INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN cache_ttl_overridden INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN openai_ws_mode INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN native_compaction_v2 INTEGER NOT NULL DEFAULT 0;

-- 3. 图片生成计费（image_size_breakdown 存 JSON 文本，空 = ''）
ALTER TABLE usage_logs ADD COLUMN image_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN image_size TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN image_input_size TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN image_output_size TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN image_input_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN image_input_cost REAL NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN image_output_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN image_output_cost REAL NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN image_size_source TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN image_size_breakdown TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN media_type TEXT NOT NULL DEFAULT '';

-- 4. 客户端信息
ALTER TABLE usage_logs ADD COLUMN user_agent TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN ip_address TEXT NOT NULL DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN session_id TEXT NOT NULL DEFAULT '';
