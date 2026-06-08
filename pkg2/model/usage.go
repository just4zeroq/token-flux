package model

// UsageRecord maps to llm_usage_records table.
// Captures per-request billing and telemetry data.
type UsageRecord struct {
	ID              int64  `json:"id"`
	RequestID       string `json:"request_id"`
	ModelSpecID     int64  `json:"model_spec_id"`
	ModelKeyID      int64  `json:"model_key_id"`
	KeyModelID      int64  `json:"key_model_id,omitempty"`
	ChannelID       int64  `json:"channel_id"`
	Capability      string `json:"capability"`
	IsStream        bool   `json:"is_stream"`
	InputTokens     int    `json:"input_tokens"`
	OutputTokens    int    `json:"output_tokens"`
	TotalTokens     int    `json:"total_tokens"`
	CostCredits     int64  `json:"cost_credits"`
	LatencyMs       int    `json:"latency_ms"`
	Status          string `json:"status"`            // success / failed / settlement_pending
	ErrorCode       string `json:"error_code,omitempty"`
	ErrorMessage    string `json:"error_message,omitempty"`
	CreatedAt       int64  `json:"created_at"`
}
