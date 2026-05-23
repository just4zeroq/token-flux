package dto

import "time"

type OrderInfo struct {
	ID            int64     `json:"id"`
	OrderNo       string    `json:"order_no"`
	BuyerUserID   int64     `json:"buyer_user_id"`
	ItemID        int64     `json:"item_id"`
	PlanType      string    `json:"plan_type"`
	AmountCredits int64     `json:"amount_credits"`
	PaymentMethod string    `json:"payment_method"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ReviewInfo struct {
	ID          int64     `json:"id"`
	BuyerUserID int64     `json:"buyer_user_id"`
	ItemID      int64     `json:"item_id"`
	Rating      int       `json:"rating"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateOrderIn struct {
	ItemID        int64  `json:"item_id" v:"required"`
	PlanType      string `json:"plan_type"`
	AmountCredits int64  `json:"amount_credits"`
	PaymentMethod string `json:"payment_method"`
}

type CreateReviewIn struct {
	ItemID int64  `json:"item_id" v:"required"`
	Rating int    `json:"rating" v:"required|between:1,5"`
	Body   string `json:"body"`
}
