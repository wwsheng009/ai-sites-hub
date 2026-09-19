# 实施计划 — ai-sites-client

> 版本：v0.2（2026-09-19）
> 前置：需求已冻结（`../requirements/requirements.md`），架构已定稿（`../architecture/architecture.md`）
> 估时基准：单人全职；所有阶段交付可运行/可验证的增量。

## 0. 里程碑总览

| 里程碑 | 内容 | 预估 | 退出标准 |
| --- | --- | --- | --- |
| M1 | 需求与设计冻结 | 已完成（本文档链评审通过即冻结） | 文档评审通过 |
| M2 | 服务端骨架：工程/配置/存储/站点 CRUD+识别 | 3d | `sites add/detect` 可用，doctor 通过 |
| M3 | 适配器与凭据：sub2api/new-api 登录与状态机 | 4d | 真实站点登录成功，凭据密文落库 |
| M4 | 同步与签到作业（含 FR-10 返利，优先） | 4.5d | 定时同步 key/分组/额度/返利；new-api 签到成功；affiliate.transfer 可运行 |
| M4.5 | 通知模块（FR-9） | 2d | 事件→策略→渠道→投递闭环；TestSend 成功；FR-10 划转告警联调 |
| M5 | cmd 完整化 | 2d | FR-7 命令全清单可用 |
| M6 | 前端（复用 sub2api 骨架） | 5d | 六个页面可用并对接 API |
| M7 | 打磨与收尾 | 2d | doctor e2e、导入导出、README/文档收尾 |

依赖链：M2 → M3 → M4（FR-10 返利任务 4.8–4.10 优先）→ M4.5（通知，依赖事件系统）→ M5/M6（可并行）→ M7。

## 1. M2 服务端骨架（3d）

| # | 任务 | 产出 |
| --- | --- | --- |
| 2.1 | Go 工程初始化：`cmd/aiclient`、cobra、zap、Viper 配置加载（YAML+env） | `go build` 通过；`aiclient version` |
| 2.2 | SQLite（glebarez/sqlite）+ 迁移执行器 + 初始 schema（architecture §3 全表） | `aiclient migrate` |
| 2.3 | `internal/secret` AES-GCM + 主密钥加载（env `AISC_SECURITY_MASTER_KEY`） | 单元测试加解密 |
| 2.4 | 站点 CRUD service + `/api/v1/sites` 路由 + 平台管理员 JWT（首版单用户） | curl 增删改查通过 |
| 2.5 | `sitedetect` 打分器 + `POST /sites/:id/detect`；`GET /api/status`、`/setup/status` 特征样本测试（可用录制 JSON 固定样本） | 对两类样本站点识别正确 |
| 2.6 | `serve` 子命令（router+静态托管 frontend/dist）+ `/health` | 浏览器访问 OK |
| 2.7 | `doctor` v1：配置/DB/密钥检查 | `aiclient doctor` 输出体检表 |

## 2. M3 适配器与凭据（4d）

| # | 任务 | 产出 |
| --- | --- | --- |
| 3.1 | `adapter` 接口 + 注册表 + `AdapterError` 分类 + 统一 HTTP 客户端（UA/超时/2MB 上限/退避） | 架构 §4 落码 |
| 3.2 | new-api adapter：Login（含 encryption-key 加密密码分支）/PAT/Refresh(cookie)/Verify | 对真实站点验证 |
| 3.3 | sub2api adapter：Login/2FA/Refresh/Verify | 对真实站点验证 |
| 3.4 | 凭据写入 API（密文）+ `auth-state` 读取 + `auth-test` | FR-2 全流程 |
| 3.5 | 凭据阶梯与状态机（token→refresh→重登；状态枚举落库） | 状态流转测试 |
| 3.6 | Turnstile/CF 拦截识别 → `turnstile_blocked/cf_blocked` 状态 | 模拟响应测试 |

## 3. M4 同步与签到作业（4d）

