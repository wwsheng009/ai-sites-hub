# 架构设计 — ai-sites-client

> 版本：v0.2（2026-09-19）
> 前置文档：`../requirements/requirements.md`（FR/NFR 编号在本文引用）、`../analysis/`（三份上游分析）
> 设计原则：单体 Go 服务 + 内置调度器；站点类型差异全部收敛在 adapter 之后；三入口（HTTP/cmd/Web）共用 service 层。

## 1. 总体架构

```
┌────────────────────────────────────────────────────────────────────┐
│                        ai-sites-client 单二进制                      │
│                                                                    │
│  ┌─────────────┐   ┌──────────────────────────────┐                │
│  │ Web 前端     │   │ HTTP API (Gin /api/v1)       │                │
│  │ (Vue3+TW    │──▶│  sites / keys / groups /     │──┐             │
│  │  复用sub2api │   │  checkins / jobs / events    │  │             │
│  │  骨架)      │   └──────────────────────────────┘  │             │
│  └─────────────┘                                     ▼             │
│                                    ┌──────────────────────────────┐│
│  ┌─────────────┐                   │        Service 层             ││
│  │ cmd (cobra) │──────────────────▶│  SiteService / SyncService    ││
│  │ sites/checkin│                   │  CheckinService / JobService  ││
│  │ /keys/serve  │                   │  AuthService(平台自身)         ││
│  └─────────────┘                   └──────────────┬───────────────┘│
│                                                   │                │
│                       ┌───────────────────────────┼──────────┐     │
│                       ▼                           ▼          ▼     │
│            ┌──────────────────┐        ┌──────────────┐ ┌────────┐ │
│            │ SiteAdapter 接口  │        │ 调度器        │ │ 存储层  │ │
│            │ ├─ sub2api       │        │ once/interval │ │SQLite  │ │
│            │ ├─ new-api       │        │ /cron + 抖动  │ │迁移文件 │ │
│            │ └─ unknown       │        └──────────────┘ └────────┘ │
│            └────────┬─────────┘                                    │
└─────────────────────┼──────────────────────────────────────────────┘
                      ▼  HTTPS（全局限速/退避/预算）
        ┌─────────────────────────────┐
        │ 上游站点（各自独立部署）        │
        │  · sub2api 实例 (/api/v1/…)  │
        │  · new-api 实例 (/api/…)     │
        └─────────────────────────────┘
```

要点：

1. **不代理 LLM 流量**：与 ai-gateway 区分，只做管理面。
2. **Adapter 是唯一知道站点类型的层**；service/存储/前端/cmd 均面向统一模型。
3. **调度器内置**，HTTP 与 cmd 都可同步触发同一 service 方法。

## 2. 目录结构（Go 工程骨架）

```
ai-sites-client/
├── cmd/aiclient/              # 单一入口（cobra）
│   └── main.go
├── internal/
│   ├── server/                # HTTP 层
│   │   ├── router.go          # SetupRouter + 路由注册（参考 sub2api 模式）
│   │   ├── middleware/        # jwt_auth / audit / ratelimit
│   │   └── routes/            # auth.go / sites.go / keys.go / jobs.go ...
│   ├── handler/               # HTTP handler（薄层，转调 service）
│   ├── service/               # 业务层
│   │   ├── site.go            # 站点 CRUD/识别/状态
│   │   ├── sitedetect/        # 自动识别（打分器，参考 ai-gateway）
│   │   ├── sync.go            # key/分组/额度同步编排
│   │   ├── checkin.go         # 签到编排与历史
│   │   ├── job/               # 作业框架：注册表/调度/执行记录/lease
│   │   ├── events.go          # 事件流水
│   │   └── notify/            # 通知：Emit/规则匹配/渠道注册表/投递（§11，M4.5）
│   ├── adapter/               # 站点类型适配层（唯一感知站点类型）
│   │   ├── adapter.go         # SiteAdapter 接口 + 注册表 + 错误类型
│   │   ├── sub2api/           # client.go / auth.go / keys.go / detect.go
│   │   └── newapi/            # client.go / auth.go / keys.go / checkin.go / detect.go
│   ├── repo/                  # 数据访问（database/sql + sqlc 或 GORM，二选一见 §7）
│   ├── model/                 # 实体与 DTO
│   ├── store/                 # SQLite 连接、迁移执行、加密封装
│   ├── config/                # YAML + env 加载
│   └── secret/                # AES-GCM 加解密（凭据静态加密）
├── frontend/                  # 复制自 sub2api frontend 骨架（Vue3+Vite+TW）
├── migrations/                # SQLite 迁移 SQL（单方言）
├── configs/config.example.yaml
├── docs/                      # 本文档中心
└── Makefile
```

