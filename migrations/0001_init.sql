-- 0001_init.sql — ai-sites-client 初始 schema（architecture §3 v0.2，SQLite 单方言）
-- 迁移器按文件名字典序执行，已执行版本记录在 schema_migrations。

CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,          -- 文件名（如 0001_init.sql）
    applied_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- 站点
CREATE TABLE IF NOT EXISTS sites (
    id            TEXT PRIMARY KEY,                -- UUID
    name          TEXT NOT NULL,
    base_url      TEXT NOT NULL UNIQUE,
    api_base_url  TEXT NOT NULL DEFAULT '',
    site_type     TEXT NOT NULL DEFAULT 'unknown'
                  CHECK (site_type IN ('sub2api','new-api','unknown')),
    status        TEXT NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active','disabled','archived')),
    detect_score  INTEGER NOT NULL DEFAULT 0,
    detect_report TEXT NOT NULL DEFAULT '{}',      -- JSON：特征证据
    checkin_cfg   TEXT NOT NULL DEFAULT '{}',      -- JSON：{enabled, mode, jitter_minutes}
    aff_cfg       TEXT NOT NULL DEFAULT '{}',      -- JSON：FR-10 自动划转规则（默认 enabled=false）
    last_probe_at DATETIME,
    latency_ms    INTEGER,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- 凭据（密文列；投影只存指纹/状态）
CREATE TABLE IF NOT EXISTS site_credentials (
    id                   TEXT PRIMARY KEY,
    site_id              TEXT NOT NULL UNIQUE REFERENCES sites(id) ON DELETE CASCADE,
    auth_mode            TEXT NOT NULL DEFAULT 'username_password'
                         CHECK (auth_mode IN ('username_password','token','hybrid')),
    username             TEXT NOT NULL DEFAULT '',
    password_cipher      BLOB,
    totp_secret_cipher   BLOB,
    access_token_cipher  BLOB,
    refresh_token_cipher BLOB,
    pat_cipher           BLOB,
    auth_state           TEXT NOT NULL DEFAULT 'none'
                         CHECK (auth_state IN ('none','ok','login_failed','totp_required',
                                               'turnstile_blocked','cf_blocked','token_expired','refresh_failed')),
    auth_state_msg       TEXT NOT NULL DEFAULT '',
    last_login_at        DATETIME,
    token_expire_hint    DATETIME,
    updated_at           DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_site_credentials_site ON site_credentials(site_id);

-- key 归一化投影
CREATE TABLE IF NOT EXISTS site_keys (
    id             TEXT PRIMARY KEY,
    site_id        TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    remote_key_id  TEXT NOT NULL,
    name           TEXT NOT NULL DEFAULT '',
    grp            TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'active',
    quota_limit    REAL,
    quota_used     REAL,
    quota_remaining REAL,
    unlimited      INTEGER NOT NULL DEFAULT 0,
    expires_at     DATETIME,
    created_at     DATETIME,
    key_source     TEXT NOT NULL DEFAULT 'masked' CHECK (key_source IN ('masked','plaintext')),
    raw_snapshot_id TEXT,
    freshness      TEXT NOT NULL DEFAULT 'missing' CHECK (freshness IN ('fresh','stale','missing')),
    last_sync_at   DATETIME,
    UNIQUE (site_id, remote_key_id)
);
CREATE INDEX IF NOT EXISTS idx_site_keys_site ON site_keys(site_id);

-- 分组/倍率投影
CREATE TABLE IF NOT EXISTS site_groups (
    id              TEXT PRIMARY KEY,
    site_id         TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    remote_group_id TEXT NOT NULL,
    name            TEXT NOT NULL DEFAULT '',
    ratio           REAL NOT NULL DEFAULT 1.0,
    available       INTEGER NOT NULL DEFAULT 1,
    "desc"          TEXT NOT NULL DEFAULT '',
    freshness       TEXT NOT NULL DEFAULT 'missing' CHECK (freshness IN ('fresh','stale','missing')),
    last_sync_at    DATETIME,
    UNIQUE (site_id, remote_group_id)
);
CREATE INDEX IF NOT EXISTS idx_site_groups_site ON site_groups(site_id);

-- 签到记录
CREATE TABLE IF NOT EXISTS checkins (
    id            TEXT PRIMARY KEY,
    site_id       TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    date          TEXT NOT NULL,                   -- YYYY-MM-DD
    state         TEXT NOT NULL
                  CHECK (state IN ('success','already','failed','skipped','unsupported','blocked_by_turnstile')),
    quota_awarded REAL,
    message       TEXT NOT NULL DEFAULT '',
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (site_id, date)
);

-- 作业与执行记录
CREATE TABLE IF NOT EXISTS jobs (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    schedule_type TEXT NOT NULL CHECK (schedule_type IN ('interval','cron','manual')),
    spec         TEXT NOT NULL DEFAULT '',
    enabled      INTEGER NOT NULL DEFAULT 1,
    jitter_seconds INTEGER NOT NULL DEFAULT 0,
    last_run_id  TEXT,
    next_run_at  DATETIME
);

CREATE TABLE IF NOT EXISTS job_runs (
    id          TEXT PRIMARY KEY,
    job_id      TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    site_id     TEXT REFERENCES sites(id) ON DELETE SET NULL,
    state       TEXT NOT NULL CHECK (state IN ('running','success','failed','skipped')),
    error_class TEXT NOT NULL DEFAULT '',
    message     TEXT NOT NULL DEFAULT '',
    started_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    finished_at DATETIME,
    dry_run     INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_job_runs_job ON job_runs(job_id);

-- 快照（上游原始响应，脱敏后存储）
CREATE TABLE IF NOT EXISTS snapshots (
    id          TEXT PRIMARY KEY,
    site_id     TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('detect','keys','groups','checkin','probe','affiliates')),
    status_code INTEGER,
    body        TEXT NOT NULL DEFAULT '{}',
    fetched_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_snapshots_site_kind ON snapshots(site_id, kind);

-- 平台事件
CREATE TABLE IF NOT EXISTS events (
    id         TEXT PRIMARY KEY,
    site_id    TEXT REFERENCES sites(id) ON DELETE SET NULL,
    type       TEXT NOT NULL,
    level      TEXT NOT NULL DEFAULT 'info' CHECK (level IN ('info','warn','error')),
    message    TEXT NOT NULL DEFAULT '',
    data       TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_events_created ON events(created_at);

-- 通知渠道 / 策略 / 投递（architecture §11，M4.5 落码；表先建）
CREATE TABLE IF NOT EXISTS notification_channels (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL UNIQUE,
    type           TEXT NOT NULL CHECK (type IN ('webhook','email')),
    enabled        INTEGER NOT NULL DEFAULT 1,
    config         TEXT NOT NULL DEFAULT '{}',
    secrets_cipher BLOB,
    created_at     DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at     DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS notification_rules (
    id               TEXT PRIMARY KEY,
    name             TEXT NOT NULL,
    enabled          INTEGER NOT NULL DEFAULT 1,
    event_type       TEXT NOT NULL DEFAULT '*',
    min_level        TEXT NOT NULL DEFAULT 'info' CHECK (min_level IN ('info','warn','error')),
    site_id          TEXT REFERENCES sites(id) ON DELETE CASCADE,
    channel_ids      TEXT NOT NULL DEFAULT '[]',
    cooldown_seconds INTEGER NOT NULL DEFAULT 0,
    created_at       DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at       DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS notification_deliveries (
    id            TEXT PRIMARY KEY,
    event_id      TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    rule_id       TEXT NOT NULL REFERENCES notification_rules(id) ON DELETE CASCADE,
    channel_id    TEXT NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    state         TEXT NOT NULL DEFAULT 'pending'
                  CHECK (state IN ('pending','succeeded','failed','suppressed')),
    attempts      INTEGER NOT NULL DEFAULT 0,
    next_retry_at DATETIME,
    last_error    TEXT NOT NULL DEFAULT '',
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    sent_at       DATETIME
);
CREATE INDEX IF NOT EXISTS idx_deliveries_state ON notification_deliveries(state, next_retry_at);

-- 站点返利投影 / 划转记录（architecture §3 v0.2，FR-10；M4 落码；表先建）
CREATE TABLE IF NOT EXISTS site_affiliates (
    id           TEXT PRIMARY KEY,
    site_id      TEXT NOT NULL UNIQUE REFERENCES sites(id) ON DELETE CASCADE,
    aff_code     TEXT NOT NULL DEFAULT '',
    rebate_rate  REAL,
    available    REAL,
    frozen       REAL,
    history      REAL,
    invitee_count INTEGER,
    aff_cfg      TEXT NOT NULL DEFAULT '{}',
    freshness    TEXT NOT NULL DEFAULT 'missing' CHECK (freshness IN ('fresh','stale','missing')),
    last_sync_at DATETIME,
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS aff_transfers (
    id             TEXT PRIMARY KEY,
    site_id        TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    amount         REAL NOT NULL,
    state          TEXT NOT NULL CHECK (state IN ('pending','success','failed','dry_run')),
    trigger_type   TEXT NOT NULL CHECK (trigger_type IN ('manual','auto_rule')),
    rule_snapshot  TEXT NOT NULL DEFAULT '{}',
    amount_before  REAL,
    error_class    TEXT NOT NULL DEFAULT '',
    message        TEXT NOT NULL DEFAULT '',
    created_at     DATETIME NOT NULL DEFAULT (datetime('now')),
    finished_at    DATETIME
);
CREATE INDEX IF NOT EXISTS idx_aff_transfers_site ON aff_transfers(site_id);
