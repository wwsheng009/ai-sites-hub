# 保存方案设计（存储层：新表、归一化写入、快照与生命周期）

> 输入：[api-inventory.md](api-inventory.md)、[sync-architecture.md](sync-architecture.md)。
> 约束：GORM 实体列名必须与 migrations 对齐，**禁止 AutoMigrate**，新表一律新增 migration 文件。

## 1. 现状存储盘点

| 表 | 内容 | 备注 |
|---|---|---|
| sites | 站点主档（base_url 唯一） | 含 checkin_cfg/aff_cfg JSON 配置 |
| site_credentials | 凭据（cipher 列，明文不落库） | auth_state 状态机 |
| site_keys | Key 归一化投影 | `(site_id, remote_key_id)` 幂等 upsert；freshness/last_sync_at |
| site_groups | 分组倍率投影 | 同上幂等 |
| site_affiliates | 返利投影 | site_id 唯一 |
| aff_transfers | 划转留证（不可逆） | 触发前 rule_snapshot |
| snapshots | 上游原始响应（脱敏） | kind + body + fetched_at |
| events | 平台事件 | 前端事件中心已接 |

已有约定沿用：投影表 `freshness`（fresh/stale/missing）+ `last_sync_at`；敏感数据走 `snapshots` 脱敏留底。

## 2. 新增表（migration `0004_sync_ext.sql`）

### 2.0 现有表增列

```sql
-- sites 表新增 sync_cfg（0001 只有 aff_cfg/checkin_cfg，无 sync_cfg）
ALTER TABLE sites ADD COLUMN sync_cfg TEXT NOT NULL DEFAULT '{}';  -- JSON：{enabled, timezone, domain_overrides}
-- snapshots.kind CHECK 枚举扩展：SQLite 需按官方 12 步流程重建表
-- 新 kind：'account','usage_log','usage_daily','models','subscription','announcements','sync_state'
```

> ⚠️ `snapshots.kind` 现有 CHECK 约束（0001_init.sql）不含新域 kind。0004 需重建 snapshots 表（create new → copy → drop → rename），同步期间写快照功能短暂不可用需在 migration 事务内完成。

### 2.1 site_sync_state —— 调度与游标（每站点 × 每域一行）

```sql
CREATE TABLE site_sync_state (
  id          TEXT PRIMARY KEY,             -- uuid
  site_id     TEXT NOT NULL,
  domain      TEXT NOT NULL,                -- keys|groups|account|affiliate|usage_log|usage_daily|models|subscription|announcements|checkin
  enabled     INTEGER NOT NULL DEFAULT 1,   -- 站点级开关（sites.sync_cfg 总开关下再细分）
  interval_s  INTEGER NOT NULL,             -- 生效周期（默认取全局，站点可覆盖）
  last_run_at TIMESTAMP,
  next_run_at TIMESTAMP,
  cursor      TEXT NOT NULL DEFAULT '{}',   -- JSON 游标（sync-architecture §4）
  fingerprint TEXT NOT NULL DEFAULT '',     -- 全量域的规范化指纹
  consecutive_failures INTEGER NOT NULL DEFAULT 0,
  last_error_class TEXT NOT NULL DEFAULT '',
  UNIQUE (site_id, domain)
);
```

> 多实例取任务用 CAS：`UPDATE site_sync_state SET next_run_at=? WHERE id=? AND next_run_at=?`（旧值条件更新，无版本列），返回 RowsAffected=0 即被其他实例抢走。

### 2.2 usage_logs —— 调用明细（增量同步的落点）

```sql
CREATE TABLE usage_logs (
  id          TEXT PRIMARY KEY,             -- uuid
  site_id     TEXT NOT NULL,
  remote_ref  TEXT NOT NULL,                -- 上游唯一 id（幂等键）
  site_key_id TEXT,                         -- FK site_keys.id（可空：上游可能不回 key 关联）
  model       TEXT NOT NULL DEFAULT '',
  grp         TEXT NOT NULL DEFAULT '',
  prompt_tokens     INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  cost        REAL NOT NULL DEFAULT 0,      -- 站点余额单位原始值
  currency    TEXT NOT NULL DEFAULT 'quota',
  status      TEXT NOT NULL DEFAULT 'success',
  occurred_at TIMESTAMP NOT NULL,
  synced_at   TIMESTAMP NOT NULL,
  UNIQUE (site_id, remote_ref)
);
CREATE INDEX idx_usage_logs_site_time  ON usage_logs (site_id, occurred_at);
CREATE INDEX idx_usage_logs_site_key   ON usage_logs (site_key_id, occurred_at);
CREATE INDEX idx_usage_logs_site_model ON usage_logs (site_id, model, occurred_at);
```

