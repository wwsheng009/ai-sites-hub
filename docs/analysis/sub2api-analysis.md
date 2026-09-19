# sub2api 项目分析

> 分析对象：`E:\projects\ai\sub2api`（Go Gin 后端 + Vue3 前端，面板 API 前缀 `/api/v1`，网关 API 前缀 `/v1`）
> 分析方式：只读代码勘察（2026-09-19）
> 结论用途：① 作为上游站点类型的对接契约；② 前端骨架直接复用来源（用户指定）

## 0. 核心结论速览

| 主题 | 结论 |
| ---- | ---- |
| 响应信封 | `{code:0, message:"success", data:...}`；分页 `{items,total,page,page_size,pages}`（`backend/internal/pkg/response/response.go:14-39`） |
| 登录方式 | 邮箱+密码 → **JWT access token + `rt_` 前缀 refresh token** 双令牌；支持 2FA(TOTP)/passkey/多 OAuth |
| 会话维持 | `Authorization: Bearer <jwt>`；前端 token 存 localStorage `auth_token`；无 cookie-session |
| **签到** | **无签到接口**（全仓 grep 无命中）——本项目签到功能只对 new-api 类站点生效 |
| Key 管理 | `GET/POST/PUT/DELETE /api/v1/keys`，用户可绑分组 `GET /api/v1/groups/available` |
| 站点识别信号 | `GET /setup/status`、`GET /api/v1/settings/public`（公开无需登录） |
| 前端骨架 | **Vue 3.4 + TS + Vite 5 + Pinia + vue-router + TailwindCSS 3.4，无 UI 组件库**；pnpm |

## 1. HTTP API 接口清单

### 1.1 登录 / 认证

路由注册：`backend/internal/server/routes/auth.go`；处理器：`backend/internal/handler/auth_handler.go`

| 方法+路径 | 认证 | 关键字段 | 源码位置 |
|---|---|---|---|
| POST `/api/v1/auth/register` | 无（限流5/min） | req: `email,password,verify_code,turnstile_token,promo_code,invitation_code,aff_code`；resp: `access_token,refresh_token,expires_in,token_type,user` | auth.go:35；auth_handler.go:50-60,179 |
| POST `/api/v1/auth/login` | 无（限流20/min） | req: `LoginRequest{email,password,turnstile_token,tencent_captcha_*}`；成功返回 token 对；2FA 时返回 `{requires_2fa:true,temp_token,user_email_masked}` | auth.go:38；auth_handler.go:77-83,239-297 |
| POST `/api/v1/auth/login/2fa` | 无 | req: `{temp_token,totp_code(6位)}` → token 对 | auth.go:41；auth_handler.go:301 |
| POST `/api/v1/auth/refresh` | 无（限流30/min） | req: `{refresh_token}`；resp: `{access_token,refresh_token,expires_in,token_type:"Bearer"}` | auth.go:54；auth_handler.go:685-710 |
| POST `/api/v1/auth/logout` | 无 | req: `{refresh_token?}` | auth.go:58；auth_handler.go:724-742 |
| POST `/api/v1/auth/revoke-all-sessions` | JWT | 撤销全部会话 | auth.go:259 |
| GET `/api/v1/auth/me` | JWT | 用户 profile + `run_mode` | auth.go:257；auth_handler.go:424-457 |
| POST `/api/v1/auth/send-verify-code` / `forgot-password` / `reset-password` / `validate-promo-code` / `validate-invitation-code` | 无 | 邮箱验证码/找回/码校验 | auth.go:50-74 |
| POST `/api/v1/auth/passkey/login/begin\|finish` | 无 | WebAuthn | auth.go:44-49 |
| `/api/v1/auth/oauth/{linuxdo,github,google,wechat,oidc,dingtalk}/start\|callback` | 无 | 多 OAuth 流 | auth.go:75-238 |
| GET `/api/v1/settings/public` | 无（按IP限流） | 站点公开配置（注册开关、OAuth 开关等）——**站点识别信号** | auth.go:246；handler/setting_handler.go:39 |

