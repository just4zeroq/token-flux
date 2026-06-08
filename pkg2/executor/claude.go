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
	Register(model.ProviderClaude, &ClaudeExecutor{})
	RegisterProtocol(model.ProtocolClaude, &ClaudeExecutor{})
}

// ClaudeExecutor handles Anthropic Claude Messages API.
// Covers: Claude, anthropic-compatible providers.
type ClaudeExecutor struct {
	channel *model.Channel
}

func (e *ClaudeExecutor) Init(channel *model.Channel) {
	e.channel = channel
}

func (e *ClaudeExecutor) NativeFormats() []EndpointCapability {
	return NativeFormat("claude", model.RelayModeClaudeMessages)
}

func (e *ClaudeExecutor) GetName() string {
	if e.channel != nil && e.channel.Name != "" {
		return e.channel.Name
	}
	return "Claude"
}

func (e *ClaudeExecutor) GetRequestURL(info *RequestInfo) (string, error) {
	baseURL := ""
	if info.Protocol != nil {
		baseURL = strings.TrimSuffix(info.Protocol.BaseURL, "/")
	}
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	switch info.RelayMode {
	case model.RelayModeChatCompletions, model.RelayModeClaudeMessages:
		return baseURL + "/v1/messages", nil
	default:
		return baseURL + "/v1/messages", nil
	}
}

func (e *ClaudeExecutor) SetupRequestHeader(header http.Header, info *RequestInfo) error {
	header.Set("x-api-key", info.ApiKey)
	header.Set("anthropic-version", "2023-06-01")
	header.Set("Content-Type", "application/json")
	if info.IsStream {
		header.Set("Accept", "text/event-stream")
	} else {
		header.Set("Accept", "application/json")
	}
	return nil
}

func (e *ClaudeExecutor) TransformRequest(ctx context.Context, info *RequestInfo, requestBody []byte) (io.Reader, error) {
	body := bodyFromBytes(requestBody)

	// Only convert if not native Claude format.
	if !shouldPassthrough(info, "claude") {
		dn, err := translator.Denormalize(body, translator.FormatClaude, info.IsStream)
		if err == nil && dn != nil {
			converted, err := dn.ConvertBody(body)
			if err == nil {
				body = converted
			}
		}
	}

	// Model name mapping.
	if info.Protocol != nil && info.Protocol.IsModelMapped && info.Protocol.UpstreamModelName != "" {
		body = replaceModelField(body, info.Protocol.UpstreamModelName)
	}

	return bytes.NewReader(body), nil
}

