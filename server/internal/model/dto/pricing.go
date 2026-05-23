package dto

import "time"

// PricingRuleInfo represents a pricing rule for a catalog item.
type PricingRuleInfo struct {
	ID            int64      `json:"id"`
	ItemID        int64      `json:"item_id"`
	Strategy      string     `json:"strategy"`
	Params        string     `json:"params"`
	EffectiveFrom time.Time  `json:"effective_from"`
	EffectiveTo   *time.Time `json:"effective_to,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// CreatePricingRuleIn is the input for creating a pricing rule.
type CreatePricingRuleIn struct {
	ItemID        int64      `json:"item_id" v:"required"`
	Strategy      string     `json:"strategy" v:"required"`
	Params        string     `json:"params" v:"required"`
	EffectiveFrom time.Time  `json:"effective_from" v:"required"`
	EffectiveTo   *time.Time `json:"effective_to,omitempty"`
}

// ExchangeRateInfo represents an exchange rate between two assets.
type ExchangeRateInfo struct {
	ID          int64     `json:"id"`
	AssetFrom   string    `json:"asset_from"`
	AssetTo     string    `json:"asset_to"`
	RateMicro   int64     `json:"rate_micro"`
	FeeRateBps  int       `json:"fee_rate_bps"`
	EffectiveAt time.Time `json:"effective_at"`
}
