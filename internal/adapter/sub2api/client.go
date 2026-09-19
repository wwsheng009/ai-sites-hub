// Package sub2api sub2api 站点适配器（architecture §4.2 契约映射）。
package sub2api

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"aiclient/internal/adapter"
)

// Adapter sub2api 实现。
type Adapter struct {
	base     *url.URL // 归一化站点基址（含 scheme，无尾斜杠）
	proxyURL string   // 出站代理（Detect 重建 adapter 时沿用）
	hc       *adapter.HTTPClient
	log      *slog.Logger
}

// UA 标识。
const UserAgent = "ai-sites-client/0.1 (+sub2api-adapter)"

// New 构建 adapter。baseURL 允许带路径（如反代子路径）；proxyURL 为出站代理（空=直连）。
func New(baseURL, proxyURL string, log *slog.Logger) (*Adapter, error) {
	u, err := normalizeBase(baseURL)
	if err != nil {
		return nil, err
	}
	hc, err := adapter.NewHTTPClient(UserAgent, 15*time.Second, 0, proxyURL, log)
	if err != nil {
		return nil, fmt.Errorf("sub2api: 构建出站客户端: %w", err)
	}
	return &Adapter{base: u, proxyURL: proxyURL, hc: hc, log: log}, nil
}

// normalizeBase 归一化基址：补 scheme、去尾斜杠。
func normalizeBase(baseURL string) (*url.URL, error) {
	s := strings.TrimSpace(baseURL)
	if s == "" {
		return nil, fmt.Errorf("sub2api: baseURL 为空")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("sub2api: 解析 baseURL %q: %w", baseURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("sub2api: 不支持的 scheme %q", u.Scheme)
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	u.RawQuery, u.Fragment = "", ""
	return u, nil
}

// endpoint 拼接 API 路径。
func (a *Adapter) endpoint(path string) string {
	return a.base.String() + path
}

// Type 实现接口。
func (a *Adapter) Type() adapter.Type { return adapter.TypeSub2API }

// Capabilities sub2api：无签到；有 affiliate（划转）。
func (a *Adapter) Capabilities() adapter.Capabilities {
	return adapter.Capabilities{
		Checkin:      string(adapter.CheckinUnsupported),
		Affiliate:    "supported",
		Refresh:      true,
		ListKeys:     true,
		ListGroups:   true,
		Quota:        true,
		PlaintextKey: false, // 列表即明文，无需单独取
	}
}

// BaseURL 返回归一化基址。
func (a *Adapter) BaseURL() string { return a.base.String() }
