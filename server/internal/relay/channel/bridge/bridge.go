// Package bridge provides a common.Adaptor wrapper around pkg/executor.Executor.
// Uses the unified Plan→ConvertRequest→Customize→DoRequest→ConvertResponse flow.
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"ai-platform/internal/relay/common"
	"ai-platform/internal/relay/constant"
	pkgexecutor "ai-platform/pkg/executor"
	"ai-platform/pkg/translator"
)

// Adaptor wraps pkg/executor.Executor to implement common.Adaptor.
type Adaptor struct {
	exec pkgexecutor.Executor
	info *common.RelayInfo

	// Cached plan from ConvertRequest, reused in DoResponse.
	plan *pkgexecutor.FormatPlan
}

// New creates a bridge adaptor wrapping the given executor.
func New(e pkgexecutor.Executor) *Adaptor {
	return &Adaptor{exec: e}
}

// Init stores relay info and initializes the inner executor.
func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info

	// Build minimal channel config for executor.
	type channelStruct struct {
		ID   int64
		Name string
	}

	ch := &channelStruct{
		ID:   info.ChannelMeta.ChannelID,
		Name: info.ChannelMeta.ChannelName,
	}

	a.exec.Init(ch)
}

// GetRequestURL delegates to the inner executor.
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	ri := buildRequestInfo(info, a.plan)
	return a.exec.GetRequestURL(ri)
}

// SetupRequestHeader delegates to the inner executor.
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	ri := buildRequestInfo(info, a.plan)
	return a.exec.SetupRequestHeader(header, ri)
}

// ConvertRequest uses FormatPlan to select upstream format, then calls
// executor.ConvertRequest + RequestCustomize in the unified flow.
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	inboundFmt := info.InboundFormat
	clientFmt := info.GetOriginalClientFormat()
	capabilities := a.exec.NativeFormats()

	// FormatPlan: select best upstream endpoint.
	plan := pkgexecutor.Plan(inboundFmt, clientFmt, capabilities)
	a.plan = &plan

	body := requestBody

	// Request format conversion if needed.
	if plan.NeedRequestConv {
		converted, err := a.exec.ConvertRequest(body, inboundFmt, plan.UpstreamFormat)
		if err != nil {
			return nil, fmt.Errorf("bridge: request conv %s→%s: %w", inboundFmt, plan.UpstreamFormat, err)
		}
		body = converted
	}

	// Update relay info for downstream steps.
	info.InboundFormat = plan.UpstreamFormat
	info.RelayMode = relayModeToInt(plan.UpstreamRelayMode)

	// Build RequestInfo for customization.
	ri := buildRequestInfo(info, a.plan)

	// Vendor-specific request customization.
	body = a.exec.RequestCustomize(body, ri)

	return bytes.NewReader(body), nil
}

// DoRequest sends the HTTP request to the upstream provider.
func (a *Adaptor) DoRequest(ctx context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	ri := buildRequestInfo(info, a.plan)

	// Read body for retry compatibility.
	bodyBytes, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, fmt.Errorf("bridge: read body: %w", err)
	}

	return a.exec.DoRequest(ri, bytes.NewReader(bodyBytes))
}

// DoResponse handles the upstream response with format conversion.
// For non-streaming: ReadAll → ResponseCustomize → ConvertResponse.
// For streaming: wrap in ResponseStream.
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	ri := buildRequestInfo(info, a.plan)

	if a.plan == nil || !a.plan.NeedResponseConv {
		// No conversion needed: copy response as-is.
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("bridge: read response: %w", err)
		}
		defer resp.Body.Close()

		// Copy headers.
		for k, v := range resp.Header {
			for _, vv := range v {
				writer.Header().Add(k, vv)
			}
		}
		writer.WriteHeader(resp.StatusCode)
		writer.Write(body)

		// Extract usage if present.
		usage := extractUsageFromBody(body)
		return usage, nil
	}

	clientFmt := info.GetOriginalClientFormat()

	if info.IsStream {
		return a.handleStreamResponse(resp, info, writer, ri, clientFmt)
	}

	// Non-streaming: buffer → customize → convert → write.
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bridge: read response: %w", err)
	}

	// Vendor-specific response customization.
	body = a.exec.ResponseCustomize(body, ri)

	// Response format conversion.
	converted, err := a.exec.ConvertResponse(body, a.plan.UpstreamFormat, clientFmt)
	if err != nil {
		// Fallback: return unconverted body.
		converted = body
	}

	// Write converted response.
	for k, v := range resp.Header {
		for _, vv := range v {
			writer.Header().Add(k, vv)
		}
	}
	writer.WriteHeader(resp.StatusCode)
	writer.Write(converted)

	usage := extractUsageFromBody(converted)
	return usage, nil
}

