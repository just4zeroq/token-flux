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
	Register(model.ProviderGemini, &GeminiExecutor{})
	Register(model.ProviderVertex, &GeminiExecutor{})
	RegisterProtocol(model.ProtocolGemini, &GeminiExecutor{})
}

// GeminiExecutor handles Google Gemini API.
// Uses RPC-style endpoints (:generateContent, :streamGenerateContent) and x-goog-api-key auth.
type GeminiExecutor struct {
	channel *model.Channel
}

func (e *GeminiExecutor) Init(channel *model.Channel) {
	e.channel = channel
}

func (e *GeminiExecutor) NativeFormats() []EndpointCapability {
	return NativeFormat("gemini", model.RelayModeGeminiChat)
}

func (e *GeminiExecutor) GetName() string {
	if e.channel != nil && e.channel.Name != "" {
		return e.channel.Name
	}
	return "Gemini"
}

func (e *GeminiExecutor) GetRequestURL(info *RequestInfo) (string, error) {
	baseURL := ""
	if info.Protocol != nil {
		baseURL = strings.TrimSuffix(info.Protocol.BaseURL, "/")
	}
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}

	modelName := info.ActualModelName
	if modelName == "" {
		modelName = info.Model
	}

	switch info.RelayMode {
	case model.RelayModeChatCompletions, model.RelayModeGeminiChat:
		if info.IsStream {
			return fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", baseURL, modelName), nil
		}
		return fmt.Sprintf("%s/v1beta/models/%s:generateContent", baseURL, modelName), nil
	case model.RelayModeEmbeddings:
		return fmt.Sprintf("%s/v1beta/models/%s:embedContent", baseURL, modelName), nil
	default:
		return fmt.Sprintf("%s/v1beta/models/%s:generateContent", baseURL, modelName), nil
	}
}

