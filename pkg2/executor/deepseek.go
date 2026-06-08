package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"ai-platform/pkg/model"
	"ai-platform/pkg/translator"
)

func init() {
	Register(model.ProviderDeepSeek, &DeepSeekExecutor{})
}

// DeepSeekExecutor handles DeepSeek API.
// OpenAI-compatible with: FIM endpoint, thinking injection, Claude-compatible endpoint.
type DeepSeekExecutor struct {
	channel *model.Channel
}

func (e *DeepSeekExecutor) Init(channel *model.Channel) {
	e.channel = channel
}

func (e *DeepSeekExecutor) NativeFormats() []EndpointCapability {
	return []EndpointCapability{
		{Format: "openai", RelayMode: model.RelayModeChatCompletions},
		{Format: "claude", RelayMode: model.RelayModeClaudeMessages},
	}
}

func (e *DeepSeekExecutor) GetName() string {
	if e.channel != nil && e.channel.Name != "" {
		return e.channel.Name
	}
	return "DeepSeek"
}

func (e *DeepSeekExecutor) GetRequestURL(info *RequestInfo) (string, error) {
	baseURL := ""
	if info.Protocol != nil {
		baseURL = strings.TrimSuffix(info.Protocol.BaseURL, "/")
	}
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	switch info.RelayMode {
	case model.RelayModeCompletions:
		betaURL := baseURL
		if !strings.HasSuffix(betaURL, "/beta") {
			betaURL += "/beta"
		}
		return betaURL + "/completions", nil
	case model.RelayModeChatCompletions:
		return baseURL + "/v1/chat/completions", nil
	case model.RelayModeClaudeMessages:
		return baseURL + "/anthropic/v1/messages", nil
	case model.RelayModeEmbeddings:
		return baseURL + "/v1/embeddings", nil
	default:
		return baseURL + "/v1/chat/completions", nil
	}
}

func (e *DeepSeekExecutor) SetupRequestHeader(header http.Header, info *RequestInfo) error {
	header.Set("Authorization", "Bearer "+info.ApiKey)
	header.Set("Content-Type", "application/json")
	if info.IsStream {
		header.Set("Accept", "text/event-stream")
	} else {
		header.Set("Accept", "application/json")
	}
	return nil
}

func (e *DeepSeekExecutor) TransformRequest(ctx context.Context, info *RequestInfo, requestBody []byte) (io.Reader, error) {
	plan := Plan(info.InboundFormat, info.ClientFormat, e.NativeFormats())

	var body []byte

	if plan.NeedRequestConv {
		converted, err := convertDeepSeekRequest(bodyFromBytes(requestBody), info.InboundFormat, plan.UpstreamFormat)
		if err != nil {
			log.Printf("[executor:deepseek] request conversion error: %v, using original body", err)
			body = bodyFromBytes(requestBody)
		} else {
			body = converted
		}
	} else {
		body = bodyFromBytes(requestBody)
	}

	// Update info for downstream (URL routing, response handling).
	info.InboundFormat = plan.UpstreamFormat
	info.RelayMode = plan.UpstreamRelayMode

	// Model mapping.
	if info.Protocol != nil && info.Protocol.IsModelMapped && info.Protocol.UpstreamModelName != "" {
		body = replaceModelField(body, info.Protocol.UpstreamModelName)
	}

	// Stream options (OpenAI format only; Claude format does not support stream_options).
	if info.IsStream && plan.UpstreamFormat != "claude" {
		body = injectStreamOptionsOpenAI(body)
	}

	// Thinking / reasoning effort injection.
	body = injectDSThinking(body, info)

	return bytes.NewReader(body), nil
}

// convertDeepSeekRequest converts request body from one format to another.
// Thin wrapper around translator.Convert (will be inlined in Phase 3).
func convertDeepSeekRequest(body []byte, from, to translator.Format) ([]byte, error) {
	return translator.Convert(body, from, to)
}

