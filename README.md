# ai-sites-client

AI 中转站点集中管理平台：集中管理大量 sub2api / new-api 站点——自动识别站点类型、凭据自动登录、定时签到、key/分组/额度/返利集中查询，提供 Web / HTTP API / cmd 三种入口。

> 状态：M1 文档已冻结（需求 v0.3 / 架构 v0.2 / 计划 v0.2），当前处于 **M2 服务端骨架** 阶段。

## 快速导航

- 文档中心：[docs/README.md](docs/README.md)
- 需求分析：[docs/requirements/requirements.md](docs/requirements/requirements.md)
- 架构设计：[docs/architecture/architecture.md](docs/architecture/architecture.md)
- 实施计划：[docs/plan/implementation-plan.md](docs/plan/implementation-plan.md)

## 开发（M2 骨架）

```bash
# 后端（Go 1.24+）
go build ./...
go test ./...

# 启动 API 服务
go run ./cmd/aiclient serve

# 数据库迁移 / 体检
go run ./cmd/aiclient migrate
go run ./cmd/aiclient doctor

# 前端（Node 20+）
cd frontend && npm install && npm run dev
```
