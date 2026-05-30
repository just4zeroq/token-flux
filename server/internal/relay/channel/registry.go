// Package channel provides the Adaptor factory for different provider types.
package channel

import (
	"ai-platform/internal/relay/channel/ali"
	"ai-platform/internal/relay/channel/aws"
	"ai-platform/internal/relay/channel/baidu_v2"
	"ai-platform/internal/relay/channel/claude"
	"ai-platform/internal/relay/channel/cloudflare"
	"ai-platform/internal/relay/channel/codex"
	"ai-platform/internal/relay/channel/coze"
	"ai-platform/internal/relay/channel/deepseek"
	"ai-platform/internal/relay/channel/dify"
	"ai-platform/internal/relay/channel/gemini"
	"ai-platform/internal/relay/channel/jimeng"
	"ai-platform/internal/relay/channel/minimax"
	"ai-platform/internal/relay/channel/mistral"
	"ai-platform/internal/relay/channel/moonshot"
	"ai-platform/internal/relay/channel/ollama"
	"ai-platform/internal/relay/channel/openai"
	"ai-platform/internal/relay/channel/siliconflow"
	"ai-platform/internal/relay/channel/submodel"
	"ai-platform/internal/relay/channel/tencent"
	"ai-platform/internal/relay/channel/vertex"
	"ai-platform/internal/relay/channel/volcengine"
	"ai-platform/internal/relay/channel/xai"
	"ai-platform/internal/relay/channel/xunfei"
	"ai-platform/internal/relay/channel/zhipu"
	"ai-platform/internal/relay/common"
	"ai-platform/internal/relay/constant"
)

// GetAdaptor returns the appropriate Adaptor for the given protocol or provider type.
// Supports provider type int (1-40, see constant.ProviderType) and "openai-compatible" string.
func GetAdaptor(providerType any) common.Adaptor {
	switch t := providerType.(type) {
	case int:
		return getByProviderType(t)
	case constant.ProviderType:
		return getByProviderType(int(t))
	case string:
		return getByProtocol(t)
	}
	return nil
}

func getByProviderType(pt int) common.Adaptor {
	switch constant.ProviderType(pt) {
	case constant.ProviderOpenAI:
		return &openai.Adaptor{}
	case constant.ProviderClaude:
		return &claude.Adaptor{}
	case constant.ProviderGemini:
		return &gemini.Adaptor{}
	case constant.ProviderAli:
		return &ali.Adaptor{}
	case constant.ProviderDeepSeek:
		return &deepseek.Adaptor{}
	case constant.ProviderZhipu:
		return &zhipu.Adaptor{}
	case constant.ProviderMoonshot:
		return &moonshot.Adaptor{}
	case constant.ProviderMistral:
		return &mistral.Adaptor{}
	case constant.ProviderXAI:
		return &xai.Adaptor{}
	case constant.ProviderSiliconFlow:
		return &siliconflow.Adaptor{}
	case constant.ProviderCloudflare:
		return &cloudflare.Adaptor{}
	case constant.ProviderSubmodel:
		return &submodel.Adaptor{}
	case constant.ProviderBaiduV2:
		return &baidu_v2.Adaptor{}
	case constant.ProviderVolcengine:
		return &volcengine.Adaptor{}
	case constant.ProviderMiniMax:
		return &minimax.Adaptor{}
	case constant.ProviderOllama:
		return &ollama.Adaptor{}
	case constant.ProviderVertex:
		return &vertex.Adaptor{}
	case constant.ProviderTencent:
		return &tencent.Adaptor{}
	case constant.ProviderXunfei:
		return &xunfei.Adaptor{}
	case constant.ProviderCoze:
		return &coze.Adaptor{}
	case constant.ProviderDify:
		return &dify.Adaptor{}
	case constant.ProviderJimeng:
		return &jimeng.Adaptor{}
	case constant.ProviderCodex:
		return &codex.Adaptor{}
	case constant.ProviderAWS:
		return &aws.Adaptor{}
	case constant.ProviderAzure:
		return &openai.Adaptor{} // Azure OpenAI is OpenAI-compatible
	// OpenAI-compatible pass-through
	case constant.ProviderAI360,
		constant.ProviderLingyi,
		constant.ProviderOpenRouter,
		constant.ProviderXInference:
		return &openai.Adaptor{}
	default:
		return nil
	}
}

func getByProtocol(protocol string) common.Adaptor {
	switch protocol {
	case "openai-compatible", "azure":
		return &openai.Adaptor{}
	case "anthropic-compatible", "claude":
		return &claude.Adaptor{}
	case "gemini-compatible", "gemini":
		return &gemini.Adaptor{}
	case "ali", "qwen":
		return &ali.Adaptor{}
	case "deepseek":
		return &deepseek.Adaptor{}
	case "zhipu", "glm":
		return &zhipu.Adaptor{}
	case "moonshot":
		return &moonshot.Adaptor{}
	case "mistral":
		return &mistral.Adaptor{}
	case "xai", "grok":
		return &xai.Adaptor{}
	case "siliconflow":
		return &siliconflow.Adaptor{}
	case "baidu", "ernie":
		return &baidu_v2.Adaptor{}
	case "volcengine", "ark":
		return &volcengine.Adaptor{}
	case "minimax":
		return &minimax.Adaptor{}
	case "ollama":
		return &ollama.Adaptor{}
	case "tencent", "hunyuan":
		return &tencent.Adaptor{}
	case "xunfei", "spark":
		return &xunfei.Adaptor{}
	case "cloudflare":
		return &cloudflare.Adaptor{}
	case "vertex":
		return &vertex.Adaptor{}
	case "aws", "bedrock":
		return &aws.Adaptor{}
	default:
		return nil
	}
}
