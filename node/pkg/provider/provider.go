package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ai-platform/pkg/executor"
	"ai-platform/pkg/model"
	"ai-platform/pkg/translator"
	"ai-platform-node/pkg/keychain"
	"ai-platform-node/pkg/types"
)

// providerName maps model.ProviderType to executor registration name.
func providerName(pt model.ProviderType) string {
	switch pt {
	case model.ProviderOpenAI:
		return "openai"
	case model.ProviderClaude:
		return "claude"
	case model.ProviderDeepSeek:
		return "deepseek"
	case model.ProviderGemini:
		return "gemini"
	default:
		return "openai"
	}
}

// newExecutor creates an executor for the given provider type and initializes it.
func newExecutor(pt model.ProviderType, ch *model.Channel) executor.Executor {
	e := executor.GetByProvider(providerName(pt))
	if e == nil {
		return nil
	}
	e.Init(ch)
	return e
}

// buildRequestInfo fills shared request-scoped fields.
func buildRequestInfo(req *types.RequestEnvelope, entry *types.KeyEntry, binding *types.KeyBinding, channel *model.Channel) *executor.RequestInfo {
	inboundFmt := req.InboundFormat
	if inboundFmt == "" {
		inboundFmt = "openai"
	}
	return &executor.RequestInfo{
		RequestID:       req.RequestID,
		RelayMode:       translator.RelayModeChatCompletions,
		IsStream:        req.Request.Stream,
		Model:           req.Model,
		ActualModelName: binding.UpstreamModelName,
		InboundFormat:   translator.Format(inboundFmt),
		ClientFormat:    translator.Format(req.ClientFormat),
		Channel:         channel,
		ApiKey:          entry.Key,
		BaseURL:         entry.BaseURL,
	}
}

// buildChannel creates a model.Channel from key entry + binding.
func buildChannel(entry *types.KeyEntry, binding *types.KeyBinding) *model.Channel {
	baseURL := entry.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &model.Channel{
		ID:           entry.ChannelID,
		ProviderType: model.ProviderOpenAI,
		Protocols: []model.ProtocolEntry{
			{
				Protocol:          model.ProtocolOpenAI,
				BaseURL:           baseURL,
				UpstreamModelName: binding.UpstreamModelName,
				IsModelMapped:     binding.UpstreamModelName != "",
			},
		},
		Status: "active",
	}
}

// applyFormatPlan runs Plan + ConvertRequest + RequestCustomize.
// Returns the converted body and updates info.InboundFormat/RelayMode.
func applyFormatPlan(body []byte, info *executor.RequestInfo, e executor.Executor) ([]byte, error) {
	inboundFmt := info.InboundFormat
	clientFmt := info.ClientFormat
	if clientFmt == "" {
		clientFmt = inboundFmt
	}
	capabilities := e.NativeFormats()
	plan := executor.Plan(inboundFmt, clientFmt, capabilities)

	var err error
	result := body
	if plan.NeedRequestConv {
		result, err = e.ConvertRequest(body, inboundFmt, plan.UpstreamFormat)
		if err != nil {
			return nil, fmt.Errorf("request conv %s→%s: %w", inboundFmt, plan.UpstreamFormat, err)
		}
	}

	info.InboundFormat = plan.UpstreamFormat
	info.RelayMode = plan.UpstreamRelayMode

	result = e.RequestCustomize(result, info)
	return result, nil
}

