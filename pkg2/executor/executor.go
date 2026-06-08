// Package executor defines the unified LLM provider executor interface.
// Each provider brand (openai, claude, gemini, deepseek, ...) implements this interface.
// Used by both server relay and local node for upstream API calls.
package executor

import (
	"context"
	"io"
	"net/http"
	"time"

	"ai-platform/pkg/model"
	"ai-platform/pkg/translator"
)

// RequestInfo carries per-request metadata through the executor pipeline.
// Populated by the caller (relay controller or node router) before calling executor methods.
type RequestInfo struct {
	RequestID       string          // unique request identifier
	RelayMode       model.RelayMode // chat, completions, embeddings, etc.
	IsStream        bool            // SSE streaming request
	Model           string          // user-requested model name
	ActualModelName string          // resolved upstream model name (after mapping)
	InboundFormat   translator.Format // request body format
	ClientFormat    translator.Format // original client format before conversion

	Channel  *model.Channel      // resolved channel configuration
	Protocol *model.ProtocolEntry // resolved protocol entry for this request (from Channel.ResolveProtocol)
	ApiKey   string               // decrypted upstream API key

	// Thinking/reasoning config (parsed from model suffix, e.g. -thinking, -high)
	ThinkingEnabled  bool
	ThinkingDisabled bool
	ReasoningEffort  string // low, medium, high, xhigh, max

	RetryIndex int       // current retry attempt (0 = first)
	StartTime  time.Time // request start time
}

// Usage captures token usage and cost from an upstream response.
// Returned by TransformResponse for billing/telemetry.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	CacheHitTokens   int `json:"cache_hit_tokens,omitempty"`
	CacheMissTokens  int `json:"cache_miss_tokens,omitempty"`
	ReasoningTokens  int `json:"reasoning_tokens,omitempty"`
}

// Executor is the unified interface for all upstream AI provider adaptors.
// Methods correspond to pipeline stages: URL → Headers → Body → HTTP → Response.
//
// Implementations:
//   - executor/openai:  OpenAI-compatible providers (OpenAI, DeepSeek, Zhipu, Moonshot, ...)
//   - executor/claude:  Anthropic Messages API (Claude, AWS Bedrock Claude)
//   - executor/gemini:  Google Gemini API
//   - executor/deepseek: DeepSeek (FIM + thinking injection)
//   - executor/ali:     Alibaba Qwen
//   - executor/baidu:   Baidu ERNIE
//   - executor/zhipu:   Zhipu GLM
type Executor interface {
	// Init pre-initializes the executor with channel metadata.
	// Called once before processing requests.
	Init(channel *model.Channel)

	// GetRequestURL builds the upstream request URL based on relay mode and channel config.
	GetRequestURL(info *RequestInfo) (string, error)

	// SetupRequestHeader sets upstream request headers (auth, content-type, accept, etc.).
	SetupRequestHeader(header http.Header, info *RequestInfo) error

	// TransformRequest converts the inbound request body into the upstream provider's native format.
	// This may involve format translation (Claude↔OpenAI↔Gemini), model name mapping,
	// and provider-specific parameter injection (thinking, stream_options, etc.).
	TransformRequest(ctx context.Context, info *RequestInfo, requestBody []byte) (io.Reader, error)

	// DoRequest sends the HTTP request to the upstream provider.
	// Uses GetRequestURL + SetupRequestHeader + TransformRequest internally.
	DoRequest(ctx context.Context, info *RequestInfo, requestBody io.Reader) (*http.Response, error)

	// TransformResponse handles the upstream response, converts it to the client's expected format,
	// writes the result to the response writer, and returns usage data.
	TransformResponse(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error)

	// GetName returns the provider name for logging and monitoring.
	GetName() string
}
