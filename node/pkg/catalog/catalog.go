// Package catalog fetches public AI model/provider listings from the platform
// and syncs them into the local SQLite database.
package catalog

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"ai-platform-node/pkg/db"
)

// Provider represents a developer/provider from the platform catalog.
type Provider struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Website         string `json:"website"`
	LogoURL         string `json:"logo_url"`
	ReputationScore int    `json:"reputation_score"`
	ModelCount      int    `json:"model_count"`
}

// Model represents a model spec from the platform catalog.
type Model struct {
	ID             int    `json:"id"`
	DeveloperName  string `json:"developer_name"`
	ModelName      string `json:"model_name"`
	ModelCode      string `json:"model_code"`
	DisplayName    string `json:"display_name"`
	ModelFamily    string `json:"model_family"`
	Capabilities   string `json:"capabilities"`
	ContextWindow  int    `json:"context_window"`
	Status         string `json:"status"`
}

// Channel represents a channel from the platform catalog.
type Channel struct {
	ID          int    `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

// ChannelModel represents a channel-model binding from the platform catalog.
type ChannelModel struct {
	ChannelID      int    `json:"channel_id"`
	ChannelName    string `json:"channel_name"`
	ModelSpecID    int    `json:"model_spec_id"`
	ModelCode      string `json:"model_code"`
	ModelName      string `json:"model_name"`
	DeveloperName  string `json:"developer_name"`
}

// PlatformURL returns the configured platform URL.
func PlatformURL() string {
	url, _ := db.GetConfig("platform_url")
	if url == "" {
		url = "http://localhost:8080" // default api port
	}
	return url
}

// FetchProviders gets the list of providers/developers from the platform catalog.
func FetchProviders() ([]Provider, error) {
	base := PlatformURL()
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/catalog/providers", base))
	if err != nil {
		return nil, fmt.Errorf("fetch providers: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data *struct {
			List []Provider `json:"list"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode providers: %w", err)
	}
	if result.Data == nil {
		return []Provider{}, nil
	}
	return result.Data.List, nil
}

// FetchModels gets all active model specs from the platform catalog.
func FetchModels() ([]Model, error) {
	base := PlatformURL()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/api/v1/catalog/models", base))
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data *struct {
			List []Model `json:"list"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}
	if result.Data == nil {
		return []Model{}, nil
	}
	return result.Data.List, nil
}

// FetchProviderModels returns models with pricing for a specific provider.
func FetchProviderModels(providerName string) ([]map[string]any, error) {
	base := PlatformURL()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/api/v1/catalog/providers/%s/models", base, providerName))
	if err != nil {
		return nil, fmt.Errorf("fetch provider models: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data *struct {
			Models []map[string]any `json:"models"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if result.Data == nil {
		return []map[string]any{}, nil
	}
	return result.Data.Models, nil
}

// FetchChannels gets active admin channels from the platform catalog.
func FetchChannels() ([]Channel, error) {
	base := PlatformURL()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/api/v1/catalog/channels", base))
	if err != nil {
		return nil, fmt.Errorf("fetch channels: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data *struct {
			List []Channel `json:"list"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode channels: %w", err)
	}
	if result.Data == nil {
		return []Channel{}, nil
	}
	return result.Data.List, nil
}

// FetchChannelModels gets all admin channel-model bindings from the platform.
func FetchChannelModels() ([]ChannelModel, error) {
	base := PlatformURL()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/api/v1/catalog/channel-models", base))
	if err != nil {
		return nil, fmt.Errorf("fetch channel-models: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data *struct {
			List []ChannelModel `json:"list"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode channel-models: %w", err)
	}
	if result.Data == nil {
		return []ChannelModel{}, nil
	}
	return result.Data.List, nil
}

// SyncAll fetches channels, models, and channel-model bindings from the platform
// and replaces all platform-sourced data in the local SQLite database.
func SyncAll() error {
	// Fetch from platform
	channels, err := FetchChannels()
	if err != nil {
		return fmt.Errorf("sync channels: %w", err)
	}
	models, err := FetchModels()
	if err != nil {
		return fmt.Errorf("sync models: %w", err)
	}
	bindings, err := FetchChannelModels()
	if err != nil {
		return fmt.Errorf("sync channel-models: %w", err)
	}

	// Clear old platform data
	db.DB().Exec("DELETE FROM llm_channel_models WHERE source = 'platform'")
	db.DB().Exec("DELETE FROM llm_model_specs WHERE source = 'platform'")
	db.DB().Exec("DELETE FROM llm_channels WHERE source = 'platform'")

	// Insert fresh channels
	for _, c := range channels {
		db.DB().Exec(`INSERT INTO llm_channels
			(id, code, name, description, source, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, 'platform', ?, ?, ?)`,
			c.ID, c.Code, c.Name, c.Description, c.Status, now(), now())
	}

	// Insert fresh models
	for _, m := range models {
		caps := []string{}
		if m.Capabilities != "" {
			caps = append(caps, m.Capabilities)
		}
		capsJSON, _ := json.Marshal(caps)
		db.DB().Exec(`INSERT INTO llm_model_specs
			(id, developer_name, model_name, model_code, display_name, model_family,
			 description, capabilities_json, context_window, source, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'platform', ?, ?, ?)`,
			m.ID, m.DeveloperName, m.ModelName, m.ModelCode, m.DisplayName, m.ModelFamily,
			"", string(capsJSON), m.ContextWindow, m.Status, now(), now())
	}

	// Insert fresh channel-model bindings
	for _, b := range bindings {
		db.DB().Exec(`INSERT INTO llm_channel_models
			(channel_id, model_spec_id, upstream_model_name, source, created_at, updated_at)
			VALUES (?, ?, ?, 'platform', ?, ?)`,
			b.ChannelID, b.ModelSpecID, b.ModelCode, now(), now())
	}

	return nil
}

func now() int64 { return time.Now().Unix() }