// ExecuteWithWriter handles an LLM request using the full pkg/executor pipeline.
// Reads the request body, resolves the key, builds channel/executor, calls
// Plan→ConvertRequest→RequestCustomize→DoRequest→ResponseCustomize→ConvertResponse,
// and writes the response to w.
// Returns translator.Usage for the caller to track.
func ExecuteWithWriter(ctx context.Context, body []byte, keyHash string, info *executor.RequestInfo, w http.ResponseWriter) (*translator.Usage, error) {
	start := time.Now()

	entry, binding, err := keychain.FindKeyByHash(keyHash)
	if err != nil {
		return nil, fmt.Errorf("resolve key: %w", err)
	}

	baseURL := entry.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	ch := &model.Channel{
		ID:           entry.ChannelID,
		ProviderType: model.ProviderOpenAI,
		Protocols: []model.ProtocolEntry{
			{
				Protocol:          model.ProtocolOpenAI,
				BaseURL:           baseURL,
				UpstreamModelName: binding.UpstreamModelName,
				IsModelMapped:     binding.UpstreamModelName != "",
			},
		},
		Status: "active",
	}

	e := newExecutor(ch.ProviderType, ch)
	if e == nil {
		return nil, fmt.Errorf("no executor for provider type %d", ch.ProviderType)
	}

	// Fill in channel info.
	info.Channel = ch
	info.ApiKey = entry.Key
	info.ActualModelName = binding.UpstreamModelName
	info.BaseURL = baseURL

	// Plan + ConvertRequest + RequestCustomize.
	convertedBody, err := applyFormatPlan(body, info, e)
	if err != nil {
		return nil, fmt.Errorf("apply format plan: %w", err)
	}

	// Build URL.
	reqURL, err := e.GetRequestURL(info)
	if err != nil {
		return nil, fmt.Errorf("get request URL: %w", err)
	}

	// Build HTTP request.
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(string(convertedBody)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if err := e.SetupRequestHeader(httpReq.Header, info); err != nil {
		return nil, fmt.Errorf("setup header: %w", err)
	}

	// Send.
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Determine client format for response conversion.
	clientFmt := info.ClientFormat
	if clientFmt == "" {
		clientFmt = info.InboundFormat // post-plan (upstream format)
	} else {
		// original client format was saved before plan
		clientFmt = info.ClientFormat
	}

	// Handle streaming vs non-streaming.
	if info.IsStream {
		return handleStreamResponse(ctx, resp, info, e, clientFmt, w, start)
	}
	return handleNonStreamResponse(ctx, resp, info, e, clientFmt, w, start)
}

func handleStreamResponse(ctx context.Context, resp *http.Response, info *executor.RequestInfo, e executor.Executor, clientFmt translator.Format, w http.ResponseWriter, start time.Time) (*translator.Usage, error) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(body))
	}

	upstreamFmt := info.InboundFormat
	streamConv, err := e.NewResponseStream(upstreamFmt, clientFmt)
	if err != nil || streamConv == nil {
		// Passthrough.
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.Copy(w, resp.Body)
		return nil, nil
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	flusher, canFlush := w.(http.Flusher)

	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])

			// Response customization.
			chunk = e.ResponseCustomize(chunk, info)

			converted, convErr := streamConv.Feed(chunk)
			if convErr == nil && len(converted) > 0 {
				w.Write(converted)
				if canFlush {
					flusher.Flush()
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
	}

	tail, _ := streamConv.End()
	if len(tail) > 0 {
		w.Write(tail)
		if canFlush {
			flusher.Flush()
		}
	}

	u := streamConv.Usage()
	if u == nil {
		return nil, nil
	}
	return u, nil
}

