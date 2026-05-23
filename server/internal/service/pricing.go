package service

import (
	"context"

	"ai-platform/internal/model/dto"
)

// IPricing is the contract for the pricing domain (pricing rules, exchange rates).
type IPricing interface {
	ListPricingRules(ctx context.Context, itemID int64) ([]*dto.PricingRuleInfo, error)
	GetPricingRule(ctx context.Context, id int64) (*dto.PricingRuleInfo, error)
	CreatePricingRule(ctx context.Context, in dto.CreatePricingRuleIn) (*dto.PricingRuleInfo, error)
	ListExchangeRates(ctx context.Context, from, to string) ([]*dto.ExchangeRateInfo, error)
}

var localPricing IPricing

func RegisterPricing(i IPricing) { localPricing = i }

func Pricing() IPricing {
	if localPricing == nil {
		panic("service.Pricing not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localPricing
}
