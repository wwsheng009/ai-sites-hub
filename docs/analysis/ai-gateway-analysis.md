# ai-gateway 项目分析（站点管理视角）

> 分析对象：`E:\projects\ai\ai-gateway`（Go + React 大型单体 AI 网关）
> 分析方式：只读代码勘察（2026-09-19）
> 结论用途：本项目（ai-sites-client）的架构与功能设计**最直接参考**——该网关内置了一套完整的「AI 中转站点目录与监控子系统（AI Sites）」，与本项目需求高度重合。

## 0. 总体结论

ai-gateway 除自身作为 LLM 代理（provider/key/负载均衡）外，内置了完整的 **AI Sites 子系统**：把 sub2api / new-api 类站点作为一等实体管理，覆盖：

- 站点登记（DB 实体 + Admin API，**不在 yaml 配置文件**）
- **自动站点类型识别**（探测 + 特征打分）
- 凭据登录（access token → refresh token → 账号密码 阶梯式）
- 渠道/分组监控、用户账号 key/额度同步、**签到**
- 报价物化、公开比价页、观测事件

## 1. 站点注册/配置

### 1.1 核心实体

`internal/model/entity/ai_site_resource.go` → `ai_site_resources` 表：

| 字段组 | 字段 |
| ------ | ---- |
| 标识 | `slug`（唯一）、`name`、`site_url`、`api_base_url`、`aff_url`（推广返利，含 `aff_lock`） |
| 类型 | `site_type`（`sub2api` / `new-api` / `unknown`）、`resource_type`、`access_mode` |
| 生命周期 | `status`：draft → publish / hide / archive |
| 商务 | `topup_ratio`、充值/注册奖励/促销字段、`auth_methods`/`payment_methods`（JSONB 列表） |
| 统计/审核 | 评分/可用率/统计计数、contributor/maintainer/reviewer 审核链字段 |

### 1.2 注册方式

1. Admin 手工创建：`POST /resources`
2. 用户提交 + 审核流（submissions approve/reject/request_changes）
3. **自动站点探测**（本项目「自动判断站点类型」的直接参考）：
   - `service/aisiteresource/site_detect.go` → `DetectSite`：抓取落地页 + 探测核心端点，解析 HTML 提取名称/logo/描述
   - 按特征打分识别 `sub2api` / `new-api`（**score ≥ 6 判定**）
   - 协议专项探测：`site_detect_sub2api.go` / `site_detect_newapi.go`，推断 API base URL 与支持协议

> 注意：`.config.example.yaml` 中没有任何 ai_site 字段——站点管理完全走 DB + Admin API。本项目应吸取「配置可发现性差」的教训（见 §7）。

## 2. 上游站点登录（凭据模型）

### 2.1 Sub2API 客户端（`service/aisitesub2api/`）

- **凭据优先级**：access token → refresh token → 用户名密码登录
- **客户端方法**（`client.go`）：`Login` / `Login2FA`(TOTP) / `Refresh` / `GetCurrentUser` / `GetAffiliateInfo` / `GetPublicSettings` / `GetHealth` / `GetSetupStatus` / `ListAPIKeys`(分页) / `ListSubscriptions` / `GetUsageStats` / `ListChannelMonitors` / `ListAvailableGroups` / `ListAnnouncements` / `GetAPIKeyUsage`
- **错误分类**：带 Code 的 `Sub2APIClientError`：`unauthorized / forbidden / blocked_by_cf / totp_required / body_too_large / ...`；上层用 `errors.As` 按 Code 决策
- 防护细节：UA=`ai-gateway-sub2api-monitor/1.0`，响应体上限 2MB
- 认证模式：`username_password | token_pair | hybrid`
- 认证状态机：`auth_failed / refresh_failed / totp_required / blocked_by_cf`（前端可精确提示）

### 2.2 New-API 客户端（`service/aisitenewapi/`）

- 认证模式：`username_password | system_access_token | hybrid`
- 使用 **system access token + `New-Api-User` 请求头**；无 refresh token，token 失效后用密码重登重签
- 读取 `/api/user/self/groups` 获取可用分组与倍率

