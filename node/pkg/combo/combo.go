package combo

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"ai-platform-node/pkg/db"
)

// ComboConfig defines a named list of models with routing strategy.
type ComboConfig struct {
	Name     string   `json:"name"`
	Models   []string `json:"models"`   // ordered list of model codes
	Strategy string   `json:"strategy"` // "fallback" | "round-robin"
	Sticky   int      `json:"sticky"`   // requests before rotate (round-robin)
	Created  int64    `json:"created_at"`
}

// ComboState tracks the current round-robin position per combo.
type ComboState struct {
	mu          sync.Mutex
	CurrentIdx  int
	StickCount  int
	LastUsedAt  time.Time
}

var (
	states  = make(map[string]*ComboState)
	statesMu sync.RWMutex
)

// List returns all combos from SQLite.
func List() ([]ComboConfig, error) {
	rows, err := db.DB().Query("SELECT name, models, strategy, sticky, created_at FROM node_combos ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ComboConfig
	for rows.Next() {
		var c ComboConfig
		var modelsJSON string
		if err := rows.Scan(&c.Name, &modelsJSON, &c.Strategy, &c.Sticky, &c.Created); err != nil {
			continue
		}
		json.Unmarshal([]byte(modelsJSON), &c.Models)
		out = append(out, c)
	}
	return out, nil
}

// Create creates a new combo.
func Create(name string, models []string, strategy string, sticky int) (*ComboConfig, error) {
	if name == "" || len(models) == 0 {
		return nil, fmt.Errorf("name and models required")
	}
	if strategy == "" {
		strategy = "fallback"
	}
	if sticky <= 0 {
		sticky = 1
	}
	modelsJSON, _ := json.Marshal(models)
	now := time.Now().Unix()

	_, err := db.DB().Exec(
		"INSERT OR REPLACE INTO node_combos(name, models, strategy, sticky, created_at) VALUES (?, ?, ?, ?, ?)",
		name, string(modelsJSON), strategy, sticky, now)
	if err != nil {
		return nil, err
	}

	statesMu.Lock()
	states[name] = &ComboState{CurrentIdx: 0, StickCount: 0}
	statesMu.Unlock()

	return &ComboConfig{Name: name, Models: models, Strategy: strategy, Sticky: sticky, Created: now}, nil
}

// Delete removes a combo.
func Delete(name string) error {
	_, err := db.DB().Exec("DELETE FROM node_combos WHERE name = ?", name)
	statesMu.Lock()
	delete(states, name)
	statesMu.Unlock()
	return err
}

// Resolve returns the next model to use for a given combo.
// For "fallback" strategy, always returns the first available model.
// For "round-robin", rotates based on sticky count.
func Resolve(name string) (string, error) {
	statesMu.RLock()
	s, exists := states[name]
	statesMu.RUnlock()

	if !exists {
		statesMu.Lock()
		s = &ComboState{CurrentIdx: 0, StickCount: 0}
		states[name] = s
		statesMu.Unlock()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Load combo config
	var modelsJSON string
	err := db.DB().QueryRow("SELECT models FROM node_combos WHERE name = ?", name).Scan(&modelsJSON)
	if err != nil {
		return "", fmt.Errorf("combo %q not found", name)
	}
	var models []string
	json.Unmarshal([]byte(modelsJSON), &models)
	if len(models) == 0 {
		return "", fmt.Errorf("combo %q has no models", name)
	}

	// Get strategy
	var strategy string
	var sticky int
	db.DB().QueryRow("SELECT strategy, sticky FROM node_combos WHERE name = ?", name).Scan(&strategy, &sticky)

	switch strategy {
	case "round-robin":
		if s.StickCount >= sticky {
			s.CurrentIdx = (s.CurrentIdx + 1) % len(models)
			s.StickCount = 0
		}
		s.StickCount++
	default:
		// fallback: always first
		s.CurrentIdx = 0
	}

	model := models[s.CurrentIdx]
	if s.CurrentIdx >= len(models) {
		s.CurrentIdx = 0
	}
	return model, nil
}

// MarkFailed marks the current model as failed and advances to the next.
func MarkFailed(name string) (nextModel string, hasMore bool, err error) {
	statesMu.RLock()
	s, exists := states[name]
	statesMu.RUnlock()

	if !exists {
		return "", false, fmt.Errorf("combo %q not found", name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var modelsJSON string
	db.DB().QueryRow("SELECT models FROM node_combos WHERE name = ?", name).Scan(&modelsJSON)
	var models []string
	json.Unmarshal([]byte(modelsJSON), &models)

	s.CurrentIdx = (s.CurrentIdx + 1) % len(models)
	s.StickCount = 0

	if s.CurrentIdx == 0 {
		return "", false, nil // cycled through all
	}
	return models[s.CurrentIdx], true, nil
}
