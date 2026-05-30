package helper

import (
	"encoding/json"

	"ai-platform/internal/relay/constant"
)

// DetectInboundFormat detects the request format from the JSON request body.
// Examines top-level keys to distinguish:
//   - openai    → has "model" and "messages" (standard chat completions)
//   - claude    → has "anthropic_version" or "messages" with array-of-objects content
//   - gemini    → has "contents" array
//   - responses → has "input" (not "messages")
func DetectInboundFormat(body []byte) constant.RelayFormat {
	if len(body) == 0 {
		return constant.RelayFormatOpenAI
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return constant.RelayFormatOpenAI
	}

	// Check Gemini: top-level "contents" array is unique to Gemini chat.
	if _, hasContents := obj["contents"]; hasContents {
		return constant.RelayFormatGemini
	}

	// Check Responses API: has "input" field (not "messages").
	if _, hasInput := obj["input"]; hasInput {
		if _, hasMessages := obj["messages"]; !hasMessages {
			return constant.RelayFormatResponses
		}
	}

	// Check Claude: anthropic_version header or messages with array content blocks.
	if _, hasVersion := obj["anthropic_version"]; hasVersion {
		return constant.RelayFormatClaude
	}
	// Claude messages can also be identified by "messages" where the first
	// message's "content" is an array of content blocks (text/image/tool_use)
	// rather than a plain string — common in Claude SDK requests.
	if messagesRaw, hasMessages := obj["messages"]; hasMessages {
		var msgs []map[string]json.RawMessage
		if err := json.Unmarshal(messagesRaw, &msgs); err == nil && len(msgs) > 0 {
			firstMsg := msgs[0]
			if _, hasSystem := obj["system"]; hasSystem {
				return constant.RelayFormatClaude
			}
			// Check if content is an array (Claude content blocks)
			if contentRaw, hasContent := firstMsg["content"]; hasContent {
				var contentStr string
				if err := json.Unmarshal(contentRaw, &contentStr); err != nil {
					// If it's not a simple string, might be JSON array — Claude style
					var contentArr []any
					if err := json.Unmarshal(contentRaw, &contentArr); err == nil && len(contentArr) > 0 {
						return constant.RelayFormatClaude
					}
				}
			}
		}
	}

	// Default: OpenAI format.
	return constant.RelayFormatOpenAI
}

// DetectFormatFromPath attempts to determine format from the request path.
// More reliable than DetectInboundFormat for ambiguous bodies.
func DetectFormatFromPath(path string) (constant.RelayFormat, bool) {
	switch path {
	case "/v1/messages":
		return constant.RelayFormatClaude, true
	case "/v1/chat/completions", "/v1/completions", "/v1/embeddings":
		return constant.RelayFormatOpenAI, true
	case "/v1/responses", "/v1/responses/compact":
		return constant.RelayFormatResponses, true
	}
	return constant.RelayFormatOpenAI, false
}

// MapProtocolToAdaptor returns the adaptor protocol name for a given protocol key.
// e.g., "openai-compatible" → "openai-compatible" (pass through)
// e.g., "gemini-compatible" → "gemini-compatible"
// e.g., "anthropic-compatible" → "anthropic-compatible"
func MapProtocolToAdaptor(protocolKey string) string {
	return protocolKey
}

// BestProtocolMatch selects the best protocol from the channel's supported protocols
// that matches or is closest to the inbound format.
// Returns the protocol key and the upstream base URL.
func BestProtocolMatch(protocolsJSON map[string]map[string]string, inboundFormat constant.RelayFormat) (protocolKey string, baseURL string) {
	if len(protocolsJSON) == 0 {
		return "", ""
	}

	// Prefer protocol that matches the inbound format.
	var preferredKey string
	switch inboundFormat {
	case constant.RelayFormatOpenAI:
		preferredKey = "openai-compatible"
	case constant.RelayFormatClaude:
		preferredKey = "anthropic-compatible"
	case constant.RelayFormatGemini:
		preferredKey = "gemini-compatible"
	case constant.RelayFormatResponses:
		preferredKey = "openai-compatible" // Responses → OpenAI
	}

	if preferredKey != "" {
		if entry, ok := protocolsJSON[preferredKey]; ok {
			return preferredKey, entry["base_url"]
		}
	}

	// Fallback: first available protocol.
	for key, entry := range protocolsJSON {
		return key, entry["base_url"]
	}

	return "", ""
}

// ParseProtocolsJSON parses the stored protocols_json into a map.
// Expected format: {"openai-compatible": {"base_url": "https://..."}}
func ParseProtocolsJSON(protocolsJSON string) map[string]map[string]string {
	var obj map[string]map[string]string
	if err := json.Unmarshal([]byte(protocolsJSON), &obj); err != nil {
		return nil
	}
	return obj
}