### 2.3 机密存储

- AES-GCM 加密（`secretCipher` + 恢复密钥）
- 投影只存加密 token 与 session 指纹；`redactJSONPayload` 脱敏
- 作业结果 payload **强制不含明文凭据**

## 3. Key/Token 列表与额度（`service/aisiteuseraccount/`）

| 能力 | 实现 | 说明 |
| ---- | ---- | ---- |
| 账号全量同步 | `account_full_sync.go` | profile + 分组 + token 列表 + 订阅 + 用量统计 → 写快照 + 投影 |
| key 额度投影 | `newAPITokenQuotaProjection` | 站点原始 `remain_quota/used_quota/unlimited` → 归一化 `limit/used/remaining` 三元组 |
| key 来源 | `newapi_token_list` / `newapi_token_plaintext_fetch` | 可拉明文 key |
| 额度展示 | `newapi_quota_display.go` | 原始 quota ↔ 展示值换算 |
| 认证种类 | `ai_site_user_private_data.go` | `newapi_api_token / newapi_system_access_token / newapi_username_password / newapi_session` |
| 用量同步 | `key_usage_sync.go` | 周期性拉 key 用量 |
| 分页 | `account_key_list_pagination.go` | 分页拉 key 列表 |
| key 导入 | `provider_token_import.go` | 把站点 key 导入网关 Provider Token 池 |
| 手工录入 | `manual_key.go` | 补充来源 |

- 管理端：每个站点 resource 可挂 monitor profile + credentials（`GET/POST /resources/:id/{new-api,sub2api}-monitor/credentials`、`/sync`）

## 4. 定时任务 / 签到 / 健康检查

### 4.1 作业框架（`service/background_job/`）

- DB 持久化作业表；`schedule_type = once | interval | cron`（自实现 5 字段 cron 解析 + timezone）
- `catalog.go` 注册内置作业（payload schema、dry_run 标志、默认调度、安全说明）
- 站点相关作业（均 cluster singleton、**默认 dry_run=true** 防误触发外部请求）：
  - `ai_site.sub2api_monitor.poll`（建议 interval 60s；per-profile `next_poll_at` 随机抖动默认 300–900s，按凭据优先级登录，采集 channel-monitors + groups，写快照与状态，状态变化生成事件）
  - `ai_site.new_api_monitor.poll`（拉分组 ratio，写 snapshots/states）
  - `ai_site.transit_offer.materialize` + `.refresh_outbox.dispatch`（报价物化，generation 事务替换，outbox 死信可 replay）
  - `ai_site.transit_well_known.collect`、`ai_site.official_baseline.collect`、`ai_site.user_account.private_refresh`、`ai_site.contribution.snapshot_materialize`
  - 防惊群：per-profile 随机轮询间隔 + singleflight

### 4.2 签到（`aisiteuseraccount/checkin.go`）

- 通过 public status 探测站点是否开启签到（`checkin_enabled`、turnstile 要求）
- 按月拉 `GetCheckinStatus` 与签到记录
- 执行模式：`status_only / external_interactive / gateway_manual`
- `maybeAutoCheckinAfterSync`：同步后自动签到
- `ListCheckinActionItems`：生成 open/completed/blocked/stale/unavailable 动作清单

### 4.3 健康检查（三层）

1. 网关自身 key 池：`resource_manager.health_check`（60s，auto_recovery，恢复阈值 0.90）+ `key_error_policies`（错误码/消息关键词 → disable_key/cooldown）
2. Provider Token 主动探测：`service/provider_token_probe`（HTTP 模板/site probe profile、小时级预算、结果回写 cooldown/disable/recover）
3. 站点级：`service/siteprobe`（可用性读取、discovery 调度器/worker、批量探测、协议模板）；sub2api 渠道状态 `healthy/degraded/unavailable` + freshness `fresh/stale/missing`

## 5. cmd/ 命令入口

