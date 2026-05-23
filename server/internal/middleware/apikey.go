package middleware

import (
	"strings"

	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

const CtxKeyAPIKeyID = "api_key_id"

func APIKeyAuth(r *ghttp.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
		r.Response.WriteStatusExit(401, map[string]any{
			"code":    401,
			"message": "missing api key",
		})
	}
	key := strings.TrimPrefix(auth, "Bearer ")
	if key == "" {
		return
	}

	info, err := service.Identity().ValidateApiKey(r.Context(), key)
	if err != nil {
		r.Response.WriteStatusExit(401, map[string]any{
			"code":    401,
			"message": err.Error(),
		})
		return
	}

	r.SetCtxVar(CtxKeyUserID, info.UserID)
	r.SetCtxVar(CtxKeyAPIKeyID, info.ID)
	r.Middleware.Next()
}
