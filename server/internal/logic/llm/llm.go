package llm

import (
	"context"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/errors/gerror"
)

type sLLM struct{}

func init() { service.RegisterLLM(New()) }

func New() *sLLM { return &sLLM{} }

// Runtime stubs — implemented in Task 5.

func (s *sLLM) ListAvailableModels(ctx context.Context, apiKeyID, userID int64) ([]*dto.OpenAIModelInfo, error) {
	return nil, gerror.New("not implemented")
}

func (s *sLLM) ProxyChatCompletions(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error) {
	return nil, gerror.New("not implemented")
}

func (s *sLLM) ProxyCompletions(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error) {
	return nil, gerror.New("not implemented")
}

func (s *sLLM) ProxyEmbeddings(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error) {
	return nil, gerror.New("not implemented")
}