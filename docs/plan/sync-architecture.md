# 数据同步架构设计（api-sites-client ↔ newapi / sub2api）

> 输入：[api-inventory.md](api-inventory.md)（上游接口盘点）。
> 现状基线：adapter 层（`internal/adapter/`）已实现 Detect/Login/Refresh/Verify/ListKeys/ListGroups/AffiliateInfo/TransferAffiliate/Checkin*；`SyncSite`（service.go）已实现 keys/groups/affiliate 三类同步，但**仅手动触发**。

## 1. 目标与原则

1. **增量优先**：所有同步尽量携带 `since` 游标，只拉变化；上游不支持增量时用全量 + ETag/指纹比对降级。
2. **能力门控**：上游没有的域（如 sub2api 无签到）走 `Capabilities` 跳过，不重试、不告警降级为 debug 事件。
3. **凭据零明文**：同步产出不含 key 明文；快照脱敏（沿用 `redactJSON`）。
4. **可观测**：每次同步产出 Event（复用 events 表），失败可归因（站点凭据失效 / 上游限流 / 网络）。
5. **对上游友好**：每站点独立限流预算，尊重 429 + Retry-After；并发 = 每站点 1，多站点间并行（默认 ≤4）。

## 2. 同步域与触发矩阵

| 域 | 数据源（newapi / sub2api） | 触发方式 | 增量策略 | 默认频率 |
|---|---|---|---|---|
| keys（Key 列表+配额） | `/api/token/` 分页 / `/api/v1/keys` | 手动 + 定时 + 事件 | 全量分页 + 指纹 diff | 5min |
| groups（分组/倍率） | `/api/ratio_config` 或 `/api/user/groups` / `/groups/available`+`/groups/rates` | 手动 + 定时 + 事件 | 全量 + ratio 比对 | 15min |
| account（余额/用量摘要） | `/api/user/self` / `auth/me` + `usage/summary` | 手动 + 定时 | 全量（单点） | 5min |
| affiliate（返利） | `/api/user/aff` / `user/aff` | 手动 + 定时 + 划转前强刷 | 全量（单点） | 10min |
| usage_log（调用明细） | `/api/log/self`（start_timestamp）/ `/v1/usage`（日期窗） | 定时 | **游标增量**（见 §4） | 15min |
| usage_daily（日聚合） | `/api/data/self` / `dashboard/trend` + `keys/:id/usage/daily` | 定时 | 按日补齐（D-1 起重算 2 天） | 每小时 |
| models（模型/价格） | `/api/pricing` / `model-plaza` | 定时 | 全量 + `pricing_version` 比对 | 6h |
| subscription（订阅） | `/api/subscription/self` / `/subscriptions(/active)` | 定时 | 全量（单点） | 1h |
| announcements | `/api/notice` / `/announcements` | 定时 | 全量 + hash 比对 | 1h |
| checkin（签到） | `GET/POST /api/user/checkin` | 定时（每日）+ 手动 | 当日状态判定 | 每日 08:05 抖动 |

> 「事件触发」= 凭据保存成功后立即同步全量 keys/groups；划转成功后强刷 affiliate。

## 3. 调度器设计

现状 `SyncSite` 是请求驱动的同步函数。新增 `internal/service/scheduler.go`：

```go
type SyncScheduler struct {
    reg      *adapter.Registry
    repo     *repo.Repo
    sem      chan struct{}        // 全局并发 ≤4
    stop     chan struct{}
    budgets  map[string]*rate.Limiter // siteID -> 令牌桶（默认 1 rps，可配置）
}

// 周期循环：每 tick 读 sites + site_sync_state（站点级 sync_cfg 列由 0004 migration 新增，见 storage-design.md §2.0），按域的 interval 判断是否到期
func (s *SyncScheduler) tick(ctx context.Context)
// 到期判定：site_sync_state.next_run_at <= now（每域一行，见 storage-design.md §3）
```

- 调度状态不存内存：`site_sync_state` 表持久化 `last_run_at/next_run_at/cursor`，进程重启不丢游标、不重复狂拉。
- 抖动：每站点启动偏移 = hash(siteID) % interval，避免多站点同刻齐打。
- 失败退避：连续失败 n 次 → next_run = now × 2^n（封顶 1h）；`auth_state` 失效时暂停该站点所有域并 emit event 等用户修复凭据。
- 手动同步走同一 `SyncSite` 入口（带 `force=true` 绕过 next_run 判定），并复用 budgets 限流。
- **防重入（借鉴 ai-gateway）**：手动/定时统一经 `singleflight.Group`（key = siteID:domain）去重并发请求；叠加新鲜度窗口——距上次成功 < 2min 的同步直接复用现有投影不重拉（`force=true` 可越过）。
- 错误分类**类型化**：adapter client 层定义错误码枚举（`ErrUnauthorized/ErrRateLimited/...` + `IsClientError(err, code)` 判定），调度器按错误码分流；禁止字符串匹配 err.Error()（ai-gateway 教训，见 ai-gateway-lessons.md §3）。

## 4. 增量游标设计（usage_log 为例）

