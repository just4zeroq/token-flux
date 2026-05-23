package service

import (
	"context"

	"ai-platform/internal/model/dto"
)

// ICatalog is the contract for the catalog domain (categories, items).
type ICatalog interface {
	ListCategories(ctx context.Context) ([]dto.CategoryInfo, error)
	ListItems(ctx context.Context, itemType, status string) ([]dto.ItemInfo, error)
	GetItem(ctx context.Context, id int64) (*dto.ItemInfo, error)
}

var localCatalog ICatalog

func RegisterCatalog(i ICatalog) { localCatalog = i }

func Catalog() ICatalog {
	if localCatalog == nil {
		panic("service.Catalog not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localCatalog
}
