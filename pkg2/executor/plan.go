// Package executor — format conversion planner.
//
// Plan decides optimal upstream format by minimizing total conversions:
//
//	score(upstream) = (input != upstream) + (output != upstream)
//
// score 0 = passthrough, 1 = one conversion, 2 = two conversions.
package executor

import (
	"ai-platform/pkg/model"
	"ai-platform/pkg/translator"
)

// EndpointCapability declares upstream format an executor supports natively.
type EndpointCapability struct {
	Format    translator.Format // "openai", "claude", "gemini"
	RelayMode model.RelayMode   // relay mode for URL routing
}

// FormatPlan is optimal conversion strategy for one request.
type FormatPlan struct {
	UpstreamFormat    translator.Format // format to send upstream
	UpstreamRelayMode model.RelayMode   // relay mode for upstream URL
	NeedRequestConv   bool               // input → upstream conversion needed
	NeedResponseConv  bool               // upstream → client output conversion needed
}

// FormatCapable is optional interface executors implement to declare
// which upstream formats they handle natively.
type FormatCapable interface {
	NativeFormats() []EndpointCapability
}

// Plan computes optimal FormatPlan.
// outputFormat defaults to inputFormat when empty.
func Plan(inputFormat, outputFormat translator.Format, caps []EndpointCapability) FormatPlan {
	if outputFormat == "" {
		outputFormat = inputFormat
	}

	// Passthrough when no caps (no conversion needed, use default).
	if len(caps) == 0 {
		return FormatPlan{}
	}

	best := FormatPlan{}
	bestScore := 999

	for _, cap := range caps {
		reqConv := inputFormat != cap.Format
		respConv := outputFormat != cap.Format

		// Check conversion path feasibility.
		if reqConv && !canConvertRequest(inputFormat, cap.Format) {
			continue
		}
		if respConv && !canConvertResponse(cap.Format, outputFormat) {
			continue
		}

		score := boolToInt(reqConv) + boolToInt(respConv)
		better := false
		if score < bestScore {
			better = true
		} else if score == bestScore && cap.Format == outputFormat {
			// Tiebreak: output format priority.
			better = true
		}

		if better {
			best = FormatPlan{
				UpstreamFormat:    cap.Format,
				UpstreamRelayMode: cap.RelayMode,
				NeedRequestConv:   reqConv,
				NeedResponseConv:  respConv,
			}
			bestScore = score
		}
	}

	return best
}

// canConvertRequest returns true when request body conversion path exists.
//
// Conversion graph:
//
//	any ──Normalize──→ openai ──OpenAIToClaude──→ claude
//	                         └──OpenAIToGemini──→ gemini
func canConvertRequest(from, to translator.Format) bool {
	if from == to {
		return true
	}
	// any → openai: translator.Normalize handles claude, gemini, responses, etc.
	if to == translator.FormatOpenAI {
		return true
	}
	// openai → claude: translator.OpenAIToClaude.
	if from == translator.FormatOpenAI && to == translator.FormatClaude {
		return true
	}
	// openai → gemini: translator.OpenAIToGemini.
	if from == translator.FormatOpenAI && to == translator.FormatGemini {
		return true
	}
	return false
}

// canConvertResponse returns true when response body conversion path exists.
//
// Conversion graph:
//
//	any ──native→openai──→ translator.Denormalize ──→ claude/gemini/responses
func canConvertResponse(from, to translator.Format) bool {
	if from == to {
		return true
	}
	// Denormalize from openai: openai → claude/gemini/openai_responses.
	if from == translator.FormatOpenAI {
		return true
	}
	// Non-openai upstream: executor native→openai then Denormalize.
	return true
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// NativeFormat is shorthand for single-format executor capability.
func NativeFormat(f translator.Format, mode model.RelayMode) []EndpointCapability {
	return []EndpointCapability{{Format: f, RelayMode: mode}}
}
