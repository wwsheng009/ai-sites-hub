// Package httpx 公告聚合 API（S5；FR-4.4）。
package httpx

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// AnnouncementOut 公告出参。
type AnnouncementOut struct {
	SiteID      string     `json:"site_id"`
	SiteName    string     `json:"site_name"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

// ListAnnouncements GET /api/v1/announcements?limit=
func (a *API) ListAnnouncements(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	rows, err := a.Svc.ListAnnouncements(c.Request.Context(), limit)
	if err != nil {
		Fail(c, err)
		return
	}
	// 补充站点名
	sites, _ := a.Svc.ListSites(c.Request.Context(), "", "", "")
	siteNames := make(map[string]string, len(sites))
	for _, s := range sites {
		siteNames[s.ID] = s.Name
	}
	out := make([]AnnouncementOut, 0, len(rows))
	for _, r := range rows {
		out = append(out, AnnouncementOut{
			SiteID:      r.SiteID,
			SiteName:    siteNames[r.SiteID],
			Title:       r.Title,
			Content:     r.Content,
			PublishedAt: r.PublishedAt,
		})
	}
	OK(c, out)
}