func handleNonStreamResponse(ctx context.Context, resp *http.Response, info *executor.RequestInfo, e executor.Executor, clientFmt translator.Format, w http.ResponseWriter, start time.Time) (*translator.Usage, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(body))
	}

	// Response customization.
	converted := e.ResponseCustomize(body, info)

	// Format conversion.
	upstreamFmt := info.InboundFormat
	if upstreamFmt != clientFmt {
		converted, err = e.ConvertResponse(converted, upstreamFmt, clientFmt)
		if err != nil {
			// Fallback: send unconverted.
			converted = body
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(converted)

	// Extract usage.
	var usageResp struct {
		Usage *translator.Usage `json:"usage"`
	}
	if err := json.Unmarshal(converted, &usageResp); err == nil && usageResp.Usage != nil {
		return usageResp.Usage, nil
	}

	latency := time.Since(start).Milliseconds()
	_ = latency
	return nil, nil
}

// estimateCost computes approximate cost in millicents (1/1000 cent).
func estimateCost(u *translator.Usage) int64 {
	if u == nil {
		return 0
	}
	// Rough: $0.15/M input tokens, $0.60/M output tokens
	// → millicents per token: input=0.015, output=0.06
	input := int64(float64(u.PromptTokens) * 0.015)
	output := int64(float64(u.CompletionTokens) * 0.06)
	return input + output
}

// PreparedRequest holds a resolved executor and request ready to execute.
type PreparedRequest struct {
	Entry   *types.KeyEntry
	Binding *types.KeyBinding
	Channel *model.Channel
	Exec    executor.Executor
	Info    *executor.RequestInfo
}

// PrepareRequest resolves a key hash and builds channel + executor.
func PrepareRequest(ctx context.Context, keyHash string, info *executor.RequestInfo) (*PreparedRequest, error) {
	entry, binding, err := keychain.FindKeyByHash(keyHash)
	if err != nil {
		return nil, fmt.Errorf("resolve key: %w", err)
	}

	baseURL := entry.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	ch := &model.Channel{
		ID:           entry.ChannelID,
		ProviderType: model.ProviderOpenAI,
		Protocols: []model.ProtocolEntry{
			{
				Protocol:          model.ProtocolOpenAI,
				BaseURL:           baseURL,
				UpstreamModelName: binding.UpstreamModelName,
				IsModelMapped:     binding.UpstreamModelName != "",
			},
		},
		Status: "active",
	}

	e := newExecutor(ch.ProviderType, ch)
	if e == nil {
		return nil, fmt.Errorf("no executor for provider type %d", ch.ProviderType)
	}

	info.Channel = ch
	info.ApiKey = entry.Key
	info.ActualModelName = binding.UpstreamModelName
	info.BaseURL = baseURL

	return &PreparedRequest{
		Entry: entry, Binding: binding, Channel: ch,
		Exec: e, Info: info,
	}, nil
}

// DoRaw calls the executor pipeline without writing to the response.
// Returns the HTTP response for caller to inspect status code.
func (p *PreparedRequest) DoRaw(ctx context.Context, body []byte) (*http.Response, error) {
	convertedBody, err := applyFormatPlan(body, p.Info, p.Exec)
	if err != nil {
		return nil, fmt.Errorf("apply format plan: %w", err)
	}

	reqURL, err := p.Exec.GetRequestURL(p.Info)
	if err != nil {
		return nil, fmt.Errorf("get request URL: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(string(convertedBody)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if err := p.Exec.SetupRequestHeader(httpReq.Header, p.Info); err != nil {
		return nil, fmt.Errorf("setup header: %w", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	return client.Do(httpReq)
}

// WriteResponse processes the upstream response using format conversion.
func (p *PreparedRequest) WriteResponse(ctx context.Context, resp *http.Response, w http.ResponseWriter) (*translator.Usage, error) {
	clientFmt := p.Info.ClientFormat
	if clientFmt == "" {
		return nil, nil
	}
	return handleNonStreamResponse(ctx, resp, p.Info, p.Exec, clientFmt, w, time.Now())
}

// Usage captures token usage and cost from an LLM call.
// Returned alongside ChatResponse for the node's own tracking.
type Usage struct {
	InputTokens  int   `json:"input_tokens"`
	OutputTokens int   `json:"output_tokens"`
	TotalTokens  int   `json:"total_tokens"`
	CostCredits  int64 `json:"cost_credits"`
	LatencyMs    int64 `json:"latency_ms"`
}

// Execute forwards a chat request to the upstream provider using pkg/executor.
// Returns the upstream response in OpenAI Chat format.
func Execute(ctx context.Context, req *types.RequestEnvelope, keyHash string) (*types.ChatResponse, error) {
	start := time.Now()

	entry, binding, err := keychain.FindKeyByHash(keyHash)
	if err != nil {
		return nil, fmt.Errorf("resolve key: %w", err)
	}

	ch := buildChannel(entry, binding)
	e := newExecutor(ch.ProviderType, ch)
	if e == nil {
		return nil, fmt.Errorf("no executor for provider type %d", ch.ProviderType)
	}

	info := buildRequestInfo(req, entry, binding, ch)
	body, err := json.Marshal(req.Request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Format plan + convert + customize.
	convertedBody, err := applyFormatPlan(body, info, e)
	if err != nil {
		return nil, fmt.Errorf("apply format plan: %w", err)
	}

	reqURL, err := e.GetRequestURL(info)
	if err != nil {
		return nil, fmt.Errorf("get request URL: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(string(convertedBody)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if err := e.SetupRequestHeader(httpReq.Header, info); err != nil {
		return nil, fmt.Errorf("setup header: %w", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(b))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Response customization.
	respBody = e.ResponseCustomize(respBody, info)

	// Convert back to client format if needed.
	clientFmt := info.ClientFormat
	upstreamFmt := info.InboundFormat
	if clientFmt != "" && upstreamFmt != clientFmt {
		converted, convErr := e.ConvertResponse(respBody, upstreamFmt, clientFmt)
		if convErr == nil {
			respBody = converted
		}
	}

	var cr types.ChatResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	_ = time.Since(start).Milliseconds()
	return &cr, nil
}

// ExecuteStream forwards a streaming request using pkg/executor.
func ExecuteStream(ctx context.Context, req *types.RequestEnvelope, keyHash string) (<-chan *types.ChunkData, <-chan error) {
	chunks := make(chan *types.ChunkData, 64)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		entry, binding, err := keychain.FindKeyByHash(keyHash)
		if err != nil {
			errs <- err
			return
		}

		ch := buildChannel(entry, binding)
		e := newExecutor(ch.ProviderType, ch)
		if e == nil {
			errs <- fmt.Errorf("no executor for provider type %d", ch.ProviderType)
			return
		}

		req.Request.Stream = true
		info := buildRequestInfo(req, entry, binding, ch)
		info.IsStream = true

		body, err := json.Marshal(req.Request)
		if err != nil {
			errs <- err
			return
		}

		// Format plan + convert + customize.
		convertedBody, err := applyFormatPlan(body, info, e)
		if err != nil {
			errs <- fmt.Errorf("apply format plan: %w", err)
			return
		}

		reqURL, err := e.GetRequestURL(info)
		if err != nil {
			errs <- fmt.Errorf("get request URL: %w", err)
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(string(convertedBody)))
		if err != nil {
			errs <- fmt.Errorf("create request: %w", err)
			return
		}
		if err := e.SetupRequestHeader(httpReq.Header, info); err != nil {
			errs <- fmt.Errorf("setup header: %w", err)
			return
		}

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			errs <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			errs <- fmt.Errorf("upstream %d: %s", resp.StatusCode, string(b))
			return
		}

		// Use ResponseStream for format conversion if needed.
		clientFmt := info.ClientFormat
		upstreamFmt := info.InboundFormat
		var streamConv executor.ResponseStream
		if clientFmt != "" && upstreamFmt != clientFmt && clientFmt != upstreamFmt {
			streamConv, _ = e.NewResponseStream(upstreamFmt, clientFmt)
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()

			if streamConv != nil {
				converted, convErr := streamConv.Feed(line)
				if convErr != nil || len(converted) == 0 {
					continue
				}
				line = converted
			}

			if !strings.HasPrefix(string(line), "data: ") {
				continue
			}
			data := strings.TrimPrefix(string(line), "data: ")
			if data == "[DONE]" {
				break
			}
			var chunk types.ChunkData
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			select {
			case chunks <- &chunk:
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			}
		}

		if streamConv != nil {
			if tail, _ := streamConv.End(); len(tail) > 0 {
				tailStr := string(tail)
				if strings.HasPrefix(tailStr, "data: ") {
					data := strings.TrimPrefix(tailStr, "data: ")
					if data != "[DONE]" {
						var chunk types.ChunkData
						if json.Unmarshal([]byte(data), &chunk) == nil {
							select {
							case chunks <- &chunk:
							default:
							}
						}
					}
				}
			}
		}
	}()

	return chunks, errs
}
