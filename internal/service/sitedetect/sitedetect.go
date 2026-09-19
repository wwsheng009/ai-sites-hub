// Package sitedetect 自动识别打分器（architecture §4.4）：并发探测两类 adapter 特征，加权判定。
package sitedetect

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"aiclient/internal/adapter"
)

// Result 识别结果（写入 sites.site_type/detect_score/detect_report + snapshots kind=detect）。
type Result struct {
	SiteType      string   `json:"site_type"`
	Score         int      `json:"score"`
	Confidence    string   `json:"confidence"` // high | low | unknown
	ReportJSON    string   `json:"-"`
	RawReport     Report   `json:"-"`
	CheckedProbes []string `json:"checked_probes"`
	LatencyMs     int64    `json:"latency_ms"`
}

// Report 打分报告（留证）。
type Report struct {
	Sub2API adapter.DetectResult `json:"sub2api"`
	NewAPI  adapter.DetectResult `json:"newapi"`
	Winner  string               `json:"winner"`
	Note    string               `json:"note"`
}

// Threshold 打分阈值（对齐 ai-gateway：score>=6 判定）。
const Threshold = 6

// Detector 打分器。
type Detector struct {
	reg *adapter.Registry
	log *slog.Logger
}

// New 构建打分器。
func New(reg *adapter.Registry, log *slog.Logger) *Detector {
	if log == nil {
		log = slog.Default()
	}
	return &Detector{reg: reg, log: log}
}

// Detect 并发探测 sub2api/new-api 特征并判定类型。
// 并列或均低于阈值 → unknown + 报告留证。
func (d *Detector) Detect(ctx context.Context, baseURL string) (*Result, error) {
	start := time.Now()

	sub, err := d.reg.Get(adapter.TypeSub2API, baseURL, d.log)
	if err != nil {
		return nil, fmt.Errorf("sitedetect: %w", err)
	}
	na, err := d.reg.Get(adapter.TypeNewAPI, baseURL, d.log)
	if err != nil {
		return nil, fmt.Errorf("sitedetect: %w", err)
	}

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		subRes adapter.DetectResult
		newRes adapter.DetectResult
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		r, err := sub.Detect(cctx, baseURL)
		mu.Lock()
		defer mu.Unlock()
		subRes = r
		if err != nil {
			subRes.Evidence = map[string]any{"error": err.Error()}
			d.log.Debug("sitedetect: sub2api 探测失败", "base_url", baseURL, "err", err)
		}
	}()
	go func() {
		defer wg.Done()
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		r, err := na.Detect(cctx, baseURL)
		mu.Lock()
		defer mu.Unlock()
		newRes = r
		if err != nil {
			newRes.Evidence = map[string]any{"error": err.Error()}
			d.log.Debug("sitedetect: new-api 探测失败", "base_url", baseURL, "err", err)
		}
	}()
	wg.Wait()

	rep := Report{Sub2API: subRes, NewAPI: newRes}
	res := &Result{LatencyMs: time.Since(start).Milliseconds()}

	switch {
	case subRes.Score >= Threshold && subRes.Score > newRes.Score:
		res.SiteType = string(adapter.TypeSub2API)
		res.Score = subRes.Score
		res.Confidence = "high"
		rep.Winner = res.SiteType
	case newRes.Score >= Threshold && newRes.Score > subRes.Score:
		res.SiteType = string(adapter.TypeNewAPI)
		res.Score = newRes.Score
		res.Confidence = "high"
		rep.Winner = res.SiteType
	case subRes.Score == newRes.Score && subRes.Score >= Threshold:
		// 并列：更新鲜的候选（分数相同但证据更多者）→ 保守 unknown
		res.SiteType = string(adapter.TypeUnknown)
		res.Score = subRes.Score
		res.Confidence = "low"
		rep.Winner = "tie"
		rep.Note = "两类特征得分并列，保守判 unknown（可手动指定 site_type 覆盖）"
	default:
		res.SiteType = string(adapter.TypeUnknown)
		res.Score = max(subRes.Score, newRes.Score)
		res.Confidence = "unknown"
		rep.Winner = "none"
		rep.Note = "特征得分未达阈值"
	}
	res.CheckedProbes = []string{"sub2api:/setup/status", "sub2api:/api/v1/settings/public", "newapi:/api/status"}

	b, err := json.Marshal(rep)
	if err != nil {
		return nil, fmt.Errorf("sitedetect: 序列化报告: %w", err)
	}
	res.RawReport = rep
	res.ReportJSON = string(b)
	return res, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
