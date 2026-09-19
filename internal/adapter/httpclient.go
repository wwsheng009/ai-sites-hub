// Package adapter 统一 HTTP 客户端：UA、超时、响应体上限、429/5xx 退避重试（architecture §4.3）。
package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	defaultTimeout = 15 * time.Second
	defaultMaxBody = 2 << 20 // 2MB（architecture §4.3 响应体上限）
	maxRetries     = 2       // 除首次外最多重试 2 次
	retryBaseDelay = 500 * time.Millisecond
)

// HTTPClient 统一客户端（每 adapter 一个，携带类型 UA）。
type HTTPClient struct {
	hc       *http.Client
	ua       string
	maxBody  int64
	maxRetry int
	log      *slog.Logger
}

// NewHTTPClient 构建客户端。
func NewHTTPClient(ua string, timeout time.Duration, maxBody int64, log *slog.Logger) *HTTPClient {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if maxBody <= 0 {
		maxBody = defaultMaxBody
	}
	if log == nil {
		log = slog.Default()
	}
	return &HTTPClient{
		hc:       &http.Client{Timeout: timeout},
		ua:       ua,
		maxBody:  maxBody,
		maxRetry: maxRetries,
		log:      log,
	}
}

// DoJSON 发起请求并解析 JSON 响应到 out（可为 nil）。
// 429/5xx 指数退避重试；401/403/4xx 不重试。
func (c *HTTPClient) DoJSON(ctx context.Context, method, url string, body any, out any) (int, []byte, error) {
	return c.DoJSONWithHeader(ctx, method, url, body, out, nil)
}

// DoJSONWithHeader 在 DoJSON 基础上附加请求头（如 Authorization）。
func (c *HTTPClient) DoJSONWithHeader(ctx context.Context, method, url string, body any, out any, headers map[string]string) (int, []byte, error) {
	var raw []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("adapter: 序列化请求体: %w", err)
		}
		raw = b
	}

	var (
		status   int
		respBody []byte
	)
	var lastErr error
	for attempt := 0; attempt <= c.maxRetry; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return status, nil, ctx.Err()
			case <-time.After(retryBaseDelay << (attempt - 1)):
			}
		}
		st, b, err := c.doOnce(ctx, method, url, raw, headers)
		if err != nil {
			return st, b, err
		}
		status, respBody = st, b
		if retryable(status) && attempt < c.maxRetry {
			lastErr = fmt.Errorf("adapter: 上游 %s %s 状态码 %d", method, url, status)
			continue
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		return status, respBody, lastErr
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return status, respBody, fmt.Errorf("adapter: 解析响应 JSON: %w", err)
		}
	}
	return status, respBody, nil
}

func (c *HTTPClient) doOnce(ctx context.Context, method, url string, body []byte, headers map[string]string) (int, []byte, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return 0, nil, fmt.Errorf("adapter: 构建请求: %w", err)
	}
	req.Header.Set("User-Agent", c.ua)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("adapter: 请求 %s: %w", url, err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, c.maxBody+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("adapter: 读取响应: %w", err)
	}
	if int64(len(b)) > c.maxBody {
		return resp.StatusCode, b[:c.maxBody], fmt.Errorf("adapter: 响应体超过上限 %d 字节", c.maxBody)
	}
	return resp.StatusCode, b, nil
}

func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}
