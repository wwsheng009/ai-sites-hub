# 前端配套路线图（sync-engine 演进）

> 前置：`implementation-plan.md` M6（六个基础页面）。本文档只覆盖**同步引擎各阶段（sync-architecture §7 S1–S5）带来的前端增量**，与 sync-architecture 的 S 阶段编号一一对应。
> 技术基线：React + TS + Vite（`frontend/`），类型与 Go 契约严格对齐（字段以 `docs/plan/storage-design.md` 各表 DDL 为准，勿虚构）。

## 0. 前置（M6 已交付的能力）

- 站点管理页 / 站点详情页（凭据状态、key、分组、签到历史、返利卡片、手动同步按钮）
- 全局 Keys 页、作业页、总览仪表盘（跨站返利汇总）
- `api/client.ts` 统一走 `/api/v1`

## S1：account 域 + 调度器骨架（后端先行，前端改动最小）

| 改动 | 说明 |
|---|---|
| 站点详情页「余额」卡片 | 数据源 `GET /sites/:id/account`（account 域投影）；newapi 站显示 quota 原始值 + `unit_note`（raw/numeric 双列，见 storage-design §2.6），**不做汇率折算**；sub2api 站显示 USD |
| 手动同步按钮态 | 请求合并期间按钮 disabled + spinner（后端 singleflight 去重，见 sync-architecture §3）；返回 `deduped=true` 时提示「同步进行中，已复用」 |
| 作业页列 | `site_sync_state` 域级状态列：`last_run_at / next_run_at / freshness`（fresh/stale 徽标） |

## S2：usage_log 增量（调用明细页）

| 改动 | 说明 |
|---|---|
| 新页面「调用明细」（站点详情 Tab 或全局页） | 分页表格：时间 / key / 模型 / 分组 / tokens / cost（带 currency 徽标）/ 状态；服务端分页（usage_logs 表索引见 storage-design §2.4） |
| 过滤器 | 时间范围、key、模型、状态；空态区分「未同步过」vs「无数据」 |
| 类型契约 | `UsageLog` 对齐 sync-architecture §5 结构体（RemoteRef 不下发给前端，换 `id`） |

## S3：usage_daily 聚合（Dashboard 升级）

| 改动 | 说明 |
|---|---|
| Dashboard 余额卡片带 freshness | stale 站点灰显 + 「上次成功同步 xx 分钟前」，点击可手动同步 |
| 近 7/30 日用量趋势图 | `GET /usage/daily?site_ids=&days=`；**按站点分组聚合展示（currency 不同不合并求和）**；newapi 站数值标注「quota」 |
| 日粒度下钻 | 点击趋势柱 → 跳转调用明细页并带上日期过滤器 |

## S4：models/pricing + subscription

| 改动 | 说明 |
|---|---|
| 新页面「模型广场」 | 跨站模型/价格对照表：模型名 / 分组倍率 / 各站点可用性；数据源 models 投影 |
| 订阅到期提醒 | 站点卡片徽标（≤7 天 warn 色）+ 事件流已有条目复用；数据源 subscription 投影 |

## S5：announcements + 签到定时

| 改动 | 说明 |
|---|---|
| 公告聚合入口 | 全局「公告」页或 Dashboard 侧栏：跨站公告列表（站点名 + 时间 + 内容，hash 去重） |
| 签到配置 | 站点详情签到卡片增加「每日自动签到」开关（写 `sites.sync_cfg`，S5 后端已支持）+ 下次执行时间展示 |

## 通用约定（贯穿各阶段）

1. **敏感红线**：任何页面不渲染 key 明文；reveal 仅走既有确认弹窗（M6 4.3 已定）。
2. **freshness 优先**：所有投影数据展示区都标注数据新鲜度，不伪造「实时」。
3. **类型生成**：每阶段后端 API 定稿后同步更新 `frontend/src/api/types.ts`，禁止手写漂移。
4. **降级展示**：`Capabilities` 不含的域（如 sub2api 无签到/无 models）直接隐藏入口，不留死按钮。
