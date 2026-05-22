package mcp

import "ai-platform/internal/service"

type sMCP struct{}

func init() { service.RegisterMCPRuntime(New()) }

func New() *sMCP { return &sMCP{} }
