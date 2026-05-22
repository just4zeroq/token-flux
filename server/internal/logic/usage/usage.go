package usage

import "ai-platform/internal/service"

type sUsage struct{}

func init() { service.RegisterUsage(New()) }

func New() *sUsage { return &sUsage{} }