func (e *ClaudeExecutor) DoRequest(ctx context.Context, info *RequestInfo, requestBody io.Reader) (*http.Response, error) {
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

func (e *ClaudeExecutor) TransformResponse(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	// Streaming + ClientFormat denormalize — convert SSE chunk-by-chunk.
	if info.IsStream && info.ClientFormat != "" && info.ClientFormat != "claude" && info.ClientFormat != info.InboundFormat {
		return e.handleStreamToClientFormat(ctx, resp, info, writer)
	}

	// Native Claude format streaming passthrough.
	if info.IsStream {
		return e.handleStream(ctx, resp, info, writer)
	}

	// Non-streaming ClientFormat denormalize.
	if info.ClientFormat != "" && info.ClientFormat != "claude" && info.ClientFormat != info.InboundFormat {
		return e.denormalizeToClientFormat(ctx, resp, info, writer)
	}

	return e.handleNonStream(ctx, resp, info, writer)
}

// handleStreamToClientFormat converts Claude SSE stream → OpenAI SSE chunks
// (optionally denormalized to ClientFormat via Denormalizer).
// Fixes the bug where io.ReadAll(resp.Body) on an SSE stream fails JSON parsing.
func (e *ClaudeExecutor) handleStreamToClientFormat(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(body))
	}

	flusher, ok := writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported by response writer")
	}

	// Resolve Denormalizer for ClientFormat (only used when target != openai).
	targetFmt := resolveFormat(info.ClientFormat)
	dn, dnErr := translator.Denormalize(nil, targetFmt, true)
	if dnErr != nil {
		dn = nil
	}

	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	if dn != nil {
		writer.Header().Set("Content-Type", dn.Header())
	} else {
		writer.Header().Set("Content-Type", "text/event-stream")
	}
	writer.WriteHeader(http.StatusOK)

	scanner := NewSSEScanner(resp.Body)
	var totalUsage *Usage
	var msgID, modelName string
	var promptTokens int

	for scanner.Scan() {
		event := scanner.Event()
		if event == nil {
			continue
		}

		eventType := parseClaudeEventType(event.Raw)

		switch eventType {
		case "message_start":
			var ms struct {
				Message struct {
					ID    string `json:"id"`
					Model string `json:"model"`
					Usage *struct {
						InputTokens int `json:"input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if json.Unmarshal(event.Data, &ms) == nil {
				msgID = ms.Message.ID
				modelName = ms.Message.Model
				if ms.Message.Usage != nil {
					promptTokens = ms.Message.Usage.InputTokens
				}
			}
			// First chunk: delta {role: assistant}.
			chunk := map[string]any{
				"id":      "chatcmpl-" + msgID,
				"object":  "chat.completion.chunk",
				"model":   modelName,
				"choices": []map[string]any{{"index": 0, "delta": map[string]string{"role": "assistant"}, "finish_reason": nil}},
			}
			writeStreamChunk(writer, flusher, chunk, dn)

		case "content_block_delta":
			var d struct {
				Delta *struct {
					Type        string `json:"type"`
					Text        string `json:"text"`
					PartialJSON string `json:"partial_json,omitempty"`
				} `json:"delta"`
			}
			if json.Unmarshal(event.Data, &d) != nil || d.Delta == nil {
				continue
			}
			if d.Delta.Type != "text_delta" || d.Delta.Text == "" {
				continue
			}
			chunk := map[string]any{
				"choices": []map[string]any{{"index": 0, "delta": map[string]string{"content": d.Delta.Text}}},
			}
			writeStreamChunk(writer, flusher, chunk, dn)

		case "message_delta":
			var md struct {
				Delta *struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
				Usage *struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if json.Unmarshal(event.Data, &md) != nil {
				continue
			}

			finishReason := "stop"
			if md.Delta != nil {
				switch md.Delta.StopReason {
				case "max_tokens":
					finishReason = "length"
				case "end_turn", "stop_sequence":
					finishReason = "stop"
				}
			}

			var usageMap map[string]any
			if md.Usage != nil {
				usageMap = map[string]any{
					"prompt_tokens":     promptTokens,
					"completion_tokens": md.Usage.OutputTokens,
					"total_tokens":      promptTokens + md.Usage.OutputTokens,
				}
				totalUsage = &Usage{
					PromptTokens:     promptTokens,
					CompletionTokens: md.Usage.OutputTokens,
					TotalTokens:      promptTokens + md.Usage.OutputTokens,
				}
				// Emit usage as separate chunk (OpenAI pattern).
				usageChunk := map[string]any{"choices": []map[string]any{}, "usage": usageMap}
				writeStreamChunk(writer, flusher, usageChunk, dn)
			}

			// Final delta with finish_reason.
			chunk := map[string]any{
				"choices": []map[string]any{{"index": 0, "delta": map[string]any{}, "finish_reason": finishReason}},
			}
			writeStreamChunk(writer, flusher, chunk, dn)

		case "message_stop":
			_, _ = writer.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()

		default:
			// ping, content_block_start, content_block_stop → skip
		}
	}

	// Finalize remaining denormalizer data.
	final, ferr := dn.Finalize()
	if dn != nil && ferr == nil && len(final) > 0 {
		_, _ = writer.Write(final)
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[executor:claude] stream scan error: %v", err)
	}

	return totalUsage, nil
}

// parseClaudeEventType extracts "event: xxx" from raw Claude SSE bytes.
func parseClaudeEventType(raw []byte) string {
	prefix := []byte("event: ")
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if bytes.HasPrefix(line, prefix) {
			return string(bytes.TrimSpace(bytes.TrimPrefix(line, prefix)))
		}
	}
	return ""
}

// writeStreamChunk marshals an OpenAI chunk as SSE data frame.
// When dn is non-nil, passes through Denormalizer.ConvertChunk first.
func writeStreamChunk(writer http.ResponseWriter, flusher http.Flusher, chunk map[string]any, dn translator.Denormalizer) {
	data, err := json.Marshal(chunk)
	if err != nil {
		return
	}

	if dn != nil {
		converted, cerr := dn.ConvertChunk(data)
		if cerr == nil && len(converted) > 0 {
			_, _ = writer.Write(converted)
			flusher.Flush()
			return
		}
	}

	_, _ = fmt.Fprintf(writer, "data: %s\n\n", data)
	flusher.Flush()
}

func (e *ClaudeExecutor) handleNonStream(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		writeJSON(writer, resp.StatusCode, body)
		return &Usage{}, nil
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(body)

	return extractClaudeUsage(body), nil
}

func (e *ClaudeExecutor) handleStream(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(body))
	}

	flusher, ok := writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(http.StatusOK)

	scanner := NewSSEScanner(resp.Body)
	var totalUsage *Usage

	for scanner.Scan() {
		event := scanner.Event()
		if event == nil {
			continue
		}

		// Capture usage from message_delta events.
		if bytes.Contains(event.Data, []byte(`"type":"message_delta"`)) {
			u := extractClaudeStreamUsage(event.Data)
			if u != nil {
				totalUsage = u
			}
		}

		_, _ = writer.Write(event.Raw)
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[executor:claude] stream error: %v", err)
	}

	return totalUsage, nil
}

func (e *ClaudeExecutor) denormalizeToOpenAI(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		writeJSON(writer, resp.StatusCode, body)
		return &Usage{}, nil
	}

	// Convert Claude response to OpenAI Chat.
	oaiBody, err := claudeResponseToOpenAI(body)
	if err != nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(body)
		return extractClaudeUsage(body), nil
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(oaiBody)

	return extractClaudeUsage(body), nil
}

func (e *ClaudeExecutor) denormalizeToClientFormat(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		writeJSON(writer, resp.StatusCode, body)
		return &Usage{}, nil
	}

	// Step 1: Convert Claude response to OpenAI Chat.
	oaiBody, err := claudeResponseToOpenAI(body)
	if err != nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(body)
		return extractClaudeUsage(body), nil
	}

	// Step 2: Denormalize OpenAI Chat to ClientFormat.
	dn, derr := translator.Denormalize(oaiBody, resolveFormat(info.ClientFormat), false)
	if derr != nil || dn == nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(oaiBody)
		return extractClaudeUsage(body), nil
	}

	out, cerr := dn.ConvertBody(oaiBody)
	if cerr != nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(oaiBody)
		return extractClaudeUsage(body), nil
	}

	writer.Header().Set("Content-Type", dn.Header())
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(out)

	return extractClaudeUsage(body), nil
}

// ---- Claude-specific extractors ----

// claudeResponseToOpenAI converts a Claude Messages API response into OpenAI Chat format.
func claudeResponseToOpenAI(body []byte) ([]byte, error) {
	var cr struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Type    string `json:"type"`
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &cr); err != nil {
		return nil, err
	}

	content := ""
	for _, block := range cr.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	finishReason := "stop"
	if cr.StopReason == "end_turn" || cr.StopReason == "stop_sequence" {
		finishReason = "stop"
	} else if cr.StopReason == "max_tokens" {
		finishReason = "length"
	}

	resp := map[string]any{
		"id":      "chatcmpl-" + cr.ID,
		"object":  "chat.completion",
		"model":   cr.Model,
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": content,
			},
			"finish_reason": finishReason,
		}},
	}
	if cr.Usage != nil {
		resp["usage"] = map[string]any{
			"prompt_tokens":     cr.Usage.InputTokens,
			"completion_tokens": cr.Usage.OutputTokens,
			"total_tokens":      cr.Usage.InputTokens + cr.Usage.OutputTokens,
		}
	}

	return json.Marshal(resp)
}

func extractClaudeUsage(body []byte) *Usage {
	var resp struct {
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}
	if resp.Usage == nil {
		return nil
	}
	return &Usage{
		PromptTokens:     resp.Usage.InputTokens,
		CompletionTokens: resp.Usage.OutputTokens,
		TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
	}
}

func extractClaudeStreamUsage(data []byte) *Usage {
	var msg struct {
		Type  string `json:"type"`
		Usage *struct {
			InputTokens      int `json:"input_tokens"`
			OutputTokens     int `json:"output_tokens"`
			CacheReadTokens  int `json:"cache_read_input_tokens"`
			CacheWriteTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil
	}
	if msg.Usage == nil {
		return nil
	}
	return &Usage{
		PromptTokens:     msg.Usage.InputTokens,
		CompletionTokens: msg.Usage.OutputTokens,
		TotalTokens:      msg.Usage.InputTokens + msg.Usage.OutputTokens,
		CacheHitTokens:   msg.Usage.CacheReadTokens,
		CacheMissTokens:  msg.Usage.CacheWriteTokens,
	}
}