### 2.3 usage_daily —— 日聚合（查询加速 + 长历史压缩）

```sql
CREATE TABLE usage_daily (
  id          TEXT PRIMARY KEY,
  site_id     TEXT NOT NULL,
  day         DATE NOT NULL,                -- 站点本地时区的自然日
  requests    INTEGER NOT NULL DEFAULT 0,
  prompt_tokens     INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  cost        REAL NOT NULL DEFAULT 0,
  currency    TEXT NOT NULL DEFAULT 'quota',
  source      TEXT NOT NULL DEFAULT 'aggregated',  -- aggregated(本地汇总) | upstream(上游日接口)
  synced_at   TIMESTAMP NOT NULL,
  UNIQUE (site_id, day, currency)
);
```

- 双来源：优先用上游日接口（newapi `/api/data/self`、sub2api trend/daily）；上游没有时由 usage_logs 本地汇总。
- `(site_id, day, currency)` 唯一：不同币种并存，不折算。

### 2.4 site_models —— 模型/价格快照

```sql
CREATE TABLE site_models (
  id          TEXT PRIMARY KEY,
  site_id     TEXT NOT NULL,
  model       TEXT NOT NULL,
  grp         TEXT NOT NULL DEFAULT '',
  ratio       REAL,
  endpoint    TEXT NOT NULL DEFAULT '',     -- chat|embedding|image|...
  vendor      TEXT NOT NULL DEFAULT '',
  enabled     INTEGER NOT NULL DEFAULT 1,
  UNIQUE (site_id, model, grp)
);
```

### 2.5 site_subscriptions —— 订阅状态

```sql
CREATE TABLE site_subscriptions (
  id          TEXT PRIMARY KEY,
  site_id     TEXT NOT NULL,
  plan_name   TEXT NOT NULL DEFAULT '',
  state       TEXT NOT NULL DEFAULT 'active',   -- active|expired|cancelled
  started_at  TIMESTAMP,
  expires_at  TIMESTAMP,
  progress    TEXT NOT NULL DEFAULT '{}',       -- JSON：用量进度（sub2api /subscriptions/progress）
  raw_snapshot_id TEXT,                         -- FK snapshots.id
  UNIQUE (site_id)                              -- 每站点只保留当前订阅摘要（多订阅时取 active 优先，其次 expires_at 最大；完整列表走 snapshots）
);
```

### 2.6 site_account —— 账号余额投影

```sql
CREATE TABLE site_account (
  site_id       TEXT PRIMARY KEY,
  balance       REAL,
  used          REAL,
  currency      TEXT NOT NULL DEFAULT 'quota',
  unit_note     TEXT NOT NULL DEFAULT '',
  subscription_state TEXT NOT NULL DEFAULT '',  -- 冗余便于列表页快速展示
  freshness     TEXT NOT NULL DEFAULT 'missing',
  last_sync_at  TIMESTAMP
);
```

### 2.7 site_announcements —— 公告

```sql
CREATE TABLE site_announcements (
  id         TEXT PRIMARY KEY,
  site_id    TEXT NOT NULL,
  remote_ref TEXT NOT NULL DEFAULT '',          -- 上游公告 id（newapi /api/notice 单条则用 hash）
  title      TEXT NOT NULL DEFAULT '',
  content    TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL,                   -- 去重与变更检测
  published_at TIMESTAMP,
  UNIQUE (site_id, content_hash)
);
```

## 3. 写入模式（归一化 upsert + 快照留证）

统一写入流程（扩展现有 `SyncSite` 分域函数化）：

```
pull (adapter) ──► normalize ──► fingerprint 比对 ──┬─ 相同：只更新 site_sync_state.last_run_at
                                                   └─ 不同：
                                                        1. upsert 投影表（幂等，UNIQUE 键冲突即更新）
                                                        2. 写 snapshots（脱敏后 body，kind=domain@remote）
                                                        3. 更新投影行 freshness='fresh', last_sync_at=now
                                                        4. 推进游标（usage_log 域）
                                                        5. emit event
```

