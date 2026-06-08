package modelspec

import (
	"fmt"
	"time"

	"ai-platform-node/pkg/db"
)

type ModelSpec struct {
	ID                int64    `json:"id"`
	DeveloperName     string   `json:"developer_name"`
	ModelName         string   `json:"model_name"`
	ModelCode         string   `json:"model_code"`
	DisplayName       string   `json:"display_name"`
	ModelFamily       string   `json:"model_family"`
	Description       string   `json:"description"`
	Capabilities      []string `json:"capabilities"`
	ContextWindow     int      `json:"context_window"`
	MaxInputTokens    int      `json:"max_input_tokens"`
	MaxOutputTokens   int      `json:"max_output_tokens"`
	SupportsStream    bool     `json:"supports_stream"`
	SupportsTools     bool     `json:"supports_tools"`
	SupportsVision    bool     `json:"supports_vision"`
	SupportsJsonMode  bool     `json:"supports_json_mode"`
	SupportsReasoning bool     `json:"supports_reasoning"`
	SupportsLogprobs  bool     `json:"supports_logprobs"`
	Status            string   `json:"status"`
	Source            string   `json:"source"`
	CreatedAt         int64    `json:"created_at"`
	UpdatedAt         int64    `json:"updated_at"`
}

func List() ([]ModelSpec, error) {
	rows, err := db.DB().Query(
		`SELECT id, developer_name, model_name, model_code, display_name, model_family,
		        description, capabilities_json, context_window, max_input_tokens, max_output_tokens,
		        supports_stream, supports_tools, supports_vision,
		        supports_json_mode, supports_reasoning, supports_logprobs,
		        status, source, created_at, updated_at
		 FROM llm_model_specs ORDER BY developer_name, model_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModelSpec
	for rows.Next() {
		var s ModelSpec
		var capsJSON string
		var stream, tools, vision, jsonMode, reasoning, logprobs int
		if err := rows.Scan(&s.ID, &s.DeveloperName, &s.ModelName, &s.ModelCode, &s.DisplayName,
			&s.ModelFamily, &s.Description, &capsJSON, &s.ContextWindow, &s.MaxInputTokens, &s.MaxOutputTokens,
			&stream, &tools, &vision, &jsonMode, &reasoning, &logprobs,
			&s.Status, &s.Source, &s.CreatedAt, &s.UpdatedAt); err != nil {
			continue
		}
		s.SupportsStream = stream == 1
		s.SupportsTools = tools == 1
		s.SupportsVision = vision == 1
		s.SupportsJsonMode = jsonMode == 1
		s.SupportsReasoning = reasoning == 1
		s.SupportsLogprobs = logprobs == 1
		s.Capabilities = parseCaps(capsJSON)
		out = append(out, s)
	}
	return out, nil
}

func GetByID(id int64) (*ModelSpec, error) {
	var s ModelSpec
	var capsJSON string
	var stream, tools, vision, jsonMode, reasoning, logprobs int
	err := db.DB().QueryRow(
		`SELECT id, developer_name, model_name, model_code, display_name, model_family,
		        description, capabilities_json, context_window, max_input_tokens, max_output_tokens,
		        supports_stream, supports_tools, supports_vision,
		        supports_json_mode, supports_reasoning, supports_logprobs,
		        status, source, created_at, updated_at
		 FROM llm_model_specs WHERE id = ?`, id,
	).Scan(&s.ID, &s.DeveloperName, &s.ModelName, &s.ModelCode, &s.DisplayName,
		&s.ModelFamily, &s.Description, &capsJSON, &s.ContextWindow, &s.MaxInputTokens, &s.MaxOutputTokens,
		&stream, &tools, &vision, &jsonMode, &reasoning, &logprobs,
		&s.Status, &s.Source, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("modelspec %d: %w", id, err)
	}
	s.SupportsStream = stream == 1
	s.SupportsTools = tools == 1
	s.SupportsVision = vision == 1
	s.SupportsJsonMode = jsonMode == 1
	s.SupportsReasoning = reasoning == 1
	s.SupportsLogprobs = logprobs == 1
	s.Capabilities = parseCaps(capsJSON)
	return &s, nil
}

func Create(developerName, modelName, modelCode, displayName, modelFamily, description, capabilitiesJSON string,
	contextWindow, maxInputTokens, maxOutputTokens int,
	supportsStream, supportsTools, supportsVision, supportsJsonMode, supportsReasoning, supportsLogprobs bool) (*ModelSpec, error) {

	now := time.Now().Unix()
	stream := boolInt(supportsStream)
	tools := boolInt(supportsTools)
	vision := boolInt(supportsVision)
	jsonMode := boolInt(supportsJsonMode)
	reasoning := boolInt(supportsReasoning)
	logprobs := boolInt(supportsLogprobs)

	if capabilitiesJSON == "" {
		capabilitiesJSON = "[]"
	}

	_, err := db.DB().Exec(
		`INSERT INTO llm_model_specs(developer_name, model_name, model_code, display_name, model_family,
		 description, capabilities_json, context_window, max_input_tokens, max_output_tokens,
		 supports_stream, supports_tools, supports_vision,
		 supports_json_mode, supports_reasoning, supports_logprobs,
		 status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
		developerName, modelName, modelCode, displayName, modelFamily,
		description, capabilitiesJSON, contextWindow, maxInputTokens, maxOutputTokens,
		stream, tools, vision, jsonMode, reasoning, logprobs, now, now)
	if err != nil {
		return nil, fmt.Errorf("create modelspec: %w", err)
	}

	var id int64
	db.DB().QueryRow("SELECT last_insert_rowid()").Scan(&id)
	return GetByID(id)
}

func Update(id int64, displayName, status string) (*ModelSpec, error) {
	now := time.Now().Unix()
	_, err := db.DB().Exec(
		`UPDATE llm_model_specs SET display_name = ?, status = ?, updated_at = ? WHERE id = ?`,
		displayName, status, now, id)
	if err != nil {
		return nil, fmt.Errorf("update modelspec: %w", err)
	}
	return GetByID(id)
}

func Delete(id int64) error {
	_, err := db.DB().Exec("DELETE FROM llm_model_specs WHERE id = ?", id)
	return err
}

func parseCaps(jsonStr string) []string {
	if jsonStr == "" || jsonStr == "[]" {
		return nil
	}
	var caps []string
	s := jsonStr
	if len(s) >= 2 {
		s = s[1 : len(s)-1]
	}
	if s == "" {
		return nil
	}
	var current []byte
	inQuote := false
	for _, c := range []byte(s) {
		switch c {
		case '"':
			inQuote = !inQuote
		case ',':
			if !inQuote {
				if len(current) > 0 {
					caps = append(caps, string(current))
				}
				current = nil
			} else {
				current = append(current, c)
			}
		case ' ', '\n', '\r', '\t':
			if inQuote {
				current = append(current, c)
			}
		default:
			current = append(current, c)
		}
	}
	if len(current) > 0 {
		caps = append(caps, string(current))
	}
	return caps
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
