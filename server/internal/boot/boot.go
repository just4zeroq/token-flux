// Package boot wires together the HTTP servers and starts the application.
package boot

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"

	"ai-platform/internal/controller/api/billing"
	"ai-platform/internal/controller/api/identity"
	llmapi "ai-platform/internal/controller/api/llm"
	"ai-platform/internal/controller/api/wallet"
	"ai-platform/internal/controller/gateway"
	"ai-platform/internal/middleware"
)

// RunAPI starts both the api (:8080) and gateway (:8081) ghttp.Servers and blocks.
func RunAPI() {
	ctx := gctx.New()

	apiSrv := g.Server("api")
	apiSrv.SetAddr(":8080")
	apiSrv.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.Recover, middleware.RequestID, middleware.CORS)
		group.GET("/health", health)

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

			// Billing routes (JWT required)
			v1.Group("/billing", func(bg *ghttp.RouterGroup) {
				bg.Middleware(middleware.JWTAuth)
				bg.GET("/balance", billing.GetBalance)
				bg.GET("/transactions", billing.ListTransactions)
				bg.POST("/credit", billing.CreditAccount)
				bg.POST("/debit", billing.DebitAccount)
				bg.POST("/recharge", billing.Recharge)
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

			// Provider LLM routes (JWT required; per-handler provider role check)
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
