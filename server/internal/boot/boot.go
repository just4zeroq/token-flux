// Package boot wires together the HTTP servers and starts the application.
package boot

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"

	"ai-platform/internal/controller/api/admin"
	"ai-platform/internal/controller/api/billing"
	"ai-platform/internal/controller/api/catalog"
	"ai-platform/internal/controller/api/identity"
	llmapi "ai-platform/internal/controller/api/llm"
	"ai-platform/internal/controller/api/market"
	"ai-platform/internal/controller/api/pricing"
	"ai-platform/internal/controller/api/usage"
	"ai-platform/internal/controller/api/wallet"
	"ai-platform/internal/controller/gateway"
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
		group.Group("/api/v1", func(v1 *ghttp.RouterGroup) {
			v1.POST("/auth/register", identity.Register)
			v1.POST("/auth/verify-email", identity.VerifyEmail)
			v1.POST("/auth/login", identity.Login)

			// JWT-protected routes
			v1.Group("/users/me", func(me *ghttp.RouterGroup) {
				me.Middleware(middleware.JWTAuth)
				me.GET("/", identity.Me)
				me.POST("/keys", identity.CreateKey)
				me.GET("/keys", identity.ListKeys)
				me.DELETE("/keys/:id", identity.DeleteKey)
			})

			// Catalog routes (public)
			v1.GET("/catalog/categories", catalog.ListCategories)
			v1.GET("/catalog/items", catalog.ListItems)
			v1.GET("/catalog/items/:id", catalog.GetItem)

			// Pricing routes
			v1.GET("/pricing/rules", pricing.ListPricingRules)
			v1.GET("/pricing/rules/:id", pricing.GetPricingRule)
			v1.POST("/pricing/rules", pricing.CreatePricingRule)
			v1.GET("/pricing/exchange-rates", pricing.ListExchangeRates)

			// Usage routes (JWT required)
			v1.Group("/usage", func(ug *ghttp.RouterGroup) {
				ug.Middleware(middleware.JWTAuth)
				ug.POST("/report", usage.ReportUsage)
				ug.GET("/records", usage.ListUsage)
				ug.GET("/stats", usage.GetUsageStats)
			})

			// Market routes
			v1.GET("/market/reviews", market.ListReviews)
			v1.Group("/market", func(mg *ghttp.RouterGroup) {
				mg.Middleware(middleware.JWTAuth)
				mg.POST("/orders", market.CreateOrder)
				mg.GET("/orders", market.ListOrders)
				mg.POST("/reviews", market.CreateReview)
			})

			// Billing routes (JWT required)
			v1.Group("/billing", func(bg *ghttp.RouterGroup) {
				bg.Middleware(middleware.JWTAuth)
				bg.GET("/balance", billing.GetBalance)
				bg.GET("/transactions", billing.ListTransactions)
				bg.POST("/credit", billing.CreditAccount)
				bg.POST("/debit", billing.DebitAccount)
			})

			// Wallet routes (JWT required)
			v1.Group("/wallet", func(wg *ghttp.RouterGroup) {
				wg.Middleware(middleware.JWTAuth)
				wg.POST("/deposit-address", wallet.CreateDepositAddress)
				wg.GET("/deposit-addresses", wallet.ListDepositAddresses)
				wg.GET("/deposits", wallet.ListDeposits)
				wg.POST("/withdraw", wallet.CreateWithdraw)
				wg.GET("/withdrawals", wallet.ListWithdrawals)
			})

			// Admin routes (JWT + AdminAuth required)
			v1.Group("/admin", func(ag *ghttp.RouterGroup) {
				ag.Middleware(middleware.JWTAuth, middleware.AdminAuth)
				ag.GET("/users", admin.ListUsers)
				ag.GET("/users/:id", admin.GetUser)
				ag.PUT("/users/:id/status", admin.UpdateUserStatus)
				ag.GET("/stats", admin.GetStats)

				// LLM admin management
				ag.POST("/llm/channels", llmapi.CreateChannel)
				ag.GET("/llm/channels", llmapi.ListChannels)
				ag.PUT("/llm/channels/:id/review", llmapi.ReviewChannel)
				ag.POST("/llm/models", llmapi.CreateModel)
				ag.GET("/llm/models", llmapi.ListModels)
				ag.PUT("/llm/models/:id/review", llmapi.ReviewModel)
				ag.POST("/llm/models/:id/prices", llmapi.UpsertModelPrice)
				ag.GET("/llm/models/:id/prices", llmapi.ListModelPrices)
				ag.GET("/llm/model-keys", llmapi.ListAllModelKeys)
				ag.GET("/llm/key-models", llmapi.ListAllKeyModels)
				ag.POST("/llm/key-models/:id/test", llmapi.TriggerKeyModelTest)
			})

			// Provider LLM routes (JWT only; per-handler role>=10 check)
			v1.Group("/provider/llm", func(pg *ghttp.RouterGroup) {
				pg.Middleware(middleware.JWTAuth)
				pg.POST("/channels", llmapi.ProviderCreateChannel)
				pg.POST("/models", llmapi.ProviderCreateModel)
				pg.GET("/models", llmapi.ProviderListModels)
				pg.POST("/model-keys", llmapi.ProviderCreateModelKey)
				pg.GET("/model-keys", llmapi.ProviderListModelKeys)
				pg.DELETE("/model-keys/:id", llmapi.ProviderDisableModelKey)
				pg.POST("/model-keys/:id/models", llmapi.ProviderBindKeyModel)
				pg.GET("/model-keys/:id/models", llmapi.ProviderListKeyModels)
				pg.POST("/key-models/:id/test", llmapi.ProviderTriggerKeyModelTest)
			})
		})
	})

	gwSrv := g.Server("gateway")
	gwSrv.SetAddr(":8081")
	gwSrv.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.Recover, middleware.RequestID, middleware.CORS)
		group.GET("/health", health)

		group.Group("/v1", func(v1 *ghttp.RouterGroup) {
			v1.Middleware(middleware.APIKeyAuth)
			v1.GET("/models", gateway.Models)
			v1.POST("/chat/completions", gateway.ChatCompletions)
			v1.POST("/completions", gateway.Completions)
			v1.POST("/embeddings", gateway.Embeddings)
		})
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
