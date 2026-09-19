//go:build webui_embed

// embed 构建：嵌入 internal/webui/dist（由 scripts/build.ps1 从 frontend/dist 同步）。
package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var distFS embed.FS

// Dist 返回嵌入的前端资源文件系统。
func Dist() http.FileSystem {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil
	}
	return http.FS(sub)
}
