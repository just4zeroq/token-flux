// Package helper — token usage normalization and provider-specific extraction.
package helper

import (
	"encoding/json"

	"ai-platform/internal/relay/common"
	"ai-platform/internal/relay/dto"
)

// NormalizeUsage ensures TotalTokens is consistent when both PromptTokens
// and CompletionTokens are known. Does NOT estimate from text — if upstream
// didn't provide token counts, settlement handles it asynchronously.
//
// Rules:
//   - TotalTokens == 0 → PromptTokens + CompletionTokens
func NormalizeUsage(u *common.Usage) *common.Usage {
	if u == nil {
		u = &common.Usage{}
	}
	if u.TotalTokens == 0 {
		u.TotalTokens = u.PromptTokens + u.CompletionTokens
	}
	return u
}

// ExtractUsage parses an upstream response body and returns token usage,
// handling provider-specific field name differences.
//
// Standard OpenAI format:
//
//	"usage": { "prompt_tokens": N, "completion_tokens": N, "total_tokens": N,
//	  "prompt_tokens_details": { "cached_tokens": N } }
//
// DeepSeek variant:
//
//	"usage": { "prompt_tokens": N, "completion_tokens": N, "total_tokens": N,
//	  "prompt_cache_hit_tokens": N, "prompt_cache_miss_tokens": N }
//	  ← cache info at usage root level, NOT inside prompt_tokens_details
//
// ExtractUsage merges both: it first unmarshals the standard OpenAI format,
// then re-parses the usage object as raw JSON to catch non-standard fields.
// Returns nil if no usage found.
func ExtractUsage(body []byte) *dto.UsageWithDetails {
	// Extract the "usage" sub-object as raw JSON.
	var raw struct {
		Usage json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal(body, &raw); err != nil || len(raw.Usage) == 0 {
		return nil
	}

	// Standard OpenAI format unmarshal.
	var usage dto.UsageWithDetails
	if err := json.Unmarshal(raw.Usage, &usage); err != nil {
		return nil
	}
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return nil
	}

	// Re-parse usage as raw map to catch provider-specific fields.
	var usageMap map[string]json.RawMessage
	if err := json.Unmarshal(raw.Usage, &usageMap); err != nil {
		return &usage
	}

	// DeepSeek: prompt_cache_hit_tokens → PromptTokensDetails.CachedTokens
	if hitRaw, ok := usageMap["prompt_cache_hit_tokens"]; ok {
		var hit int
		if json.Unmarshal(hitRaw, &hit) == nil && hit > 0 {
			if usage.PromptTokensDetails == nil {
				usage.PromptTokensDetails = &dto.TokenDetails{}
			}
			if usage.PromptTokensDetails.CachedTokens == 0 {
				usage.PromptTokensDetails.CachedTokens = hit
			}
		}
	}

	return &usage
}
