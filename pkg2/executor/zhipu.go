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
	Register(model.ProviderZhipu, &ZhipuExecutor{})
	RegisterProtocol(model.ProtocolZhipu, &ZhipuExecutor{})
}

// ZhipuExecutor handles Zhipu AI GLM API (V4, OpenAI-compatible).
// Uses /api/paas/v4/ paths for standard endpoints, /api/anthropic/v1/ for Claude protocol.
type ZhipuExecutor struct {
	channel *model.Channel
}

func (e *ZhipuExecutor) Init(channel *model.Channel) {
	e.channel = channel
}

func (e *ZhipuExecutor) NativeFormats() []EndpointCapability {
	return []EndpointCapability{
		{Format: "openai", RelayMode: model.RelayModeChatCompletions},
		{Format: "claude", RelayMode: model.RelayModeClaudeMessages},
	}
}

func (e *ZhipuExecutor) GetName() string {
	if e.channel != nil && e.channel.Name != "" {
		return e.channel.Name
	}
	return "Zhipu"
}

func (e *ZhipuExecutor) GetRequestURL(info *RequestInfo) (string, error) {
	baseURL := ""
	if info.Protocol != nil {
		baseURL = strings.TrimSuffix(info.Protocol.BaseURL, "/")
	}
	if baseURL == "" {
		baseURL = "https://open.bigmodel.cn"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	switch info.RelayMode {
	case model.RelayModeClaudeMessages:
		return baseURL + "/api/anthropic/v1/messages", nil
	case model.RelayModeChatCompletions, model.RelayModeResponses:
		return baseURL + "/api/paas/v4/chat/completions", nil
	case model.RelayModeEmbeddings:
		return baseURL + "/api/paas/v4/embeddings", nil
	case model.RelayModeImagesGenerations:
		return baseURL + "/api/paas/v4/images/generations", nil
	default:
		return baseURL + "/api/paas/v4/chat/completions", nil
	}
}

func (e *ZhipuExecutor) SetupRequestHeader(header http.Header, info *RequestInfo) error {
	header.Set("Authorization", "Bearer "+info.ApiKey)
	header.Set("Content-Type", "application/json")
	if info.IsStream {
		header.Set("Accept", "text/event-stream")
	} else {
		header.Set("Accept", "application/json")
	}
	return nil
}

func (e *ZhipuExecutor) TransformRequest(ctx context.Context, info *RequestInfo, requestBody []byte) (io.Reader, error) {
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

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawMap); err != nil {
		return bytes.NewReader(body), nil
	}

	// GLM-specific compatibility: top_p clipping, image prefix stripping.
	rawMap = applyGLMCompatibility(rawMap)

	// Inject stream_options for streaming requests.
	if info.IsStream {
		if _, exists := rawMap["stream_options"]; !exists {
			rawMap["stream_options"] = json.RawMessage(`{"include_usage":true}`)
		}
	}

	// Inject thinking/thinking-disabled params.
	rawMap = injectZhipuThinking(rawMap, info)

	// Model mapping.
	if info.Protocol != nil && info.Protocol.IsModelMapped && info.Protocol.UpstreamModelName != "" {
		rawMap["model"] = json.RawMessage(`"` + info.Protocol.UpstreamModelName + `"`)
	}

	result, err := json.Marshal(rawMap)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	return bytes.NewReader(result), nil
}

