package executor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
)

// SSEEvent represents a single SSE event (data field).
type SSEEvent struct {
	Data []byte // content after "data: " prefix
	Raw  []byte // full line including "data: " prefix + \n\n
}

// SSEScanner reads SSE streams line by line and assembles data events.
type SSEScanner struct {
	scanner *bufio.Scanner
	buf     bytes.Buffer
	err     error
}

// NewSSEScanner creates an SSE scanner from a reader.
func NewSSEScanner(r io.Reader) *SSEScanner {
	return &SSEScanner{
		scanner: bufio.NewScanner(r),
	}
}

// Scan advances to the next SSE event. Returns false when the stream ends.
func (s *SSEScanner) Scan() bool {
	if s.err != nil {
		return false
	}

	s.buf.Reset()
	for s.scanner.Scan() {
		line := s.scanner.Bytes()

		// Empty line = event boundary.
		if len(line) == 0 {
			if s.buf.Len() > 0 {
				return true
			}
			continue
		}

		s.buf.Write(line)
		s.buf.WriteByte('\n')
	}

	s.err = s.scanner.Err()
	return false
}

// Event returns the current SSE event.
func (s *SSEScanner) Event() *SSEEvent {
	raw := s.buf.Bytes()
	if len(raw) == 0 {
		return nil
	}

	data := parseSSEData(raw)
	if data == nil {
		return nil
	}

	return &SSEEvent{
		Data: data,
		Raw:  raw,
	}
}

// Err returns any scanner error.
func (s *SSEScanner) Err() error {
	if s.err != nil {
		return s.err
	}
	return s.scanner.Err()
}

// parseSSEData extracts the value of the first "data: " line.
func parseSSEData(raw []byte) []byte {
	prefix := []byte("data: ")
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if bytes.HasPrefix(line, prefix) {
			data := bytes.TrimSpace(bytes.TrimPrefix(line, prefix))
			if len(data) > 0 && string(data) != "[DONE]" {
				return data
			}
			return nil // [DONE] marker
		}
	}
	return nil
}

// extractUsage parses token usage from an SSE data chunk.
func extractStreamUsage(data []byte) *Usage {
	var chunk struct {
		Choices []struct{} `json:"choices"`
		Usage   *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil
	}
	if chunk.Usage == nil {
		return nil
	}
	return &Usage{
		PromptTokens:     chunk.Usage.PromptTokens,
		CompletionTokens: chunk.Usage.CompletionTokens,
		TotalTokens:      chunk.Usage.TotalTokens,
	}
}

// extractUsage parses token usage from a non-stream response body.
func extractUsage(body []byte) *Usage {
	var resp struct {
		Usage *struct {
			PromptTokens         int `json:"prompt_tokens"`
			CompletionTokens     int `json:"completion_tokens"`
			TotalTokens          int `json:"total_tokens"`
			PromptTokensDetails  *struct {
				CachedTokens    int `json:"cached_tokens"`
				ReasoningTokens int `json:"reasoning_tokens"`
			} `json:"prompt_tokens_details"`
			CompletionTokenDetails *struct {
				ReasoningTokens int `json:"reasoning_tokens"`
			} `json:"completion_token_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}
	if resp.Usage == nil {
		return nil
	}
	usage := &Usage{
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
	}
	if resp.Usage.PromptTokensDetails != nil {
		usage.CacheHitTokens = resp.Usage.PromptTokensDetails.CachedTokens
		usage.ReasoningTokens = resp.Usage.PromptTokensDetails.ReasoningTokens
	}
	if resp.Usage.CompletionTokenDetails != nil {
		usage.ReasoningTokens = resp.Usage.CompletionTokenDetails.ReasoningTokens
	}
	return usage
}
