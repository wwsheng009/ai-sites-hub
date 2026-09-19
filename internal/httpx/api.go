// Package httpx HTTP handler 层（architecture §6）：API 依赖注入与各域 handler。
package httpx

import (
	"aiclient/internal/service"
)

// API HTTP 层依赖集合（方法集见各域文件）。
type API struct {
	Svc *service.Services
}

// New 构建 API。
func New(svc *service.Services) *API { return &API{Svc: apiSvc(svc)} }

func apiSvc(s *service.Services) *service.Services { return s }
