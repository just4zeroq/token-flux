package translator

import (
	"encoding/json"
	"fmt"
)

// ---- Claude → OpenAI request ----

func claudeToOpenAI(body []byte) ([]byte, error) {
	var cr claudeRequest
	if err := json.Unmarshal(body, &cr); err != nil {
		return nil, fmt.Errorf("parse claude request: %w", err)
	}

	messages := make([]map[string]any, 0)

	// System prompt
	if cr.System != "" {
		messages = append(messages, map[string]any{"role": "system", "content": cr.System})
	}

	// Messages
	for _, msg := range cr.Messages {
		role := "user"
		if msg.Role == "assistant" {
			role = "assistant"
		}

		switch v := msg.Content.(type) {
		case string:
			messages = append(messages, map[string]any{"role": role, "content": v})
		case []any:
			// Multi-content block (text + image + tool_use)
			parts := make([]map[string]any, 0)
			for _, block := range v {
				bm, _ := block.(map[string]any)
				if bm == nil {
					continue
				}
				switch bm["type"] {
				case "text":
					parts = append(parts, map[string]any{"type": "text", "text": bm["text"]})
				case "image":
					if src, ok := bm["source"].(map[string]any); ok {
						parts = append(parts, map[string]any{
							"type": "image_url",
							"image_url": map[string]any{
								"url": fmt.Sprintf("data:%s;base64,%s", src["media_type"], src["data"]),
							},
						})
					}
				case "tool_use":
					messages = append(messages, map[string]any{
						"role": "assistant",
						"tool_calls": []map[string]any{{
							"id":       bm["id"],
							"type":     "function",
							"function": map[string]any{"name": bm["name"], "arguments": fmt.Sprint(bm["input"])},
						}},
					})
					continue
				case "tool_result":
					messages = append(messages, map[string]any{
						"role":         "tool",
						"tool_call_id": bm["tool_use_id"],
						"content":      fmt.Sprint(bm["content"]),
					})
					continue
				}
			}
			if len(parts) > 0 {
				messages = append(messages, map[string]any{"role": role, "content": parts})
			}
		}
	}

	req := map[string]any{
		"model":    cr.Model,
		"messages": messages,
	}
	if cr.MaxTokens > 0 {
		req["max_tokens"] = cr.MaxTokens
	}
	if cr.Temperature > 0 {
		req["temperature"] = cr.Temperature
	}
	if cr.TopP > 0 {
		req["top_p"] = cr.TopP
	}

	// Tools
	if len(cr.Tools) > 0 {
		tools := make([]map[string]any, 0)
		for _, t := range cr.Tools {
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.InputSchema,
				},
			})
		}
		req["tools"] = tools
	}

	return json.Marshal(req)
}

type claudeRequest struct {
	Model       string         `json:"model"`
	Messages    []claudeMessage `json:"messages"`
	System      string         `json:"system,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Temperature float64        `json:"temperature,omitempty"`
	TopP        float64        `json:"top_p,omitempty"`
	Tools       []claudeTool    `json:"tools,omitempty"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type claudeTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema"`
}

// ---- OpenAI → Claude response ----

type claudeDenorm struct {
	isStream bool
	id       int
}

func newClaudeDenorm(isStream bool) *claudeDenorm { return &claudeDenorm{isStream: isStream} }

func (d *claudeDenorm) Header() string {
	if d.isStream {
		return "text/event-stream"
	}
	return "application/json"
}

func (d *claudeDenorm) ConvertBody(body []byte) ([]byte, error) {
	var chat struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			InputTokens  int `json:"prompt_tokens"`
			OutputTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &chat); err != nil {
		return body, nil
	}
	d.id++
	resp := map[string]any{
		"id":         fmt.Sprintf("msg_%s", chat.ID),
		"type":       "message",
		"role":       "assistant",
		"model":      chat.Model,
		"stop_reason": "end_turn",
	}
	content := []map[string]any{}
	if len(chat.Choices) > 0 && chat.Choices[0].Message.Content != "" {
		content = append(content, map[string]any{"type": "text", "text": chat.Choices[0].Message.Content})
	}
	resp["content"] = content
	if chat.Usage.InputTokens > 0 || chat.Usage.OutputTokens > 0 {
		resp["usage"] = map[string]any{
			"input_tokens":  chat.Usage.InputTokens,
			"output_tokens": chat.Usage.OutputTokens,
		}
	}
	return json.Marshal(resp)
}

func (d *claudeDenorm) ConvertChunk(chunk []byte) ([]byte, error) {
	// For simplicity in v1: check if it's an OpenAI SSE chunk with content.
	// Strip "data: " prefix.
	data := trimSSEPrefix(chunk)
	if data == nil {
		return nil, nil
	}
	var c struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, nil
	}
	if len(c.Choices) > 0 && c.Choices[0].Delta.Content != "" {
		d.id++
		event := fmt.Sprintf(
			`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%q}}

`, c.Choices[0].Delta.Content)
		return []byte(event), nil
	}
	return nil, nil
}

func (d *claudeDenorm) Finalize() ([]byte, error) {
	return []byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"), nil
}

func trimSSEPrefix(b []byte) []byte {
	s := string(b)
	if len(s) > 6 && s[:6] == "data: " {
		r := []byte(s[6:])
		if len(r) > 0 && string(r) == "[DONE]" {
			return nil
		}
		return r
	}
	return b
}
