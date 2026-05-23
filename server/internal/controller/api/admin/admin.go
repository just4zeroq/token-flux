package admin

import (
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

func ListUsers(r *ghttp.Request) {
	page := r.Get("page", 1).Int()
	pageSize := r.Get("page_size", 20).Int()
	users, total, err := service.Admin().ListUsers(r.Context(), page, pageSize)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{
		"list":      users,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func GetUser(r *ghttp.Request) {
	userID := r.Get("id").Int64()
	if userID == 0 {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid user id"})
		return
	}
	user, err := service.Admin().GetUser(r.Context(), userID)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(user)
}

func UpdateUserStatus(r *ghttp.Request) {
	var in dto.UpdateUserStatusIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
		return
	}
	if err := service.Admin().UpdateUserStatus(r.Context(), in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"ok": true})
}

func GetStats(r *ghttp.Request) {
	stats, err := service.Admin().GetStats(r.Context())
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(stats)
}