## 3. 核心数据模型（SQLite）

> 借鉴 ai-gateway 的 Snapshot/Projection 分离，但做减法：一张快照表 + 投影字段内联，避免实体过胖。

```sql
-- 站点
sites(id PK, name, base_url UNIQUE, api_base_url, site_type CHECK(sub2api|new-api|unknown),
      status CHECK(active|disabled|archived), detect_score INT, detect_report JSON,
      checkin_cfg JSON,          -- {enabled, mode, jitter_minutes}
      last_probe_at, latency_ms, created_at, updated_at)

-- 凭据（ secrets 加密存 cipher 列；投影只存指纹/状态）
site_credentials(id PK, site_id FK, auth_mode CHECK(username_password|token|hybrid),
      username, password_cipher, totp_secret_cipher,
      access_token_cipher, refresh_token_cipher, pat_cipher,
      auth_state CHECK(ok|login_failed|totp_required|turnstile_blocked|cf_blocked|
                       token_expired|refresh_failed),
      auth_state_msg, last_login_at, token_expire_hint, updated_at)

-- key 归一化投影（展示与查询主表）
site_keys(id PK, site_id FK, remote_key_id, name, grp, status,
      quota_limit, quota_used, quota_remaining, unlimited BOOL,
      expires_at, key_source CHECK(masked|plaintext), raw_snapshot_id FK,
      freshness CHECK(fresh|stale|missing), last_sync_at, UNIQUE(site_id, remote_key_id))

-- 分组/倍率投影
site_groups(id PK, site_id FK, remote_group_id, name, ratio REAL,
      available BOOL, desc, freshness, last_sync_at)

-- 签到记录
checkins(id PK, site_id FK, date, state CHECK(success|already|failed|skipped|
      unsupported|blocked_by_turnstile), quota_awarded, message, created_at,
      UNIQUE(site_id, date))

-- 作业与执行记录
jobs(id PK, name UNIQUE, schedule_type CHECK(interval|cron|manual), spec, enabled,
      jitter_seconds, last_run_id, next_run_at)
job_runs(id PK, job_id FK, site_id NULL FK, state CHECK(running|success|failed|
      skipped), error_class, message, started_at, finished_at, dry_run BOOL)

-- 快照（上游原始响应，用于回放/排障；payload 脱敏后存储）
snapshots(id PK, site_id FK, kind CHECK(detect|keys|groups|checkin|probe),
      status_code, body JSON, fetched_at)

-- 平台事件
events(id PK, site_id NULL FK, type, level, message, data JSON, created_at)

-- 站点返利投影（FR-10；只读展示，无凭据；供跨站汇总与划转决策）
site_affiliates(id PK, site_id UNIQUE FK,
      aff_code, rebate_rate REAL,             -- 上游比例；缺失时 NULL + freshness=stale
      available REAL, frozen REAL, history REAL, -- 统一换算为站点余额单位（USD/quota 由 adapter 换算约定给出）
      invitee_count INT,
      aff_cfg JSON,                            -- 自动划转规则：{enabled(默认 false), min_amount, max_per_transfer, daily_limit, last_transfer_date}
      freshness CHECK(fresh|stale|missing), last_sync_at, updated_at)

-- 划转记录（FR-10.4；不可逆操作留证）
aff_transfers(id PK, site_id FK, amount REAL, state CHECK(pending|success|failed|dry_run),
      trigger_type CHECK(manual|auto_rule), rule_snapshot JSON, amount_before REAL,
      error_class, message, created_at, finished_at)
```

安全列说明：所有 `*_cipher` 列为 AES-GCM 密文（`internal/secret`），密钥来自 `config.security.master_key`（env 覆盖 `AISC_MASTER_KEY`）；日志与 API 默认打码。

## 4. 站点适配层（核心设计）

### 4.1 接口

