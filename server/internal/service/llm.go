package service

import (
	"context"

	"ai-platform/internal/model/dto"
)

type ILLM interface {
	CreateChannel(ctx context.Context, in dto.LLMCreateChannelIn) (*dto.LLMChannelInfo, error)
	ListChannels(ctx context.Context, in dto.LLMListChannelsIn) ([]*dto.LLMChannelInfo, int, error)
	ReviewChannel(ctx context.Context, in dto.LLMReviewChannelIn) error
	DeleteChannel(ctx context.Context, id int64) error

	// Channel-Model binding
	BindChannelModels(ctx context.Context, channelID int64, modelIDs []int64) error
	ListChannelModels(ctx context.Context, in dto.LLMListChannelModelsIn) ([]*dto.LLMChannelModelInfo, error)
	UnbindChannelModel(ctx context.Context, channelID, modelSpecID int64) error

	CreateModelSpec(ctx context.Context, in dto.LLMCreateModelSpecIn) (*dto.LLMModelSpecInfo, error)
	ListModelSpecs(ctx context.Context, in dto.LLMListModelSpecsIn) ([]*dto.LLMModelSpecInfo, int, error)
	ReviewModelSpec(ctx context.Context, in dto.LLMReviewModelSpecIn) error
	DeleteModelSpec(ctx context.Context, id int64) error

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

	// Provider settings
	GetProviderShareBps(ctx context.Context, userID int64) (int, error)
	SetProviderShareBps(ctx context.Context, userID int64, shareBps int) error
}

var localLLM ILLM

func RegisterLLM(i ILLM) { localLLM = i }

func LLM() ILLM {
	if localLLM == nil {
		panic("service.LLM not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localLLM
}
