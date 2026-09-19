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
	"net/url"
	"time"

	"golang.org/x/net/proxy"
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

// NewHTTPClient 构建客户端。proxyURL 为出站代理（空=直连；支持 http/https/socks5）。
func NewHTTPClient(ua string, timeout time.Duration, maxBody int64, proxyURL string, log *slog.Logger) (*HTTPClient, error) {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if maxBody <= 0 {
		maxBody = defaultMaxBody
	}
	if log == nil {
		log = slog.Default()
	}
	transport, err := proxyTransport(proxyURL)
	if err != nil {
		return nil, err
	}
	return &HTTPClient{
		hc:       &http.Client{Timeout: timeout, Transport: transport},
		ua:       ua,
		maxBody:  maxBody,
		maxRetry: maxRetries,
		log:      log,
	}, nil
}

// proxyTransport 按代理 URL 构建出站 Transport（空 → 直连默认）。
// 支持 http/https（CONNECT 隧道）与 socks5（含 socks5h：域名由代理解析）。
func proxyTransport(proxyURL string) (http.RoundTripper, error) {
	if proxyURL == "" {
		return http.DefaultTransport, nil
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("adapter: 解析代理 URL %q: %w", proxyURL, err)
	}
	switch u.Scheme {
	case "http", "https":
		return &http.Transport{Proxy: http.ProxyURL(u)}, nil
	case "socks5", "socks5h":
		var auth *proxy.Auth
		if u.User != nil {
			pwd, _ := u.User.Password()
			auth = &proxy.Auth{User: u.User.Username(), Password: pwd}
		}
		d, err := proxy.SOCKS5("tcp", u.Host, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("adapter: 构建 SOCKS5 拨号器: %w", err)
		}
		if cd, ok := d.(proxy.ContextDialer); ok {
			return &http.Transport{DialContext: cd.DialContext}, nil
		}
		return &http.Transport{Dial: d.Dial}, nil
	default:
		return nil, fmt.Errorf("adapter: 不支持的代理协议 %q（仅 http/https/socks5）", u.Scheme)
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
