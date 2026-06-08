package helper

import (
	"encoding/json"

	"ai-platform/pkg/translator"
)

// DetectInboundFormat detects the request format from the JSON request body.
// Delegates to translator.DetectFormat.
func DetectInboundFormat(body []byte) translator.Format {
	return translator.DetectFormat(body)
}

// DetectFormatFromPath attempts to determine format from the request path.
// Delegates to translator.DetectFormatFromPath.
func DetectFormatFromPath(path string) (translator.Format, bool) {
	return translator.DetectFormatFromPath(path)
}

// MapProtocolToAdaptor returns the adaptor protocol name for a given protocol key.
func MapProtocolToAdaptor(protocolKey string) string {
	return protocolKey
}

// BestProtocolMatch selects the best protocol from the channel's supported protocols
// that matches or is closest to the inbound format.
// Returns the protocol key and the upstream base URL.
func BestProtocolMatch(protocolsJSON map[string]map[string]string, inboundFormat translator.Format) (protocolKey string, baseURL string) {
	if len(protocolsJSON) == 0 {
		return "", ""
	}

	var preferredKey string
	switch inboundFormat {
	case translator.FormatOpenAI:
		preferredKey = "openai-compatible"
	case translator.FormatClaude:
		preferredKey = "anthropic-compatible"
	case translator.FormatGemini:
		preferredKey = "gemini-compatible"
	case translator.FormatOpenAIResponses:
		preferredKey = "openai-compatible"
	}

	if preferredKey != "" {
		if entry, ok := protocolsJSON[preferredKey]; ok {
			return preferredKey, entry["base_url"]
		}
	}

	for key, entry := range protocolsJSON {
		return key, entry["base_url"]
	}

	return "", ""
}

// ParseProtocolsJSON parses the stored protocols_json into a map.
func ParseProtocolsJSON(protocolsJSON string) map[string]map[string]string {
	var obj map[string]map[string]string
	if err := json.Unmarshal([]byte(protocolsJSON), &obj); err != nil {
		return nil
	}
	return obj
}
