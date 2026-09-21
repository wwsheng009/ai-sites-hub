# ai-sites-hub

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

## 构建发布（前端嵌入单文件）

前端构建产物可嵌入 Go 二进制，产出免部署的单文件可执行程序：

```powershell
# 一键构建：npm 构建 → 同步到 internal/webui/dist → 嵌入编译
powershell -ExecutionPolicy Bypass -File scripts/build.ps1 -WithWebUI -Version v0.1.0

# 产物 dist/aiclient.exe：serve 直接托管 Web 控制台（http://127.0.0.1:8080）
```

- 不带 `-WithWebUI` 时编译默认形态（无前端嵌入，仅 Web/HTTP API/cmd），开发调试用 `go run ./cmd/aiclient serve` + `npm run dev`
- 查看全部参数与用法：`powershell -ExecutionPolicy Bypass -File scripts/build.ps1 -h`
- 嵌入实现：build tag `webui_embed`（`internal/webui/embed.go` 的 `//go:embed all:dist`）；SPA 路由回退与静态缓存见 `internal/httpx/spa.go`
- `version` 子命令可验证构建形态：`aiclient v0.1.0 (release, webui-embedded)`
