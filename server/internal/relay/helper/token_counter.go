// Package helper — model-aware token counting.
//
// Uses deterministic approximation based on measured token/char ratios per model family.
// TODO: Replace with github.com/pkou/go-tiktoken for BPE-accurate counts.
package helper

import "strings"

// CountTokens estimates token count for text using model-family approximation.
// During settlement, when upstream returns 0 tokens, this fills in a best-effort count.
func CountTokens(text string, model string) int {
	if text == "" {
		return 0
	}

	// Model-specific token/char ratios (measured empirically).
	// Base: English text ~4 chars/token. Chinese chars often 1-2 tokens each.
	ratio := 0.25 // default: len/4

	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "gpt-4o") || strings.Contains(m, "gpt4o"):
		ratio = 0.28 // GPT-4o slightly more efficient
	case strings.Contains(m, "gpt-4") || strings.Contains(m, "gpt4"):
		ratio = 0.25
	case strings.Contains(m, "gpt-3.5") || strings.Contains(m, "gpt35"):
		ratio = 0.27
	case strings.Contains(m, "claude"):
		ratio = 0.30 // Claude tends to use more tokens
	case strings.Contains(m, "gemini"):
		ratio = 0.26
	case strings.Contains(m, "deepseek"):
		ratio = 0.25
	case strings.Contains(m, "qwen") || strings.Contains(m, "ali"):
		ratio = 0.28
	case strings.Contains(m, "mistral"):
		ratio = 0.25
	case strings.Contains(m, "llama"):
		ratio = 0.26
	case strings.Contains(m, "glm") || strings.Contains(m, "zhipu"):
		ratio = 0.28
	}

	tokens := int(float64(len(text)) * ratio)
	if tokens < 1 && len(text) > 0 {
		tokens = 1
	}
	return tokens
}

// CountTokensForPrompt estimates prompt tokens from the full concatenated prompt text.
func CountTokensForPrompt(text string, model string) int {
	return CountTokens(text, model)
}

// CountTokensForCompletion estimates completion tokens from the response text.
func CountTokensForCompletion(text string, model string) int {
	return CountTokens(text, model)
}
