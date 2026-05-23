package dto

import "time"

type UsageRecordInfo struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	ApiKeyID       int64     `json:"api_key_id,omitempty"`
	ItemID         int64     `json:"item_id"`
	Runtime        string    `json:"runtime"`
	CapabilityKind string    `json:"capability_kind"`
	InputTokens    int       `json:"input_tokens"`
	OutputTokens   int       `json:"output_tokens"`
	CallCount      int       `json:"call_count"`
	LatencyMs      int       `json:"latency_ms"`
	CostCredits    int64     `json:"cost_credits"`
	RequestID      string    `json:"request_id"`
	IsStream       bool      `json:"is_stream"`
	CreatedAt      time.Time `json:"created_at"`
}

type ReportUsageIn struct {
	UserID       int64  `json:"user_id"`
	ApiKeyID     int64  `json:"api_key_id"`
	ItemID       int64  `json:"item_id" v:"required"`
	Runtime      string `json:"runtime" v:"required"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	CallCount    int    `json:"call_count"`
	LatencyMs    int    `json:"latency_ms"`
	CostCredits  int64  `json:"cost_credits"`
	RequestID    string `json:"request_id"`
	ChannelID    int64  `json:"channel_id"`
	IsStream     bool   `json:"is_stream"`
}

type UsageStats struct {
	TotalCalls   int   `json:"total_calls"`
	TotalTokens  int   `json:"total_tokens"`
	TotalCredits int64 `json:"total_credits"`
	AvgLatencyMs int   `json:"avg_latency_ms"`
}
