// Package handler — Tokenizer interface for async lazy tokenization.
//
// Streaming requests whose upstream doesn't return usage data go through
// lazy tokenization: the response text is stored in the queue task, then the
// worker calls globalTokenizer.CountTokens() to compute an accurate token
// count before settlement.
//
// To use: call SetGlobalTokenizer() in boot.go, before starting workers.
//   SetGlobalTokenizer(tiktokenizer.New())
// boot.go already calls SetGlobalTokenizer(NewDefaultTokenizer()) by default.
//
// If no tokenizer is set, lazy settlement tasks will fail with a logged
// error and stay in the retry loop until maxSettlementRetries is reached.
package handler

import (
	"context"

	"ai-platform/internal/relay/helper"
)

// Tokenizer counts tokens in response text for settlement billing.
type Tokenizer interface {
	// CountTokens returns the number of output tokens in the given text
	// for the specified model. If model is empty, uses a reasonable
	// default for the configured upstream model.
	CountTokens(ctx context.Context, model string, text string) (int, error)
}

var globalTokenizer Tokenizer

// SetGlobalTokenizer sets the tokenizer implementation used by settlement
// workers for lazy tokenization. Must be called before StartSettlementWorkers.
func SetGlobalTokenizer(t Tokenizer) {
	globalTokenizer = t
}

// GetGlobalTokenizer returns the registered tokenizer, or nil if unset.
func GetGlobalTokenizer() Tokenizer {
	return globalTokenizer
}

// defaultTokenizer wraps helper.CountTokens (model-family ratio estimation).
// Replace with tiktoken-based implementation for BPE-accurate counts.
type defaultTokenizer struct{}

func (d *defaultTokenizer) CountTokens(_ context.Context, model string, text string) (int, error) {
	return helper.CountTokens(text, model), nil
}

// NewDefaultTokenizer creates a tokenizer using helper.CountTokens.
// This is a heuristic fallback — swap with a real tokenizer for accuracy.
func NewDefaultTokenizer() Tokenizer {
	return &defaultTokenizer{}
}
