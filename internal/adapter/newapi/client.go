// Package newapi new-api 站点适配器（architecture §4.2 契约映射）。
package newapi

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"aiclient/internal/adapter"
)

// Adapter new-api 实现。
type Adapter struct {
	base     *url.URL
	proxyURL string // 出站代理（Detect 重建 adapter 时沿用）
	hc       *adapter.HTTPClient
	log      *slog.Logger
}

// UA 标识。
const UserAgent = "ai-sites-client/0.1 (+newapi-adapter)"

// New 构建 adapter。proxyURL 为出站代理（空=直连）。
func New(baseURL, proxyURL string, log *slog.Logger) (*Adapter, error) {
	u, err := normalizeBase(baseURL)
	if err != nil {
		return nil, err
	}
	hc, err := adapter.NewHTTPClient(UserAgent, 15*time.Second, 0, proxyURL, log)
	if err != nil {
		return nil, fmt.Errorf("newapi: 构建出站客户端: %w", err)
	}
	return &Adapter{base: u, proxyURL: proxyURL, hc: hc, log: log}, nil
}

func normalizeBase(baseURL string) (*url.URL, error) {
	s := strings.TrimSpace(baseURL)
	if s == "" {
		return nil, fmt.Errorf("newapi: baseURL 为空")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("newapi: 解析 baseURL %q: %w", baseURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("newapi: 不支持的 scheme %q", u.Scheme)
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	u.RawQuery, u.Fragment = "", ""
	return u, nil
}

func (a *Adapter) endpoint(path string) string { return a.base.String() + path }

// BaseURL 返回归一化基址。
func (a *Adapter) BaseURL() string { return a.base.String() }

// Type 实现接口。
func (a *Adapter) Type() adapter.Type { return adapter.TypeNewAPI }

// Capabilities new-api：有签到（Turnstile 时受限）；有 affiliate。
func (a *Adapter) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{
		Checkin:      adapter.CapSupported,
		Affiliate:    adapter.CapSupported,
		Refresh:      false, // JWT 15min + cookie；首选 PAT 长期有效
		ListKeys:     true,
		ListGroups:   true,
		Quota:        true,
		PlaintextKey: true, // POST /api/token/:id/key 可取明文
	}
}
