package common

// ChannelCandidate is the resolved channel+key binding sent to the scheduler.
type ChannelCandidate struct {
	ChannelID         int64
	ChannelName       string
	BaseURL           string
	Priority          int
	Weight            int
	HealthScore       float64
	ModelKeyID        int64
	KeyModelID        int64
	UpstreamModelName string
	IsModelMapped     bool
	ApiKey            string // encrypted upstream key
	ProviderUserID    int64
	ModelSpecID       int64
	// Protocol info for adaptor routing.
	ProtocolKey  string // e.g. "openai-compatible", "anthropic-compatible", "gemini-compatible"
	ProtocolsJSON string // raw JSON for fallback parsing
	ChannelType  int    // ProviderType constant
	// Pricing from llm_model_key_models binding.
	ProviderShareBps  int
	CacheHitPricePer1K  int64
	CacheMissPricePer1K int64
	OutputPricePer1K    int64
}