func (a *Adaptor) handleStreamResponse(resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter, ri *pkgexecutor.RequestInfo, clientFmt translator.Format) (*common.Usage, error) {
	streamConv, err := a.exec.NewResponseStream(a.plan.UpstreamFormat, clientFmt)
	if err != nil || streamConv == nil {
		// Streaming conversion not supported: passthrough.
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		for k, v := range resp.Header {
			for _, vv := range v {
				writer.Header().Add(k, vv)
			}
		}
		writer.WriteHeader(resp.StatusCode)
		writer.Write(body)
		return nil, nil
	}

	// Set SSE content type.
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.WriteHeader(http.StatusOK)

	flusher, canFlush := writer.(http.Flusher)

	// Stream conversion loop.
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])

			// Apply response customization.
			chunk = a.exec.ResponseCustomize(chunk, ri)

			// Stream format conversion.
			converted, convErr := streamConv.Feed(chunk)
			if convErr == nil && len(converted) > 0 {
				writer.Write(converted)
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

	// End of stream.
	tail, _ := streamConv.End()
	if len(tail) > 0 {
		writer.Write(tail)
		if canFlush {
			flusher.Flush()
		}
	}

	usage := streamConv.Usage()
	if usage == nil {
		return &common.Usage{}, nil
	}
	return &common.Usage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}, nil
}

// GetChannelName returns the executor name.
func (a *Adaptor) GetChannelName() string {
	return a.exec.GetName()
}

// ---- mapping helpers ----

func buildRequestInfo(info *common.RelayInfo, plan *pkgexecutor.FormatPlan) *pkgexecutor.RequestInfo {
	ri := &pkgexecutor.RequestInfo{
		RequestID:       info.RequestID,
		IsStream:        info.IsStream,
		Model:           info.OriginModelName,
		ActualModelName: info.ChannelMeta.UpstreamModelName,
		ApiKey:          info.ChannelMeta.ApiKey,
		BaseURL:         info.ChannelMeta.BaseURL,
		InboundFormat:   info.InboundFormat,
		ClientFormat:    info.GetOriginalClientFormat(),
		ThinkingEnabled: info.ThinkingEnabled,
		ThinkingDisabled: info.ThinkingDisabled,
		ReasoningEffort: info.ReasoningEffort,
	}

	if plan != nil {
		ri.RelayMode = plan.UpstreamRelayMode
	}

	return ri
}

func extractUsageFromBody(body []byte) *common.Usage {
	var parsed struct {
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil
	}
	if parsed.Usage == nil {
		return &common.Usage{}
	}
	return &common.Usage{
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
		TotalTokens:      parsed.Usage.TotalTokens,
	}
}

func relayModeToInt(rm translator.RelayMode) int {
	switch rm {
	case translator.RelayModeChatCompletions:
		return int(constant.RelayModeChatCompletions)
	case translator.RelayModeClaudeMessages:
		return int(constant.RelayModeClaudeMessages)
	case translator.RelayModeResponses:
		return int(constant.RelayModeResponses)
	default:
		return int(constant.RelayModeUnknown)
	}
}

// Ensure interface compliance.
var _ common.Adaptor = (*Adaptor)(nil)
