# new-api 项目分析

> 分析对象：`E:\projects\ai\new-api`（QuantumNous/new-api，Go Gin 单仓库 + React SPA，管理 API 前缀 `/api`，中转 API 前缀 `/v1|/v1beta`）
> 分析方式：只读代码勘察（2026-09-19）
> 结论用途：作为上游站点类型的对接契约，特别是**签到接口**（本项目核心需求之一）与 **system access token** 认证方式

## 0. 核心结论速览

| 主题 | 结论 |
| ---- | ---- |
| 响应信封 | `{success, message, data}`（`common.ApiSuccess/ApiError`） |
| 登录方式 | `POST /api/user/login`（username+password，可加密传输）→ 返回 `access_token`（JWT，15min）+ HttpOnly 刷新 cookie `new_api_refresh`（30 天轮换） |
| PAT 认证 | 用户在系统设置页生成 **system access token**，`Authorization: Bearer <随机串>`，适合无人值守 |
| **签到** | **有**：`GET /api/user/checkin`（状态）+ `POST /api/user/checkin`（执行，需 Turnstile 时另处理）——`controller/checkin.go:16/47` |
| sk- 令牌管理 | `GET/POST/PUT/DELETE /api/token/`、明文 key `POST /api/token/:id/key`、额度 `GET /api/usage/token/`（用 sk- 令牌） |
| 分组/倍率 | `GET /api/user/self/groups`（用户可用分组）、`GET /api/ratio_config`（公开倍率） |
| 站点识别信号 | `GET /api/status`（公开，返回系统名/版本/开关/turnstile） |
| 部署 | 单二进制（web 产物嵌入）；默认 SQLite，支持 MySQL/PG（SQL_DSN），可选 Redis |

## 1. 认证机制（理解各接口"认证"列的前提）

- **三层凭证**（`middleware/auth.go:158-187` `classifyDashboardCredential`）：
  1. **Dashboard 会话 JWT**：登录后签发，`Authorization: Bearer <jwt>`。HS256，TTL 15min（`service/auth_token.go:20`），claims 含 `token_use/sid/uv/sv`（:43-52），签名 key 由 `SessionSecret` HMAC 派生；会话本体存 DB `user_session` 表 + 缓存（`service/auth_session.go:130`）
  2. **用户 PAT（access_token）**：`Authorization: Bearer <随机串>`，校验 `model.ValidateAccessToken`（`model/user.go:1228`）——**无人值守定时任务首选**
  3. **中转令牌 sk-xxx**：仅 relay/usage 接口，`middleware/auth.go:361` `TokenAuth`；只读版 `TokenAuthReadOnly`（:288）
- **刷新令牌**：HttpOnly cookie `new_api_refresh`（`service/auth_session.go:16`，格式 `sid.secret`，30 天），每次 refresh 轮换（:217），重放窗口 30s
- **中间件链**（`router/api-router.go:15-22`）：`RouteTag → gzip → AccessTokenAudit → BodyStorageCleanup → GlobalAPIRateLimit`；鉴权入口 `UserAuth/AdminAuth/RootAuth`（`middleware/auth.go:100-116`）→ `authHelper`（:47），Admin 写操作自动审计（:70-80）
- 敏感操作需二次安全验证 `RequireSecurityProof`（短期 proof JWT，`auth_token.go:126`）

## 2. HTTP API 清单

### 2.1 公开接口（无需登录）

