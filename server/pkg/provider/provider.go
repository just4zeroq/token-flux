// Package provider defines the LLM provider interface and a default HTTP executor.
// Used by both the platform relay system and the local node.
package provider

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"ai-platform/pkg/translator"
)

// Provider is an LLM API provider that can be called with a normalized OpenAI request.
type Provider interface {
	// ID returns the provider identifier (e.g., "openai", "deepseek").
	ID() string
	// Call sends an OpenAI-formatted request to the provider and returns the raw response.
	Call(ctx context.Context, apiKey, baseURL string, body []byte) (*http.Response, error)
	// NativeFormat returns the provider's native API format.
	NativeFormat() translator.Format
}

// DefaultHTTPProvider is a generic OpenAI-compatible HTTP provider.
type DefaultHTTPProvider struct {
	id     string
	format translator.Format
}

// New creates a DefaultHTTPProvider for OpenAI-compatible APIs.
func New(id string) *DefaultHTTPProvider {
	return &DefaultHTTPProvider{id: id, format: translator.FormatOpenAI}
}

func (p *DefaultHTTPProvider) ID() string                 { return p.id }
func (p *DefaultHTTPProvider) NativeFormat() translator.Format { return p.format }

func (p *DefaultHTTPProvider) Call(ctx context.Context, apiKey, baseURL string, body []byte) (*http.Response, error) {
	url := baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Body = io.NopCloser(bytes.NewReader(body))
	return http.DefaultClient.Do(req)
}
