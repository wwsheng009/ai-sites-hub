# ai-gateway 通知子系统专项分析（通知模块设计参考）

> 版本：v1.0（2026-09-19）
> 来源：`E:\projects\ai\ai-gateway` 只读勘察（本文件所列文件/类型/常量均经源码验证）
> 用途：为本平台 FR-9（通知模块）提供事实依据与设计参考；只取设计模式，不复制实现

## 1. 子系统定位与代码分布

| 层 | 位置 | 关键内容 |
| --- | --- | --- |
| 实体 | `internal/model/entity/notification_*.go` | `NotificationChannel`、`NotificationEvent`、`NotificationDispatch`、`NotificationTemplate`、`NotificationRouteState`、`NotificationInboxItem`、`UserNotificationSubscription`、`NotificationWechatAccount` |
| 服务 | `internal/service/notification/` | `service.go`（入口 Service）、`delivery.go`（渠道投递）、`telegram_sync.go` / `wechat_sync.go`（会话同步）、`wechat_auth.go`（微信登录） |
| HTTP | `gateway/router/notifications_admin.go` + `notifications_user.go`、`gateway/handlers/user_notification_preferences.go` | 管理端 / 用户端两套路由 |
| 外围 | `internal/service/usernotification`、`service/aisitereviewnotification`、`service/background_job/*notification*outbox*` | 用户侧收件箱、AI 站点审核通知、outbox 异步投递作业 |

要点：**实体拆得非常细**——事件（发生什么）、渠道（发到哪）、投递记录（发得怎样）、模板（怎么发）、路由状态（渠道会话态）、收件箱（站内信）各自独立成表。这是"事件→策略→渠道→投递"四段式可追溯设计的直接证据。

## 2. 渠道模型（实测源码常量）

`entity/notification_channel.go` 定义的全部渠道类型：

| 常量 | 值 | 说明 |
| --- | --- | --- |
| `NotificationChannelTelegram` | `telegram` | Bot sendMessage，支持 polling 模式自动同步 chat_id |
| `NotificationChannelWeChatSession` | `wechat_session` | 自建 ilink bot 会话，带 route_key/route_state 会话态 |
| `NotificationChannelEmail` | `email` | `net/smtp`，支持 SSL/TLS/STARTTLS（`smtp_network`），PlainAuth |
| `NotificationChannelInApp` | `in_app` | 站内收件箱 |

⚠️ **重要事实**：ai-gateway **没有通用 webhook 渠道**。本平台 FR-9.3 的 webhook 渠道属于**新增设计**，不是从 ai-gateway 移植的——分析文档如实记录，避免后续开发者误以为可照抄。

### 2.1 渠道配置模式

- `NotificationChannel`（`notification_channel.go:70`）含 `ChannelType string` + `Config JSONMap`（渠道特有配置，如 `smtp_host/port/network/from/to/username`、`bot_token/chat_id`），配套 `*_config_contracts.go` 提供类型安全取值（`SMTPHostValue()` 等）与契约测试
- 密钥类配置有独立契约文件（`notification_channel_secret_config_contracts.go`），**错误信息刻意不回显密码内容**（`delivery.go:750 annotateEmailSMTPSendError`）——直接支撑本平台 NFR-3
- `helpers_test.go` 专门有"配置值不得泄漏进诊断输出"的测试用例（`webhookSecret/`wechatContextToken` should-not-leak）——值得本平台测试继承

### 2.2 投递骨架（`delivery.go`）

- `resolveTarget()`：按渠道类型补齐目标信息（telegram 缺 chat_id 时触发 polling 同步；wechat 校验 route_state），缺前置条件直接返回可读错误
- `deliver()`：`switch ChannelType` 分发到 `deliverTelegram / deliverWeChat / deliverEmail(内联) / in_app`
- 失败统一 `annotate*SendError` 包装，带重试语义标注（如 `retried_without_session=%t`）但不回显凭据

## 3. Service 入口 API（`service.go`）

`Service` 的输入结构即本平台通知 service 的接口参考：

```go
type EventInput struct{}     // 记录事件（通知触发源）
type ChannelInput struct{}   // 渠道 CRUD
type TemplateInput struct{}  // 模板 CRUD
type RouteStateInput struct{}// 会话态管理
type TestSendInput struct{}  // 渠道测试发送（→ FR-9.3"测试发送"来源）
type InboxQuery struct{}     // 收件箱查询
```

查询侧：`DispatchListQuery/DispatchListResult`（投递记录查询）、`EventListQuery/EventListResult`（事件查询）——对应本平台 FR-9.5"事件→策略→投递匹配过程可追溯"。

## 4. 与本平台通知模块的取舍（→ architecture §11）

| ai-gateway 概念 | 本平台采纳 | 理由 |
| --- | --- | --- |
| Event / Channel / Dispatch 三实体分离 | ✅ 完整采纳 | 可追溯性核心，表设计见 architecture §3 |
| Template（消息模板） | ✅ 简化采纳（每类事件默认模板 + 可覆盖） | 单管理员场景不需要用户级模板 |
| RouteState（telegram/wechat 会话态） | ❌ 一期不采纳 | 本平台一期渠道（webhook/SMTP）无会话态 |
| telegram / wechat_session 渠道 | ❌ 一期不实现 | 依赖会话同步复杂度；渠道注册表留扩展位 |
| email(SMTP) | ✅ 采纳 | 投递实现直接参考（TLS/STARTTLS/错误脱敏） |
| in_app 收件箱 | ⏸ 二期可选 | 本平台 events 页面已覆盖大部分站内可见性 |
| TestSend（测试发送） | ✅ 采纳 | FR-9.3 |
| outbox 异步投递作业 | ✅ 采纳为 `notification.dispatch` 后台作业 | 与本平台作业框架天然契合 |

## 5. 对本平台的具体设计约束（由源码证据得出）

1. 渠道错误信息不得含凭据片段（ai-gateway `annotateEmailSMTPSendError` / not-leak 测试是现成范式）
2. SMTP 网络模式需显式可配（ssl/tls/starttls + IPv4-first 回退，见 `dialEmailSMTP`），不同邮箱服务商行为差异大
3. 投递记录必须区分 `pending/succeeded/failed` 并保留失败原因与重试计数，供 outbox 作业重试
4. 渠道配置走"契约取值函数"而非裸 JSONMap 读字符串，避免拼写错误静默失效
5. webhook（本平台新增）需自行定义：超时、签名头（HMAC）、成功判定（2xx）、重试策略