| 方法+路径 | 认证 | 关键字段 | 源码位置 |
|---|---|---|---|
| GET `/api/status` | 无 | 系统名/版本/注册开关/OAuth 开关/turnstile/价格等站点配置——**站点识别信号** | controller/misc.go:44；api-router.go:26 |
| GET/POST `/api/setup` | 无 | 初始化向导 | api-router.go:24-25 |
| GET `/api/notice`, `/about`, `/home_page_content`, `/user-agreement`, `/privacy-policy` | 无 | 公告/页面内容 | api-router.go:30-35 |
| GET `/api/pricing` | 模块开关 | 模型倍率/价格 | api-router.go:36；controller/pricing.go |
| GET `/api/ratio_config` | 无 | 分组倍率 | api-router.go:60 |
| GET `/api/verification` | 限流+Turnstile | 发邮箱验证码 `?email=` | api-router.go:44 |
| GET `/api/user/login/encryption-key` | 无 | 密码加密公钥 `{enabled,kid,public_key}` | controller/user.go:36；api-router.go:78 |

### 2.2 登录 / 注册 / 会话

| 方法+路径 | 认证 | 关键字段 | 源码位置 |
|---|---|---|---|
| POST `/api/user/login` | 限流+Turnstile | 请求 `{username,password}` 或 `{password_encrypted,encryption_key_id}`；成功 `data:{access_token, token_type, access_expires_at, session:{sid,login_method,ip,...}, user:{id,username,role,quota,...}}`，并 Set-Cookie `new_api_refresh` | api-router.go:79；controller/user.go:53,198 |
| POST `/api/user/login/2fa`、`/login/verify` | 无 | 2FA/邮箱验证质询 | api-router.go:80-81 |
| POST `/api/user/passkey/login/begin`、`/finish` | 无 | WebAuthn | api-router.go:84-85 |
| POST `/api/user/auth/refresh` | Refresh Cookie(+`X-Auth-Session` 头) | 轮换后返回同登录结构（新 access_token） | api-router.go:75；controller/auth_session.go:17 |
| POST `/api/user/auth/logout` | Refresh Cookie 或 Bearer | 撤销会话、清 cookie | api-router.go:76；auth_session.go:47 |
| POST `/api/user/register` | 限流+Turnstile | `{username,password,email?,verification_code?,aff_code?}` | api-router.go:77；controller/user.go:217 |
| GET `/api/oauth/:provider`、`/oauth/wechat`、`/oauth/telegram/login` | 无/TryUserAuth | OAuth 回调 | api-router.go:53-59 |

### 2.3 用户自助接口（UserAuth = Bearer JWT 或 PAT）

| 方法+路径 | 认证 | 关键字段 | 源码位置 |
|---|---|---|---|
| GET `/api/user/self` | UserAuth | 用户 DTO：quota/used_quota/group/aff_*/permissions 等 | api-router.go:98；controller/user.go:464,491 |
| PUT/DELETE `/api/user/self` | UserAuth | 改资料/注销 | api-router.go:100-101 |
| GET `/api/user/self/groups`、`/api/user/models` | UserAuth | 可用分组、可用模型列表 | api-router.go:97,99；user.go:615 |
| GET `/api/user/sessions`；DELETE `/api/user/sessions/:sid`；POST `.../sessions/revoke-others` | 需浏览器会话（PAT 不可用） | 会话列表/踢出 | api-router.go:94-96 |
| GET/POST `/api/user/token`；GET `/api/user/token/status`；DELETE `/api/user/token` | UserAuth+安全验证 | **PAT 的生成/状态/吊销**（管理端接入的关键） | api-router.go:102-105；controller/access_token.go:15,24,51 |
| GET/POST `/api/user/2fa/*`、`passkey/*` | UserAuth | 二次验证管理 | api-router.go:129-133 |
| GET `/api/user/aff` | UserAuth | 邀请码 | api-router.go:112 |
| **GET `/api/user/checkin`** | UserAuth | `{month?}` → `{enabled,min_quota,max_quota,stats}` | api-router.go:136；controller/checkin.go:16 |
| **POST `/api/user/checkin`** | UserAuth(+Turnstile) | 返回 `{quota_awarded,checkin_date}` | api-router.go:137；checkin.go:47 |
| GET/POST `/api/user/topup/*` | UserAuth | 充值订单（epay/stripe/creem/waffo） | api-router.go:113-124 |
| POST `/api/user/aff_transfer` | UserAuth | 邀请返利划转 | api-router.go:125 |

