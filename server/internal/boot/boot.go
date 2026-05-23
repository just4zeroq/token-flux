// Package boot wires together the HTTP servers and starts the application.
package boot

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"

	"ai-platform/internal/controller/api/identity"
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

		// Identity routes (auth)
		group.Group("/api/v1", func(g *ghttp.RouterGroup) {
			g.POST("/auth/register", identity.Register)
			g.POST("/auth/verify-email", identity.VerifyEmail)
			g.POST("/auth/login", identity.Login)

			// JWT-protected routes
			g.Group("/users/me", func(g *ghttp.RouterGroup) {
				g.Middleware(middleware.JWTAuth)
				g.GET("", identity.Me)
				g.POST("/keys", identity.CreateKey)
				g.GET("/keys", identity.ListKeys)
				g.DELETE("/keys/:id", identity.DeleteKey)
			})
		})
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