func (e *ZhipuExecutor) DoRequest(ctx context.Context, info *RequestInfo, requestBody io.Reader) (*http.Response, error) {
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

func (e *ZhipuExecutor) TransformResponse(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	// Claude inbound → delegate to ClaudeExecutor.
	if info.InboundFormat == "claude" {
		ce := &ClaudeExecutor{}
		ce.Init(info.Channel)
		return ce.TransformResponse(ctx, resp, info, writer)
	}

	// Standard OpenAI-compatible response.
	oe := &OpenAIExecutor{}
	oe.Init(info.Channel)
	return oe.TransformResponse(ctx, resp, info, writer)
}

// ---- Zhipu-specific helpers ----

// applyGLMCompatibility handles GLM-specific compatibility:
// - top_p upper bound clipping (GLM requires < 1.0)
// - base64 image URL prefix stripping (GLM requires raw base64 data)
func applyGLMCompatibility(rawMap map[string]json.RawMessage) map[string]json.RawMessage {
	// TopP clipping: GLM requires top_p < 1.0.
	if topPRaw, ok := rawMap["top_p"]; ok {
		var topP float64
		if err := json.Unmarshal(topPRaw, &topP); err == nil && topP >= 1.0 {
			rawMap["top_p"] = json.RawMessage(`0.99`)
		}
	}

	// Strip base64 image URL prefixes: GLM vision models require raw base64 data.
	if messagesRaw, ok := rawMap["messages"]; ok {
		messagesBytes, changed := stripImageURLPrefixes(messagesRaw)
		if changed {
			rawMap["messages"] = messagesBytes
		}
	}

	return rawMap
}

// stripImageURLPrefixes strips "data:image/...;base64," prefixes from image_url fields.
// GLM vision models require pure base64 data without data URI prefix.
func stripImageURLPrefixes(messagesRaw json.RawMessage) (json.RawMessage, bool) {
	var messages []json.RawMessage
	if err := json.Unmarshal(messagesRaw, &messages); err != nil {
		return messagesRaw, false
	}

	changed := false
	for i, msgRaw := range messages {
		var msg map[string]json.RawMessage
		if err := json.Unmarshal(msgRaw, &msg); err != nil {
			continue
		}

		contentRaw, ok := msg["content"]
		if !ok {
			continue
		}

		// content may be string or array; only handle array form.
		var contentParts []map[string]json.RawMessage
		if err := json.Unmarshal(contentRaw, &contentParts); err != nil {
			continue
		}

		partChanged := false
		for j, part := range contentParts {
			typeRaw, ok := part["type"]
			if !ok {
				continue
			}
			var partType string
			if err := json.Unmarshal(typeRaw, &partType); err != nil || partType != "image_url" {
				continue
			}

			imgURLRaw, ok := part["image_url"]
			if !ok {
				continue
			}

			var imgURL map[string]json.RawMessage
			if err := json.Unmarshal(imgURLRaw, &imgURL); err != nil {
				continue
			}

			urlRaw, ok := imgURL["url"]
			if !ok {
				continue
			}

			var url string
			if err := json.Unmarshal(urlRaw, &url); err != nil {
				continue
			}

			if strings.HasPrefix(url, "data:image/") {
				if idx := strings.Index(url, ","); idx != -1 {
					url = url[idx+1:]
					imgURL["url"], _ = json.Marshal(url)
					part["image_url"], _ = json.Marshal(imgURL)
					contentParts[j] = part
					partChanged = true
				}
			}
		}

		if partChanged {
			msg["content"], _ = json.Marshal(contentParts)
			messages[i], _ = json.Marshal(msg)
			changed = true
		}
	}

	if !changed {
		return messagesRaw, false
	}

	result, err := json.Marshal(messages)
	if err != nil {
		return messagesRaw, false
	}
	return result, true
}

// injectZhipuThinking injects thinking parameters for GLM models.
//
// GLM thinking param (GLM-4.5+):
//   - thinking.type: "enabled"(default) / "disabled"
//   - thinking.clear_thinking: controls whether to clear historical reasoning_content
//
// Priority: client-set > suffix-routing > GLM default (enabled).
func injectZhipuThinking(rawMap map[string]json.RawMessage, info *RequestInfo) map[string]json.RawMessage {
	// Client already set thinking param → do not intervene.
	if _, clientSet := rawMap["thinking"]; clientSet {
		return rawMap
	}

	// -nothinking suffix → explicit disable.
	if info.ThinkingDisabled {
		rawMap["thinking"] = json.RawMessage(`{"type":"disabled"}`)
		return rawMap
	}

	// -thinking suffix → explicit enable.
	if info.ThinkingEnabled {
		rawMap["thinking"] = json.RawMessage(`{"type":"enabled"}`)
		return rawMap
	}

	return rawMap
}