### 2.4 令牌（sk- 令牌）管理（UserAuth + `TokenOperationAudit`，api-router.go:273-287）

| 方法+路径 | 认证 | 关键字段 | 源码位置 |
|---|---|---|---|
| GET `/api/token/` | UserAuth | 分页列表，**key 打码返回** | controller/token.go:130 |
| GET `/api/token/search?keyword=` | UserAuth | 搜索 | token.go:144 |
| GET `/api/token/auto-groups` | UserAuth | auto 分组选项 | api-router.go:279 |
| GET `/api/token/:id` | UserAuth | 单条详情（打码） | api-router.go:280 |
| POST `/api/token/:id/key` | UserAuth | **返回完整明文 key** | api-router.go:281；token.go:188 |
| POST `/api/token/` | UserAuth | 创建：`{name, expired_time, remain_quota, unlimited_quota, model_limits, allow_ips, group, auto_groups}` | token.go:278 |
| PUT `/api/token/` | UserAuth | 更新 | api-router.go:283 |
| DELETE `/api/token/:id`；POST `/api/token/batch` | UserAuth | 删除/批量删除 | token.go:361,495 |
| POST `/api/token/batch/keys` | UserAuth | 批量取明文 key | token.go:521 |
| GET `/api/usage/token/` | **sk- 令牌**（`Authorization: Bearer sk-...`） | 额度：`{object:"token_usage",name,total_granted,total_used,total_available,unlimited_quota,model_limits,expires_at}` | api-router.go:289-297；token.go:229 |
| GET `/api/log/token` | sk- 令牌（只读） | 按令牌查消费日志 | api-router.go:348；controller/log.go:79 |

### 2.5 可用模型列表

| 方法+路径 | 认证 | 说明 |
|---|---|---|
| GET `/api/models` | UserAuth | 面板用模型列表 |
| GET `/v1/models`、`/v1beta/models` | sk- 令牌（支持 `?key=`/`x-api-key`/`x-goog-api-key`） | OpenAI 兼容列表 |
| GET `/api/channel/models`、`/models_enabled` | Admin | 渠道可选/启用模型 |

### 2.6 渠道/Channel 管理（AdminAuth + authz 细粒度权限，`router/channel-router.go:19-82`）

| 方法+路径 | 权限 | 说明 |
|---|---|---|
| GET `/api/channel/`、`/search`、`/:id` | ChannelRead | 渠道列表/搜索/详情 |
| GET `/api/channel/test`、`/test/:id` | ChannelOperate | 测试全部/单渠道 |
| GET `/api/channel/update_balance`、`/update_balance/:id` | ChannelOperate | 更新渠道余额 |
| POST `/api/channel/` | SensitiveWrite | 新建 |
| PUT `/api/channel/` | ChannelWrite | 更新 |
| DELETE `/api/channel/:id`、`/disabled`；POST `/batch` | SensitiveWrite | 删除/批量/清禁用 |
| POST `/api/channel/:id/key` | **RootAuth+安全验证** | 查看渠道密钥 |
| POST `/api/channel/:id/status`、`/status/batch`、`/tag/*`、`/copy/:id`、`/multi_key/manage` | Operate/Write | 状态/标签/复制/多 key 管理 |
| POST `/api/channel/upstream_updates/detect(_all)`、`/apply(_all)` | Operate/Write | 上游模型变更检测与应用 |
| GET `/api/channel/fetch_models/:id` | Operate | 拉取上游模型 |

> 用户/系统管理（Admin/Root，略举）：GET `/api/user/`、POST `/api/user/manage`、GET `/api/log/`、`/log/stat`、GET `/api/data/`（看板）、`/api/option/*`（Root 配置）、`/api/redemption/*`（兑换码）——api-router.go:144-164, 299-344。

