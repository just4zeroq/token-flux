package service

import (
	"context"

	"ai-platform/internal/model/dto"
)

type ISettlement interface {
	Submit(ctx context.Context, in dto.SettlementSubmitIn) (*dto.SettlementRecordInfo, error)
	Settle(ctx context.Context, settlementID int64) (*dto.TransactionInfo, error)
	SubmitAndSettle(ctx context.Context, in dto.SettlementSubmitIn) (*dto.TransactionInfo, error)
	CreateRecharge(ctx context.Context, in dto.RechargeSettlementIn) (*dto.TransactionInfo, error)
	RestoreOverdraftKeys(ctx context.Context, userID int64) error
}

var localSettlement ISettlement

func RegisterSettlement(i ISettlement) { localSettlement = i }

func Settlement() ISettlement {
	if localSettlement == nil {
		panic("service.Settlement not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localSettlement
}
