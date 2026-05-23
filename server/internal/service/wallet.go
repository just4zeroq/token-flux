package service

import (
	"context"

	"ai-platform/internal/model/dto"
)

type IWallet interface {
	CreateDepositAddress(ctx context.Context, userID int64, chain string) (*dto.DepositAddressInfo, error)
	ListDepositAddresses(ctx context.Context, userID int64) ([]*dto.DepositAddressInfo, error)
	ListDeposits(ctx context.Context, userID int64) ([]*dto.ChainDepositInfo, error)
	CreateWithdraw(ctx context.Context, userID int64, in dto.CreateWithdrawIn) (*dto.WithdrawRequestInfo, error)
	ListWithdrawals(ctx context.Context, userID int64) ([]*dto.WithdrawRequestInfo, error)
}

var localWallet IWallet

func RegisterWallet(i IWallet) { localWallet = i }

func Wallet() IWallet {
	if localWallet == nil {
		panic("service.Wallet not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localWallet
}
