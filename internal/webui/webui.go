// Package webui 前端静态资源注入（单文件二进制分发，见 scripts/build.ps1）。
//
// 两种构建形态：
//   - 默认构建：无前端产物依赖，Dist 返回 nil，serve 仅提供 API。
//   - embed 构建（go build -tags webui_embed）：internal/webui/dist 全量嵌入，
//     serve 同时提供 API 与 SPA（/api/* 保持 JSON 404，其余路径走 SPA fallback）。
//
// 由 scripts/build.ps1 负责把 frontend/dist 同步到 internal/webui/dist 再编译。
package webui

// HasFrontend 报告当前二进制是否嵌入前端资源。
func HasFrontend() bool { return Dist() != nil }