```go
// internal/adapter/adapter.go
type SiteAdapter interface {
    Type() Type // sub2api | new-api

    // 识别与探测（公开端点，无需凭据）
    Detect(ctx context.Context, baseURL string) (DetectResult, error) // score/特征证据

    // 认证（凭据阶梯 + 状态机，见 FR-2）
    Login(ctx context.Context, cred Credentials) (AuthState, error)
    Refresh(ctx context.Context, cred Credentials) (AuthState, error) // sub2api RT；new-api PAT 无 RT
    Verify(ctx context.Context, a AuthCtx) error                      // 轻量校验登录态

    // 能力查询（各 adapter 自报能力，上层按能力跳过）
    Capabilities() Capabilities // Checkin: supported/unsupported/blocked_by_turnstile

    // 数据同步
    ListKeys(ctx context.Context, a AuthCtx, page Page) (KeyPage, error)
    ListGroups(ctx context.Context, a AuthCtx) ([]Group, error)
    Quota(ctx context.Context, a AuthCtx) (AccountQuota, error) // 账号级额度

    // 站点返利（FR-10；能力位 Capabilities.Affiliate gate，unsupported 返回错误不再重试）
    AffiliateInfo(ctx context.Context, a AuthCtx) (AffiliateInfo, error) // 归一化：code/rate/available/frozen/history/invitees
    TransferAffiliate(ctx context.Context, a AuthCtx, amount Amount) (TransferResult, error) // amount 携带单位与换算约定；不可逆操作，作业层默认 dry_run

    // 签到（仅 new-api 实现）
    CheckinStatus(ctx context.Context, a AuthCtx) (CheckinStatus, error)
    Checkin(ctx context.Context, a AuthCtx) (CheckinResult, error)
}
```

### 4.2 两类 adapter 的契约映射（来源：analysis/ 文档）

| 统一能力 | sub2api adapter | new-api adapter |
| --- | --- | --- |
| Detect | `GET /setup/status`、`GET /api/v1/settings/public` 特征 | `GET /api/status`（含 version/system_name）特征 |
| Login | `POST /api/v1/auth/login`（2FA: `/login/2fa`） | `POST /api/user/login`（+`new_api_refresh` cookie） |
| Refresh | `POST /api/v1/auth/refresh`（RT 轮换） | JWT 15min：`/api/user/auth/refresh`（cookie）；**首选 PAT 长期有效** |
| Verify | `GET /api/v1/auth/me` | `GET /api/user/self` |
| ListKeys | `GET /api/v1/keys`（分页） | `GET /api/token/`（打码）+ 可选 `POST /api/token/:id/key` 取明文 |
| ListGroups | `GET /api/v1/groups/available` + `groups/rates` | `GET /api/user/self/groups` + `GET /api/ratio_config` |
| Quota | `GET /api/v1/user/platform-quotas` | `GET /api/usage/token/`（sk-）或 `user/self` quota |
| AffiliateInfo | `GET /api/v1/user/aff`（aff_code/rate/frozen/邀请人明细；ai-gateway client.go:517 同款调用） | `GET /api/user/aff` + `GET /api/user/self`（aff_quota/aff_history_quota/aff_count/inviter_id） |
| TransferAffiliate | `POST /api/v1/user/aff/transfer`（routes/user.go:35） | `POST /api/user/aff_transfer`（body `{quota}`，上游原始 quota 单位） |
| Checkin | — Capabilities 返回 `unsupported` | `GET/POST /api/user/checkin`；Turnstile → `blocked_by_turnstile` |

### 4.3 统一错误与防护

- 错误类型：`AdapterError{Code: unauthorized|forbidden|rate_limited|blocked_by_cf|turnstile_required|totp_required|upstream_error|unsupported}`（参考 ai-gateway 错误分类客户端），`errors.As` 判定。
- 统一 HTTP 客户端：UA 标识、超时、**响应体上限（默认 2MB）**、429/5xx 指数退避、每站点串行 + 全局并发上限（默认 4）、按小时探测预算。

### 4.4 自动识别打分器（`internal/service/sitedetect`）

- 输入：base URL。步骤：抓 `/`、`/api/status`、`/setup/status`、`/api/v1/settings/public`、`/api/notice` 等；解析 JSON 关键字段与 HTML 特征（title、静态资源、i18n key）。
- 打分：各特征加权求和，`score>=6` 且唯一最高者判定类型（对齐 ai-gateway 阈值）；并列/低分 → `unknown` + 报告留证（写入 `sites.detect_report` 与 snapshots）。
- 人工覆盖优先：`site_type` 手动指定后跳过自动识别。

