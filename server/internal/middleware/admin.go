package middleware

import (
	"github.com/gogf/gf/v2/net/ghttp"
)

// AdminAuth checks that the authenticated user has admin role (role >= 100).
// Must be used AFTER JWTAuth (which sets the role context var).
func AdminAuth(r *ghttp.Request) {
	role := r.GetCtxVar(CtxKeyRole).Int()
	if role < 100 {
		r.Response.WriteStatusExit(403, map[string]any{
			"code": 403, "message": "admin access required",
		})
		return
	}
	r.Middleware.Next()
}