func (e *DeepSeekExecutor) DoRequest(ctx context.Context, info *RequestInfo, requestBody io.Reader) (*http.Response, error) {
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

	timeout := 120
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

func (e *DeepSeekExecutor) TransformResponse(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	// Claude inbound → delegate to claude executor for response.
	if info.InboundFormat == "claude" {
		ce := &ClaudeExecutor{}
		ce.Init(info.Channel)
		return ce.TransformResponse(ctx, resp, info, writer)
	}

	// Standard OpenAI response handling (delegate to openai executor).
	oe := &OpenAIExecutor{}
	oe.Init(info.Channel)
	return oe.TransformResponse(ctx, resp, info, writer)
}

// ---- DeepSeek-specific helpers ----


// ---- Request format conversion helpers ----

type dsMsg struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// openAIReqToClaudeReq converts an OpenAI Chat request body to Claude Messages format.
func openAIReqToClaudeReq(body []byte) ([]byte, error) {
	var req struct {
		Model       string  `json:"model"`
		Messages    []dsMsg `json:"messages"`
		MaxTokens   int     `json:"max_tokens"`
		Temperature float64 `json:"temperature"`
		Stream      bool    `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	claudeMsgs := make([]map[string]any, 0, len(req.Messages))
	var systemText string
	for _, m := range req.Messages {
		if m.Role == "system" {
			var content string
			json.Unmarshal(m.Content, &content)
			systemText = content
			continue
		}
		var content string
		json.Unmarshal(m.Content, &content)
		claudeMsgs = append(claudeMsgs, map[string]any{"role": m.Role, "content": content})
	}

	result := map[string]any{
		"model":    req.Model,
		"messages": claudeMsgs,
		"stream":   req.Stream,
	}
	if req.MaxTokens > 0 {
		result["max_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		result["temperature"] = req.Temperature
	}
	if systemText != "" {
		result["system"] = systemText
	}

	return json.Marshal(result)
}

// claudeReqToOpenAIReq converts a Claude Messages request body to OpenAI Chat format.
func claudeReqToOpenAIReq(body []byte) ([]byte, error) {
	var req struct {
		Model       string          `json:"model"`
		Messages    json.RawMessage `json:"messages"`
		System      json.RawMessage `json:"system,omitempty"`
		MaxTokens   int             `json:"max_tokens"`
		Temperature float64         `json:"temperature"`
		Stream      bool            `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	// Parse messages as generic array.
	var msgs []map[string]any
	if err := json.Unmarshal(req.Messages, &msgs); err != nil {
		return nil, err
	}

	// Prepend system message from Claude system field.
	if len(req.System) > 0 {
		var sysText string
		if json.Unmarshal(req.System, &sysText) == nil && sysText != "" {
			msgs = append([]map[string]any{{"role": "system", "content": sysText}}, msgs...)
		}
	}

	result := map[string]any{
		"model":    req.Model,
		"messages": msgs,
		"stream":   req.Stream,
	}
	if req.MaxTokens > 0 {
		result["max_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		result["temperature"] = req.Temperature
	}

	return json.Marshal(result)
}
func injectDSThinking(body []byte, info *RequestInfo) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}

	if info.ThinkingDisabled {
		raw["thinking"] = json.RawMessage(`{"type":"disabled"}`)
		delete(raw, "reasoning_effort")
		result, _ := json.Marshal(raw)
		return result
	}

	if info.ThinkingEnabled || info.ReasoningEffort != "" {
		// Only inject if client hasn't set thinking.
		if _, exists := raw["thinking"]; !exists {
			if info.ReasoningEffort != "" {
				effort := mapDSEffort(info.ReasoningEffort)
				raw["thinking"] = json.RawMessage(`{"type":"enabled"}`)
				raw["reasoning_effort"], _ = json.Marshal(effort)
			} else {
				raw["thinking"] = json.RawMessage(`{"type":"enabled"}`)
			}
			result, _ := json.Marshal(raw)
			return result
		}
	}

	return body
}

// mapDSEffort maps common reasoning effort values to DeepSeek values.
func mapDSEffort(effort string) string {
	switch effort {
	case "low", "medium", "minimal":
		return "high"
	case "xhigh", "max":
		return "max"
	default:
		return "high"
	}
}
