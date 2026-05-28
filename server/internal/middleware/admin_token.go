package middleware

import (
	"os"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
)

const CtxKeyAdminUserID = "admin_user_id"

// AdminTokenAuth validates the request against the ADMIN_API_TOKEN env var.
// No JWT, no user table — pure static token comparison.
func AdminTokenAuth(r *ghttp.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	expected := os.Getenv("ADMIN_API_TOKEN")
	if expected == "" || token != expected {
		r.Response.WriteStatusExit(401, map[string]any{
			"code": 401, "message": "unauthorized",
		})
		return
	}
	r.SetCtxVar(CtxKeyAdminUserID, int64(0))
	r.Middleware.Next()
}