func (e *GeminiExecutor) SetupRequestHeader(header http.Header, info *RequestInfo) error {
	header.Set("x-goog-api-key", info.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return nil
}

func (e *GeminiExecutor) TransformRequest(ctx context.Context, info *RequestInfo, requestBody []byte) (io.Reader, error) {
	// Gemini native → passthrough (strip stream field).
	if shouldPassthrough(info, "gemini") {
		body := stripStreamField(requestBody)
		return bytes.NewReader(body), nil
	}

	// Convert from OpenAI/Claude → Gemini via translator.
	var tf translator.Format
	switch info.InboundFormat {
	case "openai":
		tf = translator.FormatOpenAI
	case "claude":
		tf = translator.FormatClaude
	case "openai_responses":
		tf = translator.FormatOpenAIResponses
	default:
		tf = translator.FormatOpenAI
	}

	// Normalize to OpenAI first.
	normalized, err := translator.Normalize(bodyFromBytes(requestBody), tf)
	if err != nil {
		log.Printf("[executor:gemini] normalize to openai error: %v", err)
		normalized = bodyFromBytes(requestBody)
	}

	// Then convert OpenAI → Gemini format.
	geminiBody, err := convertOpenAIToGemini(normalized, info)
	if err != nil {
		log.Printf("[executor:gemini] convert to gemini error: %v", err)
		return bytes.NewReader(normalized), nil
	}

	return bytes.NewReader(geminiBody), nil
}

func (e *GeminiExecutor) DoRequest(ctx context.Context, info *RequestInfo, requestBody io.Reader) (*http.Response, error) {
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

func (e *GeminiExecutor) TransformResponse(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	// Streaming + ClientFormat denormalize.
	if info.IsStream && info.ClientFormat != "" && info.ClientFormat != "gemini" && info.ClientFormat != info.InboundFormat {
		return e.handleStreamToClientFormat(ctx, resp, info, writer)
	}

	// Native Gemini format streaming passthrough.
	if info.IsStream {
		return e.handleStream(ctx, resp, info, writer)
	}

	// Non-streaming ClientFormat denormalize.
	if info.ClientFormat != "" && info.ClientFormat != "gemini" && info.ClientFormat != info.InboundFormat {
		return e.denormalizeToClientFormat(ctx, resp, info, writer)
	}

	// Native non-streaming.
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

	return extractGeminiUsage(body), nil
}

// handleStream reads Gemini SSE stream and passthroughs raw data events.
func (e *GeminiExecutor) handleStream(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
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

	scanner := NewSSEScanner(resp.Body)
	var totalUsage *Usage

	for scanner.Scan() {
		event := scanner.Event()
		if event == nil {
			continue
		}

		// Track usage from final data.
		if bytes.Contains(event.Data, []byte("usageMetadata")) {
			u := extractGeminiUsage(event.Data)
			if u != nil {
				totalUsage = u
			}
		}

		_, _ = writer.Write(event.Raw)
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[executor:gemini] stream scan error: %v", err)
	}

	return totalUsage, nil
}

// handleStreamToClientFormat converts Gemini SSE stream to OpenAI SSE chunks
// (optionally denormalized to ClientFormat via Denormalizer).
func (e *GeminiExecutor) handleStreamToClientFormat(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(body))
	}

	flusher, ok := writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported by response writer")
	}

	// Resolve Denormalizer for ClientFormat.
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

	for scanner.Scan() {
		event := scanner.Event()
		if event == nil {
			continue
		}

		// Parse Gemini response.
		var gr struct {
			Candidates []struct {
				Content struct {
					Role  string `json:"role"`
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
				FinishReason string `json:"finishReason"`
			} `json:"candidates"`
			UsageMetadata *struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
				TotalTokenCount      int `json:"totalTokenCount"`
			} `json:"usageMetadata"`
		}
		if err := json.Unmarshal(event.Data, &gr); err != nil {
			continue
		}

		// Extract text from first candidate.
		content := ""
		if len(gr.Candidates) > 0 {
			for _, p := range gr.Candidates[0].Content.Parts {
				content += p.Text
			}
		}

		// OpenAI role chunk on first non-empty content.
		if content != "" {
			chunk := map[string]any{
				"choices": []map[string]any{{"index": 0, "delta": map[string]string{"content": content}}},
			}
			writeStreamChunk(writer, flusher, chunk, dn)
		}

		// Finish reason.
		if len(gr.Candidates) > 0 && gr.Candidates[0].FinishReason != "" {
			fr := "stop"
			switch gr.Candidates[0].FinishReason {
			case "MAX_TOKENS":
				fr = "length"
			case "SAFETY", "BLOCKLIST":
				fr = "content_filter"
			}
			chunk := map[string]any{
				"choices": []map[string]any{{"index": 0, "delta": map[string]any{}, "finish_reason": fr}},
			}
			writeStreamChunk(writer, flusher, chunk, dn)
		}

		// Usage.
		if gr.UsageMetadata != nil {
			um := gr.UsageMetadata
			usageMap := map[string]any{
				"prompt_tokens":     um.PromptTokenCount,
				"completion_tokens": um.CandidatesTokenCount,
				"total_tokens":      um.TotalTokenCount,
			}
			totalUsage = &Usage{
				PromptTokens:     um.PromptTokenCount,
				CompletionTokens: um.CandidatesTokenCount,
				TotalTokens:      um.TotalTokenCount,
			}
			usageChunk := map[string]any{"choices": []map[string]any{}, "usage": usageMap}
			writeStreamChunk(writer, flusher, usageChunk, dn)
		}
	}

	// [DONE] marker for OpenAI-compatible streams.
	_, _ = writer.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()

	// Finalize.
	final, ferr := dn.Finalize()
	if dn != nil && ferr == nil && len(final) > 0 {
		_, _ = writer.Write(final)
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[executor:gemini] stream scan error: %v", err)
	}

	return totalUsage, nil
}

func (e *GeminiExecutor) denormalizeToOpenAI(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		writeJSON(writer, resp.StatusCode, body)
		return &Usage{}, nil
	}

	// Convert Gemini response to OpenAI Chat.
	oaiBody, err := geminiResponseToOpenAI(body)
	if err != nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(body)
		return extractGeminiUsage(body), nil
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(oaiBody)

	return extractGeminiUsage(body), nil
}

func (e *GeminiExecutor) denormalizeToClientFormat(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		writeJSON(writer, resp.StatusCode, body)
		return &Usage{}, nil
	}

	// Step 1: Convert Gemini response to OpenAI Chat.
	oaiBody, err := geminiResponseToOpenAI(body)
	if err != nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(body)
		return extractGeminiUsage(body), nil
	}

	// Step 2: Denormalize OpenAI Chat to ClientFormat.
	dn, derr := translator.Denormalize(oaiBody, resolveFormat(info.ClientFormat), false)
	if derr != nil || dn == nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(oaiBody)
		return extractGeminiUsage(body), nil
	}

	out, cerr := dn.ConvertBody(oaiBody)
	if cerr != nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(oaiBody)
		return extractGeminiUsage(body), nil
	}

	writer.Header().Set("Content-Type", dn.Header())
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(out)

	return extractGeminiUsage(body), nil
}

