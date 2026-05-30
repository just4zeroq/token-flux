package types

import "encoding/json"

// ---- WebSocket Tunnel Messages ----

type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`

	// Inline fields for common message types
	RequestID string `json:"request_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ---- Auth ----

type AuthPayload struct {
	NodeID    string `json:"node_id"`
	Signature string `json:"signature"`
}

// ---- Registration ----

type ModelBinding struct {
	ModelCode  string `json:"model_code"`  // from platform catalog
	KeyHash    string `json:"key_hash"`    // HASH(local_key + channel + model_name)
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
	RequestID string      `json:"request_id"`
	Model     string      `json:"model"`
	KeyHash   string      `json:"key_hash"`
	Request   ChatRequest `json:"request"`
}

// ---- Local Key Management ----

type KeyEntry struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Key       string `json:"key"`
	ChannelID string `json:"channel_id"`    // platform channel reference
	BaseURL   string `json:"base_url"`      // provider endpoint
	Status    string `json:"status"`        // active / disabled
	CreatedAt int64  `json:"created_at"`
}

type KeyBinding struct {
	ID         string `json:"id"`
	KeyID      string `json:"key_id"`
	ModelCode  string `json:"model_code"`
	ModelName  string `json:"model_name"`
	KeyHash    string `json:"key_hash"`
	Shared     bool   `json:"shared"`
	CreatedAt  int64  `json:"created_at"`
}
