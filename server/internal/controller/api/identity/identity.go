package identity

import (
	"ai-platform/internal/middleware"
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

func Register(r *ghttp.Request) {
	var in dto.RegisterIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
		return
	}
	out, err := service.Identity().Register(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

func VerifyEmail(r *ghttp.Request) {
	var in dto.VerifyEmailIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
		return
	}
	out, err := service.Identity().VerifyEmail(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

func Login(r *ghttp.Request) {
	var in dto.LoginIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
		return
	}
	out, err := service.Identity().Login(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

func Me(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	user, err := service.Identity().GetUser(r.Context(), userID)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(user)
}

func CreateKey(r *ghttp.Request) {
	var in dto.CreateApiKeyIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
		return
	}
	userID := middleware.GetUserID(r)
	out, err := service.Identity().CreateApiKey(r.Context(), userID, in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

func ListKeys(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	keys, err := service.Identity().ListApiKeys(r.Context(), userID)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(keys)
}

func DeleteKey(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	keyID := r.Get("id").Int64()
	if keyID == 0 {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid key id"})
		return
	}
	if err := service.Identity().DeleteApiKey(r.Context(), userID, keyID); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"ok": true})
}
