# 上游站点用户侧 API 盘点（new-api / sub2api）

> 调研基准：`E:\projects\ai\new-api`（目录名 new-api，下文简称 newapi）与 `E:\projects\ai\sub2api` 的实际路由与 controller/handler 代码（2026-09-19）。
> 范围：**普通用户/终端调用方可访问**的接口；admin 专属接口仅作边界说明，不展开。
> 本文档是 [sync-architecture.md](sync-architecture.md) 与 [storage-design.md](storage-design.md) 的输入。

## 认证形态总览

| 形态 | newapi | sub2api |
|---|---|---|
| 面板会话 | session cookie（`/api/user/auth/refresh` 续期） | JWT access token（`/api/v1/auth/refresh` 续期） |
| 系统访问令牌（PAT） | `/api/user/token` 生成，`Authorization` 头；`/api/token/*` 与 `/api/user/self` 均可用 | 无（PAT 概念不存在，统一走 JWT） |
| API Key（中继） | `sk-xxx`，`TokenAuth()`（Bearer / x-api-key / ?key=）；`TokenAuthReadOnly()` 为只读宽松版 | `sk-xxx`，`APIKeyAuthMiddleware`（Bearer / x-api-key） |
| 公开接口 | `/api/status`、`/api/notice`、`/api/pricing`（按模块开关）等 | `/health`、`/api/v1/settings/public`、`/model-plaza`（OptionalJWT）等 |

> ai-sites-hub 现有 adapter 已按此实现认证阶梯：newapi = session + PAT（masked key），sub2api = JWT（refresh token 自动续期）。

## 一、newapi 用户侧接口清单

### 1. 公开 / 探测类（无需认证，站点识别可用）

| 方法 | 路径 | 用途 | 关键字段 |
|---|---|---|---|
| GET | `/api/status` | 站点状态与前端配置 | `version/system_name/register_enabled/turnstile_site_key/oauth开关/quota_per_unit` |
| GET | `/api/notice` / `/api/about` / `/api/home_page_content` | 公告/关于/首页内容 | 纯文本 |
| GET | `/api/pricing` | 模型价格与倍率 | `{data: pricing[], vendors, group_ratio, usable_group, supported_endpoint, pricing_version}` |
| GET | `/api/ratio_config` | 模型倍率配置 | 需站点开启 `IsExposeRatioEnabled`，否则 403 |
| GET | `/api/user/groups` | 分组列表（未登录取默认分组） | `{分组名: {ratio, desc}}` |
| GET | `/api/setup` | 初始化状态 | `{status, root_init, database_type}` |
| GET | `/api/verification` | 发送邮箱验证码 | query `email`，Turnstile |
| GET | `/api/perf-metrics(/summary)` | 性能指标 | 按模块开关 |

### 2. 认证 / 账户安全（UserAuth）

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | `/api/user/login`、`/api/user/login/2fa`、`/api/user/login/verify`、`/api/user/login/passkey/(begin\|finish)`、`/api/user/passkey/login/(begin\|finish)` | 登录族（含加密公钥 `/api/user/login/encryption-key`） |
| POST | `/api/user/auth/refresh`、`/api/user/auth/logout` | 会话续期/登出 |
| POST | `/api/oauth/:provider`、`/api/oauth/state`、`/api/oauth/wechat`、`/api/oauth/telegram/login` | OAuth 登录/绑定（TryUserAuth） |
| GET/PUT/DELETE | `/api/user/self` | 自身信息（含 `quota, used_quota, aff_code, group`）/改资料/注销 |
| GET/DELETE | `/api/user/sessions(/:sid)`、POST `/api/user/sessions/revoke-others` | 会话管理 |
| GET/POST/DELETE | `/api/user/token(/status)` | 系统 access token（PAT）管理（生成需安全证明） |
| GET/POST/DELETE | `/api/user/passkey`、`/api/user/passkey/register/*`、`passkey/verify/*` | Passkey 管理 |
| GET | `/api/user/2fa/status`；POST `/2fa/setup|enable|disable|backup_codes` | 2FA |
| POST | `/api/verify`；GET `/api/verify/methods` | 敏感操作通用二次验证 |
| PUT | `/api/user/setting` | 用户设置（`quota_warning_threshold/webhook_url/notification_email/bark_url...`） |
| POST | `/api/oauth/email/bind/(start|resend|bind)`、`/api/oauth/wechat/bind` 等 | 绑定邮箱/微信/Telegram |

### 3. 令牌（API Key）管理（UserAuth）—— 同步核心数据源

