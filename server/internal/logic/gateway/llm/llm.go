package llm

import "ai-platform/internal/service"

type sLLM struct{}

func init() { service.RegisterLLMRuntime(New()) }

func New() *sLLM { return &sLLM{} }
