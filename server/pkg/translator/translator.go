// Package translator provides pure format translation between LLM API formats.
// No platform dependencies — used by both the server and local node.
package translator

import "encoding/json"

// Format is an LLM API protocol format.
type Format string

const (
	FormatOpenAI          Format = "openai"
	FormatClaude           Format = "claude"
	FormatGemini           Format = "gemini"
	FormatOpenAIResponses  Format = "openai_responses"
)

// Message is a canonical message used across all formats.
type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content,omitempty"`
}

// ---- Request: any format → OpenAI ----

// Normalize converts an inbound request body into OpenAI Chat Completions format.
// Supported from: FormatClaude, FormatGemini, FormatOpenAIResponses, FormatOpenAI (passthrough).
func Normalize(body []byte, from Format) ([]byte, error) {
	switch from {
	case FormatOpenAI:
		return body, nil
	case FormatClaude:
		return claudeToOpenAI(body)
	case FormatGemini:
		return geminiToOpenAI(body)
	case FormatOpenAIResponses:
		return responsesToOpenAI(body)
	default:
		return body, nil
	}
}

// ---- Response: OpenAI → any format ----

// Denormalize converts an OpenAI Chat Completions response into the target format.
// Supported to: FormatClaude, FormatGemini, FormatOpenAIResponses, FormatOpenAI (passthrough).
func Denormalize(body []byte, to Format, isStream bool) (Denormalizer, error) {
	switch to {
	case FormatOpenAI:
		return &passthroughDenorm{}, nil
	case FormatClaude:
		return newClaudeDenorm(isStream), nil
	case FormatGemini:
		return newGeminiDenorm(), nil
	case FormatOpenAIResponses:
		return newResponsesDenorm(), nil
	default:
		return &passthroughDenorm{}, nil
	}
}

// Denormalizer converts an upstream OpenAI Chat Completions response stream/body
// into client format. Implementations handle streaming vs non-streaming.
type Denormalizer interface {
	// Header returns the Content-Type for this response format.
	Header() string
	// ConvertBody converts a full non-streaming response body.
	ConvertBody(body []byte) ([]byte, error)
	// ConvertChunk converts a single SSE chunk from the upstream.
	// Returns nil,nil if no output for this chunk (e.g. buffering).
	ConvertChunk(chunk []byte) ([]byte, error)
	// Finalize returns any buffered output at the end of a stream.
	Finalize() ([]byte, error)
}

// passthroughDenorm is a no-op denormalizer for FormatOpenAI.
type passthroughDenorm struct{}

func (d *passthroughDenorm) Header() string                    { return "application/json" }
func (d *passthroughDenorm) ConvertBody(b []byte) ([]byte, error) { return b, nil }
func (d *passthroughDenorm) ConvertChunk(b []byte) ([]byte, error) { return b, nil }
func (d *passthroughDenorm) Finalize() ([]byte, error)           { return nil, nil }

// DetectFormat detects the format of a raw request body.
func DetectFormat(body []byte) Format {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return FormatOpenAI
	}
	if _, ok := obj["contents"]; ok {
		return FormatGemini
	}
	if _, ok := obj["input"]; ok {
		if _, ok := obj["messages"]; !ok {
			return FormatOpenAIResponses
		}
	}
	if _, ok := obj["anthropic_version"]; ok {
		return FormatClaude
	}
	if _, ok := obj["system"]; ok {
		return FormatClaude
	}
	return FormatOpenAI
}
