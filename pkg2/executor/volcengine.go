package executor

import "ai-platform/pkg/model"

func init() {
	// Volcengine uses OpenAI-compatible API format.
	// Register provider mapping to OpenAIExecutor.
	Register(model.ProviderVolcengine, &OpenAIExecutor{})
}
