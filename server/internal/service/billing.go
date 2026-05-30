package service

import (
	"context"

	"ai-platform/internal/model/dto"
)

type IBilling interface {
	EnsureAccount(ctx context.Context, ownerType string, ownerID int64, asset string) (*dto.AccountInfo, error)
	GetBalance(ctx context.Context, ownerType string, ownerID int64, asset string) (*dto.AccountInfo, error)
	ListTransactions(ctx context.Context, ownerType string, ownerID int64, page, pageSize int) ([]*dto.TransactionInfo, int, error)
	CreditAccount(ctx context.Context, in dto.CreditAccountIn) (*dto.TransactionInfo, error)
	DebitAccount(ctx context.Context, in dto.DebitAccountIn) (*dto.TransactionInfo, error)
	RechargeCredits(ctx context.Context, userID int64, amountCredits int64, refType string, refID int64) (*dto.TransactionInfo, error)

	// Admin
	ListAllAccounts(ctx context.Context, ownerType, asset string, page, pageSize int) ([]*dto.AccountInfo, int, error)
	ListAllTransactions(ctx context.Context, page, pageSize int) ([]*dto.TransactionInfo, int, error)
}

var localBilling IBilling

func RegisterBilling(i IBilling) { localBilling = i }

func Billing() IBilling {
	if localBilling == nil {
		panic("service.Billing not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localBilling
}
