-- 0004_sync_ext.sql — 同步引擎扩展（site_sync_state + site_account + 现有表扩列）
-- 迁移器按文件名顺序执行；本文件幂等（IF NOT EXISTS / ADD COLUMN 判存在）。

-- 1. sites 新增 sync_cfg（调度配置 JSON：{enabled, timezone, domain_overrides, budgets}）
ALTER TABLE sites ADD COLUMN sync_cfg TEXT NOT NULL DEFAULT '{}';

-- 2. site_sync_state：调度与游标（每站点 × 每域一行）
CREATE TABLE IF NOT EXISTS site_sync_state (
  id                   TEXT PRIMARY KEY,
  site_id              TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
  domain               TEXT NOT NULL CHECK (domain IN ('keys','groups','account','affiliate','usage_log','usage_daily','models','subscription','announcements','checkin')),
  enabled              INTEGER NOT NULL DEFAULT 1,
  interval_s           INTEGER NOT NULL DEFAULT 300,
  last_run_at          TIMESTAMP,
  next_run_at          TIMESTAMP,
  cursor               TEXT NOT NULL DEFAULT '{}',
  fingerprint          TEXT NOT NULL DEFAULT '',
  consecutive_failures INTEGER NOT NULL DEFAULT 0,
  last_error_class     TEXT NOT NULL DEFAULT '',
  UNIQUE (site_id, domain)
);
CREATE INDEX IF NOT EXISTS idx_sync_state_site ON site_sync_state(site_id);
CREATE INDEX IF NOT EXISTS idx_sync_state_next  ON site_sync_state(next_run_at);

-- 3. site_account：账号余额投影（currency 不折算）
CREATE TABLE IF NOT EXISTS site_account (
  site_id          TEXT PRIMARY KEY REFERENCES sites(id) ON DELETE CASCADE,
  balance          REAL,
  used             REAL,
  currency         TEXT NOT NULL DEFAULT 'quota',
  unit_note        TEXT NOT NULL DEFAULT '',
  subscription_state TEXT NOT NULL DEFAULT '',
  freshness        TEXT NOT NULL DEFAULT 'missing' CHECK (freshness IN ('fresh','stale','missing')),
  last_sync_at     TIMESTAMP
);

-- 4. snapshots.kind CHECK 扩展（SQLite 重建表策略）：
--    重建 snapshots_new → copy → drop → rename，保持原有索引与数据。
CREATE TABLE IF NOT EXISTS snapshots_new (
  id          TEXT PRIMARY KEY,
  site_id     TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
  kind        TEXT NOT NULL CHECK (kind IN ('detect','keys','groups','checkin','probe','affiliates','account','usage_log','usage_daily','models','subscription','announcements','sync_state')),
  status_code INTEGER,
  body        TEXT NOT NULL DEFAULT '{}',
  fetched_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO snapshots_new(id, site_id, kind, status_code, body, fetched_at)
  SELECT id, site_id, kind, status_code, body, fetched_at FROM snapshots WHERE kind IN ('detect','keys','groups','checkin','probe','affiliates');
DROP TABLE snapshots;
ALTER TABLE snapshots_new RENAME TO snapshots;
CREATE INDEX IF NOT EXISTS idx_snapshots_site_kind ON snapshots(site_id, kind);
