// Package repo GORM 数据访问（M2 骨架：sites/credentials/keys/groups/affiliates/events）。
package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"aiclient/internal/model"
)

// ErrNotFound 记录不存在。
var ErrNotFound = errors.New("repo: 记录不存在")

// Repo 数据访问集合。
type Repo struct {
	db *gorm.DB
}

// New 构建 Repo。
func New(db *gorm.DB) *Repo { return &Repo{db: db} }

// DB 暴露底层句柄（事务场景用）。
func (r *Repo) DB() *gorm.DB { return r.db }

func newID() string { return uuid.NewString() }

// ---- Site ----

// CreateSiteInput 新建站点入参。
type CreateSiteInput struct {
	Name     string
	BaseURL  string
	ProxyURL string // 站点级出站代理（可空；空=回落全局 proxy.url）
	SiteType string // 可空：由识别流程回填
}

// CreateSite 创建站点（base_url 唯一）。
func (r *Repo) CreateSite(ctx context.Context, in CreateSiteInput) (*model.Site, error) {
	s := &model.Site{
		ID:       newID(),
		Name:     in.Name,
		BaseURL:  in.BaseURL,
		ProxyURL: in.ProxyURL,
		SiteType: in.SiteType,
		Status:   "active",
	}
	if err := r.db.WithContext(ctx).Create(s).Error; err != nil {
		return nil, fmt.Errorf("repo: 创建站点: %w", err)
	}
	return s, nil
}

// GetSite 按 id 查询。
func (r *Repo) GetSite(ctx context.Context, id string) (*model.Site, error) {
	var s model.Site
	if err := r.db.WithContext(ctx).First(&s, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repo: 查询站点: %w", err)
	}
	return &s, nil
}

// ListSites 站点列表（status 过滤）。
func (r *Repo) ListSites(ctx context.Context, status string) ([]model.Site, error) {
	q := r.db.WithContext(ctx).Model(&model.Site{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var out []model.Site
	if err := q.Order("created_at DESC").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 站点列表: %w", err)
	}
	return out, nil
}

// SiteFilter 站点列表过滤。
type SiteFilter struct {
	Status   string
	SiteType string
	Q        string // name/base_url 模糊
}

// ListSitesByFilter 带过滤的站点列表。
func (r *Repo) ListSitesByFilter(ctx context.Context, f SiteFilter) ([]model.Site, error) {
	q := r.db.WithContext(ctx).Model(&model.Site{})
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.SiteType != "" {
		q = q.Where("site_type = ?", f.SiteType)
	}
	if f.Q != "" {
		like := "%" + f.Q + "%"
		q = q.Where("name LIKE ? OR base_url LIKE ?", like, like)
	}
	var out []model.Site
	if err := q.Order("created_at DESC").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 站点列表: %w", err)
	}
	return out, nil
}

// UpdateSite 更新站点（nil 字段跳过）。
func (r *Repo) UpdateSite(ctx context.Context, id string, fn func(*model.Site)) (*model.Site, error) {
	var s model.Site
	if err := r.db.WithContext(ctx).First(&s, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repo: 查询站点: %w", err)
	}
	fn(&s)
	if err := r.db.WithContext(ctx).Save(&s).Error; err != nil {
		return nil, fmt.Errorf("repo: 保存站点: %w", err)
	}
	return &s, nil
}

