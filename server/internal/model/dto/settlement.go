package dto

import "time"

type SettlementSubmitIn struct {
	ProductType            string `json:"product_type"`
	RefType                string `json:"ref_type"`
	RefID                  int64  `json:"ref_id"`
	ConsumerUserID         int64  `json:"consumer_user_id"`
	ProviderUserID         int64  `json:"provider_user_id"`
	CostCredits            int64  `json:"cost_credits"`
	ProviderRevenueCredits int64  `json:"provider_revenue_credits"`
	CommissionCredits      int64  `json:"commission_credits"`
	PointsToConsumer       int64  `json:"points_to_consumer"`
	PointsToProvider       int64  `json:"points_to_provider"`
}

type RechargeSettlementIn struct {
	UserID          int64  `json:"user_id"`
	RefType         string `json:"ref_type"`
	RefID           int64  `json:"ref_id"`
	RechargeCredits int64  `json:"recharge_credits"`
}

type SettlementRecordInfo struct {
	ID                     int64     `json:"id"`
	ProductType            string    `json:"product_type"`
	RefType                string    `json:"ref_type"`
	RefID                  int64     `json:"ref_id"`
	ConsumerUserID         int64     `json:"consumer_user_id"`
	ProviderUserID         int64     `json:"provider_user_id"`
	CostCredits            int64     `json:"cost_credits"`
	ProviderRevenueCredits int64     `json:"provider_revenue_credits"`
	CommissionCredits      int64     `json:"commission_credits"`
	PointsToConsumer       int64     `json:"points_to_consumer"`
	PointsToProvider       int64     `json:"points_to_provider"`
	TransactionID          int64     `json:"transaction_id"`
	Status                 string    `json:"status"`
	ErrorMessage           string    `json:"error_message"`
	CreatedAt              time.Time `json:"created_at"`
	SettledAt              time.Time `json:"settled_at,omitzero"`
}
