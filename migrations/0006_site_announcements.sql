-- 0006_site_announcements.sql — 公告聚合表（S5）
-- 公告为站点元数据（FR-4.4，api-inventory /api/notice → /announcements）。
-- 幂等：UNIQUE(site_id, content_hash) 去重；content_hash 为空串时退为 (site_id, remote_ref)。

CREATE TABLE IF NOT EXISTS site_announcements (
  id           TEXT PRIMARY KEY,
  site_id      TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
  remote_ref   TEXT NOT NULL DEFAULT '',          -- 上游公告 id（newapi /api/notice 单条则用 hash）
  title        TEXT NOT NULL DEFAULT '',
  content      TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '',         -- 去重与变更检测
  published_at DATETIME,
  created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
  updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_announcements_hash ON site_announcements(site_id, content_hash) WHERE content_hash <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uq_announcements_ref ON site_announcements(site_id, remote_ref) WHERE remote_ref <> '';
CREATE INDEX IF NOT EXISTS idx_announcements_site_published ON site_announcements(site_id, published_at DESC);
