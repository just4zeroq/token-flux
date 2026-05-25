package llm

import (
	"ai-platform/internal/service"
)

type sLLM struct{}

func init() { service.RegisterLLM(New()) }

func New() *sLLM { return &sLLM{} }

// Runtime methods (ListAvailableModels, ProxyChatCompletions, ProxyCompletions,
// ProxyEmbeddings) are implemented in runtime.go / openai.go.
