// Package service doctor 体检报告（cmd doctor 与 GET /doctor 共用）。
package service

import (
	"context"
	"fmt"
	"time"
)

// DoctorCheck 单项检查。
type DoctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok | warn | fail
	Detail string `json:"detail"`
}

// DoctorReport 体检报告。
type DoctorReport struct {
	Overall   string        `json:"overall"` // ok | warn | fail
	CheckedAt time.Time     `json:"checked_at"`
	Checks    []DoctorCheck `json:"checks"`
}

// Doctor 执行体检：主密钥、数据库连通、站点类型分布、过期凭据。
func (s *Services) Doctor(ctx context.Context) *DoctorReport {
	rep := &DoctorReport{CheckedAt: time.Now(), Overall: "ok"}
	add := func(name, status, detail string) {
		rep.Checks = append(rep.Checks, DoctorCheck{Name: name, Status: status, Detail: detail})
		switch status {
		case "fail":
			rep.Overall = "fail"
		case "warn":
			if rep.Overall != "fail" {
				rep.Overall = "warn"
			}
		}
	}

	// 1) 主密钥（FR-2/NFR-3 前提）
	if s.Sec == nil {
		add("master_key", "fail", "未配置主密钥（security.master_key / AISC_SECURITY_MASTER_KEY），凭据写入与登录测试不可用")
	} else {
		add("master_key", "ok", "已配置 AES-256-GCM 主密钥")
	}

	// 2) 数据库连通
	qctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := s.Repo.ListSites(qctx, ""); err != nil {
		add("database", "fail", fmt.Sprintf("数据库查询失败: %v", err))
	} else {
		add("database", "ok", "SQLite 连接正常")
	}

	// 3) 站点类型分布（unknown 提示补识别）
	sites, err := s.Repo.ListSites(qctx, "")
	if err == nil {
		unknown := 0
		for _, st := range sites {
			if st.SiteType == "unknown" || st.SiteType == "" {
				unknown++
			}
		}
		if unknown > 0 {
			add("site_types", "warn", fmt.Sprintf("%d/%d 个站点类型未知，可 POST /sites/:id/detect 或手动指定", unknown, len(sites)))
		} else {
			add("site_types", "ok", fmt.Sprintf("共 %d 个站点，类型均已识别或手动指定", len(sites)))
		}

		// 4) 登录态健康（token_expired/refresh_failed 提示）
		expired := 0
		for _, st := range sites {
			if c, err := s.Repo.GetCredential(qctx, st.ID); err == nil {
				if c.AuthState == "token_expired" || c.AuthState == "refresh_failed" || c.AuthState == "login_failed" {
					expired++
				}
			}
		}
		if expired > 0 {
			add("auth_states", "warn", fmt.Sprintf("%d 个站点登录态异常（auth-test / 重新配置凭据）", expired))
		} else {
			add("auth_states", "ok", "所有站点登录态正常")
		}
	}

	return rep
}
