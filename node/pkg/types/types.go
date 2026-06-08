package types

import "encoding/json"

// ---- WebSocket Tunnel Messages ----

type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`

	// Inline fields for common message types
	RequestID string `json:"request_id,omitempty"`
	Error     string `json:"error,omitempty"`
	NodeID    string `json:"node_id,omitempty"`
	Nonce     string `json:"nonce,omitempty"`
}

// ---- Auth ----

type AuthPayload struct {
	NodeID    string `json:"node_id"`
	Signature string `json:"signature"`
}

// ---- Registration ----

type ModelBinding struct {
	ModelCode  string `json:"model_code"` // from platform catalog
	KeyHash    string `json:"key_hash"`   // HASH(local_key + channel + model_name)
	InputPrice int64  `json:"input_price_per_1k"`
	OutputPrice int64 `json:"output_price_per_1k"`
	CachePrice int64  `json:"cache_hit_price_per_1k,omitempty"`
}

type RegisterPayload struct {
	Bindings []ModelBinding `json:"bindings"`
}

// ---- Request / Response ----

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
	// Additional OpenAI-compatible fields
	MaxTokens   *int     `json:"max_tokens,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChunkData struct {
	Choices []ChoiceDelta `json:"choices"`
}

type ChoiceDelta struct {
	Delta Delta `json:"delta"`
	Index int   `json:"index"`
}

type Delta struct {
	Content string `json:"content,omitempty"`
	Role    string `json:"role,omitempty"`
}

type ChatResponse struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Message Message `json:"message"`
	Index   int     `json:"index"`
}

// ---- Internal Routing ----

type RequestEnvelope struct {
	RequestID string `json:"request_id"`
	Model     string `json:"model"`
	KeyHash   string `json:"key_hash"`
	// Format fields for non-tunnel clients (set by server.go handlers, carried through tunnel).
	InboundFormat string `json:"inbound_format,omitempty"` // "openai", "claude", "gemini", "openai_responses"
	ClientFormat  string `json:"client_format,omitempty"`  // original client format for response denormalize
	Request       ChatRequest `json:"request"`
}

// ---- Node API Key (for sk-xxx gateway auth) ----

// APIKey represents a local node API key stored in node_api_keys.
type APIKey struct {
	ID        int64  `json:"id"`
	Key       string `json:"key,omitempty"`       // full key, only returned on creation
	KeyPrefix string `json:"key_prefix"`           // masked for display
	Label     string `json:"label"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

// ---- Local Key Management ----

// KeyEntry represents an upstream API key stored in llm_model_keys.
type KeyEntry struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`               // human-readable label
	Key               string `json:"-"`                  // raw key value (never serialized)
	KeyMasked         string `json:"key_masked"`         // masked for display (sk-****)
	ChannelID         int64  `json:"channel_id"`          // platform channel reference
	BaseURL           string `json:"base_url"`            // upstream API base URL
	Status            string `json:"status"`              // active / disabled
	QuotaLimitCredits int64  `json:"quota_limit_credits"` // max credits
	QuotaUsedCredits  int64  `json:"quota_used_credits"`  // credits consumed
	CreatedAt         int64  `json:"created_at"`
}

// KeyBinding maps a model key to a model spec (llm_model_key_models).
type KeyBinding struct {
	ID                int64  `json:"id"`
	ModelKeyID        int64  `json:"model_key_id"`          // parent llm_model_keys.id
	ModelSpecID       int64  `json:"model_spec_id"`         // platform model spec (0 for node-local)
	UpstreamModelName string `json:"upstream_model_name"`   // actual model name sent to upstream
	ModelCode         string `json:"model_code"`            // platform model code for routing
	KeyHash           string `json:"key_hash"`              // HASH(key + channel + model)
	Shared            bool   `json:"shared"`                // true if registered with router/platform
	InputPrice        int64  `json:"input_price"`
	OutputPrice       int64  `json:"output_price"`
	Priority          int    `json:"priority"`              // lower = higher routing priority
	Weight            int    `json:"weight"`                // weighted selection
	Status            string `json:"status"`                // active / private / disabled
	LastError         string `json:"last_error,omitempty"`
	CreatedAt         int64  `json:"created_at"`
}
