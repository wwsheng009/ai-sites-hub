// Package httpx 路由注册（architecture §6 域划分；M2 骨架：健康/站点/凭据/同步/返利/事件）。
package httpx

import (
	"github.com/gin-gonic/gin"

	"aiclient/internal/service"
)

// SetupRouter 构建引擎并注册全部路由。
func SetupRouter(svc *service.Services) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	api := New(svc)
	v1 := r.Group("/api/v1")

	// system
	v1.GET("/health", api.Health)
	v1.GET("/doctor", api.Doctor)

	// sites（FR-1）
	v1.GET("/sites", api.ListSites)
	v1.POST("/sites", api.CreateSite)
	v1.GET("/sites/:id", api.GetSite)
	v1.PUT("/sites/:id", api.UpdateSite)
	v1.DELETE("/sites/:id", api.DeleteSite)
	v1.POST("/sites/:id/detect", api.DetectSite)

	// credentials（FR-2）
	v1.PUT("/sites/:id/credentials", api.PutCredentials)
	v1.GET("/sites/:id/auth-state", api.GetAuthState)
	v1.POST("/sites/:id/auth-test", api.AuthTest)

	// sync（FR-4）
	v1.POST("/sites/:id/sync", api.SyncSite)
	v1.POST("/sync/all", api.SyncAll)

	// keys / groups（FR-4.2/4.4）
	v1.GET("/keys", api.ListKeys)
	v1.GET("/groups", api.ListGroups)

	// affiliates（FR-10）
	v1.GET("/affiliates", api.ListAffiliates)
	v1.GET("/affiliates/site/:siteId", api.GetAffiliate)
	v1.GET("/affiliates/site/:siteId/rule", api.GetAffiliateRule)
	v1.PUT("/affiliates/site/:siteId/rule", api.PutAffiliateRule)
	v1.POST("/affiliates/site/:siteId/transfer", api.TransferAffiliate)
	v1.GET("/affiliates/site/:siteId/transfers", api.ListTransfers)

	// events（FR-5.3）
	v1.GET("/events", api.ListEvents)

	return r
}

// ---- system ----

// Health GET /api/v1/health。
func (a *API) Health(c *gin.Context) {
	OK(c, gin.H{"status": "up", "version": "0.1.0"})
}

// Doctor GET /api/v1/doctor（M2 骨架版：无主密钥告警）。
func (a *API) Doctor(c *gin.Context) {
	OK(c, a.Svc.Doctor(c.Request.Context()))
}

// ---- sites ----

// ListSites GET /api/v1/sites?status=&site_type=&q=
func (a *API) ListSites(c *gin.Context) {
	sites, err := a.Svc.ListSites(c.Request.Context(),
		c.Query("status"), c.Query("site_type"), c.Query("q"))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, sites)
}

type createSiteReq struct {
	Name      string `json:"name"`
	BaseURL   string `json:"base_url" binding:"required"`
	SiteType  string `json:"site_type"`
	DetectNow *bool  `json:"detect_now"`
}

// CreateSite POST /api/v1/sites。
func (a *API) CreateSite(c *gin.Context) {
	var req createSiteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, BadReq(err.Error()))
		return
	}
	site, err := a.Svc.CreateSite(c.Request.Context(), service.CreateSiteInput{
		Name: req.Name, BaseURL: req.BaseURL, ManualType: req.SiteType, DetectNow: req.DetectNow,
	})
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, site)
}

// GetSite GET /api/v1/sites/:id。
func (a *API) GetSite(c *gin.Context) {
	site, err := a.Svc.GetSite(c.Request.Context(), c.Param("id"))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, site)
}

type updateSiteReq struct {
	Name     *string `json:"name"`
	Status   *string `json:"status"`
	SiteType *string `json:"site_type"`
}

// UpdateSite PUT /api/v1/sites/:id。
func (a *API) UpdateSite(c *gin.Context) {
	var req updateSiteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, BadReq(err.Error()))
		return
	}
	site, err := a.Svc.UpdateSite(c.Request.Context(), c.Param("id"), service.UpdateSiteInput{
		Name: req.Name, Status: req.Status, SiteType: req.SiteType,
	})
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, site)
}

// DeleteSite DELETE /api/v1/sites/:id。
func (a *API) DeleteSite(c *gin.Context) {
	if err := a.Svc.DeleteSite(c.Request.Context(), c.Param("id")); err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"deleted": true})
}

// DetectSite POST /api/v1/sites/:id/detect。
func (a *API) DetectSite(c *gin.Context) {
	site, err := a.Svc.DetectSite(c.Request.Context(), c.Param("id"))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, site)
}

