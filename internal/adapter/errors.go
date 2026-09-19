// Package errors（adapter 包内）AdapterError 统一错误分类（architecture §4.3）。
package adapter

import (
	"errors"
	"fmt"
)

// Code 错误分类码（对齐 site_credentials.auth_state 与作业 error_class）。
type Code string

const (
	CodeUnauthorized      Code = "unauthorized"
	CodeForbidden         Code = "forbidden"
	CodeRateLimited       Code = "rate_limited"
	CodeBlockedByCF       Code = "blocked_by_cf"
	CodeTurnstileRequired Code = "turnstile_required"
	CodeTOTPRequired      Code = "totp_required"
	CodeUpstreamError     Code = "upstream_error"
	CodeUnsupported       Code = "unsupported"
)

// AdapterError 统一错误类型（errors.As 判定）。
type AdapterError struct {
	Code    Code
	Message string
	Cause   error
}

func (e *AdapterError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("adapter %s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("adapter %s: %s", e.Code, e.Message)
}

func (e *AdapterError) Unwrap() error { return e.Cause }

// CodeOf 提取错误链中的 Code；未找到时返回 CodeUpstreamError + false。
// 用于调度器按错误码分流（禁止 strings.Contains(err.Error(), ...)）。
func CodeOf(err error) (Code, bool) {
	var ae *AdapterError
	if errors.As(err, &ae) {
		return ae.Code, true
	}
	return "", false
}

// IsCode 判定错误链是否匹配指定 Code。
func IsCode(err error, code Code) bool {
	c, _ := CodeOf(err)
	return c == code
}
func NewErr(code Code, msg string, cause error) *AdapterError {
	return &AdapterError{Code: code, Message: msg, Cause: cause}
}
