// SPA 静态资源挂载（webui.Dist() 提供 fs；无嵌入时为空操作）。
// 路由规则：/api/* 保持 JSON 404；其余路径优先命中静态文件，未命中回退 index.html（前端路由深链）。
package httpx

import (
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// AttachSPA 挂载前端单页应用；fsys 为 nil（API-only 构建）时跳过。
func AttachSPA(r *gin.Engine, fsys http.FileSystem) {
	if fsys == nil {
		return
	}
	h := spaHandler(fsys)
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if p == "/api" || strings.HasPrefix(p, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "接口不存在", "data": nil})
			return
		}
		h.ServeHTTP(c.Writer, c.Request)
	})
}

// spaHandler 静态文件 + index.html fallback；assets 走长缓存，index 禁缓存。
func spaHandler(fsys http.FileSystem) http.Handler {
	fileServer := http.FileServer(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upath := path.Clean(r.URL.Path)
		if strings.HasPrefix(upath, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}

		if f, err := fsys.Open(upath); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		serveIndex(w, r, fsys)
	})
}

// serveIndex 回退 index.html（SPA 深链 /sites/:id 等非文件路径）。
func serveIndex(w http.ResponseWriter, r *http.Request, fsys http.FileSystem) {
	f, err := fsys.Open("/index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, "index.html", st.ModTime(), f)
}
