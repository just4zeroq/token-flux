// Package boot wires together the HTTP servers and starts the application.
package boot

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"

	adminapi "ai-platform/internal/controller/api/admin"
	"ai-platform/internal/controller/api/billing"
	catalogapi "ai-platform/internal/controller/api/catalog"
	"ai-platform/internal/controller/api/developer"
	usageapi "ai-platform/internal/controller/api/usage"
	"ai-platform/internal/controller/api/identity"
	llmapi "ai-platform/internal/controller/api/llm"
	"ai-platform/internal/controller/api/payment"
	"ai-platform/internal/controller/api/wallet"
	"ai-platform/internal/controller/gateway"
	"ai-platform/internal/middleware"
	"ai-platform/internal/relay/handler"
)

// RunAPI starts the HTTP servers specified by `which`: "all", "api", "gateway", "admin".
// Set AI_SERVER env var to control which servers start. Defaults to "all".
func RunAPI(which string) {
	ctx := gctx.New()

	// Run DB migrations before starting servers.
	if err := RunMigrations(ctx); err != nil {
		g.Log().Fatalf(ctx, "migration failed: %v", err)
	}

	if which == "all" || which == "api" {
		apiSrv := g.Server("api")
		apiSrv.SetAddr(":8080")
		apiSrv.Group("/", func(group *ghttp.RouterGroup) {
			group.Middleware(middleware.Recover, middleware.RequestID, middleware.CORS)
			group.GET("/health", health)

			group.Group("/api/v1", func(v1 *ghttp.RouterGroup) {
				v1.POST("/auth/register", identity.Register)
				v1.POST("/auth/verify-email", identity.VerifyEmail)
				v1.POST("/auth/login", identity.Login)
					// JWT-protected group for apply
					v1.Group("/", func(auth *ghttp.RouterGroup) {
						auth.Middleware(middleware.JWTAuth)
						auth.POST("/provider/apply", identity.ApplyProvider)
					})

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
						bg.GET("/provider-stats", billing.ProviderSettlementStats)
				})

				// Usage routes (JWT required)
				v1.Group("/usage", func(ug *ghttp.RouterGroup) {
					ug.Middleware(middleware.JWTAuth)
					ug.GET("/stats", usageapi.Stats)
					ug.GET("/records", usageapi.Records)
					ug.GET("/stats-by-model", usageapi.StatsByModel)
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
						pg.GET("/channels", llmapi.ProviderListChannels)
					pg.POST("/models", llmapi.ProviderCreateModel)
					pg.GET("/models", llmapi.ProviderListModels)
					pg.POST("/model-keys", llmapi.ProviderCreateModelKey)
					pg.GET("/model-keys", llmapi.ProviderListModelKeys)
					pg.DELETE("/model-keys/:id", llmapi.ProviderDisableModelKey)
					pg.POST("/model-keys/:id/models", llmapi.ProviderBindKeyModel)
					pg.GET("/model-keys/:id/models", llmapi.ProviderListKeyModels)
					pg.POST("/key-models/:id/test", llmapi.ProviderTriggerKeyModelTest)
				})

				// Payment routes (JWT required)
				v1.Group("/payment", func(pg *ghttp.RouterGroup) {
					pg.Middleware(middleware.JWTAuth)
					pg.POST("/recharge", payment.Recharge)
					pg.GET("/orders", payment.ListOrders)
					pg.GET("/orders/:orderNo", payment.GetOrder)
				})

				// Payment gateway callbacks (public — called by Alipay/WeChat servers)
				v1.POST("/payment/notify/alipay", payment.AlipayNotify)
				v1.POST("/payment/notify/wechat", payment.WechatNotify)

				// Developer list (public — used by frontend model filter)
				v1.GET("/developers", developer.ListDevelopers)

				// Model catalog (public — used by frontend model browser)
				v1.GET("/catalog/models", catalogapi.ListModels)
				v1.GET("/catalog/models/:code", catalogapi.GetModelDetail)
				v1.GET("/catalog/providers", catalogapi.ListProviders)
				v1.GET("/catalog/providers/:name/models", catalogapi.GetProviderModels)
				v1.GET("/catalog/suppliers", catalogapi.ListSuppliers)
				v1.GET("/catalog/suppliers/:username", catalogapi.GetSupplierModels)
			})
		})

		if err := apiSrv.Start(); err != nil {
			g.Log().Fatalf(ctx, "api server start failed: %v", err)
		}
	}

	if which == "all" || which == "gateway" {
		gwSrv := g.Server("gateway")
		gwSrv.SetAddr(":8081")
		gwSrv.Group("/", func(group *ghttp.RouterGroup) {
			group.Middleware(middleware.Recover, middleware.RequestID, middleware.CORS)
			group.GET("/health", health)

			group.Group("/v1", func(v1 *ghttp.RouterGroup) {
					v1.Middleware(middleware.APIKeyAuth)
					v1.GET("/models", gateway.Models)
					v1.POST("/chat/completions", gateway.RelayChatCompletions)
					v1.POST("/completions", gateway.RelayCompletions)
					v1.POST("/embeddings", gateway.RelayEmbeddings)
					v1.POST("/responses", gateway.RelayResponses)
					v1.POST("/messages", gateway.RelayChatCompletions) // Claude native path
				})
		})

		if err := gwSrv.Start(); err != nil {
			g.Log().Fatalf(ctx, "gateway server start failed: %v", err)
		}
	}

	if which == "all" || which == "admin" {
		adminSrv := g.Server("admin")
		adminSrv.SetAddr(":8082")
		adminSrv.Group("/", func(group *ghttp.RouterGroup) {
			group.Middleware(middleware.Recover, middleware.RequestID, middleware.CORS)
			group.GET("/health", health)

			group.Group("/api/admin", func(admin *ghttp.RouterGroup) {
				admin.Middleware(middleware.AdminTokenAuth)

				// Users
				admin.GET("/users", adminapi.ListUsers)
					admin.POST("/users", adminapi.CreateUser)
				admin.GET("/users/:id", adminapi.GetUser)
				admin.PUT("/users/:id/status", adminapi.UpdateUserStatus)
				admin.PUT("/users/:id/role", adminapi.UpdateUserRole)
					admin.DELETE("/users/:id", adminapi.DeleteUser)
				// Provider applications
				admin.GET("/provider-applications", adminapi.ListProviderApplications)
				admin.POST("/provider-applications/:id/review", adminapi.ReviewProviderApplication)

				// Developers
				admin.GET("/developers", adminapi.ListDevelopers)

				// Billing
				admin.GET("/accounts", adminapi.ListAccounts)
				admin.GET("/balances", adminapi.GetBalance)
				admin.GET("/transactions", adminapi.ListTransactions)
				admin.POST("/recharge", adminapi.AdminRecharge)
				admin.POST("/adjust-balance", adminapi.AdminAdjustBalance)

				// Payment
				admin.GET("/payment/orders", adminapi.ListPaymentOrders)
				admin.GET("/payment/channels", adminapi.ListPaymentChannels)
				admin.POST("/payment/channels", adminapi.CreatePaymentChannel)
				admin.PUT("/payment/channels/:id", adminapi.UpdatePaymentChannel)

				// Settlements
				// Invoices
					admin.GET("/invoices", adminapi.ListInvoices)
					admin.POST("/invoices", adminapi.CreateInvoice)
					admin.GET("/invoices/:id", adminapi.GetInvoice)
					admin.PUT("/invoices/:id/status", adminapi.UpdateInvoiceStatus)

					// Settlements
					admin.GET("/settlements", adminapi.ListSettlements)

				// LLM
				admin.Group("/llm", func(llm *ghttp.RouterGroup) {
					llm.POST("/channels", adminapi.CreateChannel)
					llm.GET("/channels", adminapi.ListChannels)
					llm.PUT("/channels/:id/review", adminapi.ReviewChannel)
						llm.DELETE("/channels/:id", adminapi.DeleteChannel)
					llm.POST("/models", adminapi.CreateModel)
					llm.GET("/models", adminapi.ListModels)
					llm.PUT("/models/:id/review", adminapi.ReviewModel)
						llm.DELETE("/models/:id", adminapi.DeleteModel)
					llm.GET("/model-keys", adminapi.ListAllModelKeys)
					llm.GET("/key-models", adminapi.ListAllKeyModels)
					llm.POST("/key-models/:id/test", adminapi.TriggerKeyModelTest)
				// Provider settings
				llm.GET("/provider-settings/:userId", adminapi.GetProviderSetting)
				llm.POST("/provider-settings/:userId", adminapi.SetProviderSetting)
			})
		})
	})

		if err := adminSrv.Start(); err != nil {
			g.Log().Fatalf(ctx, "admin server start failed: %v", err)
		}
	}

	g.Log().Infof(ctx, "ai-platform %s started", which)

	// Start async settlement workers.
	handler.SetGlobalTokenizer(handler.NewTiktokenizer())
	handler.StartSettlementWorkers(ctx, 4)
	defer handler.StopSettlementWorkers()

	g.Wait()
}

func health(r *ghttp.Request) {
	r.Response.WriteJson(g.Map{"status": "ok"})
}