### newapi：`GET /api/log/self?start_timestamp=<unix>`（秒级）
- 游标存 `site_sync_state.cursor`（JSON：`{"last_ts": 1729310400, "last_ids": []}`）。
- 拉取窗口 [last_ts, now-60s]（预留 60s 上游写入延迟）；按 `id` 去重（同秒多条）。
- 分页拉到 `has_more=false` 或空页为止，成功后推进游标。失败则游标不推进（at-least-once，靠唯一键幂等）。

### sub2api：`GET /api/v1/usage?start_date=&end_date=`（日期粒度）
- 无秒级游标 → 按日拉取，当日数据仅接近结束时拉一次 + 次日凌晨重拉昨日修正（上游聚合口径可能回填）。
- 游标 `{"last_day": "2026-09-18"}`。

### 指纹降级（无增量的域）
- keys/groups/models：全量拉取后计算规范化 JSON SHA-256，与 `site_sync_state.fingerprint` 相同则跳过写库只更新 `last_run_at`。

### 分页防护（借鉴 ai-gateway `account_key_list_pagination.go`）
- 全量分页域统一：pageSize=200、maxPages=100 封顶（防上游异常分页死循环）；跨页校验 total/pages 一致性、page 漂移、按 remote_id 去重；越界即报 parse 错误并留快照。

## 5. 归一化与 adapter 扩展

`SiteAdapter` 接口新增（全部带默认不支持语义，`ErrUnsupported` 不重试）：

```go
type UsageLogFilter struct{ Since *time.Time; Until *time.Time; KeyID string }
type UsageLog struct {
    RemoteRef   string    // 上游日志/记录唯一 id（幂等键）
    KeyRemoteID string    // 关联 site_keys.remote_key_id
    Model       string
    Group       string
    PromptTokens, CompletionTokens int64
    Cost        float64   // 归一化到「站点余额单位」（newapi=quota, sub2api=USD）
    Currency    string    // quota | USD
    Status      string    // success | error
    OccurredAt  time.Time
}
ListUsage(ctx, a AuthCtx, f UsageLogFilter, page Page) (UsageLogPage, error)
UsageDaily(ctx, a AuthCtx, from, to time.Time) ([]UsageDailyPoint, error)
ListModels(ctx, a AuthCtx) ([]ModelPrice, error)   // name/ratio/group/endpoint
Subscription(ctx, a AuthCtx) (*SubscriptionInfo, error)
Quota(ctx, a AuthCtx) (AccountQuota, error)         // 已定义，补实现
```

- 货币归一化原则：**不换算汇率**，保留上游单位（`currency` 字段标明），聚合展示时按站点分组，避免 quota↔USD 误算。
- newapi `quota_per_unit`（来自 `/api/status`）**动态读取**（借鉴 ai-gateway `newapi_quota_display.go`，默认 500000 仅作 fallback——不同 newapi 部署系数不同，硬编码必错）。落库采用 **raw/numeric 双列**（见 storage-design §2.6），换算系数事后可修正。现有 `internal/adapter/newapi/keys.go:223` 硬编码 500000，S1 改造。
- `Capabilities` 结构需同步扩展（新增 `UsageLog/UsageDaily/Models/Subscription bool` 等字段），保持「能力门控」原则在新域同样生效；各 adapter 未实现的方法返回 `ErrUnsupported`。
- **凭据会话管理（借鉴 ai-gateway）**：sub2api 业务请求 401 时自动 refresh 一次并重试（现有 Refresh 能力复用），仅 refresh 也失败才归为 auth_failed；单域失败不放大——多域同步按域记录 endpointErrors，失败域标 partial，其余域投影照常落库。

## 6. 失败分类与事件

| error_class | 判定 | 处理 |
|---|---|---|
| auth_failed | 401/403，或 refresh 失败 | 置 credential.auth_state，暂停站点调度，emit warn event |
| rate_limited | 429 | 读 Retry-After 推迟 next_run，不记 warn（info） |
| upstream_error | 5xx | 退避重试（≤3，指数） |
| unsupported | ErrUnsupported | 该域标记 unsupported，跳过（debug event，仅首次） |
| network | 超时/DNS/代理失败 | 退避重试，不影响 auth_state |
| parse | 200 但结构不符 | 全量快照 + error event（可能上游升级） |

所有同步动作 emit event：`sync.keys.success` / `sync.usage.error` 等，data 内含 duration、counts、cursor 推进值（不含敏感字段）。

## 7. 分阶段实施

| 阶段 | 内容 | 验收 |
|---|---|---|
| S1 | account 域（**sub2api Quota 补实现**——newapi 已有 `/api/user/self` 实现；数据源待确认 `/user/profile` 或 `/platform-quotas` 是否含 balance）+ 调度器骨架 + site_sync_state 迁移 | 定时同步 keys/groups/aff 自动跑；重启后游标/周期不丢 |
| S2 | usage_log 增量同步（两站游标实现）+ usage_logs 表 | 断点续传：中断后重启不丢不重（唯一键幂等验证） |
| S3 | usage_daily 聚合 + Dashboard 余额/用量卡片 | 前端 Dashboard 展示跨站余额与近 7 日趋势 |
| S4 | models/pricing + subscription 同步 | 模型广场页、订阅到期提醒事件 |
| S5 | announcements + 签到定时 + 事件触发细化 | 每日签到自动执行且可配置；公告聚合展示 |

> 前端配套改动（页面/组件/类型）见 [frontend-roadmap.md](frontend-roadmap.md)（S3 起逐阶段补充）。