// ---- Helpers ----

func stripStreamField(body []byte) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}
	delete(raw, "stream")
	result, _ := json.Marshal(raw)
	return result
}

// convertOpenAIToGemini performs basic OpenAI→Gemini request conversion.
// This is a simplified version of server's gemini converter.
func convertOpenAIToGemini(body []byte, info *RequestInfo) ([]byte, error) {
	var openaiReq struct {
		Model       string          `json:"model"`
		Messages    []openaiMsg     `json:"messages"`
		MaxTokens   *int            `json:"max_tokens"`
		Temperature *float64        `json:"temperature"`
		TopP        *float64        `json:"top_p"`
		Stream      bool            `json:"stream"`
		Tools       json.RawMessage `json:"tools,omitempty"`
	}
	if err := json.Unmarshal(body, &openaiReq); err != nil {
		return body, nil // passthrough on parse failure
	}

	geminiReq := map[string]any{
		"contents": convertMessagesToGemini(openaiReq.Messages),
	}

	genConfig := map[string]any{}
	if openaiReq.MaxTokens != nil {
		genConfig["max_output_tokens"] = *openaiReq.MaxTokens
	}
	if openaiReq.Temperature != nil {
		genConfig["temperature"] = *openaiReq.Temperature
	}
	if openaiReq.TopP != nil {
		genConfig["top_p"] = *openaiReq.TopP
	}
	if len(genConfig) > 0 {
		geminiReq["generationConfig"] = genConfig
	}

	if len(openaiReq.Tools) > 0 {
		geminiReq["tools"] = json.RawMessage(fmt.Sprintf(`[{"function_declarations":%s}]`, string(openaiReq.Tools)))
	}

	return json.Marshal(geminiReq)
}

type openaiMsg struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

func convertMessagesToGemini(msgs []openaiMsg) []map[string]any {
	var contents []map[string]any
	var systemParts []string

	for _, msg := range msgs {
		switch msg.Role {
		case "system":
			systemParts = append(systemParts, string(msg.Content))
		case "user":
			contents = append(contents, map[string]any{
				"role": "user",
				"parts": []map[string]any{
					{"text": string(msg.Content)},
				},
			})
		case "assistant":
			contents = append(contents, map[string]any{
				"role": "model",
				"parts": []map[string]any{
					{"text": string(msg.Content)},
				},
			})
		}
	}

	// Prepend system as first user message if present (Gemini has no system role).
	if len(systemParts) > 0 {
		contents = append([]map[string]any{{
			"role": "user",
			"parts": []map[string]any{
				{"text": "System instruction: " + strings.Join(systemParts, "\n")},
			},
		}}, contents...)
	}

	return contents
}

// geminiResponseToOpenAI converts a Gemini API response into OpenAI Chat format.
func geminiResponseToOpenAI(body []byte) ([]byte, error) {
	var gr struct {
		Candidates []struct {
			Content struct {
				Role  string `json:"role"`
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata *struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &gr); err != nil {
		return nil, err
	}

	content := ""
	if len(gr.Candidates) > 0 {
		for _, p := range gr.Candidates[0].Content.Parts {
			content += p.Text
		}
	}

	finishReason := "stop"
	if len(gr.Candidates) > 0 {
		switch gr.Candidates[0].FinishReason {
		case "MAX_TOKENS":
			finishReason = "length"
		case "SAFETY", "BLOCKLIST":
			finishReason = "content_filter"
		}
	}

	resp := map[string]any{
		"id":      "chatcmpl-gemini",
		"object":  "chat.completion",
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": content,
			},
			"finish_reason": finishReason,
		}},
	}
	if gr.UsageMetadata != nil {
		resp["usage"] = map[string]any{
			"prompt_tokens":     gr.UsageMetadata.PromptTokenCount,
			"completion_tokens": gr.UsageMetadata.CandidatesTokenCount,
			"total_tokens":      gr.UsageMetadata.TotalTokenCount,
		}
	}

	return json.Marshal(resp)
}

func extractGeminiUsage(body []byte) *Usage {
	var resp struct {
		UsageMetadata *struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}
	if resp.UsageMetadata == nil {
		return nil
	}
	return &Usage{
		PromptTokens:     resp.UsageMetadata.PromptTokenCount,
		CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
		TotalTokens:      resp.UsageMetadata.TotalTokenCount,
	}
}
