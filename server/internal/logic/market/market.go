package market

import "ai-platform/internal/service"

type sMarket struct{}

func init() { service.RegisterMarket(New()) }

func New() *sMarket { return &sMarket{} }