| # | 任务 | 产出 |
| --- | --- | --- |
| 4.1 | 作业框架（catalog/调度/抖动/job_runs/dry_run 默认）+ 5 个内置作业注册 | 架构 §5 落码 |
| 4.2 | `sync.keys_groups`：ListKeys/ListGroups/Quota + 归一化投影 + snapshot 落库 + freshness | FR-4.1–4.4 |
| 4.3 | new-api 明文 key 抓取开关（默认关）+ 打码/`reveal` API（审计事件） | FR-4.5 + NFR-3 |
| 4.4 | `checkin.daily`：CheckinStatus→Checkin→checkins 记录；`blocked_by_turnstile` 待人工清单 | FR-3 |
| 4.5 | `probe.sites`、`auth.refresh`、`housekeeping.snapshots` | FR-5 |
| 4.6 | 事件系统（events 写入点接入各 service）+ `GET /events` | FR-5.3 |
| 4.7 | 全局限速/预算实现 + 429 退避联调 | NFR-4 |
| 4.8 | 返利同步（FR-10.1/10.2，优先）：AffiliateInfo 契约 + 能力位 + site_affiliates 投影 + snapshot | aff list 跨站可见 |
| 4.9 | `affiliate.transfer` 作业（FR-10.4）：阈值/频控/单笔上限、默认 dry_run、aff_transfers 记录与事件 | dry_run 闭环验证 |
| 4.10 | `GET /affiliates*` API（汇总/config/transfer，确认+审计） | curl 验收 FR-10.3/10.4 |

## 3.5 M4.5 通知模块（2d，插在 M4 之后执行）

专项分析：`../analysis/ai-gateway-notification.md`；设计：architecture §11。

| # | 任务 | 产出 |
| --- | --- | --- |
| 4.5.1 | 通知 3 表迁移 + `internal/service/notify`（Emit/规则匹配/冷却抑制） | 匹配矩阵单元测试 |
| 4.5.2 | Notifier 注册表 + WebhookNotifier（HMAC-SHA256 签名）+ EmailNotifier（SMTP ssl/starttls，错误脱敏） | TestSend 对真实 webhook/邮箱成功 |
| 4.5.3 | `notification.dispatch` 作业接入 catalog（30s 间隔、指数退避、5 次上限） | 投递状态机测试 |
| 4.5.4 | HTTP API：channels/rules CRUD + test + test-match + deliveries 查询 | curl 验收 FR-9.5 |
| 4.5.5 | cmd `notify ...` 命令 + 前端通知设置页（渠道/策略/投递记录） | FR-9 收口 |

> 联调要求：FR-10.4 划转成功/失败事件在真实规则下各触发一次 webhook 投递（依赖 M4 任务 4.9）。

## 4. M5 cmd 完整化（2d）

命令清单（FR-7.2）逐个落码，全部走 service 层：

- `sites list|add|detect|enable|disable|remove|import|export`
- `auth login|status`、`checkin run|history`、`keys list|reveal`、`groups list`、`aff list|transfer`（FR-10）
- `sync run`、`jobs list|run|status`、`doctor`、`serve`

验收：脱离前端可完成「接入站点→同步→签到→查询」全流程（脚本化演示）。

## 5. M6 前端（5d，可与 M5 并行）

| # | 任务 | 产出 |
| --- | --- | --- |
| 6.1 | 复制 sub2api frontend 骨架（清单见 analysis §3），清理业务页面，`api/client.ts` 指向 `/api/v1` | `pnpm dev` 可登录平台 |
| 6.2 | 登录页 + 路由守卫 | — |
| 6.3 | 站点管理页（列表/徽标/新增向导：URL→识别→凭据→测试登录） | FR-8.2 |
| 6.4 | 站点详情页（凭据状态/key/分组/签到历史/返利卡片+手动划转/事件/手动按钮） | FR-10.3/10.4 |
| 6.5 | 全局 Keys 页（筛选/排序/打码+reveal 确认） | — |
| 6.6 | 作业页 + 总览仪表盘（含跨站返利汇总） | FR-10.3 |
| 6.7 | 暗色模式与 i18n 校对；`pnpm build` 产物嵌入 `serve` | NFR-1 |

## 6. M7 打磨与收尾（2d）

- `doctor` v2：各站点连通性/凭据状态/作业健康/affiliate 能力位汇总
- `sites import/export`（`--with-secrets` 显式确认）
- 快照脱敏复查（grep 密码字段模式）、限流压测抽查
- README（部署/配置/命令示例）、更新 docs 文档状态
- 回归：M2–M6 退出标准复核

## 7. 风险与对策（执行视角）

| 风险 | 阶段 | 对策 |
| --- | --- | --- |
| 无可用真实站点测试 | M3/M4 | 搭建本地 sub2api/new-api 容器（docker-compose）作为联调靶场；接口样本录制为固定 fixture |
| Turnstile 无法自动化 | M4 | 明确降级路径（待人工清单），不阻塞主流程 |
| 上游 API 字段漂移 | 全程 | adapter 字段容错 + snapshot 留证；fixture 版本对齐上游 VERSION |
| 前端骨架复制后样式缺失/依赖版本漂移 | M6 | 复制后先跑通 `pnpm dev` 原样页面再改造；锁定 pnpm-lock |
| 主密钥丢失导致凭据不可解 | M2 | `doctor` 检测解密能力；文档明确备份要求 |

