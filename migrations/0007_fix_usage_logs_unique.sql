-- 0007_fix_usage_logs_unique.sql — 修复 usage_logs 唯一索引（SQLite 不支持部分唯一索引作为 ON CONFLICT 冲突目标）
-- 0005 创建的 uq_usage_logs_remote_ref 是 partial index（WHERE remote_ref <> ''），
-- SQLite 的 ON CONFLICT (site_id, remote_ref) 无法使用 partial index 作为冲突目标。
-- 修复：改为全量唯一索引。remote_ref 为空时，uq_usage_logs_fingerprint 哈希指纹索引兜底去重。

DROP INDEX IF EXISTS uq_usage_logs_remote_ref;
CREATE UNIQUE INDEX uq_usage_logs_remote_ref ON usage_logs(site_id, remote_ref);
