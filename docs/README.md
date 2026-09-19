# ai-sites-client 文档中心

AI 中转站点集中管理平台（sub2api / new-api 多站点管理）的文档中心。

## 项目定位

集中管理大量 AI 中转站点（主要是 sub2api / new-api 部署的站点），提供：

- 站点信息登记与**站点类型自动识别**
- **自动登录**（账号密码 / session 维持）
- **定时签到**（后台作业）
- 集中获取各站点的 **AI Key（token）列表 / 分组 / 额度 / 站点返利（affiliate）**等信息；返利支持阈值自动划转（FR-10）
- 多端操作入口：**Web 前端 / 服务端 API / cmd 命令行管理**

## 目录结构

```
docs/
├── README.md            # 本文件：文档索引
├── requirements/        # 需求分析文档目录
│   └── README.md        # 需求文档索引与需求分析
├── analysis/            # 参考项目分析文档目录
│   ├── README.md        # 分析文档索引
│   ├── sub2api-analysis.md
│   ├── new-api-analysis.md
│   ├── ai-gateway-analysis.md
│   └── ai-gateway-notification.md   # 通知子系统专项分析（FR-9 依据）
├── architecture/        # 架构设计文档目录
│   └── README.md
└── plan/                # 实施计划文档目录
    └── README.md
```

## 文档阅读顺序

1. `requirements/` — 先看需求分析：明确范围、功能清单、非功能约束
2. `analysis/` — 三个参考项目的 API/架构勘察结论（本项目的对接契约依据）
3. `architecture/` — 服务端 / 前端 / cmd / 后台作业的整体架构设计
4. `plan/` — 分阶段实施计划与里程碑

## 技术选型基线（当前结论，详见 architecture）

- 后端：Golang（参考 ai-gateway 的 cmd + internal 分层）
- 前端：直接复用 `E:\projects\ai\sub2api` 前端骨架与样式（仅骨架，不引入其业务后端功能）
- 后台作业：服务端内置调度器（定时签到 / 定时刷新 key 与额度）
- 配置：YAML 配置 + 持久化存储（sqlite 起步）