## 8. 同步引擎演进（M8，M7 之后按 sync-architecture §7 分阶段执行）

> 设计与借鉴结论见 [sync-architecture.md](sync-architecture.md) 与 [ai-gateway-lessons.md](ai-gateway-lessons.md)；存储见 [storage-design.md](storage-design.md)（migration `0004_sync_ext`）。前端配套见 [frontend-roadmap.md](frontend-roadmap.md)。

| 阶段 | 核心任务（对齐 sync-architecture §7） | 关键借鉴项落地 |
| --- | --- | --- |
| S1 | site_sync_state 迁移 + scheduler 骨架 + account 域（sub2api Quota 补实现）+ newapi `quota_per_unit` 动态化（改造 `internal/adapter/newapi/keys.go:223`） | 防重入 singleflight、错误码类型化、分页防护 |
| S2 | usage_log 游标增量（newapi 秒级 / sub2api 按日）+ usage_logs 表 | 幂等键（含 remote_ref 哈希兜底）、at-least-once + 唯一键去重 |
| S3 | usage_daily 聚合 + Dashboard 数据 API | 时区口径、raw/numeric 双列 |
| S4 | models/pricing + subscription 同步 | 指纹降级、能力门控扩展 |
| S5 | announcements + 签到定时化 | ✅ 落地：0006 migration + ListAnnouncements + SyncCheckin + scheduler checkin/announcements 域 |

### S1 落地记录（已完成）

**已落代码**：
- `migrations/0004_sync_ext.sql` — `sites.sync_cfg` 增列、`site_sync_state` 表、`site_account` 表、`snapshots` 表重建扩展 `kind` 枚举。
- `internal/model/model.go` — `Site.SyncCfg`、`SiteSyncState`、`SiteAccount` 实体。
- `internal/repo/repo.go` — `ListSyncStateDue`/`UpsertSyncState`/`MarkSyncStateRun`/`UpsertAccount`/`GetAccount`/`ListAccounts`。
- `internal/adapter/errors.go` — `CodeOf`/`IsCode` 类型化判定（代替 `strings.Contains`）。
- `internal/adapter/newapi/quota.go`（新）+ `keys.go`/`affiliate.go` — `quota_per_unit` 动态读取 `/api/status`（缓存 + 500000 fallback），替换硬编码。
- `internal/adapter/sub2api/keys.go` — `Quota()` best-effort 接 `/api/v1/user/platform-quotas`，无契约回退占位。
- `internal/service/scheduler.go`（新） — tick 循环、site_sync_state 调度、singleflight per siteID:domain、2min 新鲜度窗口、指数退避（2^n 封顶 1h）、站点级令牌桶限流、按错误码分流。
- `internal/service/service.go` — `SyncAccount` + `AuthContext` 公开、`SyncSite` 接入 account 域。
- `internal/httpx/router.go` — `GET /api/v1/account`、`GET /api/v1/accounts`。
- `cmd/aiclient/...` + `svcwire/wire.go` — serve 启动/停止调度器。

**验收**：`go build ./...` + `go vet ./...` 均通过；`migrate` 成功应用 4 个迁移；`doctor` 连通 SQLite 并正常报告。下一站 S2（usage_log 游标增量）。

### S2 落地记录（已完成）

**已落代码**：
- `migrations/0005_usage_logs.sql` — `usage_logs` 表 + 双唯一键（`remote_ref` 优先 / 指纹哈希兜底）、`usage_daily` 日聚合表（S3 预留 schema）。
- `internal/model/model.go` — `UsageLog` 实体。
- `internal/adapter/adapter.go` — `UsageLog` 类型 + `UsageLogs(ctx, atx, since)` 接口 + `Capabilities.UsageLogs` 门。
- `internal/adapter/newapi/quota.go` — `UsageLogs` 实现 `GET /api/log/self?start_timestamp=<unix>`（秒级游标）。
- `internal/adapter/sub2api/usage.go`（新） — `UsageLogs` 实现 `GET /api/v1/usage?start_date=<date>`（按日游标）。
- `internal/repo/repo.go` — `GetSyncState`/`InsertUsageLogs`（分批 200 + OnConflict upsert）/`ListUsageLogs`。
- `internal/service/service.go` — `SyncUsageLog`（游标读 `site_sync_state.cursor` JSON `{"since"}`，写回最晚 ts）+ `SyncResult.UsageLogsSync`。
- `internal/service/scheduler.go` — `usage_log` 域加入 `ensureSiteDomains`/`syncDomain` 调度，401 冻结 credential。

