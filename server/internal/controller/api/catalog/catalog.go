package catalog

import (
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

// ListCategories handles GET /catalog/categories
func ListCategories(r *ghttp.Request) {
	categories, err := service.Catalog().ListCategories(r.Context())
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{
			"code": 500, "message": err.Error(),
		})
		return
	}
	r.Response.WriteJson(categories)
}

// ListItems handles GET /catalog/items
func ListItems(r *ghttp.Request) {
	itemType := r.Get("type").String()
	status := r.Get("status").String()
	items, err := service.Catalog().ListItems(r.Context(), itemType, status)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{
			"code": 500, "message": err.Error(),
		})
		return
	}
	r.Response.WriteJson(items)
}

// GetItem handles GET /catalog/items/:id
func GetItem(r *ghttp.Request) {
	id := r.Get("id").Int64()
	if id == 0 {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid item id",
		})
		return
	}
	item, err := service.Catalog().GetItem(r.Context(), id)
	if err != nil {
		r.Response.WriteStatusExit(404, map[string]any{
			"code": 404, "message": err.Error(),
		})
		return
	}
	r.Response.WriteJson(item)
}
