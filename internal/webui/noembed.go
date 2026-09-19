//go:build !webui_embed

// 默认构建：不嵌入前端（无需 frontend/dist 存在，前端走 `npm run dev` 独立开发）。
package webui

import "net/http"

// Dist 未嵌入前端，返回 nil。
func Dist() http.FileSystem { return nil }
