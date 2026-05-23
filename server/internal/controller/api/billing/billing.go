package billing

import (
	"ai-platform/internal/middleware"
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

func GetBalance(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	asset := r.Get("asset", "credits").String()
	acc, err := service.Billing().GetBalance(r.Context(), "user", userID, asset)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(acc)
}

func ListTransactions(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	page := r.Get("page", 1).Int()
	pageSize := r.Get("page_size", 20).Int()
	txs, total, err := service.Billing().ListTransactions(r.Context(), "user", userID, page, pageSize)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{
		"list":      txs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func CreditAccount(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	var in dto.CreditAccountIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
		return
	}
	// Authenticated user can only credit their own account
	in.OwnerType = "user"
	in.OwnerID = userID
	tx, err := service.Billing().CreditAccount(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(tx)
}

func DebitAccount(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	var in dto.DebitAccountIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
		return
	}
	// Authenticated user can only debit their own account
	in.OwnerType = "user"
	in.OwnerID = userID
	tx, err := service.Billing().DebitAccount(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(tx)
}
