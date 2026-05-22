// Package boot wires together the HTTP servers and starts the application.
package boot

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"

	"ai-platform/internal/middleware"
)

// RunAPI starts both the api (:8080) and gateway (:8081) ghttp.Servers and blocks.
// All domain logic is registered via service interfaces during package init();
// this function only wires routes and starts servers.
func RunAPI() {
	ctx := gctx.New()

	apiSrv := g.Server("api")
	apiSrv.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.Recover, middleware.RequestID, middleware.CORS)
		group.GET("/health", health)
	})

	gwSrv := g.Server("gateway")
	gwSrv.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.Recover, middleware.RequestID, middleware.CORS)
		group.GET("/health", health)
	})

	g.Log().Info(ctx, "ai-platform starting: api :8080 + gateway :8081")
	g.Wait()
}

func health(r *ghttp.Request) {
	r.Response.WriteJson(g.Map{"status": "ok"})
}
