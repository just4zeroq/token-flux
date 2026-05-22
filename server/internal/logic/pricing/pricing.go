package pricing

import "ai-platform/internal/service"

type sPricing struct{}

func init() { service.RegisterPricing(New()) }

func New() *sPricing { return &sPricing{} }
