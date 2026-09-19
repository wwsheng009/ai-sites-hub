// Package httpx HTTP 层：统一响应 {code, message, data}（architecture §6）。
package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"aiclient/internal/service"
)

// OK 200 成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": data})
}

// badReq 可识别的 400 错误（handler 校验用）。
type badReq struct{ msg string }

func (b *badReq) Error() string    { return b.msg }
func (b *badReq) BadRequest() bool { return true }

// BadReq 构建 400 错误。
func BadReq(msg string) error { return &badReq{msg: msg} }

// Fail 错误响应：err 映射 HTTP 状态码（ErrNotFound → 404，badReq → 400）。
func Fail(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	msg := err.Error()

	var br *badReq
	switch {
	case err == service.ErrNotFound || errors.Is(err, service.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.As(err, &br):
		status, code = http.StatusBadRequest, "bad_request"
	}
	c.JSON(status, gin.H{"code": code, "message": msg, "data": nil})
}
