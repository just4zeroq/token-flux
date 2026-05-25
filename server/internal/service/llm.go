package service

import (
	"context"

	"ai-platform/internal/model/dto"
)

type ILLM interface {
	CreateChannel(ctx context.Context, in dto.LLMCreateChannelIn) (*dto.LLMChannelInfo, error)
	ListChannels(ctx context.Context, in dto.LLMListChannelsIn) ([]*dto.LLMChannelInfo, int, error)
	ReviewChannel(ctx context.Context, in dto.LLMReviewChannelIn) error

	CreateModelSpec(ctx context.Context, in dto.LLMCreateModelSpecIn) (*dto.LLMModelSpecInfo, error)
	ListModelSpecs(ctx context.Context, in dto.LLMListModelSpecsIn) ([]*dto.LLMModelSpecInfo, int, error)
	ReviewModelSpec(ctx context.Context, in dto.LLMReviewModelSpecIn) error

	UpsertModelPrice(ctx context.Context, in dto.LLMUpsertModelPriceIn) (*dto.LLMModelPriceInfo, error)
	ListModelPrices(ctx context.Context, modelSpecID int64) ([]*dto.LLMModelPriceInfo, error)

	CreateModelKey(ctx context.Context, providerUserID int64, in dto.LLMCreateModelKeyIn) (*dto.LLMModelKeyInfo, error)
	ListModelKeys(ctx context.Context, in dto.LLMListModelKeysIn) ([]*dto.LLMModelKeyInfo, int, error)
	DisableModelKey(ctx context.Context, providerUserID, keyID int64, isAdmin bool) error

	BindKeyModel(ctx context.Context, providerUserID int64, in dto.LLMBindKeyModelIn) (*dto.LLMKeyModelInfo, error)
	ListKeyModels(ctx context.Context, in dto.LLMListKeyModelsIn) ([]*dto.LLMKeyModelInfo, int, error)
	TriggerKeyModelTest(ctx context.Context, keyModelID int64) error

	ListAvailableModels(ctx context.Context, apiKeyID, userID int64) ([]*dto.OpenAIModelInfo, error)
	ProxyChatCompletions(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error)
	ProxyCompletions(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error)
	ProxyEmbeddings(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error)
}

var localLLM ILLM

func RegisterLLM(i ILLM) { localLLM = i }

func LLM() ILLM {
	if localLLM == nil {
		panic("service.LLM not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localLLM
}
