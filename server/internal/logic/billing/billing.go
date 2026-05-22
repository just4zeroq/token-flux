package billing

import "ai-platform/internal/service"

type sBilling struct{}

func init() { service.RegisterBilling(New()) }

func New() *sBilling { return &sBilling{} }