## 5. 后台作业框架（`internal/service/job`）

- **调度**：进程内 ticker 秒级扫描 `jobs.next_run_at`；`interval` 与 5 字段 `cron`（带时区）两种 spec；每次执行叠加 `jitter_seconds` 随机抖动；**全项目只用这一个调度实现**（吸取 ai-gateway 双实现教训）。
- **执行**：单进程串行队列（站点级并发见 §4.3）；执行写 `job_runs`（含 `dry_run`）；失败分类 `error_class` 供重试策略。
- **注册的内置作业**（`catalog.go`，参考 ai-gateway catalog 模式）：

| 作业名 | 默认调度 | 作用 | 对应 FR |
| --- | --- | --- | --- |
| `probe.sites` | interval 10m | 站点连通性/延迟 | FR-5.1 |
| `auth.refresh` | interval 30m | 登录态维持/token 续期 | FR-2.2/2.3 |
| `sync.keys_groups` | interval 60m（抖动 0–600s） | key/分组/额度同步（含 FR-10.1 返利概览，站点级可关） | FR-4.1–4.4、FR-10.1–10.2 |
| `checkin.daily` | cron `30 8 * * *`（站点级抖动 ±30m） | 签到 | FR-3.1–3.4 |
| `affiliate.transfer` | cron `0 10 * * *`（默认 dry_run=true） | 阈值自动划转（规则默认关，单日频控/上限） | FR-10.4 |
| `housekeeping.snapshots` | cron `0 4 * * *` | 清理过期快照/事件 | NFR-7 |

- **安全默认**：`sync.*` 与 `checkin.daily` 默认 `dry_run=true`（首次启动不外呼），确认配置后由 cmd `jobs run` 或前端开启；作业结果不含明文凭据（NFR-3）。
- 单机部署无 lease 竞争，但 `job_runs` 保留 `lease_until` 字段预留多实例（二期）。

## 6. HTTP API 设计（本平台 `/api/v1`）

统一响应 `{code, message, data}`；错误码沿用 AdapterError 分类。路由按域文件注册（参考 sub2api `routes/` 模式）。

| 域 | 端点 | 说明 |
| --- | --- | --- |
| auth | `POST /auth/login`、`POST /auth/refresh`、`GET /auth/me` | 平台管理员 JWT（NFR：预留 PAT） |
| sites | `GET/POST /sites`、`GET/PUT/DELETE /sites/:id`、`POST /sites/:id/detect` | CRUD + 触发识别（FR-1） |
| credentials | `PUT /sites/:id/credentials`、`GET /sites/:id/auth-state`、`POST /sites/:id/auth-test` | 凭据写入（密文落库）、状态机读取、登录测试 |
| sync | `POST /sites/:id/sync`、`POST /sync/all` | 手动同步（FR-4） |
| keys | `GET /keys?site_id=&group=&status=&expiring=&low_quota=&q=`、`GET /keys/:id`、`POST /keys/:id/reveal` | 跨站汇总（FR-4.2/4.5）；reveal 需二次确认+审计 |
| groups | `GET /groups?site_id=` | 分组倍率汇总 |
| checkins | `GET /checkins?site_id=&month=`、`POST /checkins/run` | 历史 + 手动签到（FR-3） |
| jobs | `GET /jobs`、`POST /jobs/:name/run`、`GET /job-runs?job_name=` | 作业管理（§5） |
| events | `GET /events?since=&site_id=` | 事件流水（FR-5.3） |
| affiliates | `GET /affiliates?site_id=&q=`、`GET /affiliates/summary`、`PUT /affiliates/:id/config`（自动划转规则）、`POST /affiliates/:id/transfer {amount}` | 返利汇总/规则配置/手动划转（FR-10.3/10.4；确认弹窗+审计，不可逆） |
| notifications | `GET/POST /notifications/channels`、`PUT/DELETE /notifications/channels/:id`、`POST /notifications/channels/:id/test`、`GET/POST /notifications/rules`、`POST /notifications/rules/:id/test-match`、`GET /notifications/deliveries` | 通知渠道/策略/投递记录（§11，FR-9） |
| system | `GET /health`、`GET /doctor` | 健康检查/体检报告 |

