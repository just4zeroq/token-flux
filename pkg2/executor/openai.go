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
	Register(model.ProviderOpenAI, &OpenAIExecutor{})
	Register(model.ProviderAzure, &OpenAIExecutor{})
	Register(model.ProviderAI360, &OpenAIExecutor{})
	Register(model.ProviderLingyi, &OpenAIExecutor{})
	Register(model.ProviderOpenRouter, &OpenAIExecutor{})
	Register(model.ProviderXInference, &OpenAIExecutor{})
	Register(model.ProviderOllama, &OpenAIExecutor{})
	Register(model.ProviderSiliconFlow, &OpenAIExecutor{})
	Register(model.ProviderSubmodel, &OpenAIExecutor{})
	RegisterProtocol(model.ProtocolOpenAI, &OpenAIExecutor{})
}

// OpenAIExecutor handles OpenAI-compatible providers.
// Covers: OpenAI, DeepSeek, Zhipu, Moonshot, Volcengine, Mistral, xAI,
// AI360, Lingyi, OpenRouter, XInference, Ollama, SiliconFlow, Submodel.
type OpenAIExecutor struct {
	channel *model.Channel
}

func (e *OpenAIExecutor) Init(channel *model.Channel) {
	e.channel = channel
}

func (e *OpenAIExecutor) NativeFormats() []EndpointCapability {
	return NativeFormat("openai", model.RelayModeChatCompletions)
}

func (e *OpenAIExecutor) GetName() string {
	if e.channel != nil && e.channel.Name != "" {
		return e.channel.Name
	}
	return "OpenAI"
}

