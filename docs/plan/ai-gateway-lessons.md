# ai-gateway 同步实现借鉴报告

> 来源：`E:\projects\ai\ai-gateway` 实际代码盘点（internal/service/aisiteuseraccount、aisitenewapi、aisitesub2api、background_job）。
> 结论用于修订 [sync-architecture.md](sync-architecture.md) 与 [storage-design.md](storage-design.md)，文中「已采纳」条目均已落入对应文档。

## 1. ai-gateway 已实现的同步能力一览

| 能力 | 实现 | 源码锚点（相对 ai-gateway 根） |
|---|---|---|
| newapi 全量账户同步 | profile / status / 分组 / tokens 分页 / 订阅 / 用量窗口聚合 / 签到 / 公告 | `internal/service/aisiteuseraccount/newapi_account_sync.go` |
| sub2api 全量账户同步 | profile / keys / subscriptions / usage stats / affiliate，单端点失败不放大（partial） | `account_full_sync.go` |
| 轻量 key-only 用量同步 | 仅凭 key（sha256 指纹标识）拉 `/api/usage/token/` | `key_usage_sync.go:275,522` |
| 返利同步 | remote/manual/previous 三态合并 | `affiliate.go:32-101` |
| 站点级公共监控 poll | 60s 周期，channel-monitors/groups，独立于用户私有域 | `internal/service/background_job/builtins.go` |
| 持久化 job 调度 | DB lease 抢占 + cluster_singleton/per-instance 作用域 | `background_job/service.go:966,1160,1176` |
| quota 显示换算 | `quota_per_unit` 从 `/api/status` 动态读取，500000 仅 fallback | `aisiteuseraccount/newapi_quota_display.go:14,24-51` |

**注意**：ai-gateway 用户私有域**没有**逐条用量日志游标增量（只有窗口聚合 summary）；本项目的 usage_log 游标增量设计（sync-architecture §4）是超出它的改进，保留。

## 2. 采纳的借鉴项（已修订进方案）

| # | 借鉴项 | 落点 |
|---|---|---|
| 1 | **分页防护**：pageSize=200 + maxPages=100 封顶；校验 total/pages 跨页一致、page 漂移、ID 去重 | sync-architecture §4 |
| 2 | **防重入**：手动/定时同步经 singleflight（key=siteID:domain）+ 新鲜度窗口（2min 内直接复用投影）消抖 | sync-architecture §3 |
| 3 | **quota 系数动态化**：`quota_per_unit` 从 `/api/status` 读取（现有 `internal/adapter/newapi/keys.go:223` 硬编码 500000，需改造）；raw/numeric 双列存储，换算系数事后可修正 | sync-architecture §5、storage-design §2.2/2.6 |
| 4 | **凭据失效分流**：`credential_invalid` 类错误冻结该站点自动调度、标记待人工重连（对应本项目 auth_failed 暂停调度——沿用并强化：错误码稳定化） | sync-architecture §6 |
| 5 | **401 自动 refresh 一次再重试**（sub2api JWT）；refresh 与业务请求解耦 | sync-architecture §5（adapter 现有 Refresh 能力复用） |
| 6 | **错误分类类型化**：client 层定义错误码枚举 + `IsClientError(err, code)` 判定，禁止 `strings.Contains(err.Error(), ...)`（ai-gateway 的教训：其 fallback 消息匹配脆弱） | sync-architecture §6 |
| 7 | **partial unique index + 软删除**：唯一键带 `WHERE deleted_at IS NULL`（SQLite 3.8+ 支持），站点断开重连不撞唯一约束 | storage-design §3 规则 1 |
| 8 | **单域失败不放大**：多域同步按域记录 endpointErrors，失败域标 partial，其余投影照常落库 | sync-architecture §6 |
| 9 | **快照三级留证**：redacted_payload（脱敏）+ 可选加密原文 + sensitivity_level；本项目已有 redactJSON，采纳「敏感分级」思想：keys/凭据类 kind=high | storage-design §3 规则 3 |
| 10 | **affiliate 三态合并**：远端优先、失败回退上次值、本地手动值永不被覆盖——推广到所有「本地可编辑 + 远端可同步」字段 | sync-architecture §2 备注 |

## 3. 明确不采纳（ai-gateway 的过度设计/教训）

1. **自研 5 字段 cron 解析器**：本项目只需 interval 轮询，用 DB `next_run_at` + tick 即可。
2. **巨型单文件 service**（builtins.go ~1.9 万行）：本项目按域拆分 `scheduler.go` / 各域同步文件。
3. **用户私有快照无保留策略**（官方明说不清理）：本项目维持「20 条/站点/域滚动删除」。
4. **无显式退避**（固定 tick 重试）：本项目维持指数退避（2^n 封顶 1h）。
5. **投影 builder 与编排强耦合**（一函数产 4 entity）：本项目按域函数化、builder 独立可测。
6. **哨兵 UUID 字面量比对**（甚至有损坏字面量 `auto_refresh.go:133`）：用显式 zero 判断。
