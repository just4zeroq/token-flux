package middleware

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
)

const RequestIDHeader = "X-Request-Id"

// CORS allows any origin for now; tighten in production.
func CORS(r *ghttp.Request) {
	corsOpts := r.Response.DefaultCORSOptions()
	r.Response.CORS(corsOpts)
	r.Middleware.Next()
}

// RequestID injects a request id into the response header and gctx for log correlation.
func RequestID(r *ghttp.Request) {
	id := r.Header.Get(RequestIDHeader)
	if id == "" {
		id = guid.S()
	}
	r.SetCtxVar("request_id", id)
	r.Response.Header().Set(RequestIDHeader, id)
	r.Middleware.Next()
}

// Recover converts panics into 500 responses without crashing the server.
func Recover(r *ghttp.Request) {
	defer func() {
		if err := recover(); err != nil {
			g.Log().Errorf(r.Context(), "panic recovered: %v", err)
			r.Response.ClearBuffer()
			r.Response.WriteStatus(500, g.Map{
				"code":    500,
				"message": "internal server error",
			})
		}
	}()
	r.Middleware.Next()
}
