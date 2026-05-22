package wallet

import "ai-platform/internal/service"

type sWallet struct{}

func init() { service.RegisterWallet(New()) }

func New() *sWallet { return &sWallet{} }
