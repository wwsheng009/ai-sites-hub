// Package service 返利记录查询（FR-10 审计留证）。
package service

import (
	"context"

	"aiclient/internal/model"
)

// ListTransfers 划转记录（按站点，最新在前，上限 100 条）。
func (s *Services) ListTransfers(ctx context.Context, siteID string) ([]model.AffTransfer, error) {
	return s.Repo.ListAffTransfers(ctx, siteID, 100)
}
