package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ai-platform/pkg/model"
	"ai-platform/pkg/translator"
)

func init() {
	Register(model.ProviderMiniMax, &MiniMaxExecutor{})
}

// MiniMaxExecutor handles MiniMax API.
// Uses /v1/text/chatcompletion_v2 for chat, /anthropic/v1/messages for Claude protocol.
type MiniMaxExecutor struct {
	channel *model.Channel
}

func (e *MiniMaxExecutor) Init(channel *model.Channel) {
	e.channel = channel
}

func (e *MiniMaxExecutor) NativeFormats() []EndpointCapability {
	return []EndpointCapability{
		{Format: "openai", RelayMode: model.RelayModeChatCompletions},
		{Format: "claude", RelayMode: model.RelayModeClaudeMessages},
	}
}

func (e *MiniMaxExecutor) GetName() string {
	if e.channel != nil && e.channel.Name != "" {
		return e.channel.Name
	}
	return "MiniMax"
}

func (e *MiniMaxExecutor) GetRequestURL(info *RequestInfo) (string, error) {
	baseURL := ""
	if info.Protocol != nil {
		baseURL = strings.TrimSuffix(info.Protocol.BaseURL, "/")
	}
	if baseURL == "" {
		baseURL = "https://api.minimax.chat"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	switch info.RelayMode {
	case model.RelayModeClaudeMessages:
		return baseURL + "/anthropic/v1/messages", nil
	case model.RelayModeChatCompletions:
		return baseURL + "/v1/text/chatcompletion_v2", nil
	case model.RelayModeImagesGenerations:
		return baseURL + "/v1/image/generation", nil
	default:
		return "", fmt.Errorf("minimax: unsupported relay mode: %d", info.RelayMode)
	}
}

func (e *MiniMaxExecutor) SetupRequestHeader(header http.Header, info *RequestInfo) error {
	header.Set("Authorization", "Bearer "+info.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return nil
}

func (e *MiniMaxExecutor) TransformRequest(ctx context.Context, info *RequestInfo, requestBody []byte) (io.Reader, error) {
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

	// Model mapping.
	if info.Protocol != nil && info.Protocol.IsModelMapped && info.Protocol.UpstreamModelName != "" {
		body = replaceModelField(body, info.Protocol.UpstreamModelName)
	}

	return bytes.NewReader(body), nil
}

func (e *MiniMaxExecutor) DoRequest(ctx context.Context, info *RequestInfo, requestBody io.Reader) (*http.Response, error) {
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

func (e *MiniMaxExecutor) TransformResponse(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	// Claude inbound → delegate to ClaudeExecutor.
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
