package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"ai-platform-node/pkg/keychain"
	"ai-platform-node/pkg/types"
	pkgprovider "ai-platform/pkg/provider"
	"ai-platform/pkg/translator"
)

// defaultProvider is a shared OpenAI-compatible HTTP provider used by the node.
var defaultProvider = pkgprovider.New("openai")

// Execute forwards a chat request to the upstream provider using pkg/provider + pkg/translator.
func Execute(ctx context.Context, req *types.RequestEnvelope, keyHash string) (*types.ChatResponse, error) {
	entry, err := keychain.FindKeyByHash(keyHash)
	if err != nil {
		return nil, fmt.Errorf("resolve key: %w", err)
	}

	body, _ := json.Marshal(req.Request)

	// Normalize to OpenAI format if needed (node receives raw format from local tools).
	normalized, err := translator.Normalize(body, translator.FormatOpenAI)
	if err != nil {
		return nil, fmt.Errorf("normalize: %w", err)
	}

	baseURL := urlFor(entry)
	resp, err := defaultProvider.Call(ctx, entry.Key, baseURL, normalized)
	if err != nil {
		return nil, fmt.Errorf("call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(b))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var cr types.ChatResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &cr, nil
}

// ExecuteStream forwards a streaming request using the shared provider + translator.
func ExecuteStream(ctx context.Context, req *types.RequestEnvelope, keyHash string) (<-chan *types.ChunkData, <-chan error) {
	chunks := make(chan *types.ChunkData, 64)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		entry, err := keychain.FindKeyByHash(keyHash)
		if err != nil {
			errs <- err
			return
		}
		req.Request.Stream = true
		body, _ := json.Marshal(req.Request)

		normalized, err := translator.Normalize(body, translator.FormatOpenAI)
		if err != nil {
			errs <- err
			return
		}

		baseURL := urlFor(entry)
		resp, err := defaultProvider.Call(ctx, entry.Key, baseURL, normalized)
		if err != nil {
			errs <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			errs <- fmt.Errorf("upstream %d: %s", resp.StatusCode, string(b))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}
			var chunk types.ChunkData
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			select {
			case chunks <- &chunk:
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			}
		}
	}()

	return chunks, errs
}

func urlFor(e *types.KeyEntry) string {
	if e.BaseURL != "" {
		return strings.TrimRight(e.BaseURL, "/")
	}
	return "https://api.openai.com/v1"
}