## 7. 技术选型决策

| 决策点 | 选择 | 理由 |
| --- | --- | --- |
| HTTP 框架 | Gin | 三个参考项目一致；中间件生态成熟 |
| 数据访问 | GORM + SQLite（纯 Go driver: glebarez/sqlite） | 单二进制零 CGO 依赖（Windows 友好）；迁移用 SQL 文件手写，不用 AutoMigrate 管理演进 |
| 调度 | 自研 ticker + cron 表达式解析（单实现） | 需求简单（每日签到+轮询），引入 robfig/cron 亦可，但必须单点实现 |
| 凭据加密 | AES-256-GCM，`internal/secret` | 对齐 ai-gateway secretCipher；主密钥 env/配置注入 |
| cmd | cobra | ai-gateway 验证过的多子命令模式 |
| 配置 | Viper（YAML + env 覆盖） | sub2api 同款 |
| 日志 | zap + 脱敏钩子 | 全家桶一致 |
| 前端 | Vue3 + TS + Vite + Pinia + Tailwind（复制 sub2api 骨架） | 用户指定；无组件库骨架最轻 |

## 8. 前端架构（复用 sub2api 骨架）

- 复制 `frontend/` 工程文件与 `src/` 基建（清单见 `../analysis/sub2api-analysis.md` §3），删除业务页面与 payment/admin 等无关域，保留：
  - `api/client.ts`（axios + token 刷新拦截器）改造指向本平台 `/api/v1`
  - `router/index.ts`、`stores/auth.ts`、`stores/app.ts`、i18n、layout 与通用组件、Tailwind 样式体系
- 新增页面（FR-8.2）：`views/sites/`（列表+新增向导+详情）、`views/keys/`（跨站汇总）、`views/jobs/`、`views/dashboard/`
- 路由守卫沿用 `requiresAuth`；暗色模式沿用 Tailwind 方案。

## 9. 关键时序

### 9.1 站点接入（新增向导）

```
前端/cmd → POST /sites {name, base_url}
        → SiteService.Detect: sitedetect 打分 → 更新 site_type/detect_report
        → PUT /sites/:id/credentials {auth_mode, secrets}
        → POST /sites/:id/auth-test: adapter.Login → auth_state=ok
        → POST /sites/:id/sync: 拉取 keys/groups/quota → 投影落库
```

### 9.2 每日签到（checkin.daily）

```
调度器到点(+抖动) → 枚举 active 站点 → adapter.CheckinStatus
  ├─ unsupported(sub2api) → checkins.state=skipped
  ├─ disabled/turnstile → state=blocked_by_turnstile，加入待人工清单
  └─ enabled → adapter.Checkin → success{quota_awarded} / already
→ 写 checkins + events → 事件可被前端/cmd 查询
```

### 9.3 登录态维持（auth.refresh）

```
PAT 模式(new-api)：Verify 失败→ auth_state=login_failed → 事件告警（PAT 不过期）
JWT/token 模式：Verify 失败→ 尝试 Refresh → 成功更新密文
                              └ 失败→(hybrid)密码重登→仍失败→ auth_state=refresh_failed + 事件
```

### 9.4 站点返利同步与自动划转（FR-10）

```
sync.keys_groups（含返利）→ adapter.AffiliateInfo → 投影落库 site_affiliates + snapshot
  └─ Capabilities.Affiliate=unsupported → freshness=missing，跳过不重试

affiliate.transfer（默认 dry_run=true）→ 遍历 aff_cfg.enabled=true 且 available ≥ min_amount 的站点
  ├─ 频控检查（单站点单日 1 次 + 全局单日上限 + 单笔上限）不过 → 记录原因，跳过
  ├─ dry_run=true → 只写 aff_transfers(state=dry_run) + 事件，不调上游
  └─ 执行 → adapter.TransferAffiliate → success/failed → aff_transfers + 事件
       └─ 事件可被 FR-9 规则通知（划转失败即告警）
```

## 10. 与三个参考项目的关系总结