| 方法 | 路径 | 用途 | 关键字段 |
|---|---|---|---|
| GET | `/api/token/` | 分页列出我的令牌（key 脱敏） | query `p/page_size`；item: `id/name/key(掩码)/status/group/remain_quota/used_quota/unlimited_quota/expired_time/created_time/auto_groups` |
| GET | `/api/token/search` | 搜索令牌 | `keyword/token` + 分页 |
| GET | `/api/token/:id` | 单个令牌（脱敏） | — |
| POST | `/api/token/:id/key` | 查看完整 key（**CriticalRateLimit，同步禁用**） | `data.key` |
| POST/PUT | `/api/token/` | 新建/更新令牌 | `name/expired_time(-1无限)/remain_quota/unlimited_quota/model_limits(_enabled)/allow_ips/group/auto_groups` |
| DELETE | `/api/token/:id`、POST `/api/token/batch(/keys)` | 删除/批量操作 | — |
| GET | `/api/token/auto-groups` | 可用自动分组 | `{groups, max_count}` |

### 4. 充值 / 兑换 / 额度 / 返利 / 签到 / 订阅（UserAuth）

| 方法 | 路径 | 用途 | 关键字段 |
|---|---|---|---|
| GET | `/api/user/aff` | 邀请码（无则生成） | `data.aff_code` |
| POST | `/api/user/aff_transfer` | 划转邀请额度到余额 | `{quota}`（✅ adapter 已实现） |
| GET | `/api/user/topup/info` | 充值配置 | `enable_*_topup/pay_methods/min_topup/amount_options` |
| POST | `/api/user/topup` | 兑换码兑换 | `{key}` → `data.quota` |
| GET | `/api/user/topup/self` | 充值记录 | 分页 |
| POST | `/api/user/pay`、`/api/user/(stripe\|creem\|waffo(-pancake)?)/(pay\|amount)` | 各渠道下单/预估 | — |
| GET | `/api/user/checkin` | 签到状态/历史 | query `month`；`{enabled, min_quota, max_quota, stats}`（✅ 状态已实现） |
| POST | `/api/user/checkin` | 每日签到 | `{quota_awarded, checkin_date}`（✅ 已实现） |
| GET | `/api/subscription/plans` | 可购套餐 | `[SubscriptionPlan]` |
| GET | `/api/subscription/self` | 我的订阅 | `{billing_preference, subscriptions, all_subscriptions}` |
| PUT | `/api/subscription/self/preference` | 计费偏好 | `{billing_preference}` |
| POST | `/api/subscription/(balance\|epay\|stripe\|creem\|waffo-pancake)/pay` | 购买订阅 | `{plan_id}` |

### 5. 日志 / 看板 / 用量（UserAuth 或 TokenAuthReadOnly）—— 同步核心数据源

| 方法 | 路径 | 认证 | 用途 | 关键字段 |
|---|---|---|---|---|
| GET | `/api/log/self` | UserAuth | 我的调用日志（**支持 `start_timestamp` 增量**） | `type/model_name/token_name/group/request_id` + 分页 |
| GET | `/api/log/self/stat` | UserAuth | 用量统计 | `{quota, rpm, tpm}` |
| GET | `/api/log/token` | TokenAuthReadOnly | 按 token 查日志 | `data.logs[]` |
| GET | `/api/usage/token/` | TokenAuthReadOnly | token 用量摘要 | `{total_granted, total_used, total_available, unlimited_quota, expires_at}` |
| GET | `/api/data/self` | UserAuth | 按日额度数据（跨度≤1月） | `start_timestamp/end_timestamp` |
| GET | `/api/data/flow/self` | UserAuth | 按日流水数据 | 同上 |
| GET | `/api/audit/self` | UserAuth | 审计日志 | 分页 + `category` |
| GET | `/api/mj/self`、`/api/task/self`、`/api/task/:task_id/artifacts` | UserAuth | MJ/异步任务 | 分页 + 时间窗 |
| GET | `/api/models`、`/api/user/self/models` | UserAuth | 模型清单（全量/按可用组） | `query group`（`auto`=自动组） |
| GET | `/api/rankings`、`/api/perf-metrics` | 模块开关 | 排行/性能 | `period` |
| GET | `/dashboard/billing/subscription`、`/dashboard/billing/usage` | TokenAuth | OpenAI 兼容订阅/用量 | `hard_limit_usd/total_usage` |

### 6. 中继 API（终端调用方，TokenAuth）