func (e *OpenAIExecutor) GetRequestURL(info *RequestInfo) (string, error) {
	baseURL := ""
	if info.Protocol != nil {
		baseURL = strings.TrimSuffix(info.Protocol.BaseURL, "/")
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	switch info.RelayMode {
	case model.RelayModeChatCompletions:
		return baseURL + "/v1/chat/completions", nil
	case model.RelayModeCompletions:
		return baseURL + "/v1/completions", nil
	case model.RelayModeEmbeddings:
		return baseURL + "/v1/embeddings", nil
	case model.RelayModeClaudeMessages:
		return baseURL + "/v1/chat/completions", nil
	case model.RelayModeResponses:
		return baseURL + "/v1/responses", nil
	default:
		return baseURL + "/v1/chat/completions", nil
	}
}

func (e *OpenAIExecutor) SetupRequestHeader(header http.Header, info *RequestInfo) error {
	header.Set("Authorization", "Bearer "+info.ApiKey)
	header.Set("Content-Type", "application/json")
	if info.IsStream {
		header.Set("Accept", "text/event-stream")
	} else {
		header.Set("Accept", "application/json")
	}
	return nil
}

func (e *OpenAIExecutor) TransformRequest(ctx context.Context, info *RequestInfo, requestBody []byte) (io.Reader, error) {
	var body []byte

	if shouldPassthrough(info, "openai") {
		body = bodyFromBytes(requestBody)
	} else {
		// Non-OpenAI format → convert to OpenAI.
		var tf translator.Format
		switch info.InboundFormat {
		case "claude":
			tf = translator.FormatClaude
		case "gemini":
			tf = translator.FormatGemini
		case "openai_responses":
			tf = translator.FormatOpenAIResponses
		default:
			tf = translator.FormatOpenAI
		}
		converted, err := translator.Normalize(bodyFromBytes(requestBody), tf)
		if err != nil {
			log.Printf("[executor:openai] normalize error: %v, using original body", err)
			body = bodyFromBytes(requestBody)
		} else {
			body = converted
		}
	}

	// Model name mapping.
	if info.Protocol != nil && info.Protocol.IsModelMapped && info.Protocol.UpstreamModelName != "" {
		body = replaceModelField(body, info.Protocol.UpstreamModelName)
	}

	// Stream options injection (for usage tracking in streaming responses).
	if info.IsStream {
		body = injectStreamOptionsOpenAI(body)
	}

	// Reasoning effort injection (from -high, -low suffix routing).
	if info.ReasoningEffort != "" {
		body = injectReasoningEffortOpenAI(body, info.ReasoningEffort)
	}

	return bytes.NewReader(body), nil
}

func (e *OpenAIExecutor) DoRequest(ctx context.Context, info *RequestInfo, requestBody io.Reader) (*http.Response, error) {
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

	client := &http.Client{
		Timeout: secondsAsDuration(timeout),
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	return resp, nil
}

func (e *OpenAIExecutor) TransformResponse(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	isStream := info.IsStream

	// Denormalize if client expects non-OpenAI format.
	if info.ClientFormat != "" && info.ClientFormat != "openai" && info.ClientFormat != info.InboundFormat {
		dn, err := translator.Denormalize(nil, resolveFormat(info.ClientFormat), isStream)
		if err == nil && dn != nil {
			return e.writeDenormalized(ctx, resp, info, writer, dn)
		}
	}

	// Native OpenAI format.
	if isStream {
		return e.handleChatStream(ctx, resp, info, writer)
	}
	return e.handleChatNonStream(ctx, resp, info, writer)
}

// ---- Response handlers ----

func (e *OpenAIExecutor) handleChatNonStream(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if isOpenAIError(body) {
			writeJSON(writer, resp.StatusCode, body)
			return &Usage{}, nil
		}
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(body))
	}

	// If model was mapped, restore original model name in response.
	if info.Protocol != nil && info.Protocol.IsModelMapped {
		body = replaceModelField(body, info.Model)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(body)

	return extractUsage(body), nil
}

func (e *OpenAIExecutor) handleChatStream(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(body))
	}

	flusher, ok := writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported by response writer")
	}

	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(http.StatusOK)

	var totalUsage *Usage
	scanner := NewSSEScanner(resp.Body)

	for scanner.Scan() {
		event := scanner.Event()

		if event == nil {
			continue
		}

		// [DONE] signal.
		if bytes.Equal(event.Data, doneBytes) {
			_, _ = writer.Write(event.Raw)
			flusher.Flush()
			break
		}

		// Usage chunk (stream_options: {include_usage: true}).
		if bytes.Contains(event.Data, []byte(`"usage"`)) {
			u := extractStreamUsage(event.Data)
			if u != nil {
				totalUsage = u
			}
		}

		_, _ = writer.Write(event.Raw)
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[executor:openai] stream scan error: %v", err)
	}

	return totalUsage, nil
}

func (e *OpenAIExecutor) writeDenormalized(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter, dn translator.Denormalizer) (*Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		writeJSON(writer, resp.StatusCode, body)
		return &Usage{}, nil
	}

	out, err := dn.ConvertBody(body)
	if err != nil {
		return nil, fmt.Errorf("denormalize: %w", err)
	}

	writer.Header().Set("Content-Type", dn.Header())
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(out)

	return extractUsage(body), nil
}

// ---- Helpers ----

var doneBytes = []byte("[DONE]")

func bodyFromBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func replaceModelField(body []byte, modelName string) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}
	raw["model"] = json.RawMessage(`"` + modelName + `"`)
	result, _ := json.Marshal(raw)
	return result
}

func injectStreamOptionsOpenAI(body []byte) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}
	if _, exists := raw["stream_options"]; !exists {
		raw["stream_options"] = json.RawMessage(`{"include_usage":true}`)
		result, _ := json.Marshal(raw)
		return result
	}
	return body
}

func injectReasoningEffortOpenAI(body []byte, effort string) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}
	if _, exists := raw["reasoning_effort"]; !exists {
		raw["reasoning_effort"], _ = json.Marshal(effort)
		result, _ := json.Marshal(raw)
		return result
	}
	return body
}

func isOpenAIError(body []byte) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return false
	}
	_, has := raw["error"]
	return has
}

func writeJSON(writer http.ResponseWriter, statusCode int, body []byte) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	_, _ = writer.Write(body)
}

func resolveFormat(f translator.Format) translator.Format {
	return f
}
