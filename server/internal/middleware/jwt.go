package middleware

import (
	"strings"

	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

const CtxKeyUserID = "user_id"
const CtxKeyUsername = "username"
const CtxKeyRole = "role"

func JWTAuth(r *ghttp.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
		r.Response.WriteStatusExit(401, map[string]any{
			"code":    401,
			"message": "missing authorization header",
		})
	}

	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" {
		return
	}

	claims, err := service.Identity().ValidateToken(r.Context(), token)
	if err != nil {
		r.Response.WriteStatusExit(401, map[string]any{
			"code":    401,
			"message": "invalid or expired token",
		})
		return
	}

	r.SetCtxVar(CtxKeyUserID, claims.UserID)
	r.SetCtxVar(CtxKeyUsername, claims.Username)
	r.SetCtxVar(CtxKeyRole, claims.Role)
	r.Middleware.Next()
}

func GetUserID(r *ghttp.Request) int64 {
	return r.GetCtxVar(CtxKeyUserID).Int64()
}