- OpenAI/Anthropic/Gemini 三格式：`/v1/chat/completions`、`/v1/completions`、`/v1/messages`、`/v1/responses(/compact)`、`/v1/embeddings`、`/v1/models(/:model)`、`/v1beta/models/*`、`/v1/audio/*`、`/v1/images/(generations|edits)`、`/v1/rerank`、`/v1/moderations`、`/v1/realtime`(WS)。
- 绘图/音乐/任务类：`/mj/submit/*`、`/mj/task/:id/fetch`、`/v1/tasks/:key`（suno 等任务插件）、`/v1/video(/generations)`。
- Playground（UserAuth）：`POST /pg/chat/completions`。

### 7. 边界说明

- 管理员/Root 专属（`/api/user/:id`、`/api/channel/*`、`/api/redemption/*`、`/api/option/*`、全量 `/api/log/`、`/api/data/`、订阅管理）不在本清单。
- 全 `/api` 路由套 gzip + `AccessTokenAudit` + 全局限流；敏感路由另有 `CriticalRateLimit` 与 Turnstile —— **同步器必须按站点限流预算节流**。

## 二、sub2api 用户侧接口清单

### 1. 通用（公开）

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/health` | 健康检查 |
| GET | `/setup/status` | setup 状态（恒 completed） |
| POST | `/api/event_logging/batch` | Claude Code 遥测吞掉（恒 200） |

### 2. 认证 / 账户（auth.go）

| 方法 | 路径 | 用途 | 关键字段 |
|---|---|---|---|
| POST | `/api/v1/auth/register` | 注册 | `email/password/verify_code/promo_code/invitation_code/aff_code` → `access_token/refresh_token` |
| POST | `/api/v1/auth/login`、`/login/2fa`、`/passkey/login/(begin|finish)` | 登录族 | AuthResponse |
| POST | `/api/v1/auth/refresh`、`/logout` | JWT 刷新/登出（✅ adapter 已实现 refresh） | `refresh_token` → 新 token 对 |
| POST | `/api/v1/auth/send-verify-code`、`/forgot-password`、`/reset-password` | 验证码/密码重置 | — |
| POST | `/api/v1/auth/validate-promo-code`、`/validate-invitation-code` | 注册前校验 | `{valid, bonus_amount}` |
| GET/POST | `/api/v1/auth/oauth/{linuxdo,github,google,wechat,oidc,dingtalk}/*` | 多供应商 OAuth（start/callback/bind/complete-registration/create-account/bind-login + pending 交换） | — |
| GET | `/api/v1/settings/public` | 公开设置（✅ 已用于站点识别 detect.go） | — |
| GET | `/api/v1/auth/me` | 当前用户 | User dto（✅ 已实现 Verify） |
| POST | `/api/v1/auth/revoke-all-sessions`、`/auth/oauth/bind-token` | 会话撤销/OAuth 绑定预置 | — |

### 3. 用户自管理（user.go，JWT）

| 方法 | 路径 | 用途 | 关键字段 |
|---|---|---|---|
| GET | `/api/v1/user/profile` | 个人资料（绑定来源） | `email_bound/linuxdo_bound/...` |
| PUT | `/api/v1/user`、`/user/password` | 资料/改密 | `old_password/new_password` |
| GET | `/api/v1/user/aff` | 邀请返利详情（✅ adapter 已实现） | — |
| POST | `/api/v1/user/aff/transfer` | 返利转余额（✅ 已实现） | `{transferred_quota, balance}` |
| POST/DELETE | `/api/v1/user/account-bindings/*`、`/user/auth-identities/bind/start` | 账号绑定/解绑 | — |
| GET | `/api/v1/user/platform-quotas` | 平台配额视图 | — |
| POST/PUT/DELETE | `/api/v1/user/notify-email/*` | 通知邮箱管理 | — |
| GET/POST | `/api/v1/user/totp/*`（status/setup/enable/disable/step-up） | TOTP | — |
| GET/POST/PATCH/DELETE | `/api/v1/user/passkeys(/:id)(/register/*)` | Passkey 管理 | — |
| GET | `/api/v1/user/api-keys/:id/usage/daily` | 单 Key 日用量 | `timezone` |

### 4. API Key / 分组 / 渠道（JWT）—— 同步核心数据源

| 方法 | 路径 | 用途 | 关键字段 |
|---|---|---|---|
| GET | `/api/v1/keys`、`/keys/:id` | 我的 Key 列表/详情 | `name/group_id/status/ip_whitelist/quota/expires_at/rate_limit_*` |
| POST/PUT/DELETE | `/api/v1/keys(/:id)` | Key CRUD | `custom_key/quota(USD)/expires_in_days/reset_quota` |
| GET | `/api/v1/groups/available`、`/groups/rates`、`/channels/available` | 分组/倍率/渠道（✅ groups 已实现） | — |

### 5. 用量统计 / Dashboard（JWT，Heavy 限流）—— 同步核心数据源

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/usage` | 用量记录（`start_date/end_date/model/api_key_id/billing_type/...` 过滤，**支持时间窗增量**） |
| GET | `/api/v1/usage/:id`、`/usage/stats`、`/usage/errors(/:id)` | 详情/聚合/错误记录 |
| GET | `/api/v1/usage/dashboard/(stats\|trend\|models\|snapshot-v2)` | 仪表盘 |
| POST | `/api/v1/usage/dashboard/api-keys-usage` | 按 Key 用量汇总（body 传过滤） |

### 6. 订阅 / 支付 / 卡密 / 模型广场（payment.go + user.go）

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/payment/(config\|checkout-info\|plans\|limits)` | 支付配置/收银台/套餐/限额 |
| POST/GET | `/api/v1/payment/orders(/verify\|/my\|/:id\|/:id/cancel\|/:id/refund-request)` | 订单全生命周期 |
| POST | `/api/v1/payment/public/orders/(verify\|resolve)` | 匿名查单（resume token） |
| GET/POST | `/api/v1/payment/webhook/{easypay,alipay,wxpay,stripe,airwallex}` | 支付回调（验签） |
| GET | `/api/v1/subscriptions(/active\|/progress\|/summary)` | 我的订阅 |
| POST | `/api/v1/redeem`；GET `/redeem/history` | 卡密兑换 |
| GET | `/api/v1/model-plaza` | 模型广场（OptionalJWT，含个人专属分组/倍率） |

### 7. 其他

- `GET /api/v1/announcements(/:id/read)` 公告；`GET /api/v1/channel-monitors(/:id/status)`、`/channel-monitor-v2/*` 渠道监控；`GET /api/v1/pages/:slug` 自定义页面。
- 网关中继（API Key）：`/v1/messages(/count_tokens)`、`/v1/chat/completions`、`/v1/responses*`、`/v1/models(/:model)`、`/v1/usage`、`/v1/embeddings`（OpenAI 平台）、`/v1/images*`、`/v1/videos*`（Grok）、`/v1/tts|stt|custom-voices*`、`/v1/web_search|x_search`、`/v1beta/models*`（Gemini）、`/backend-api/codex/*`、`/antigravity/*`、`GET /v1/sub2api/billing`（Key 计费/余额）。
- 边界：`/api/v1/admin/*` 全部管理员专属；backend 模式下 `BackendModeUserGuard` 对非管理员 403。

## 三、两站能力矩阵（对同步方案的意义）

| 数据域 | newapi | sub2api | 归一化可行性 |
|---|---|---|---|
| Key 列表+配额 | `/api/token/`（脱敏，quota 单位=quota） | `/api/v1/keys`（quota 单位=USD） | ✅ 已实现（adapter.SiteKey） |
| 分组/倍率 | `/api/user/groups` 或 `/api/ratio_config` | `/api/v1/groups/available` + `/groups/rates` | ✅ 已实现 |
| 账号余额 | `/api/user/self.quota` | `user/profile` 余额字段 | ⚠️ Quota() 接口已有，需补实现 |
| 返利 | `/api/user/aff` + `aff_transfer` | `/api/v1/user/aff` + `aff/transfer` | ✅ 已实现（含划转留证） |
| 签到 | `GET/POST /api/user/checkin` | ❌ 无 | ✅ adapter 能力门控 |
| 调用日志/用量 | `/api/log/self`（秒级时间戳增量） | `/api/v1/usage`（日期窗增量） | ❌ 待新增 `ListUsage` |
| 日聚合统计 | `/api/data/self`（≤1 月窗） | `/usage/dashboard/trend`、`usage/daily` | ❌ 待新增 `UsageDaily` |
| 模型/价格 | `/api/pricing`、`/api/user/self/models` | `/model-plaza`、`/channels/available` | ❌ 待新增 `ListModels` |
| 订阅 | `/api/subscription/self` | `/subscriptions(/active\|/summary)` | ❌ 待新增 `SubscriptionInfo` |
| 公告 | `/api/notice` | `/announcements` | ❌ 待新增（低优先级） |
| Token 刷新 | session refresh（PAT 长效） | `/auth/refresh`（JWT） | ✅ 已实现 |
| 明文 Key | `POST /api/token/:id/key`（严格限流） | `custom_key`（创建时可自定）/详情 | ⚠️ 仅按需手动触发，不进同步 |
