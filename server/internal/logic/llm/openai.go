package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

type normalizedUsage struct {
	InputTokens     int
	CacheHitTokens  int
	CacheMissTokens int
	OutputTokens    int
	TotalTokens     int
}

// proxyOpenAICompatible calls an OpenAI-compatible upstream endpoint.
// Returns status, body, response headers (Content-Type), and error.
func proxyOpenAICompatible(ctx context.Context, baseURL, apiKey, path string, body []byte) (int, []byte, map[string]string, error) {
	fullURL := strings.TrimRight(baseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, gerror.Wrap(err, "build upstream request failed")
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, nil, gerror.Wrap(err, "upstream call failed")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, nil, gerror.Wrap(err, "read upstream response failed")
	}

	headers := map[string]string{}
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		headers["Content-Type"] = ct
	}
	return resp.StatusCode, respBody, headers, nil
}

func extractRequestedModel(body []byte) (string, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return "", gerror.Wrap(err, "parse request body failed")
	}
	raw, ok := m["model"]
	if !ok {
		return "", gerror.New("missing 'model' field in request")
	}
	var model string
	if err := json.Unmarshal(raw, &model); err != nil {
		return "", gerror.Wrap(err, "'model' field is not a string")
	}
	if model == "" {
		return "", gerror.New("'model' field is empty")
	}
	return model, nil
}

func replaceRequestModel(body []byte, upstreamModel string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, gerror.Wrap(err, "parse request body failed")
	}
	newModel, err := json.Marshal(upstreamModel)
	if err != nil {
		return nil, gerror.Wrap(err, "marshal upstream model failed")
	}
	m["model"] = newModel
	out, err := json.Marshal(m)
	if err != nil {
		return nil, gerror.Wrap(err, "re-marshal request body failed")
	}
	return out, nil
}

// normalizeOpenAIUsage extracts usage statistics from an OpenAI-compatible response.
// Supports OpenAI prompt_tokens_details.cached_tokens and DeepSeek-style
// prompt_cache_hit_tokens / prompt_cache_miss_tokens.
func normalizeOpenAIUsage(body []byte) normalizedUsage {
	var parsed struct {
		Usage struct {
			PromptTokens         int `json:"prompt_tokens"`
			CompletionTokens     int `json:"completion_tokens"`
			TotalTokens          int `json:"total_tokens"`
			PromptCacheHitTokens int `json:"prompt_cache_hit_tokens"`
			PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
			PromptTokensDetails  struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(body, &parsed)

	u := normalizedUsage{
		InputTokens:  parsed.Usage.PromptTokens,
		OutputTokens: parsed.Usage.CompletionTokens,
		TotalTokens:  parsed.Usage.TotalTokens,
	}

	switch {
	case parsed.Usage.PromptCacheHitTokens > 0 || parsed.Usage.PromptCacheMissTokens > 0:
		u.CacheHitTokens = parsed.Usage.PromptCacheHitTokens
		u.CacheMissTokens = parsed.Usage.PromptCacheMissTokens
	case parsed.Usage.PromptTokensDetails.CachedTokens > 0:
		u.CacheHitTokens = parsed.Usage.PromptTokensDetails.CachedTokens
		u.CacheMissTokens = u.InputTokens - u.CacheHitTokens
		if u.CacheMissTokens < 0 {
			u.CacheMissTokens = 0
		}
	default:
		u.CacheHitTokens = 0
		u.CacheMissTokens = u.InputTokens
	}

	if u.TotalTokens == 0 {
		u.TotalTokens = u.InputTokens + u.OutputTokens
	}
	return u
}
