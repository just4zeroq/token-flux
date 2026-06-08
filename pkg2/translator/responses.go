package translator

import (
	"encoding/json"
	"fmt"
)

// ---- Responses → OpenAI request ----

func responsesToOpenAI(body []byte) ([]byte, error) {
	var rr responsesRequest
	if err := json.Unmarshal(body, &rr); err != nil {
		return nil, fmt.Errorf("parse responses request: %w", err)
	}

	messages := make([]map[string]any, 0)

	// Instructions → system message
	instructions := extractStr(rr.Instructions)
	if instructions != "" {
		messages = append(messages, map[string]any{"role": "system", "content": instructions})
	}

	// Input → messages
	inputMsgs := responsesInputToMessages(rr.Input)
	messages = append(messages, inputMsgs...)

	req := map[string]any{
		"model":    rr.Model,
		"messages": messages,
	}
	if rr.MaxOutputTokens > 0 {
		req["max_tokens"] = rr.MaxOutputTokens
	}
	if rr.Temperature > 0 {
		req["temperature"] = rr.Temperature
	}
	if rr.TopP > 0 {
		req["top_p"] = rr.TopP
	}

	return json.Marshal(req)
}

type responsesRequest struct {
	Model           string          `json:"model"`
	Input           json.RawMessage `json:"input"`
	Instructions    json.RawMessage `json:"instructions"`
	MaxOutputTokens int             `json:"max_output_tokens,omitempty"`
	Temperature     float64         `json:"temperature,omitempty"`
	TopP            float64         `json:"top_p,omitempty"`
}

func extractStr(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	json.Unmarshal(raw, &s)
	return s
}

func responsesInputToMessages(input json.RawMessage) []map[string]any {
	if len(input) == 0 {
		return nil
	}
	var s string
	if err := json.Unmarshal(input, &s); err == nil {
		return []map[string]any{{"role": "user", "content": s}}
	}
	var items []json.RawMessage
	if err := json.Unmarshal(input, &items); err != nil {
		return nil
	}
	var msgs []map[string]any
	for _, raw := range items {
		var item struct {
			Type    string          `json:"type"`
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
			CallID  string          `json:"call_id,omitempty"`
			Output  string          `json:"output,omitempty"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		if item.Type == "function_call_output" {
			msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": item.CallID, "content": item.Output})
			continue
		}
		role := item.Role
		if role == "" {
			role = "user"
		}
		var text string
		if err := json.Unmarshal(item.Content, &text); err == nil {
			msgs = append(msgs, map[string]any{"role": role, "content": text})
		}
	}
	return msgs
}

// ---- OpenAI → Responses response ----

type responsesDenorm struct{}

func newResponsesDenorm() *responsesDenorm { return &responsesDenorm{} }

func (d *responsesDenorm) Header() string { return "application/json" }

func (d *responsesDenorm) ConvertBody(body []byte) ([]byte, error) {
	var chat struct {
		ID      string `json:"id"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &chat); err != nil {
		return body, nil
	}

	output := make([]map[string]any, 0)
	if len(chat.Choices) > 0 && chat.Choices[0].Message.Content != "" {
		output = append(output, map[string]any{
			"type":    "message",
			"id":      "msg_" + chat.ID,
			"status":  "completed",
			"role":    "assistant",
			"content": []map[string]any{{"type": "output_text", "text": chat.Choices[0].Message.Content}},
		})
	}

	return json.Marshal(map[string]any{
		"id":      "resp_" + chat.ID,
		"object":  "response",
		"status":  "completed",
		"model":   chat.Model,
		"output":  output,
		"usage": map[string]any{
			"input_tokens":  chat.Usage.PromptTokens,
			"output_tokens": chat.Usage.CompletionTokens,
			"total_tokens":  chat.Usage.TotalTokens,
		},
	})
}

func (d *responsesDenorm) ConvertChunk(chunk []byte) ([]byte, error) { return chunk, nil }
func (d *responsesDenorm) Finalize() ([]byte, error)                { return nil, nil }
