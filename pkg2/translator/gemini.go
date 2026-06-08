package translator

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ---- Gemini → OpenAI request ----

// OpenAIToGemini converts an OpenAI Chat request body to Gemini format.
// Pure data conversion—does not depend on executor types.
func OpenAIToGemini(body []byte) ([]byte, error) {
	var openaiReq struct {
		Model       string              `json:"model"`
		Messages    []openaiRequestMsg  `json:"messages"`
		MaxTokens   *int                `json:"max_tokens"`
		Temperature *float64            `json:"temperature"`
		TopP        *float64            `json:"top_p"`
		Stream      bool                `json:"stream"`
		Tools       json.RawMessage     `json:"tools,omitempty"`
	}
	if err := json.Unmarshal(body, &openaiReq); err != nil {
		return nil, nil // passthrough on parse failure
	}

	geminiReq := map[string]any{
		"contents": convertMessagesToGemini(openaiReq.Messages),
	}

	genConfig := map[string]any{}
	if openaiReq.MaxTokens != nil {
		genConfig["max_output_tokens"] = *openaiReq.MaxTokens
	}
	if openaiReq.Temperature != nil {
		genConfig["temperature"] = *openaiReq.Temperature
	}
	if openaiReq.TopP != nil {
		genConfig["top_p"] = *openaiReq.TopP
	}
	if len(genConfig) > 0 {
		geminiReq["generationConfig"] = genConfig
	}

	if len(openaiReq.Tools) > 0 {
		geminiReq["tools"] = json.RawMessage(fmt.Sprintf(`[{"function_declarations":%s}]`, string(openaiReq.Tools)))
	}

	return json.Marshal(geminiReq)
}

type openaiRequestMsg struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

func convertMessagesToGemini(msgs []openaiRequestMsg) []map[string]any {
	var contents []map[string]any
	var systemParts []string

	for _, msg := range msgs {
		switch msg.Role {
		case "system":
			systemParts = append(systemParts, string(msg.Content))
		case "user":
			contents = append(contents, map[string]any{
				"role": "user",
				"parts": []map[string]any{
					{"text": string(msg.Content)},
				},
			})
		case "assistant":
			contents = append(contents, map[string]any{
				"role": "model",
				"parts": []map[string]any{
					{"text": string(msg.Content)},
				},
			})
		}
	}

	// Prepend system as first user message (Gemini has no system role).
	if len(systemParts) > 0 {
		contents = append([]map[string]any{{
			"role": "user",
			"parts": []map[string]any{
				{"text": "System instruction: " + strings.Join(systemParts, "\n")},
			},
		}}, contents...)
	}

	return contents
}

func geminiToOpenAI(body []byte) ([]byte, error) {
	var gr geminiRequest
	if err := json.Unmarshal(body, &gr); err != nil {
		return nil, fmt.Errorf("parse gemini request: %w", err)
	}

	messages := make([]map[string]any, 0)

	// systemInstruction
	if gr.SystemInstruction != nil {
		if text, ok := firstTextPart(gr.SystemInstruction.Parts); ok {
			messages = append(messages, map[string]any{"role": "system", "content": text})
		}
	}

	// contents → messages
	for _, c := range gr.Contents {
		role := "user"
		if c.Role == "model" {
			role = "assistant"
		}
		text, hasText := firstTextPart(c.Parts)
		if !hasText {
			// Check for functionCall
			for _, p := range c.Parts {
				if p.FunctionCall != nil {
					messages = append(messages, map[string]any{
						"role": "assistant",
						"tool_calls": []map[string]any{{
							"id":       p.FunctionCall.Name,
							"type":     "function",
							"function": map[string]any{"name": p.FunctionCall.Name, "arguments": p.FunctionCall.Args},
						}},
					})
				}
				if p.FunctionResponse != nil {
					messages = append(messages, map[string]any{
						"role":         "tool",
						"tool_call_id": p.FunctionResponse.Name,
						"content":      fmt.Sprint(p.FunctionResponse.Response),
					})
				}
			}
			continue
		}
		messages = append(messages, map[string]any{"role": role, "content": text})
	}

	req := map[string]any{
		"model":    gr.Model,
		"messages": messages,
	}
	if gr.GenerationConfig != nil {
		if gr.GenerationConfig.MaxOutputTokens > 0 {
			req["max_tokens"] = gr.GenerationConfig.MaxOutputTokens
		}
		if gr.GenerationConfig.Temperature > 0 {
			req["temperature"] = gr.GenerationConfig.Temperature
		}
		if gr.GenerationConfig.TopP > 0 {
			req["top_p"] = gr.GenerationConfig.TopP
		}
	}

	// Tools from functionDeclarations
	if len(gr.Tools) > 0 {
		tools := make([]map[string]any, 0)
		for _, td := range gr.Tools {
			for _, fd := range td.FunctionDeclarations {
				tools = append(tools, map[string]any{
					"type": "function",
					"function": map[string]any{
						"name":        fd.Name,
						"description": fd.Description,
						"parameters":  fd.Parameters,
					},
				})
			}
		}
		req["tools"] = tools
	}

	return json.Marshal(req)
}

type geminiRequest struct {
	Model              string             `json:"model"`
	Contents           []geminiContent    `json:"contents"`
	SystemInstruction  *geminiInstruction `json:"systemInstruction,omitempty"`
	GenerationConfig   *geminiGenConfig   `json:"generationConfig,omitempty"`
	Tools              []geminiToolDecl   `json:"tools,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                `json:"text,omitempty"`
	FunctionCall     *geminiFuncCall       `json:"functionCall,omitempty"`
	FunctionResponse *geminiFuncResponse   `json:"functionResponse,omitempty"`
}

type geminiFuncCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type geminiFuncResponse struct {
	Name     string `json:"name"`
	Response any    `json:"response"`
}

type geminiInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type geminiGenConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
}

type geminiToolDecl struct {
	FunctionDeclarations []geminiFuncDecl `json:"functionDeclarations"`
}

type geminiFuncDecl struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

func firstTextPart(parts []geminiPart) (string, bool) {
	for _, p := range parts {
		if p.Text != "" {
			return p.Text, true
		}
	}
	return "", false
}

// ---- OpenAI → Gemini response ----

type geminiDenorm struct{}

func newGeminiDenorm() *geminiDenorm { return &geminiDenorm{} }

func (d *geminiDenorm) Header() string { return "application/json" }

func (d *geminiDenorm) ConvertBody(body []byte) ([]byte, error) {
	var chat struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &chat); err != nil {
		return body, nil
	}
	parts := []map[string]any{}
	if len(chat.Choices) > 0 && chat.Choices[0].Message.Content != "" {
		parts = append(parts, map[string]any{"text": chat.Choices[0].Message.Content})
	}
	return json.Marshal(map[string]any{
		"candidates": []map[string]any{{
			"content": map[string]any{
				"role":  "model",
				"parts": parts,
			},
		}},
	})
}

func (d *geminiDenorm) ConvertChunk(chunk []byte) ([]byte, error) {
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
		event := fmt.Sprintf(`data: {"candidates":[{"content":{"role":"model","parts":[{"text":%q}]}}]}`, c.Choices[0].Delta.Content)
		return []byte(event + "\n\n"), nil
	}
	return nil, nil
}

func (d *geminiDenorm) Finalize() ([]byte, error) { return nil, nil }
