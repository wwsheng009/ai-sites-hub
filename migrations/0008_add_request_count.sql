-- 0008_add_request_count.sql — 为 usage_daily 添加 request_count 列（记录日聚合的日志条数）
-- 借鉴 sub2api /usage/stats 的 total_requests 概念。

ALTER TABLE usage_daily ADD COLUMN request_count INTEGER NOT NULL DEFAULT 0;