| 入口 | 职责 |
| ---- | ---- |
| `cmd/gateway` | 主程序（cobra）：`start/stop/restart/kill/status/version/backup-config`；`cluster start\|status\|stop`；`rolling-restart`；`config effective\|explain\|snapshot\|provider-table`；`db backup\|restore\|inspect\|migrations\|migrate`；`setup check\|status\|doctor\|init` |
| `cmd/gateway-edge` | Edge 反代入口 |
| `cmd/gwmon` | 终端 TUI 监控面板（Bubble Tea 风格） |
| `cmd/gateway-ops` | 独立 go.mod 运维工具 |
| `cmd/aicli`、`logcli`、`logschema`、`data-view` | CLI 客户端、日志查询/模式、数据查看 |
| 其余 | 开发/测试辅助（llm-mock、sse-debug、sse-stress 等） |

> 本项目参考：cobra 多子命令 + 与 HTTP API 共用 service 层的模式；`status/doctor/db backup` 等运维子命令值得照搬。

## 6. internal/ 分层概览

- `config`：yaml+env+DB 快照的配置事实源与 watcher；`dbmigration`：165+ 版 SQL 迁移（**每版同时维护 postgres/sqlite 两份**）；`dbbackup`
- `model/entity`：全部 GORM 实体；`repository/*`：按域分仓储
- `service/*`（65+ 域）：`aisiteresource`、`aisitesub2api`、`aisitenewapi`、`aisiteuseraccount`、`aisiteevents`、`siteprobe`、`provider_token_*`、`background_job`、`notification` 等
- `gateway/`：HTTP 层（handlers/router/middleware），如 `handlers/admin_ai_site_resource.go`（站点 CRUD/监控/审核/事件策略 ~60 条路由）、`user_ai_site_account.go`（用户账号/同步/签到/key/测试模型）
- 其它：`edgeproxy`、`transformer`、`pricing`、`modelcatalog`、`taskqueue`、`setupcheck` 等

### 前端

React 18 + TypeScript + Vite 5 + antd 5 + zustand + Tailwind，pnpm，playwright e2e。站点相关页面：`pages/admin/ai-sites/`（70+ 文件）、`pages/public-ai-sites/`、`pages/user/ai-sites/AISitePortalPage`；路由 `config/routes.ts` 集中管理 + 权限点。

## 7. 可复用设计模式 与 坑

### 可复用模式（本项目直接采用）

1. **Snapshot / Projection / State 三分离**：原始响应存 `*_snapshots`，可读投影存 `*_states`，配 `freshness_status`/`projection_status`——外部站点数据不稳定的标准解法
2. **Typed JSONB Payload**：`SchemaVersion + Normalize() + Scan/Value`，列内容演进可控
3. **凭据阶梯 + 认证状态机**：access token → refresh → 密码重登；状态显式枚举（`blocked_by_cf/totp_required/login_required`）
4. **作业安全默认**：外探作业默认 `dry_run=true`、singleton、lease 锁、结果 payload 无凭据、随机抖动 + singleflight
5. **Outbox + 死信 replay**
6. **观测事件 + 事件策略**（如 `ai_site.newapi_group.created/rate_changed`）
7. **错误分类客户端**：`XxxClientError{Code}` + `errors.As`
8. **探测预算/限速**：避免打爆上游
9. **站点类型识别**：落地页 + 核心端点探测 + 特征打分（≥6 判定）
10. **归一化投影**：各站点原始字段 → 统一 `limit/used/remaining` 模型

### 坑/风险（本项目避免）

- 迁移双引擎（每版 postgres+sqlite 各一份）维护成本极高 → 本项目先只支持 sqlite
- 配置可发现性差（站点在 DB，yaml 无提示）→ 本项目提供 `sites import/export` cmd
- `AISiteResource` 实体过胖（目录+商务+返利+统计+审核混一张表）→ 本项目拆分站点表与统计/凭据表
- 仓库根目录散落 exe/pid/log 运行产物 → 本项目 gitignore 严格收敛
- cron 解析器复制两份产生漂移 → 本项目只用一个调度实现
