package admin

import (
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

// ========== Users ==========

func ListUsers(r *ghttp.Request) {
	pageNum := r.Get("pageNum", 1).Int()
	pageSize := r.Get("pageSize", 20).Int()

	var status, role *int
	if s := r.Get("status"); s != nil {
		v := s.Int()
		status = &v
	}
	if rl := r.Get("role"); rl != nil {
		v := rl.Int()
		role = &v
	}
	keyword := r.Get("keyword", "").String()

	list, total, err := service.Identity().ListUsers(r.Context(), pageNum, pageSize, status, role, keyword)
	if err != nil {
		fail(r, err.Error())
		return
	}
	page(r, list, total, pageNum, pageSize)
}

func GetUser(r *ghttp.Request) {
	userID := r.Get("id").Int64()
	user, err := service.Identity().GetUser(r.Context(), userID)
	if err != nil {
		fail(r, err.Error())
		return
	}
	ok(r, user)
}

func UpdateUserStatus(r *ghttp.Request) {
	userID := r.Get("id").Int64()
	status := r.Get("status").Int()
	if err := service.Identity().UpdateUserStatus(r.Context(), userID, status); err != nil {
		fail(r, err.Error())
		return
	}
	okMsg(r, "updated")
}

func UpdateUserRole(r *ghttp.Request) {
	userID := r.Get("id").Int64()
	role := r.Get("role").Int()
	if err := service.Identity().UpdateUserRole(r.Context(), userID, role); err != nil {
		fail(r, err.Error())
		return
	}
	okMsg(r, "updated")
}

func CreateUser(r *ghttp.Request) {
	var in dto.CreateUserIn
	if err := r.Parse(&in); err != nil {
		fail(r, "invalid params: "+err.Error())
		return
	}
	out, err := service.Identity().CreateUser(r.Context(), in)
	if err != nil {
		fail(r, err.Error())
		return
	}
	ok(r, out)
}

func DeleteUser(r *ghttp.Request) {
	userID := r.Get("id").Int64()
	if err := service.Identity().UpdateUserStatus(r.Context(), userID, 0); err != nil {
		fail(r, err.Error())
		return
	}
	okMsg(r, "deleted")
}

// ── Provider Applications ──

func ListProviderApplications(r *ghttp.Request) {
	pageNum := r.Get("pageNum", 1).Int()
	pageSize := r.Get("pageSize", 20).Int()
	status := r.Get("status").String()

	list, total, err := service.Identity().ListProviderApplications(r.Context(), pageNum, pageSize, status)
	if err != nil {
		fail(r, err.Error())
		return
	}
	page(r, list, total, pageNum, pageSize)
}

func ReviewProviderApplication(r *ghttp.Request) {
	appID := r.Get("id").Int64()
	if appID == 0 {
		fail(r, "invalid application id")
		return
	}
	var in dto.ReviewProviderApplicationIn
	if err := r.Parse(&in); err != nil {
		fail(r, "invalid params: "+err.Error())
		return
	}
	if err := service.Identity().ReviewProviderApplication(r.Context(), appID, in); err != nil {
		fail(r, err.Error())
		return
	}
	okMsg(r, "reviewed")
}
