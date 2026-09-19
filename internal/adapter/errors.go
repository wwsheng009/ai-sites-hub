// Package errors（adapter 包内）AdapterError 统一错误分类（architecture §4.3）。
package adapter

import "fmt"

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

// NewErr 构建 AdapterError。
func NewErr(code Code, msg string, cause error) *AdapterError {
	return &AdapterError{Code: code, Message: msg, Cause: cause}
}