**Token 机制**（`backend/internal/service/auth_service.go`）：
- Access token = JWT HS256，claims 含 `user_id`、`session_id`（对应 RT family），有效期 `JWT.AccessTokenExpireMinutes/ExpireHour`（auth_service.go:59-69, 1408-1450）
- Refresh token = `rt_` 前缀随机串，sha 存 Redis，TTL `RefreshTokenExpireDays`，支持家族撤销/轮换/重用检测（:1712-1863）
- 面板认证中间件 `server/middleware/jwt_auth.go:40-50`：`Authorization: Bearer <jwt>`
- 管理端 `admin_auth.go:27-78`：JWT Bearer + admin 角色
- 网关认证 `api_key_auth.go:54-94`：`Authorization: Bearer <APIKey>` / `x-api-key` / `x-goog-api-key`
- 无 Cookie-Session；JWT 无状态 + Redis RT 会话绑定（IP+UA，`router.go:62`）

### 1.2 用户信息 / 额度

| 方法+路径 | 认证 | 关键字段 | 位置 |
|---|---|---|---|
| GET `/api/v1/user/profile` | JWT | 用户 profile | user.go:31 |
| PUT `/api/v1/user` / PUT `/user/password` | JWT | 更新资料/改密 | user.go:32-33 |
| GET `/api/v1/user/platform-quotas` | JWT | 各平台额度 | user.go:41 |
| GET `/api/v1/usage`、`/usage/stats`、`/usage/dashboard/{stats,trend,models,snapshot-v2}` | JWT | 用量统计/仪表盘 | user.go:99-113 |

### 1.3 令牌 / Key 管理（用户侧，本项目主要消费方）

| 方法+路径 | 认证 | 关键字段 | 位置 |
|---|---|---|---|
| GET `/api/v1/keys` | JWT | query: `page,page_size,sort_by,sort_order,search`；分页 Key 列表 | user.go:78；api_key_handler.go:106 |
| GET/PUT/DELETE `/api/v1/keys/:id` | JWT | Update: `name,group_id,status,ip_whitelist,ip_blacklist,quota,expires_at,reset_quota,rate_limit_{5h,1d,7d}` | user.go:79-82 |
| POST `/api/v1/keys` | JWT | `CreateAPIKeyRequest{name,group_id,custom_key,ip_whitelist,ip_blacklist,quota(USD),expires_in_days,rate_limit_*}` | user.go:80 |
| GET `/api/v1/groups/available` | JWT | 用户可绑分组 | user.go:88 |
| GET `/api/v1/groups/rates` | JWT | 用户专属分组倍率 | user.go:89 |

**分组管理（管理员）**：`routes/admin.go:328-353` —— `GET/POST /admin/groups`、`GET/PUT/DELETE /admin/groups/:id`、`/all`、`/usage-summary`、`/:id/stats`、`/:id/api-keys`；管理 Key 改组：`PUT /admin/api-keys/:id`（admin.go:185-188）。

### 1.4 签到 / 定时任务

- **无签到接口**：全仓 grep `签到|checkin|check_in` 无业务命中。最接近的激励机制是卡密兑换 `POST /api/v1/redeem`。
- 定时任务为服务端内部作业（无 HTTP 触发）：`robfig/cron`（备份 `service/backup_service.go:230`、运维清理 `ops_cleanup_service.go:166`、定时报告、渠道定时测试）+ 大量 `time.NewTicker`（订阅过期、token 刷新、余额探测等）。

### 1.5 站点配置类接口

| 方法+路径 | 认证 | 位置 |
|---|---|---|
| GET `/setup/status` | 无 | routes/common.go:23 —— **站点识别信号** |
| GET/PUT `/api/v1/admin/settings` | admin | routes/admin.go:558-562 |

### 1.6 网关（业务转发，Key 认证）

`routes/gateway.go:186-515`：`/v1/chat/completions`、`/v1/messages`、`/v1/models`、`/v1/embeddings`、`/v1/images/*`、`/v1/videos/*`、`/v1beta/models/*`（Gemini）、`/backend-api/codex/*`、**`GET /v1/sub2api/billing`（Key 额度查询）**。

## 2. 后端技术栈与架构

