// Package handler — tiktoken-based tokenizer for lazy settlement.
//
// Uses github.com/pkoukk/tiktoken-go with offline BPE loader to compute
// BPE-accurate token counts without network calls.
//
// Encoder instances are cached by model name to avoid repeated initialization.
// Falls back to cl100k_base for unknown model names (covers GPT-4, GPT-3.5,
// and most OpenAI-compatible models).
package handler

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/pkoukk/tiktoken-go"
	tiktoken_loader "github.com/pkoukk/tiktoken-go-loader"

	"ai-platform/internal/relay/helper"
)

var (
	tiktokenizerInitOnce sync.Once
)

// initTiktokenLoader sets up the offline BPE loader once per process lifetime.
// Without this, tiktoken-go tries to download BPE files from OpenAI's CDN.
func initTiktokenLoader() {
	tiktokenizerInitOnce.Do(func() {
		tiktoken.SetBpeLoader(tiktoken_loader.NewOfflineLoader())
	})
}

// Tiktokenizer implements Tokenizer using BPE token counts from tiktoken-go.
type Tiktokenizer struct {
	mu       sync.RWMutex
	encoders map[string]*tiktoken.Tiktoken
}

// NewTiktokenizer creates a Tiktokenizer with offline BPE loader initialized.
// Call SetGlobalTokenizer(NewTiktokenizer()) in boot.go.
func NewTiktokenizer() *Tiktokenizer {
	initTiktokenLoader()
	return &Tiktokenizer{
		encoders: make(map[string]*tiktoken.Tiktoken),
	}
}

// CountTokens returns the number of BPE tokens in text for the given model.
// Prefers tiktoken BPE encoding; falls back to heuristic estimation if unavailable.
func (tz *Tiktokenizer) CountTokens(ctx context.Context, model string, text string) (int, error) {
	if text == "" {
		return 0, nil
	}

	// Try tiktoken first (BPE-accurate).
	n, err := tz.countWithTiktoken(model, text)
	if err == nil {
		return n, nil
	}

	// Fallback: heuristic estimation.
	g.Log().Warningf(ctx, "[Tiktokenizer] tiktoken failed for model=%q err=%v — fallback to heuristic", model, err)
	return helper.CountTokens(text, model), nil
}

// countWithTiktoken attempts BPE-accurate token count via tiktoken-go.
func (tz *Tiktokenizer) countWithTiktoken(model string, text string) (int, error) {
	if model == "" {
		model = "gpt-4"
	}

	enc, err := tz.getEncoder(model)
	if err != nil {
		return 0, err
	}

	tokens := enc.Encode(text, nil, nil)
	return len(tokens), nil
}

// getEncoder returns a cached or new tiktoken encoder for the given model.
// Unknown models fall back to cl100k_base.
func (tz *Tiktokenizer) getEncoder(model string) (*tiktoken.Tiktoken, error) {
	tz.mu.RLock()
	enc, ok := tz.encoders[model]
	tz.mu.RUnlock()
	if ok {
		return enc, nil
	}

	enc, err := tiktoken.EncodingForModel(model)
	if err != nil {
		// Unknown model — fall back to cl100k_base (GPT-4).
		enc, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return nil, err
		}
	}

	tz.mu.Lock()
	tz.encoders[model] = enc
	tz.mu.Unlock()
	return enc, nil
}
