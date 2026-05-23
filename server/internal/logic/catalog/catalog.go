package catalog

import (
	"context"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

type sCatalog struct{}

func init() { service.RegisterCatalog(New()) }

func New() *sCatalog { return &sCatalog{} }

// ListCategories returns all catalog categories ordered by id.
func (s *sCatalog) ListCategories(ctx context.Context) ([]dto.CategoryInfo, error) {
	records, err := g.DB().Model("catalog_categories").Ctx(ctx).Order("id ASC").All()
	if err != nil {
		return nil, gerror.Wrap(err, "query categories failed")
	}
	categories := make([]dto.CategoryInfo, 0, len(records))
	for _, r := range records {
		cat := dto.CategoryInfo{
			ID:   r["id"].Int64(),
			Name: r["name"].String(),
		}
		if !r["parent_id"].IsEmpty() {
			v := r["parent_id"].Int64()
			cat.ParentID = &v
		}
		categories = append(categories, cat)
	}
	return categories, nil
}

// ListItems returns catalog items, optionally filtered by type and/or status.
func (s *sCatalog) ListItems(ctx context.Context, itemType, status string) ([]dto.ItemInfo, error) {
	m := g.DB().Model("catalog_items").Ctx(ctx)
	if itemType != "" {
		m = m.Where("type", itemType)
	}
	if status != "" {
		m = m.Where("status", status)
	}
	records, err := m.Order("id ASC").All()
	if err != nil {
		return nil, gerror.Wrap(err, "query items failed")
	}
	return s.mapItems(ctx, records)
}

// GetItem returns a single catalog item by id.
func (s *sCatalog) GetItem(ctx context.Context, id int64) (*dto.ItemInfo, error) {
	record, err := g.DB().Model("catalog_items").Ctx(ctx).Where("id", id).One()
	if err != nil {
		return nil, gerror.Wrap(err, "query item failed")
	}
	if record.IsEmpty() {
		return nil, gerror.New("item not found")
	}
	items, err := s.mapItems(ctx, gdb.Result{record})
	if err != nil {
		return nil, err
	}
	return &items[0], nil
}

// mapItems converts raw DB records to ItemInfo DTOs, loading tags per item.
func (s *sCatalog) mapItems(ctx context.Context, records gdb.Result) ([]dto.ItemInfo, error) {
	items := make([]dto.ItemInfo, 0, len(records))
	for _, r := range records {
		item := dto.ItemInfo{
			ID:           r["id"].Int64(),
			OwnerUserID:  r["owner_user_id"].Int64(),
			Type:         r["type"].String(),
			Name:         r["name"].String(),
			Description:  r["description"].String(),
			Status:       r["status"].String(),
			ReviewStatus: r["review_status"].String(),
			Version:      r["version"].Int(),
			ConfigJSON:   r["config_json"].String(),
			CreatedAt:    r["created_at"].Time(),
			UpdatedAt:    r["updated_at"].Time(),
		}

		tagValues, err := g.DB().Model("catalog_item_tags").Ctx(ctx).
			Where("item_id", item.ID).Array("tag")
		if err != nil {
			return nil, gerror.Wrap(err, "query tags failed")
		}
		tags := make([]string, 0, len(tagValues))
		for _, t := range tagValues {
			tags = append(tags, t.String())
		}
		item.Tags = tags
		items = append(items, item)
	}
	return items, nil
}
