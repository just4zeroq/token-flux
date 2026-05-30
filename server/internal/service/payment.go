package service

import (
	"context"
	"net/http"

	"ai-platform/internal/model/dto"
)

type IPayment interface {
	// User-facing: create a recharge order, returns payment URL
	CreateRecharge(ctx context.Context, userID int64, in dto.CreateRechargeIn) (*dto.CreateRechargeOut, error)

	// List user's payment orders
	ListOrders(ctx context.Context, userID int64, page, pageSize int) ([]*dto.PaymentOrderInfo, int, error)

	// Get single order
	GetOrder(ctx context.Context, userID int64, orderNo string) (*dto.PaymentOrderInfo, error)

	// Handle channel notify callback (public, no auth)
	HandleNotify(ctx context.Context, channel string, req *http.Request) error

	// Admin: list all orders with filters
	ListAllOrders(ctx context.Context, channel, status, keyword string, page, pageSize int) ([]*dto.PaymentOrderInfo, int, error)

	// Admin: channel management
	ListChannels(ctx context.Context, channel string, page, pageSize int) ([]*dto.PaymentChannelInfo, int, error)
	CreateChannel(ctx context.Context, in dto.CreatePaymentChannelIn) error
	UpdateChannel(ctx context.Context, id int64, in dto.UpdatePaymentChannelIn) error
}

var localPayment IPayment

func RegisterPayment(i IPayment) { localPayment = i }

func Payment() IPayment {
	if localPayment == nil {
		panic("service.Payment not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localPayment
}
