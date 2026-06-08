package model

// ModelSpec maps to llm_model_specs table.
// Defines a registered AI model with its capabilities.
type ModelSpec struct {
	ID                int64    `json:"id"`
	DeveloperName     string   `json:"developer_name"`    // e.g. "OpenAI"
	ModelName         string   `json:"model_name"`        // upstream model name
	ModelCode         string   `json:"model_code"`        // platform model code, unique
	DisplayName       string   `json:"display_name"`      // human readable
	ModelFamily       string   `json:"model_family"`      // e.g. "gpt-4", "claude-3"
	Description       string   `json:"description"`
	Capabilities      []string `json:"capabilities"`      // ["chat","vision","tools","reasoning"]
	ContextWindow     int      `json:"context_window"`
	MaxInputTokens    int      `json:"max_input_tokens"`
	MaxOutputTokens   int      `json:"max_output_tokens"`
	SupportsStream    bool     `json:"supports_stream"`
	SupportsTools     bool     `json:"supports_tools"`
	SupportsVision    bool     `json:"supports_vision"`
	SupportsReasoning bool     `json:"supports_reasoning"`
	Status            string   `json:"status"`             // active / pending / disabled
	CreatedAt         int64    `json:"created_at"`
	UpdatedAt         int64    `json:"updated_at"`
}

// ModelPricing maps to llm_model_prices table.
type ModelPricing struct {
	ID              int64  `json:"id"`
	ModelSpecID     int64  `json:"model_spec_id"`
	Capability      string `json:"capability"`
	InputPriceMicro int64  `json:"input_price_per_1k"`   // per 1K input tokens, in micro (1/10^6)
	OutputPriceMicro int64 `json:"output_price_per_1k"`  // per 1K output tokens
	CacheHitPriceMicro int64 `json:"cache_hit_price_per_1k"`
	Status          string `json:"status"`
	EffectiveFrom   int64  `json:"effective_from"`
	EffectiveTo     int64  `json:"effective_to,omitempty"`
}
