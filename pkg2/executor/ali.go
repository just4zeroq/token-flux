package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ai-platform/pkg/model"
	"ai-platform/pkg/translator"
)

func init() {
	Register(model.ProviderAli, &AliExecutor{})
	RegisterProtocol(model.ProtocolAli, &AliExecutor{})
}

// AliExecutor handles Alibaba Cloud DashScope API (Qwen models).
// Uses /compatible-mode/ paths for OpenAI, /apps/anthropic/ for Claude protocol.
// Requires X-DashScope-SSE header for streaming.
type AliExecutor struct {
	channel *model.Channel
}

func (e *AliExecutor) Init(channel *model.Channel) {
	e.channel = channel
}

func (e *AliExecutor) NativeFormats() []EndpointCapability {
	return []EndpointCapability{
		{Format: "openai", RelayMode: model.RelayModeChatCompletions},
		{Format: "claude", RelayMode: model.RelayModeClaudeMessages},
	}
}

func (e *AliExecutor) GetName() string {
	if e.channel != nil && e.channel.Name != "" {
		return e.channel.Name
	}
	return "Ali"
}

func (e *AliExecutor) GetRequestURL(info *RequestInfo) (string, error) {
	baseURL := ""
	if info.Protocol != nil {
		baseURL = strings.TrimSuffix(info.Protocol.BaseURL, "/")
	}
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	switch info.RelayMode {
	case model.RelayModeClaudeMessages:
		return baseURL + "/apps/anthropic/v1/messages", nil
	case model.RelayModeChatCompletions:
		return baseURL + "/compatible-mode/v1/chat/completions", nil
	case model.RelayModeCompletions:
		return baseURL + "/compatible-mode/v1/completions", nil
	case model.RelayModeEmbeddings:
		return baseURL + "/compatible-mode/v1/embeddings", nil
	case model.RelayModeImagesGenerations:
		return baseURL + "/api/v1/services/aigc/text2image/image-synthesis", nil
	default:
		return baseURL + "/compatible-mode/v1/chat/completions", nil
	}
}

func (e *AliExecutor) SetupRequestHeader(header http.Header, info *RequestInfo) error {
	header.Set("Authorization", "Bearer "+info.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")

	if info.IsStream {
		header.Set("X-DashScope-SSE", "enable")
	}

	return nil
}

func (e *AliExecutor) TransformRequest(ctx context.Context, info *RequestInfo, requestBody []byte) (io.Reader, error) {
	// Claude inbound: passthrough, only model mapping.
	if info.InboundFormat == "claude" {
		if info.Protocol != nil && info.Protocol.IsModelMapped && info.Protocol.UpstreamModelName != "" {
			body := replaceModelField(bodyFromBytes(requestBody), info.Protocol.UpstreamModelName)
			return bytes.NewReader(body), nil
		}
		return bytes.NewReader(bodyFromBytes(requestBody)), nil
	}

	body := bodyFromBytes(requestBody)

	// Non-OpenAI format → normalize to OpenAI first.
	if !shouldPassthrough(info, "openai") {
		var tf translator.Format
		switch info.InboundFormat {
		case "gemini":
			tf = translator.FormatGemini
		case "openai_responses":
			tf = translator.FormatOpenAIResponses
		default:
			tf = translator.FormatOpenAI
		}
		converted, err := translator.Normalize(body, tf)
		if err != nil {
			// keep original on error
		} else {
			body = converted
		}
	}

	// DashScope-specific parameter adaptation (top_p clipping).
	body = applyDashScopeCompatibility(body)

	// Model mapping.
	if info.Protocol != nil && info.Protocol.IsModelMapped && info.Protocol.UpstreamModelName != "" {
		body = replaceModelField(body, info.Protocol.UpstreamModelName)
	}

	// Stream options for usage tracking.
	if info.IsStream {
		body = injectStreamOptionsOpenAI(body)
	}

	return bytes.NewReader(body), nil
}

func (e *AliExecutor) DoRequest(ctx context.Context, info *RequestInfo, requestBody io.Reader) (*http.Response, error) {
	reqURL, err := e.GetRequestURL(info)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if err := e.SetupRequestHeader(httpReq.Header, info); err != nil {
		return nil, fmt.Errorf("setup header: %w", err)
	}

	timeout := 60
	if info.Channel != nil && info.Channel.Settings.TimeoutSeconds > 0 {
		timeout = info.Channel.Settings.TimeoutSeconds
	}

	client := &http.Client{Timeout: secondsAsDuration(timeout)}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	return resp, nil
}

func (e *AliExecutor) TransformResponse(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	// Claude inbound → delegate to ClaudeExecutor for native response handling.
	if info.InboundFormat == "claude" {
		ce := &ClaudeExecutor{}
		ce.Init(info.Channel)
		return ce.TransformResponse(ctx, resp, info, writer)
	}

	// Standard OpenAI-compatible response handling.
	oe := &OpenAIExecutor{}
	oe.Init(info.Channel)
	return oe.TransformResponse(ctx, resp, info, writer)
}

// ---- Ali-specific helpers ----

// applyDashScopeCompatibility handles DashScope parameter constraints.
// DashScope requires top_p in (0, 1) open interval.
func applyDashScopeCompatibility(body []byte) []byte {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawMap); err != nil {
		return body
	}

	if topPRaw, ok := rawMap["top_p"]; ok {
		var topP float64
		if err := json.Unmarshal(topPRaw, &topP); err == nil {
			if topP >= 1.0 {
				topP = 0.999
			} else if topP <= 0 {
				topP = 0.001
			}
			capped, _ := json.Marshal(topP)
			rawMap["top_p"] = capped
		}
	}

	result, err := json.Marshal(rawMap)
	if err != nil {
		return body
	}
	return result
}