**验收**：`go build ./...`（GOGC=40）、`go vet ./internal/...`、`go test ./internal/...` 均通过；`migrate` 成功应用 5 个迁移；`usage_logs`/`usage_daily` 表结构与 partial unique index 校验通过。下一站 S3（usage_daily 聚合 + Dashboard 数据 API）。

### S3 落地记录（已完成）

**已落代码**：
- `internal/model/model.go` — `UsageDaily` 实体。
- `internal/repo/repo.go` — `AggregateUsageDaily`（`INSERT ... GROUP BY ... ON CONFLICT(site_id,day,model_name) DO UPDATE`，幂等 upsert）、`ListUsageDaily`。
- `internal/service/service.go` — `AggregateUsageDaily`/`ListUsageDaily`/`ListUsageLogs` 入口。
- `internal/httpx/usage.go`（新） — `GET /sites/:id/usage/logs` + `GET /sites/:id/usage/daily` + `POST /sites/:id/usage/daily` handler。
- `internal/httpx/router.go` — 路由注册。
- `internal/service/scheduler.go` — `usage_daily` 域（每日聚合，`interval_s=86400`）+ `syncUsageDaily` 桥接。

**验收**：build + vet + test 均通过；迁移 5/5 应用；`AggregateUsageDaily` SQL 聚合正确（gpt-4o pt=180/tt=300/amt=1.5；gpt-4o-mini pt=30/tt=50/amt=0.2）；ON CONFLICT upsert 幂等（重跑仍 2 行同和）。

### S4 落地记录（已完成）

**已落代码**：
- `internal/model/model.go` — `SiteModel` 聚合视图实体。
- `internal/repo/repo.go` — `ListSiteModels`（`usage_daily` 汇总 + `site_account` 余额左连接，`LIMIT ?` 分页）。
- `internal/service/service.go` — `ListSiteModels` 入口。
- `internal/httpx/models.go`（新） — `GET /api/v1/models?limit=` handler。
- `internal/httpx/router.go` — 路由注册。

**验收**：build + vet + test 均通过；`ListSiteModels` SQL 聚合正确（SiteA/gpt-4o 300 tok/1.5 amt/balance 50；SiteB/gpt-4o-mini 50 tok/0.2 amt/balance 100）；`currency` 仅标明不折算。

> ⚠️ **未完成**：subscription 同步（api-inventory L191 「❌ 待新增 `SubscriptionInfo`」）。`site_account.subscription_state` 字段已留空，前端订阅到期提醒徽标未实现，待 upstream `/api/subscription/self` 对接。

### S5 落地记录（已完成）

**已落代码**：
- `migrations/0006_site_announcements.sql`（新） — `site_announcements` 表 + 双唯一键（`content_hash`/`remote_ref`，partial）+ 索引。
- `internal/model/model.go` — `SiteAnnouncement` + `SiteCheckin` 实体。
- `internal/repo/repo.go` — `ListAnnouncements`（跨站聚合，按 `published_at DESC`）、`UpsertAnnouncement`（幂等 upsert）、`UpsertCheckin`（`UNIQUE(site_id, date)`）。
- `internal/service/service.go` — `ListAnnouncements` + `SyncCheckin`（CheckinStatus→Checkin→记录 checkins，能力门控）。
- `internal/httpx/announcements.go`（新） — `GET /api/v1/announcements?limit=` handler。
- `internal/httpx/router.go` — 路由注册。
- `internal/service/scheduler.go` — `announcements` + `checkin` 域加入 `ensureSiteDomains` + `syncDomain` switch（`syncAnnouncements` 占位/能力门控；`syncCheckin` 调用 `SyncCheckin`，401 冻结 credential）。

**验收**：build + vet + test 均通过；迁移 6/6 应用；`site_announcements` 表结构与 unique index 校验通过；`ListAnnouncements` 查询正确（按发布时间倒序），`UpsertAnnouncement` 幂等。

> ⚠️ **未完成**：`syncAnnouncements` 为占位实现（adapter 尚无公告接口，api-inventory L192 「❌ 待新增（低优先级）」），domain 行已占位，能力门控跳过。待 upstream `/api/notice` 对接后实现 `UpsertAnnouncement`。

 验收基线沿用 sync-architecture §7 各阶段「验收」列。

## 9. 明确不做（本计划范围外）

- LLM 请求代理、站点内渠道管理（Q3 暂缓）、多用户/多租户、telegram/微信等会话型通知渠道（渠道注册表已留扩展位）、多实例部署与分布式锁（字段已预留）、站长侧 affiliate 运营管理（FR-10.6）