- **Go 1.27 + Gin v1.9.1**；JWT `golang-jwt/jwt/v5`；ORM **Ent**（`backend/ent/schema/`）；**PostgreSQL**（migrations/）；**Redis**（缓存/限流/RT 会话）；Viper 配置；Google Wire DI；zap 日志
- **分层**：`cmd/server/`（入口+wire）→ `internal/server/`（router、middleware/、routes/）→ `internal/handler/` → `internal/service/`（业务）→ `internal/repository/` → `internal/domain/`（常量/值对象）、`internal/config/`、`internal/pkg/`（response/errors 横切）、`internal/web/`（嵌入前端）、`internal/setup/`
- **后台作业**：进程内 goroutine ticker（约 40 个 service）+ robfig/cron；usage worker pool

### API 路由注册模式（照抄形态的入口）

- 总装：`backend/internal/server/router.go` — `SetupRouter`:23（中间件链 :59-71），`registerRoutes`:99
- 模块注册（`backend/internal/server/routes/`）：`common.go:10`（健康检查/setup status）、`auth.go:16`、`user.go:12`、`admin.go`（超 700 行）、`gateway.go:186`、`payment.go:26`
- 模式：每模块一个 `RegisterXxxRoutes(v1 *gin.RouterGroup, h *Handlers, ...中间件)`；认证用 `jwtAuth/adminAuth/apiKeyAuth/stepUpAuth` 参数注入；变更操作统一挂 `auditLog`；面板接口挂 `panelRateLimiter`（router.go:120-134）

## 3. 前端骨架（本项目直接复用）

- **技术栈**：Vue 3.4 + TypeScript 5.6 + Vite 5 + Pinia 2 + vue-router 4.2 + vue-i18n 9 + axios；**无 UI 组件库**，自建组件 + TailwindCSS 3.4；chart.js/vue-chartjs；vitest；pnpm
- **目录**：`src/{api,components,composables,constants,features,i18n,router,stores,styles,types,utils,views}`；api 层按域拆文件（auth/keys/groups/user/usage/payment…）
- **路由清单**（`src/router/index.ts`，meta 含 `requiresAuth/requiresAdmin` 守卫）：
  - 公开：`/login`、`/register`、`/forgot-password`、`/reset-password`、`/email-verify`、`/auth/*` OAuth 回调、`/legal/:id`
  - 用户：`/dashboard`、**`/keys`(KeysView)**、`/usage`、`/redeem`、`/affiliate`、`/profile`、`/subscriptions`、`/payment/*`
  - 管理：`/admin/dashboard`、`/admin/users`、**`/admin/groups`(GroupsView)**、`/admin/channels/*`、`/admin/accounts`、**`/admin/settings`(SettingsView)**、`/admin/ops`、`/admin/audit-logs` 等

### 可复用骨架文件清单（复制起点）

| 类别 | 文件 |
| ---- | ---- |
| 工程 | `frontend/package.json`、`vite.config.ts`、`tailwind.config.js`、`postcss.config.js`、`tsconfig.json`、`index.html` |
| 入口 | `src/main.ts`、`src/App.vue` |
| 基建 | `src/api/client.ts`（axios+token 刷新拦截器）、`src/api/tokenRefresh.ts`、`src/api/url.ts`、`src/router/index.ts`、`src/stores/auth.ts`、`src/stores/app.ts`、`src/types/index.ts`、`src/i18n/index.ts` |
| API 模板 | `src/api/auth.ts`、`src/api/keys.ts`、`src/api/groups.ts`、`src/api/index.ts` |
| 页面模板 | `src/views/auth/LoginView.vue`、`src/views/user/KeysView.vue`、`src/views/user/DashboardView.vue`、`src/views/admin/GroupsView.vue`、`src/views/admin/SettingsView.vue`、`src/components/layout/`、`src/components/common/` |

## 4. 对本项目的对接要点

1. **登录**：`POST /api/v1/auth/login` 存 token 对；带 JWT 调 `/api/v1/auth/me` 验证；过期用 `/api/v1/auth/refresh` 轮换。
2. **Key 列表**：`GET /api/v1/keys`（分页）+ `GET /api/v1/groups/available`；额度字段在 key 对象内（quota/expired_at/rate_limit_*）。
3. **签到**：sub2api 站点不支持 → adapter 的 checkin 能力返回 `unsupported`。
4. **站点识别**：`GET /setup/status` 或 `GET /api/v1/settings/public` 返回 200 且结构匹配即判为 sub2api。
5. **限流注意**：登录 20/min、refresh 30/min、按 IP 限流 —— 定时作业需控制频率并做 429 退避。