关键规则：

1. **幂等键**：所有投影表以 `(site_id, remote_*_id)` 或 `(site_id, remote_ref)` 为幂等键；usage_logs 的 `remote_ref` 缺失时退化为 `sha256(model|occurred_at_unix|tokens|cost)`（写入前算好存列）。注意：同秒同参数的多次调用会合并为一条，明细计数偏少可接受（该退化仅在 newapi 日志缺 `request_id` / sub2api usage 缺 `id` 时触发，两站实测均返回 id，此为兜底路径）。借鉴 ai-gateway：站点凭据解绑重连场景下唯一键带 `WHERE deleted_at IS NULL`（partial unique index，SQLite 3.8+ 支持），配合软删除避免重连撞约束——keys/groups/keys 类投影如需软删时按此实现。
2. **freshness 生命周期**：同步成功 → fresh；到 `interval×2` 未成功 → stale（由调度器 tick 时批量降级，避免查询时实时计算）；站点删除 → 级联清空投影（沿用现有 DeleteSite 行为并补新表：新表一律加 `REFERENCES sites(id) ON DELETE CASCADE`，与 0001 约定一致）。
3. **快照留存**：每域每次「内容有变化」的同步写一条 snapshot（kind=`keys`/`groups`/…），保留最近 N=20 条/站点/域，超出滚动删除（调度器内顺手做）。**敏感分级（借鉴 ai-gateway 三级留证）**：含 key 掩码/凭据指纹的 kind（keys/affiliates）标 sensitivity=high，仅站点元数据的 kind（models/announcements）标 low，便于按级收紧查询权限。
4. **删除传播**：全量域（keys/groups/models）同步后，对本地有而远端无的行做软删除标记（`status='missing'` 或直接删行）。keys 采用删行 + event 记录（含名称），避免幽灵 Key。
5. **时区口径**：usage_daily 的 `day` 按**站点本地时区**（sub2api 请求参数带 `timezone`，newapi 秒级时间戳按 UTC 落 usage_logs、日聚合时按站点配置时区划分）。站点配置存 `sites.sync_cfg.timezone`（缺省 UTC）。
6. **绝不存明文 key**；usage_log 不落请求/响应内容（上游日志接口本就不含 body）。

## 4. 查询与前端消费

| 前端功能 | 查询 | 支撑索引 |
|---|---|---|
| Dashboard 余额卡片 | `site_account` 全量 | PK |
| Dashboard 用量趋势（7/30 日） | `usage_daily` 按天聚合，多站点按 currency 分组 | `(site_id, day)` |
| Key 详情用量 | `usage_daily`（全局）+ `usage_logs`（近 24h 明细） | idx_usage_logs_site_key |
| 模型广场（跨站比价） | `site_models` join sites | `(site_id, model)` |
| 事件中心 | `events`（已有） | — |

API 层新增（httpx/router.go）：`GET /api/v1/usage/daily?days=30`、`GET /api/v1/models`、`GET /api/v1/subscriptions`、`GET /api/v1/account`（均为本地库读，不触发上游）。

## 5. 数据保留与清理策略

| 数据 | 保留 |
|---|---|
| usage_logs 明细 | 90 天滚动（每日调度器清理） |
| usage_daily | 永久（体量可控） |
| snapshots | 20 条/站点/域滚动 |
| events | 30 天滚动 |
| aff_transfers | 永久（不可逆操作留证） |

清理任务并入调度器每日 tick（低峰执行，每批 ≤1000 行防锁表）。

## 6. 风险与对策

| 风险 | 对策 |
|---|---|
| 上游接口版本漂移（字段变更） | parse 错误 → 全量快照 + error event；adapter 解析宽容（json 宽松 unmarshal 沿用现状） |
| 限流封禁（尤其 newapi CriticalRateLimit） | budgets 令牌桶 + 429 Retry-After 尊重；明文 key 查询接口同步链路永不调用 |
| 游标丢失重复拉取 | usage_logs UNIQUE 幂等 + 游标只在成功后推进 |
| 多实例部署重复调度 | site_sync_state 行级乐观锁（next_run_at 带版本条件更新），单实例假设下天然安全 |
| quota/USD 混算 | 全链路 currency 字段强制携带，聚合按币种分组，不做汇率折算 |
