package middleware

import (
	"os"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

const CtxKeyAdminUserID = "admin_user_id"

// AdminTokenAuth validates the request against the ADMIN_API_TOKEN env var
// or server.admin.apiToken config. Config value takes priority, env var fallback.
func AdminTokenAuth(r *ghttp.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	expected := g.Cfg().MustGet(r.Context(), "server.admin.apiToken").String()
	if expected == "" {
		expected = os.Getenv("ADMIN_API_TOKEN")
	}
	if expected == "" || token != expected {
		r.Response.WriteStatusExit(401, map[string]any{
			"code": 401, "message": "unauthorized",
		})
		return
	}
	r.SetCtxVar(CtxKeyAdminUserID, int64(0))
	r.Middleware.Next()
}
