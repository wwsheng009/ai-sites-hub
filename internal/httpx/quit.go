// Package httpx 路由注册（architecture §6 域划分；M2 骨架：健康/站点/凭据/同步/返利/事件）。
package httpx

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// RegisterQuitHandler 注册 POST /-/quit 优雅退出端点（仅本地控制用，无鉴权）。
// 调用方传入的 shutdown 函数会在独立 goroutine 中执行，响应立即返回。
func RegisterQuitHandler(r *gin.Engine, shutdown func(ctx context.Context) error) {
	r.POST("/-/quit", func(c *gin.Context) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = shutdown(ctx)
		}()
		c.JSON(http.StatusOK, gin.H{"status": "shutting down"})
	})
}