// DeleteSite 删除站点（级联由 FK 处理）。
func (r *Repo) DeleteSite(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Delete(&model.Site{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("repo: 删除站点: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Credential ----

// UpsertCredentialInput 凭据写入入参（cipher 由 service 层加密后传入）。
type UpsertCredentialInput struct {
	SiteID             string
	AuthMode           string
	Username           string
	PasswordCipher     []byte
	TOTPSecretCipher   []byte
	AccessTokenCipher  []byte
	RefreshTokenCipher []byte
	PATCipher          []byte
}

// UpsertCredential 创建或更新凭据（site_id 唯一）。
func (r *Repo) UpsertCredential(ctx context.Context, in UpsertCredentialInput) (*model.SiteCredential, error) {
	var c model.SiteCredential
	err := r.db.WithContext(ctx).First(&c, "site_id = ?", in.SiteID).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c = model.SiteCredential{ID: newID(), SiteID: in.SiteID}
	case err != nil:
		return nil, fmt.Errorf("repo: 查询凭据: %w", err)
	}
	c.AuthMode = in.AuthMode
	if in.Username != "" {
		c.Username = in.Username
	}
	if in.PasswordCipher != nil {
		c.PasswordCipher = in.PasswordCipher
	}
	if in.TOTPSecretCipher != nil {
		c.TOTPSecretCipher = in.TOTPSecretCipher
	}
	if in.AccessTokenCipher != nil {
		c.AccessTokenCipher = in.AccessTokenCipher
	}
	if in.RefreshTokenCipher != nil {
		c.RefreshTokenCipher = in.RefreshTokenCipher
	}
	if in.PATCipher != nil {
		c.PATCipher = in.PATCipher
	}
	if err := r.db.WithContext(ctx).Save(&c).Error; err != nil {
		return nil, fmt.Errorf("repo: 保存凭据: %w", err)
	}
	return &c, nil
}

// GetCredential 按 site_id 查询凭据。
func (r *Repo) GetCredential(ctx context.Context, siteID string) (*model.SiteCredential, error) {
	var c model.SiteCredential
	if err := r.db.WithContext(ctx).First(&c, "site_id = ?", siteID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repo: 查询凭据: %w", err)
	}
	return &c, nil
}

// UpdateCredentialState 更新认证状态机字段。
func (r *Repo) UpdateCredentialState(ctx context.Context, siteID, state, msg string, extra func(*model.SiteCredential)) error {
	q := r.db.WithContext(ctx).Model(&model.SiteCredential{}).
		Where("site_id = ?", siteID).
		Updates(map[string]any{
			"auth_state":     state,
			"auth_state_msg": msg,
			"updated_at":     time.Now(),
		})
	if extra != nil {
		var c model.SiteCredential
		if err := r.db.WithContext(ctx).First(&c, "site_id = ?", siteID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("repo: 查询凭据: %w", err)
		}
		extra(&c)
		if err := r.db.WithContext(ctx).Save(&c).Error; err != nil {
			return fmt.Errorf("repo: 保存凭据: %w", err)
		}
		return nil
	}
	if q.Error != nil {
		return fmt.Errorf("repo: 更新凭据状态: %w", q.Error)
	}
	return nil
}

// ---- SiteKey ----

// UpsertKeys 批量 upsert key 投影（UNIQUE(site_id, remote_key_id)）。
func (r *Repo) UpsertKeys(ctx context.Context, siteID string, keys []model.SiteKey) error {
	if len(keys) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range keys {
			keys[i].ID = newID()
			keys[i].SiteID = siteID
			now := time.Now()
			keys[i].LastSyncAt = &now
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "site_id"}, {Name: "remote_key_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"name", "grp", "status", "quota_limit", "quota_used", "quota_remaining", "unlimited", "expires_at", "key_source", "freshness", "last_sync_at"}),
			}).Create(&keys[i]).Error; err != nil {
				return fmt.Errorf("repo: upsert key %s: %w", keys[i].RemoteKeyID, err)
			}
		}
		return nil
	})
}

// ListKeys 查询 key 投影。
func (r *Repo) ListKeys(ctx context.Context, siteID string) ([]model.SiteKey, error) {
	var out []model.SiteKey
	q := r.db.WithContext(ctx)
	if siteID != "" {
		q = q.Where("site_id = ?", siteID)
	}
	if err := q.Order("site_id, remote_key_id").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: key 列表: %w", err)
	}
	return out, nil
}

// ---- SiteGroup ----

// UpsertGroups 批量 upsert 分组投影。
func (r *Repo) UpsertGroups(ctx context.Context, siteID string, groups []model.SiteGroup) error {
	if len(groups) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range groups {
			groups[i].ID = newID()
			groups[i].SiteID = siteID
			now := time.Now()
			groups[i].LastSyncAt = &now
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "site_id"}, {Name: "remote_group_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"name", "ratio", "available", "desc", "freshness", "last_sync_at"}),
			}).Create(&groups[i]).Error; err != nil {
				return fmt.Errorf("repo: upsert group %s: %w", groups[i].RemoteGroupID, err)
			}
		}
		return nil
	})
}

// ListGroups 查询分组投影。
func (r *Repo) ListGroups(ctx context.Context, siteID string) ([]model.SiteGroup, error) {
	var out []model.SiteGroup
	q := r.db.WithContext(ctx)
	if siteID != "" {
		q = q.Where("site_id = ?", siteID)
	}
	if err := q.Order("site_id, remote_group_id").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 分组列表: %w", err)
	}
	return out, nil
}

// ---- SiteAffiliate（FR-10）----

