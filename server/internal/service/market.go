package service

import (
	"context"

	"ai-platform/internal/model/dto"
)

type IMarket interface {
	CreateOrder(ctx context.Context, userID int64, in dto.CreateOrderIn) (*dto.OrderInfo, error)
	ListOrders(ctx context.Context, userID int64) ([]*dto.OrderInfo, error)
	CreateReview(ctx context.Context, userID int64, in dto.CreateReviewIn) (*dto.ReviewInfo, error)
	ListReviews(ctx context.Context, itemID int64) ([]*dto.ReviewInfo, error)
}

var localMarket IMarket

func RegisterMarket(i IMarket) { localMarket = i }

func Market() IMarket {
	if localMarket == nil {
		panic("service.Market not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localMarket
}
