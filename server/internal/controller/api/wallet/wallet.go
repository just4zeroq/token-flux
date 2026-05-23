package wallet

import (
	"ai-platform/internal/middleware"
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

func CreateDepositAddress(r *ghttp.Request) {
	var in dto.CreateDepositAddressIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
		return
	}
	userID := middleware.GetUserID(r)
	out, err := service.Wallet().CreateDepositAddress(r.Context(), userID, in.Chain)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

func ListDepositAddresses(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	addrs, err := service.Wallet().ListDepositAddresses(r.Context(), userID)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(addrs)
}

func ListDeposits(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	deposits, err := service.Wallet().ListDeposits(r.Context(), userID)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(deposits)
}

func CreateWithdraw(r *ghttp.Request) {
	var in dto.CreateWithdrawIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
		return
	}
	userID := middleware.GetUserID(r)
	out, err := service.Wallet().CreateWithdraw(r.Context(), userID, in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

func ListWithdrawals(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	withdrawals, err := service.Wallet().ListWithdrawals(r.Context(), userID)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(withdrawals)
}
