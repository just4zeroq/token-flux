package admin

import (
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

// ========== Accounts ==========

func ListAccounts(r *ghttp.Request) {
	pageNum := r.Get("pageNum", 1).Int()
	pageSize := r.Get("pageSize", 20).Int()
	ownerType := r.Get("ownerType", "").String()
	asset := r.Get("asset", "").String()

	list, total, err := service.Billing().ListAllAccounts(r.Context(), ownerType, asset, pageNum, pageSize)
	if err != nil {
		fail(r, err.Error())
		return
	}
	page(r, list, total, pageNum, pageSize)
}

func GetBalance(r *ghttp.Request) {
	userID := r.Get("userId").Int64()
	asset := r.Get("asset", "credits").String()

	info, err := service.Billing().GetBalance(r.Context(), "user", userID, asset)
	if err != nil {
		fail(r, err.Error())
		return
	}
	ok(r, info)
}

// ========== Transactions ==========

func ListTransactions(r *ghttp.Request) {
	pageNum := r.Get("pageNum", 1).Int()
	pageSize := r.Get("pageSize", 20).Int()

	list, total, err := service.Billing().ListAllTransactions(r.Context(), pageNum, pageSize)
	if err != nil {
		fail(r, err.Error())
		return
	}
	page(r, list, total, pageNum, pageSize)
}

// ========== Recharge ==========

func AdminRecharge(r *ghttp.Request) {
	userID := r.Get("userId").Int64()
	amountCredits := r.Get("amountCredits").Int64()
	note := r.Get("note", "").String()

	if userID == 0 || amountCredits <= 0 {
		fail(r, "userId and positive amountCredits required")
		return
	}

	refType := r.Get("refType", "admin_recharge").String()
	refID := r.Get("refId", 0).Int64()

	refType = storeNote(refType, note)

	tx, err := service.Billing().RechargeCredits(r.Context(), userID, amountCredits, refType, refID)
	if err != nil {
		fail(r, err.Error())
		return
	}
	ok(r, tx)
}

// ========== Credits / Points Adjustment ==========

func AdminAdjustBalance(r *ghttp.Request) {
	userID := r.Get("userId").Int64()
	amountMicro := r.Get("amountMicro").Int64()
	asset := r.Get("asset", "credits").String()
	note := r.Get("note", "").String()

	if userID == 0 || amountMicro == 0 {
		fail(r, "userId and non-zero amountMicro required")
		return
	}
	refType := storeNote("admin_adjust", note)

	var (
		tx  *dto.TransactionInfo
		err error
	)

	if amountMicro > 0 {
		tx, err = service.Billing().CreditAccount(r.Context(), dto.CreditAccountIn{
			Asset:       asset,
			AmountMicro: amountMicro,
			OwnerType:   "user",
			OwnerID:     userID,
			RefType:     refType,
		})
	} else {
		tx, err = service.Billing().DebitAccount(r.Context(), dto.DebitAccountIn{
			Asset:       asset,
			AmountMicro: -amountMicro,
			OwnerType:   "user",
			OwnerID:     userID,
			RefType:     refType,
		})
	}

	if err != nil {
		fail(r, err.Error())
		return
	}
	ok(r, tx)
}
