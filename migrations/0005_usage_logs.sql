-- 0005_usage_logs.sql — 用量日志增量表（S2）
-- 游标增量：newapi /api/log/self（秒级 start_timestamp），sub2api /api/v1/usage（按日 start_date）。
-- 幂等：at-least-once 写入 + 唯一键去重（remote_ref 优先；无 remote_ref 时退为哈希指纹）。

-- 1. usage_logs：调用日志投影（currency 仅标明，不折算）
CREATE TABLE IF NOT EXISTS usage_logs (
  id              TEXT PRIMARY KEY,
  site_id         TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
  remote_ref      TEXT NOT NULL DEFAULT '',
  ts              DATETIME NOT NULL,
  model_name      TEXT NOT NULL DEFAULT '',
  api_key_id      TEXT NOT NULL DEFAULT '',
  api_key_mask    TEXT NOT NULL DEFAULT '',
  prompt_tokens   INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens    INTEGER NOT NULL DEFAULT 0,
  amount          REAL NOT NULL DEFAULT 0,
  currency        TEXT NOT NULL DEFAULT 'quota',
  status          TEXT NOT NULL DEFAULT 'ok' CHECK (status IN ('ok','err','abort')),
  err_code        TEXT NOT NULL DEFAULT '',
  fetched_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- 2. 幂等键：remote_ref 优先；退化为 (site_id, ts, model_name, api_key_id, total_tokens, amount) 哈希。
CREATE UNIQUE INDEX IF NOT EXISTS uq_usage_logs_remote_ref ON usage_logs(site_id, remote_ref) WHERE remote_ref <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uq_usage_logs_fingerprint ON usage_logs(site_id, ts, model_name, api_key_id, total_tokens, amount);
CREATE INDEX IF NOT EXISTS idx_usage_logs_site_ts ON usage_logs(site_id, ts);
CREATE INDEX IF NOT EXISTS idx_usage_logs_model ON usage_logs(model_name);

-- 3. 用量日聚合表（S3 预留 schema）
CREATE TABLE IF NOT EXISTS usage_daily (
  id              TEXT PRIMARY KEY,
  site_id         TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
  day             DATE NOT NULL,
  model_name      TEXT NOT NULL DEFAULT '',
  prompt_tokens   INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens    INTEGER NOT NULL DEFAULT 0,
  amount          REAL NOT NULL DEFAULT 0,
  currency        TEXT NOT NULL DEFAULT 'quota',
  UNIQUE (site_id, day, model_name)
);
CREATE INDEX IF NOT EXISTS idx_usage_daily_site_day ON usage_daily(site_id, day);