## 3. 后台定时任务机制

无 cron 库，全部 goroutine 轮询/定时器（`main.go:85-178`）：

- `go model.SyncChannelCache(SyncFrequency)` — 渠道缓存同步（:106）
- `go model.SyncOptions` — 配置热更新（:115）；`go controller.SyncTaskPlugins`（:116）；`go authz.StartPolicySync`（:119）
- `go model.UpdateQuotaData` — 消费看板按日聚合（:122）
- `CHANNEL_UPDATE_FREQUENCY` → `controller.AutomaticallyUpdateChannels` 渠道余额定期更新（:124-130）
- `service.StartCodexCredentialAutoRefreshTask`（:133）、`StartSubscriptionQuotaResetTask`（:136）、`StartSystemInstanceReporter`（:140）
- **系统任务框架**（DB 租约去重+历史）：`RegisterScheduledSystemTasks` + `StartSystemTaskRunner`（:158-159），注册渠道定时测试/上游模型更新检测/Midjourney 轮询/异步任务轮询四类 handler（controller/system_task_handlers.go:20-24）
- `BATCH_UPDATE_ENABLED=true` 时 `model.InitBatchUpdater` 批量落库额度（:161-165）；`StartAuthArtifactCleanup`（:378）

## 4. 前端概要（web/）

- React + **TanStack Router/Query/Table**、rsbuild、oxlint/vitest、i18next（web/package.json:26-60）。API 客户端 `web/src/lib/api.ts`/`http-client.ts`，会话态 `stores/auth-store.ts`（存 access_token 并自动 refresh）
- 路由：`/sign-in`、`/sign-up`、`/forgot-password`（`routes/(auth)/`）；登录后令牌管理页 **`/keys`**、渠道 `/channels`、用户 `/users`、日志 `/usage-logs`、钱包 `/wallet`、`/profile`、`/security`、`/system-settings` 等

> 注：本项目前端按用户要求采用 sub2api 的 Vue3 骨架，不采用 new-api 的 React 前端；此节仅作了解。

## 5. 部署形态

- **单二进制**：Go 编译，web 构建产物以 `WebAssets` 嵌入并由 `SetRouter` 挂载（router/main.go:15-18）；Dockerfile/docker-compose、systemd unit（new-api.service）
- **数据库**：默认 SQLite（未设 `SQL_DSN`，model/main.go:163-165,180-183）；支持 MySQL/PostgreSQL（`SQL_DSN`）；日志库可独立用 ClickHouse（`LOG_SQL_DSN`）；可选 Redis

## 6. 对本项目的对接要点

1. **登录**：`POST /api/user/login` → 存 `data.access_token` + `new_api_refresh` cookie；15min 过期后 `POST /api/user/auth/refresh` 续期。
2. **无人值守首选 PAT**：登录一次后在系统设置生成 system access token（或 `POST /api/user/token` + 安全验证），后续所有定时任务用 `Authorization: Bearer <PAT>`，免去 15min JWT 轮换；PAT 失效再回退密码重登。
3. **签到**：先 `GET /api/user/checkin?month=...` 看 `enabled`，再 `POST /api/user/checkin` 执行；站点开启 Turnstile 时无法全自动（记录 `blocked_by_turnstile` 状态转人工）。
4. **Key 列表**：`GET /api/token/`（打码）→ 需要明文时 `POST /api/token/:id/key` 或 `POST /api/token/batch/keys`；额度同步可 `GET /api/usage/token/`。
5. **分组**：`GET /api/user/self/groups`（可用分组+倍率）、`GET /api/token/auto-groups`。
6. **站点识别**：`GET /api/status` 返回含 `version/system_name` 等字段即判为 new-api。
7. **登录态注意**：`/api/user/sessions*` 接口 PAT 不可用；敏感 PAT 生成需二次安全验证——自动化流程应在首次人工登录时完成 PAT 生成。
