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
	apiSrv.SetAddr(":8080")
	apiSrv.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.Recover, middleware.RequestID, middleware.CORS)
		group.GET("/health", health)
	})

	gwSrv := g.Server("gateway")
	gwSrv.SetAddr(":8081")
	gwSrv.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.Recover, middleware.RequestID, middleware.CORS)
		group.GET("/health", health)
	})

	if err := apiSrv.Start(); err != nil {
		g.Log().Fatalf(ctx, "api server start failed: %v", err)
	}
	if err := gwSrv.Start(); err != nil {
		g.Log().Fatalf(ctx, "gateway server start failed: %v", err)
	}

	g.Log().Info(ctx, "ai-platform started: api :8080 + gateway :8081")
	g.Wait()
}

func health(r *ghttp.Request) {
	r.Response.WriteJson(g.Map{"status": "ok"})
}
