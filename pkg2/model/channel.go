package model

// ProtocolEntry represents one protocol + base_url pair for a channel.
// A single channel can support multiple protocols (e.g., DeepSeek supports
// both openai-compatible and anthropic-compatible via different base URLs).
type ProtocolEntry struct {
	Protocol          ProtocolType `json:"protocol"`           // "openai-compatible", "anthropic-compatible", "gemini-compatible"
	BaseURL           string       `json:"base_url"`           // upstream API base URL for this protocol
	UpstreamModelName string       `json:"upstream_model_name"` // fixed model name for this protocol (optional)
	IsModelMapped     bool         `json:"is_model_mapped"`     // if true, replace model field with UpstreamModelName
}

// Channel maps to llm_channels table (PostgreSQL) / SQLite equivalent.
// Represents an upstream AI provider connection. Each channel can have
// multiple protocol endpoints (e.g., one channel for DeepSeek serving
// both OpenAI and Claude protocols via different URLs).
type Channel struct {
	ID              int64           `json:"id"`
	Code            string          `json:"code"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	ProviderType    ProviderType    `json:"provider_type"`     // 1=OpenAI, 2=Claude, 8=DeepSeek ...
	Protocols       []ProtocolEntry `json:"protocols"`         // multiple protocol endpoints
	ApiKeyEncrypted string          `json:"-"`                 // AES-GCM encrypted, never serialized
	Settings        ChannelSettings `json:"settings"`
	Status          string          `json:"status"`            // active / disabled / pending
	CreatedAt       int64           `json:"created_at"`
	UpdatedAt       int64           `json:"updated_at"`
}

// ResolveProtocol picks the ProtocolEntry matching the inbound format label.
// E.g., "openai" → first openai-compatible protocol entry.
// Returns nil if no match found.
func (c *Channel) ResolveProtocol(inboundFormat string) *ProtocolEntry {
	if inboundFormat == "" {
		inboundFormat = "openai"
	}
	// Map common inbound format labels to protocol types.
	proto := inboundToProto(inboundFormat)
	for i := range c.Protocols {
		if string(c.Protocols[i].Protocol) == proto {
			return &c.Protocols[i]
		}
	}
	// Fallback: try openai-compatible if available (most common).
	if proto != "openai-compatible" {
		for i := range c.Protocols {
			if string(c.Protocols[i].Protocol) == "openai-compatible" {
				return &c.Protocols[i]
			}
		}
	}
	// Fallback: return first protocol entry.
	if len(c.Protocols) > 0 {
		return &c.Protocols[0]
	}
	return nil
}

func inboundToProto(f string) string {
	// Map common inbound format labels to protocol types.
	// "openai" → "openai-compatible"
	// "claude" → "anthropic-compatible"
	// "gemini" → "gemini-compatible"
	switch f {
	case "openai", "openai_responses":
		return "openai-compatible"
	case "claude":
		return "anthropic-compatible"
	case "gemini":
		return "gemini-compatible"
	default:
		return f // passthrough for exact protocol names
	}
}

// ChannelSettings holds per-channel operational configuration.
type ChannelSettings struct {
	TimeoutSeconds int               `json:"timeout_seconds"`          // request timeout, default 60
	RetryCount     int               `json:"retry_count"`              // retry attempts
	UseProxy       bool              `json:"use_proxy"`                // use system proxy
	HeaderOverride map[string]string `json:"header_override,omitempty"` // extra/modified request headers
}
