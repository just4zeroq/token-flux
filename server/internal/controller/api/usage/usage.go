package usage

import (
	"ai-platform/internal/middleware"
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

// ReportUsage handles POST /usage/report
func ReportUsage(r *ghttp.Request) {
	var in dto.ReportUsageIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
		return
	}
	in.UserID = middleware.GetUserID(r)
	out, err := service.Usage().ReportUsage(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{
			"code": 500, "message": err.Error(),
		})
		return
	}
	r.Response.WriteJson(out)
}

// ListUsage handles GET /usage/records
func ListUsage(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	page := r.Get("page").Int()
	if page < 1 {
		page = 1
	}
	pageSize := r.Get("page_size").Int()
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	records, total, err := service.Usage().ListUsage(r.Context(), userID, page, pageSize)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{
			"code": 500, "message": err.Error(),
		})
		return
	}
	r.Response.WriteJson(map[string]any{
		"records":   records,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetUsageStats handles GET /usage/stats
func GetUsageStats(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	stats, err := service.Usage().GetUsageStats(r.Context(), userID)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{
			"code": 500, "message": err.Error(),
		})
		return
	}
	r.Response.WriteJson(stats)
}