// UpsertAffiliate 创建/更新返利投影（site_id 唯一）。
func (r *Repo) UpsertAffiliate(ctx context.Context, a *model.SiteAffiliate) error {
	var existing model.SiteAffiliate
	err := r.db.WithContext(ctx).First(&existing, "site_id = ?", a.SiteID).Error
	now := time.Now()
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		a.ID = newID()
		a.LastSyncAt = &now
		if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
			return fmt.Errorf("repo: 创建返利投影: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("repo: 查询返利投影: %w", err)
	}
	a.ID = existing.ID
	a.AffCfg = existing.AffCfg // 保留本地规则配置
	a.LastSyncAt = &now
	if err := r.db.WithContext(ctx).Model(&model.SiteAffiliate{}).
		Where("site_id = ?", a.SiteID).
		Updates(map[string]any{
			"aff_code":      a.AffCode,
			"rebate_rate":   a.RebateRate,
			"available":     a.Available,
			"frozen":        a.Frozen,
			"history":       a.History,
			"invitee_count": a.InviteeCount,
			"freshness":     a.Freshness,
			"last_sync_at":  now,
			"updated_at":    now,
		}).Error; err != nil {
		return fmt.Errorf("repo: 更新返利投影: %w", err)
	}
	return nil
}

// GetAffiliate 按 site_id 查询返利投影。
func (r *Repo) GetAffiliate(ctx context.Context, siteID string) (*model.SiteAffiliate, error) {
	var a model.SiteAffiliate
	if err := r.db.WithContext(ctx).First(&a, "site_id = ?", siteID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repo: 查询返利投影: %w", err)
	}
	return &a, nil
}

// ListAffiliates 返利投影列表（可按 site_ids 过滤）。
func (r *Repo) ListAffiliates(ctx context.Context, siteIDs []string) ([]model.SiteAffiliate, error) {
	var out []model.SiteAffiliate
	q := r.db.WithContext(ctx).Model(&model.SiteAffiliate{})
	if len(siteIDs) > 0 {
		q = q.Where("site_id IN ?", siteIDs)
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 返利列表: %w", err)
	}
	return out, nil
}

// UpdateAffiliateCfg 更新划转规则 JSON（FR-10.4 配置）；投影不存在时以
// freshness=missing 的最小行 upsert（单管理员场景，无并发竞争担忧）。
func (r *Repo) UpdateAffiliateCfg(ctx context.Context, siteID, cfgJSON string) error {
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&model.SiteAffiliate{}).
		Where("site_id = ?", siteID).
		Updates(map[string]any{"aff_cfg": cfgJSON, "updated_at": now})
	if res.Error != nil {
		return fmt.Errorf("repo: 更新划转规则: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		row := model.SiteAffiliate{
			ID: newID(), SiteID: siteID,
			AffCfg: cfgJSON, Freshness: "missing", UpdatedAt: now,
		}
		if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
			return fmt.Errorf("repo: 创建划转规则投影: %w", err)
		}
	}
	return nil
}

// InsertAffTransfer 记录划转。
func (r *Repo) InsertAffTransfer(ctx context.Context, t *model.AffTransfer) error {
	if t.ID == "" {
		t.ID = newID()
	}
	if err := r.db.WithContext(ctx).Create(t).Error; err != nil {
		return fmt.Errorf("repo: 记录划转: %w", err)
	}
	return nil
}

// ListAffTransfers 划转记录（按站点）。
func (r *Repo) ListAffTransfers(ctx context.Context, siteID string, limit int) ([]model.AffTransfer, error) {
	var out []model.AffTransfer
	q := r.db.WithContext(ctx).Model(&model.AffTransfer{})
	if siteID != "" {
		q = q.Where("site_id = ?", siteID)
	}
	if limit <= 0 {
		limit = 100
	}
	if err := q.Order("created_at DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 划转记录: %w", err)
	}
	return out, nil
}

// ---- Snapshot / Event ----

// InsertSnapshot 存快照（body 已脱敏）。
func (r *Repo) InsertSnapshot(ctx context.Context, siteID, kind, body string, statusCode *int) error {
	s := &model.Snapshot{ID: newID(), SiteID: siteID, Kind: kind, Body: body, StatusCode: statusCode}
	if err := r.db.WithContext(ctx).Create(s).Error; err != nil {
		return fmt.Errorf("repo: 存快照: %w", err)
	}
	return nil
}

// InsertEvent 记平台事件。
func (r *Repo) InsertEvent(ctx context.Context, e *model.Event) error {
	if e.ID == "" {
		e.ID = newID()
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return fmt.Errorf("repo: 记事件: %w", err)
	}
	return nil
}

// ListEvents 事件流水（since/site_id 过滤）。
func (r *Repo) ListEvents(ctx context.Context, since *time.Time, siteID string, limit int) ([]model.Event, error) {
	q := r.db.WithContext(ctx).Model(&model.Event{})
	if since != nil {
		q = q.Where("created_at >= ?", *since)
	}
	if siteID != "" {
		q = q.Where("site_id = ?", siteID)
	}
	if limit <= 0 {
		limit = 200
	}
	var out []model.Event
	if err := q.Order("created_at DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, fmt.Errorf("repo: 事件列表: %w", err)
	}
	return out, nil
}