// ---- credentials ----

// PutCredentials PUT /api/v1/sites/:id/credentials。
func (a *API) PutCredentials(c *gin.Context) {
	var req service.PutCredentialsInput
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, BadReq(err.Error()))
		return
	}
	if err := a.Svc.PutCredentials(c.Request.Context(), c.Param("id"), req); err != nil {
		Fail(c, err)
		return
	}
	OK(c, gin.H{"saved": true})
}

// GetAuthState GET /api/v1/sites/:id/auth-state。
func (a *API) GetAuthState(c *gin.Context) {
	st, err := a.Svc.GetAuthState(c.Request.Context(), c.Param("id"))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, st)
}

// AuthTest POST /api/v1/sites/:id/auth-test。
func (a *API) AuthTest(c *gin.Context) {
	st, err := a.Svc.AuthTest(c.Request.Context(), c.Param("id"))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, st)
}

// ---- sync ----

// SyncSite POST /api/v1/sites/:id/sync。
func (a *API) SyncSite(c *gin.Context) {
	res, err := a.Svc.SyncSite(c.Request.Context(), c.Param("id"))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, res)
}

// SyncAll POST /api/v1/sync/all。
func (a *API) SyncAll(c *gin.Context) {
	sites, err := a.Svc.ListSites(c.Request.Context(), "active", "", "")
	if err != nil {
		Fail(c, err)
		return
	}
	results := make([]any, 0, len(sites))
	for _, s := range sites {
		res, err := a.Svc.SyncSite(c.Request.Context(), s.ID)
		if err != nil {
			results = append(results, gin.H{"site_id": s.ID, "error": err.Error()})
			continue
		}
		results = append(results, res)
	}
	OK(c, results)
}

// ---- keys / groups ----

// ListKeys GET /api/v1/keys?site_id=
func (a *API) ListKeys(c *gin.Context) {
	keys, err := a.Svc.ListKeys(c.Request.Context(), c.Query("site_id"))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, keys)
}

// ListGroups GET /api/v1/groups?site_id=
func (a *API) ListGroups(c *gin.Context) {
	groups, err := a.Svc.ListGroups(c.Request.Context(), c.Query("site_id"))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, groups)
}

// ---- affiliates（FR-10）----

// ListAffiliates GET /api/v1/affiliates?site_id=
func (a *API) ListAffiliates(c *gin.Context) {
	rows, err := a.Svc.ListAffiliates(c.Request.Context(), c.Query("site_id"))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, rows)
}

// GetAffiliate GET /api/v1/affiliates/site/:siteId。
func (a *API) GetAffiliate(c *gin.Context) {
	row, err := a.Svc.GetAffiliate(c.Request.Context(), c.Param("siteId"))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, row)
}

// GetAffiliateRule GET /api/v1/affiliates/site/:siteId/rule。
func (a *API) GetAffiliateRule(c *gin.Context) {
	rule, err := a.Svc.GetAffiliateRule(c.Request.Context(), c.Param("siteId"))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, rule)
}

// PutAffiliateRule PUT /api/v1/affiliates/site/:siteId/rule。
func (a *API) PutAffiliateRule(c *gin.Context) {
	var req service.AffRule
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, BadReq(err.Error()))
		return
	}
	if err := a.Svc.PutAffiliateRule(c.Request.Context(), c.Param("siteId"), req); err != nil {
		Fail(c, err)
		return
	}
	OK(c, req)
}

// TransferAffiliate POST /api/v1/affiliates/site/:siteId/transfer。
func (a *API) TransferAffiliate(c *gin.Context) {
	var req service.TransferAffiliateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, BadReq(err.Error()))
		return
	}
	t, err := a.Svc.TransferAffiliate(c.Request.Context(), c.Param("siteId"), req)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, t)
}

// ListTransfers GET /api/v1/affiliates/site/:siteId/transfers。
func (a *API) ListTransfers(c *gin.Context) {
	ts, err := a.Svc.ListTransfers(c.Request.Context(), c.Param("siteId"))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, ts)
}

// ---- events ----

// ListEvents GET /api/v1/events?site_id=&limit=
func (a *API) ListEvents(c *gin.Context) {
	events, err := a.Svc.ListEvents(c.Request.Context(), c.Query("site_id"), nil, intOrDefault(c.Query("limit"), 200))
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, events)
}

func intOrDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return def
		}
		n = n*10 + int(ch-'0')
	}
	if n == 0 {
		return def
	}
	return n
}
