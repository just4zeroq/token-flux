package service

type IPricing interface{}

var localPricing IPricing

func RegisterPricing(i IPricing) { localPricing = i }

func Pricing() IPricing {
	if localPricing == nil {
		panic("service.Pricing not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localPricing
}