| 参考项目 | 取什么 | 不取什么 |
| --- | --- | --- |
| sub2api | 前端骨架（Vue3+TW）、routes 注册模式、JWT 双 token 对接契约、affiliate 用户侧契约（aff 概览+划转） | PostgreSQL/Redis/Ent、业务后端功能、站长侧 affiliate 管理 |
| new-api | 签到/PAT/token 管理对接契约、系统任务框架思路、aff 概览/划转契约（quota 单位） | React 前端、渠道管理功能 |
| ai-gateway | AI Sites 全套设计模式（识别打分/凭据阶梯/状态机/快照投影/作业 catalog/预算限速）、cobra cmd 组织、GetAffiliateInfo 客户端调用范式 | 多数据库迁移、电商/商城、LLM 代理等无关域 |

## 11. 通知模块（FR-9，参考 `../analysis/ai-gateway-notification.md`）

采用 ai-gateway 验证过的 **事件→策略→渠道→投递** 四段式，做单管理员减法。

### 11.1 数据模型（在 §3 基础上新增 3 表，events 复用）

```sql
-- 通知渠道（webhook/SMTP 凭据用 *_cipher 加密，复用 internal/secret）
notification_channels(id PK, name UNIQUE, type CHECK(webhook|email), enabled BOOL,
      config JSON,                -- 非敏感字段：url 路径、smtp_host/port/network/from/to
      secrets_cipher,             -- 敏感字段整体加密：webhook HMAC secret、smtp password
      created_at, updated_at)

-- 通知策略（事件类别 × 级别 × 站点 → 渠道）
notification_rules(id PK, name, enabled BOOL, event_type TEXT,     -- 精确匹配或 `*` 通配
      min_level CHECK(info|warn|error), site_id NULL FK,          -- NULL = 全部站点
      channel_ids JSON, cooldown_seconds INT DEFAULT 0,           -- 静默窗口（同类事件冷却）
      created_at, updated_at)

-- 投递记录（outbox 模式；由 notification.dispatch 作业异步消费重试）
notification_deliveries(id PK, event_id FK, rule_id FK, channel_id FK,
      state CHECK(pending|succeeded|failed|suppressed),           -- suppressed=冷却窗口内跳过
      attempts INT DEFAULT 0, next_retry_at NULL, last_error,
      created_at, sent_at)
```

### 11.2 组件与流程

```
events 写入点(签到/登录/探测/同步) ──同步调用──▶ notify.Emit(event)
                                                   │ 规则匹配(含 cooldown 检查)
                                                   ├─ 命中 → 为每个 channel 建 deliveries(pending)
                                                   └─ 未命中 → 无投递记录（匹配结果可在规则测试接口回放）
notification.dispatch 作业(默认 30s 间隔) ──▶ 取 pending/到期重试 → Notifier.Send
                                                   ├─ 成功 → succeeded
                                                   └─ 失败 → attempts+1, 指数退避 next_retry_at(最多 5 次)
```

- **渠道注册表**（取 ai-gateway `deliver()` 的 switch 思路，改为注册表）：`Notifier` 接口 `Send(ctx, msg) error`；一期实现 `WebhookNotifier`（POST JSON + `X-AISC-Signature: HMAC-SHA256`，2xx 即成功）、`EmailNotifier`（`net/smtp`，网络模式 ssl/starttls 可配，错误信息脱敏——范式见 ai-gateway `annotateEmailSMTPSendError`）
- **TestSend**：`POST /api/v1/notifications/channels/:id/test` → 构造合成事件直发，不落 deliveries（对齐 FR-9.3）
- **匹配追溯**：`POST /api/v1/notifications/rules/:id/test-match {event}` 返回"是否命中/被冷却抑制/将投递到哪些渠道"（FR-9.5）

### 11.3 与后台作业框架的关系

`notification.dispatch` 作为第 6 个内置作业进入 catalog（`schedule_type=interval, spec=30s`），复用 job_runs 记录；cooldown 抑制结果 `state=suppressed` 不重试。

### 11.4 安全约束（NFR-3）

- `secrets_cipher` 与凭据同套 AES-GCM；渠道配置 API 返回时敏感字段仅回显掩码（`sk-***` / `******`）
- 投递 payload 只含事件元数据（site_name/type/level/message/时间），不含凭据与明文 key；webhook 签名密钥不进日志
- 单元测试继承 ai-gateway 的 not-leak 断言（诊断输出不得含 secret/webhookSecret 片段）
