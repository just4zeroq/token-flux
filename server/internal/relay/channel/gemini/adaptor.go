package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ai-platform/internal/relay/common"
	"ai-platform/internal/relay/constant"
	"ai-platform/internal/relay/helper"
	"ai-platform/internal/relay/override"
)

// Adaptor Gemini 供应商适配器
type Adaptor struct {
	info *common.RelayInfo
}

// Init 初始化适配器
func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

func (a *Adaptor) getRelayAction(info *common.RelayInfo) (string, error) {
	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeChatCompletions, constant.RelayModeGeminiChat:
		if info.IsStream {
			return "streamGenerateContent", nil
		}
		return "generateContent", nil
	case constant.RelayModeEmbeddings:
		return "embedContent", nil
	case constant.RelayModeImagesGenerations:
		if strings.HasPrefix(info.ChannelMeta.UpstreamModelName, "imagen") {
			return "predict", nil
		}
		return "generateContent", nil
	default:
		return "", fmt.Errorf("unsupported relay mode for Gemini: %d", info.RelayMode)
	}
}

// GetRequestURL 构建上游请求 URL
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")
	model := info.ChannelMeta.UpstreamModelName

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeChatCompletions, constant.RelayModeGeminiChat:
		if info.IsStream {
			return fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", baseURL, model), nil
		}
		return fmt.Sprintf("%s/v1beta/models/%s:generateContent", baseURL, model), nil
	case constant.RelayModeEmbeddings:
		return fmt.Sprintf("%s/v1beta/models/%s:embedContent", baseURL, model), nil
	case constant.RelayModeImagesGenerations:
		if strings.HasPrefix(model, "imagen") {
			return fmt.Sprintf("%s/v1beta/models/%s:predict", baseURL, model), nil
		}
		return fmt.Sprintf("%s/v1beta/models/%s:generateContent", baseURL, model), nil
	default:
		return "", fmt.Errorf("unsupported relay mode for Gemini: %d", info.RelayMode)
	}
}

// SetupRequestHeader 设置上游请求头
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("x-goog-api-key", info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")

	if info.RequestHeaders != nil {
		for _, h := range []string{"X-Request-Id"} {
			if v := info.RequestHeaders.Get(h); v != "" {
				header.Set(h, v)
			}
		}
	}

	return nil
}

// ConvertRequest 根据入站格式转换请求体为 Gemini 格式
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	if constant.RelayMode(info.RelayMode) == constant.RelayModeImagesGenerations {
		if strings.HasPrefix(info.ChannelMeta.UpstreamModelName, "imagen") {
			return convertImageRequest(requestBody, info)
		}
		return convertImageRequestToChat(requestBody, info)
	}

	var converted io.Reader
	switch info.InboundFormat {
	case constant.RelayFormatGemini:
		cleaned := helper.StripStreamField(requestBody)
		converted = bytes.NewReader(cleaned)
	case constant.RelayFormatOpenAI:
		r, err := ConvertOpenAIToGemini(requestBody, info)
		if err != nil {
			return nil, err
		}
		converted = r
	case constant.RelayFormatClaude:
		r, err := ConvertClaudeToGemini(requestBody, info)
		if err != nil {
			return nil, err
		}
		converted = r
	case constant.RelayFormatResponses:
		r, err := ConvertResponsesToGemini(requestBody, info)
		if err != nil {
			return nil, err
		}
		converted = r
	default:
		r, err := ConvertOpenAIToGemini(requestBody, info)
		if err != nil {
			return nil, err
		}
		converted = r
	}

	if info.ThinkingEnabled || info.ReasoningEffort != "" {
		converted = injectGeminiThinking(converted, info)
	}

	return converted, nil
}

// injectGeminiThinking 注入 Gemini thinking 配置
func injectGeminiThinking(r io.Reader, info *common.RelayInfo) io.Reader {
	body, err := io.ReadAll(r)
	if err != nil {
		return r
	}
	var req map[string]json.RawMessage
	if err := json.Unmarshal(body, &req); err != nil {
		return bytes.NewReader(body)
	}

	if info.ThinkingEnabled {
		var maxTokens int
		if mt, ok := req["maxOutputTokens"]; ok {
			_ = json.Unmarshal(mt, &maxTokens)
		}
		if maxTokens < 128 {
			maxTokens = 8192
		}
		budget := maxTokens * 80 / 100
		if budget < 128 {
			budget = 128
		}
		req["thinkingConfig"] = json.RawMessage(fmt.Sprintf(`{"thoughtBudget":%d,"includeThoughts":true}`, budget))
	} else if info.ReasoningEffort != "" {
		req["thinkingConfig"] = json.RawMessage(fmt.Sprintf(`{"thinkingLevel":"%s","includeThoughts":true}`,
			strings.ToUpper(info.ReasoningEffort)))
	}

	result, err := json.Marshal(req)
	if err != nil {
		return bytes.NewReader(body)
	}
	return bytes.NewReader(result)
}

// DoRequest 发送请求到上游
func (a *Adaptor) DoRequest(ctx context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	reqURL, err := a.GetRequestURL(info)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	if err := a.SetupRequestHeader(httpReq.Header, info); err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}

	if hdrOverrides, hdrErr := override.ApplyHeaderOverride(info); hdrErr == nil && len(hdrOverrides) > 0 {
		override.MergeHeaderOverrides(httpReq.Header, hdrOverrides)
	}

	timeout := info.ChannelMeta.Settings.TimeoutSeconds
	if timeout <= 0 {
		timeout = 60
	}

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request failed: %w", err)
	}

	return resp, nil
}

// DoResponse 处理上游响应，根据客户端格式分发
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	if constant.RelayMode(info.RelayMode) == constant.RelayModeImagesGenerations {
		if strings.HasPrefix(info.ChannelMeta.UpstreamModelName, "imagen") {
			return handleImagenResponse(ctx, resp, info, writer)
		}
		return handleBananaImageResponse(ctx, resp, writer)
	}

	clientFormat := info.GetOriginalClientFormat()

	switch clientFormat {
	case constant.RelayFormatGemini:
		return a.handleGeminiNativeResponse(ctx, resp, info, writer)
	case constant.RelayFormatOpenAI:
		if info.IsStream {
			return a.handleStreamToOpenAI(ctx, resp, info, writer)
		}
		return a.handleNonStreamToOpenAI(ctx, resp, info, writer)
	default:
		if info.IsStream {
			return a.handleStreamToOpenAI(ctx, resp, info, writer)
		}
		return a.handleNonStreamToOpenAI(ctx, resp, info, writer)
	}
}

// GetChannelName 返回渠道名称
func (a *Adaptor) GetChannelName() string {
	return "Gemini"
}

// 确保接口实现
var _ common.Adaptor = (*Adaptor)(nil)
