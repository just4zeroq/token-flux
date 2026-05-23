package market

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

type sMarket struct{}

func init() { service.RegisterMarket(New()) }

func New() *sMarket { return &sMarket{} }

func (s *sMarket) CreateOrder(ctx context.Context, userID int64, in dto.CreateOrderIn) (*dto.OrderInfo, error) {
	now := time.Now()
	orderNo := fmt.Sprintf("ORD%s%04d", now.Format("20060102150405"), rand.Intn(10000))
	planType := in.PlanType
	if planType == "" {
		planType = "free"
	}
	paymentMethod := in.PaymentMethod
	if paymentMethod == "" {
		paymentMethod = "credits"
	}
	status := "pending"

	result, err := g.DB().Model("orders").Ctx(ctx).Data(g.Map{
		"order_no":       orderNo,
		"buyer_user_id":  userID,
		"item_id":        in.ItemID,
		"plan_type":      planType,
		"amount_credits": in.AmountCredits,
		"payment_method": paymentMethod,
		"status":         status,
		"created_at":     now,
		"updated_at":     now,
	}).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "create order failed")
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, gerror.Wrap(err, "get last insert id failed")
	}
	return &dto.OrderInfo{
		ID:            id,
		OrderNo:       orderNo,
		BuyerUserID:   userID,
		ItemID:        in.ItemID,
		PlanType:      planType,
		AmountCredits: in.AmountCredits,
		PaymentMethod: paymentMethod,
		Status:        status,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (s *sMarket) ListOrders(ctx context.Context, userID int64) ([]*dto.OrderInfo, error) {
	var orders []*dto.OrderInfo
	err := g.DB().Model("orders").Ctx(ctx).
		Where("buyer_user_id", userID).
		Order("id DESC").
		Scan(&orders)
	if err != nil {
		return nil, gerror.Wrap(err, "list orders failed")
	}
	return orders, nil
}

func (s *sMarket) CreateReview(ctx context.Context, userID int64, in dto.CreateReviewIn) (*dto.ReviewInfo, error) {
	if in.Rating < 1 || in.Rating > 5 {
		return nil, gerror.New("rating must be between 1 and 5")
	}
	now := time.Now()

	result, err := g.DB().Model("reviews").Ctx(ctx).Data(g.Map{
		"buyer_user_id": userID,
		"item_id":       in.ItemID,
		"rating":        in.Rating,
		"body":          in.Body,
		"created_at":    now,
	}).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "create review failed")
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, gerror.Wrap(err, "get last insert id failed")
	}
	return &dto.ReviewInfo{
		ID:          id,
		BuyerUserID: userID,
		ItemID:      in.ItemID,
		Rating:      in.Rating,
		Body:        in.Body,
		CreatedAt:   now,
	}, nil
}

func (s *sMarket) ListReviews(ctx context.Context, itemID int64) ([]*dto.ReviewInfo, error) {
	var reviews []*dto.ReviewInfo
	m := g.DB().Model("reviews").Ctx(ctx).Order("id DESC")
	if itemID > 0 {
		m = m.Where("item_id", itemID)
	}
	err := m.Scan(&reviews)
	if err != nil {
		return nil, gerror.Wrap(err, "list reviews failed")
	}
	return reviews, nil
}
