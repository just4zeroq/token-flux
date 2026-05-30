package developer

import (
	"context"
	"time"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/frame/g"
)

type sDeveloper struct{}

func init() { service.RegisterDeveloper(New()) }

func New() *sDeveloper { return &sDeveloper{} }

// developerRow mirrors the developers table row.
type developerRow struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Website     string    `json:"website"`
	LogoURL     string    `json:"logo_url"`
	SortOrder   int       `json:"sort_order"`
	Status      int       `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (r *developerRow) toDTO() *dto.DeveloperInfo {
	return &dto.DeveloperInfo{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Website:     r.Website,
		LogoURL:     r.LogoURL,
		SortOrder:   r.SortOrder,
		Status:      r.Status,
	}
}

// ListActive returns all active developers, ordered by sort_order then name.
func (s *sDeveloper) ListActive(ctx context.Context) ([]*dto.DeveloperInfo, error) {
	var rows []developerRow
	err := g.DB().Model("developers").Ctx(ctx).
		Where("status", 1).
		Order("sort_order ASC, name ASC").
		Scan(&rows)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []*dto.DeveloperInfo{}, nil
	}
	out := make([]*dto.DeveloperInfo, len(rows))
	for i, r := range rows {
		out[i] = r.toDTO()
	}
	return out, nil
}
