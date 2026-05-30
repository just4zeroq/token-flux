package dto

import "time"

// ProtocolEntry represents one protocol+base_url pair.
type ProtocolEntry struct {
	Protocol string `json:"protocol" v:"required"`
	BaseURL  string `json:"base_url" v:"required"`
}

// List of allowed protocol keys for validation.
var AllowedProtocolKeys = []string{
	"openai-compatible",
	"anthropic-compatible",
	"gemini-compatible",
	"azure-openai",
}

type LLMChannelInfo struct {
	ID               int64           `json:"id"`
	Code             string          `json:"code"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	ProtocolsJson    string          `json:"protocols_json"`
	Status           string          `json:"status"`
	SourceType       string          `json:"source_type"`
	Priority         int             `json:"priority"`
	Weight           int             `json:"weight"`
	CreatedByUserID  int64           `json:"created_by_user_id"`
	ReviewedByUserID int64           `json:"reviewed_by_user_id"`
	ReviewedAt       time.Time       `json:"reviewed_at,omitzero"`
	ReviewNote       string          `json:"review_note"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type LLMCreateChannelIn struct {
	Code            string          `json:"code" v:"required"`
	Name            string          `json:"name" v:"required"`
	Description     string          `json:"description"`
	Protocols       []ProtocolEntry `json:"protocols" v:"required"`
	SourceType      string          `json:"-"`
	CreatedByUserID int64           `json:"-"`
}

type LLMListChannelsIn struct {
	Status          string `json:"status"`
	SourceType      string `json:"source_type"`
	CreatedByUserID int64  `json:"-"`
	ProviderUserID  int64  `json:"-"`
	Page            int    `json:"page"`
	Size            int    `json:"size"`
}

type LLMReviewChannelIn struct {
	ID         int64  `json:"-"`
	Status     string `json:"status" v:"required|in:active,disabled,rejected"`
	ReviewNote string `json:"review_note"`
	ReviewerID int64  `json:"-"`
}

type LLMModelSpecInfo struct {
	ID                  int64     `json:"id"`
	DeveloperName       string    `json:"developer_name"`
	ModelName           string    `json:"model_name"`
	ModelCode           string    `json:"model_code"`
	DisplayName         string    `json:"display_name"`
	ModelFamily         string    `json:"model_family"`
	Description         string    `json:"description"`
	CapabilitiesJson    string    `json:"capabilities_json"`
	ContextWindow       int       `json:"context_window"`
	MaxInputTokens      int       `json:"max_input_tokens"`
	MaxOutputTokens     int       `json:"max_output_tokens"`
	SupportsStream      bool      `json:"supports_stream"`
	SupportsTools       bool      `json:"supports_tools"`
	SupportsVision      bool      `json:"supports_vision"`
	SupportsJsonMode    bool      `json:"supports_json_mode"`
	SupportsReasoning   bool      `json:"supports_reasoning"`
	SupportsLogprobs    bool      `json:"supports_logprobs"`
	SupportedParamsJson string    `json:"supported_params_json"`
	DefaultParamsJson   string    `json:"default_params_json"`
	ParamLimitsJson     string    `json:"param_limits_json"`
	SourceType          string    `json:"source_type"`
	CreatedByUserID     int64     `json:"created_by_user_id"`
	Status              string    `json:"status"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type LLMCreateModelSpecIn struct {
	DeveloperName       string `json:"developer_name" v:"required"`
	ModelName           string `json:"model_name" v:"required"`
	ModelCode           string `json:"model_code"`
	DisplayName         string `json:"display_name"`
	ModelFamily         string `json:"model_family"`
	Description         string `json:"description"`
	CapabilitiesJson    string `json:"capabilities_json" v:"required"`
	ContextWindow       int    `json:"context_window"`
	MaxInputTokens      int    `json:"max_input_tokens"`
	MaxOutputTokens     int    `json:"max_output_tokens"`
	SupportsStream      bool   `json:"supports_stream"`
	SupportsTools       bool   `json:"supports_tools"`
	SupportsVision      bool   `json:"supports_vision"`
	SupportsJsonMode    bool   `json:"supports_json_mode"`
	SupportsReasoning   bool   `json:"supports_reasoning"`
	SupportsLogprobs    bool   `json:"supports_logprobs"`
	SupportedParamsJson string `json:"supported_params_json"`
	DefaultParamsJson   string `json:"default_params_json"`
	ParamLimitsJson     string `json:"param_limits_json"`
	SourceType          string `json:"-"`
	CreatedByUserID     int64  `json:"-"`
}

type LLMListModelSpecsIn struct {
	Status          string `json:"status"`
	SourceType      string `json:"source_type"`
	CreatedByUserID int64  `json:"-"`
	ProviderUserID  int64  `json:"-"`
	Page            int    `json:"page"`
	Size            int    `json:"size"`
}

type LLMReviewModelSpecIn struct {
	ID         int64  `json:"-"`
	Status     string `json:"status" v:"required|in:active,disabled,rejected"`
	ReviewNote string `json:"review_note"`
	ReviewerID int64  `json:"-"`
}

// --- Key Model Binding ---

type LLMCreateKeyModelBinding struct {
	ModelSpecID       int64  `json:"model_spec_id" v:"required"`
	UpstreamModelName string `json:"upstream_model_name" v:"required"`
	CacheHitPricePer1K  int64 `json:"cache_hit_price_per_1k"`
	CacheMissPricePer1K int64 `json:"cache_miss_price_per_1k"`
	OutputPricePer1K    int64 `json:"output_price_per_1k"`
}

type LLMCreateModelKeyIn struct {
	ChannelID     int64                      `json:"channel_id" v:"required"`
	Name          string                     `json:"name"`
	Key           string                     `json:"key" v:"required"`
	ModelBindings []LLMCreateKeyModelBinding `json:"model_bindings"`
}

type LLMModelKeyInfo struct {
	ID               int64     `json:"id"`
	ProviderUserID   int64     `json:"provider_user_id"`
	ChannelID        int64     `json:"channel_id"`
	Name             string    `json:"name"`
	KeyMasked        string    `json:"key_masked"`
	QuotaUsedCredits int64     `json:"quota_used_credits"`
	Status           string    `json:"status"`
	LastTestAt       time.Time `json:"last_test_at,omitzero"`
	LastTestStatus   string    `json:"last_test_status"`
	LastTestError    string    `json:"last_test_error"`
	TestAttempts     int       `json:"test_attempts"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type LLMListModelKeysIn struct {
	ProviderUserID int64
	IsAdmin        bool
	Status         string
	Page           int
	Size           int
}

type LLMBindKeyModelIn struct {
	ModelKeyID        int64  `json:"-"`
	ModelSpecID       int64  `json:"model_spec_id" v:"required"`
	UpstreamModelName string `json:"upstream_model_name" v:"required"`
	CacheHitPricePer1K  int64 `json:"cache_hit_price_per_1k"`
	CacheMissPricePer1K int64 `json:"cache_miss_price_per_1k"`
	OutputPricePer1K    int64 `json:"output_price_per_1k"`
}

type LLMKeyModelInfo struct {
	ID                  int64     `json:"id"`
	ModelKeyID          int64     `json:"model_key_id"`
	ModelSpecID         int64     `json:"model_spec_id"`
	UpstreamModelName   string    `json:"upstream_model_name"`
	QuotaUsedCredits    int64     `json:"quota_used_credits"`
	Status              string    `json:"status"`
	CacheHitPricePer1K  int64     `json:"cache_hit_price_per_1k"`
	CacheMissPricePer1K int64     `json:"cache_miss_price_per_1k"`
	OutputPricePer1K    int64     `json:"output_price_per_1k"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	LastErrorCode       string    `json:"last_error_code"`
	LastErrorMessage    string    `json:"last_error_message"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type LLMListKeyModelsIn struct {
	ProviderUserID int64
	IsAdmin        bool
	ModelKeyID     int64
	ModelSpecID    int64
	Status         string
	Page           int
	Size           int
}

type OpenAIModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type OpenAIProxyRequest struct {
	UserID     int64
	ApiKeyID   int64
	RequestID  string
	Capability string
	RawBody    []byte
	IsStream   bool
}

// Provider settings
type ProviderSettingInfo struct {
	UserID   int64 `json:"user_id"`
	ShareBps int   `json:"share_bps"`
}

type OpenAIProxyResponse struct {
	StatusCode int
	Body       []byte
	Headers    map[string]string
}
