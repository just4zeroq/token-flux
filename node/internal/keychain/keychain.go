package keychain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"ai-platform-node/internal/db"
	"ai-platform-node/internal/types"

	"github.com/google/uuid"
)

// ComputeHash returns HASH(local_key + channel + model_name).
func ComputeHash(localKey, channelID, modelName string) string {
	h := hmac.New(sha256.New, []byte(localKey))
	h.Write([]byte(channelID + "|" + modelName))
	return hex.EncodeToString(h.Sum(nil))
}

// FindKeyByHash looks up a local key using the uploaded hash.
func FindKeyByHash(hash string) (*types.KeyEntry, error) {
	r := db.DB().QueryRow(
		"SELECT k.id, k.label, k.key_value, k.channel_id, k.base_url, k.status, k.created_at "+
			"FROM bindings b JOIN keys k ON b.key_id = k.id "+
			"WHERE b.key_hash = ? AND k.status = 'active'", hash)
	var e types.KeyEntry
	err := r.Scan(&e.ID, &e.Label, &e.Key, &e.ChannelID, &e.BaseURL, &e.Status, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("key not found for hash %s: %w", hash, err)
	}
	return &e, nil
}

// AddKey inserts a new local API key.
func AddKey(label, keyValue, channelID, baseURL string) (*types.KeyEntry, error) {
	id := uuid.New().String()
	now := time.Now().Unix()
	_, err := db.DB().Exec(
		"INSERT INTO keys(id, label, key_value, channel_id, base_url, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, label, keyValue, channelID, baseURL, now)
	if err != nil {
		return nil, err
	}
	return &types.KeyEntry{ID: id, Label: label, Key: keyValue, ChannelID: channelID, BaseURL: baseURL, Status: "active", CreatedAt: now}, nil
}

// ListKeys returns all stored keys (without raw key value).
func ListKeys() ([]types.KeyEntry, error) {
	rows, err := db.DB().Query("SELECT id, label, channel_id, base_url, status, created_at FROM keys")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.KeyEntry
	for rows.Next() {
		var e types.KeyEntry
		rows.Scan(&e.ID, &e.Label, &e.ChannelID, &e.BaseURL, &e.Status, &e.CreatedAt)
		out = append(out, e)
	}
	return out, nil
}

// AddBinding binds a key to a platform model and computes the hash.
func AddBinding(keyID, modelCode, modelName string, shared bool) (*types.KeyBinding, error) {
	// Look up the key
	var kv string
	var channelID string
	err := db.DB().QueryRow("SELECT key_value, channel_id FROM keys WHERE id = ?", keyID).Scan(&kv, &channelID)
	if err != nil {
		return nil, fmt.Errorf("key not found: %w", err)
	}
	h := ComputeHash(kv, channelID, modelName)
	id := uuid.New().String()
	now := time.Now().Unix()
	s := 0
	if shared {
		s = 1
	}
	_, err = db.DB().Exec(
		"INSERT INTO bindings(id, key_id, model_code, model_name, key_hash, shared, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, keyID, modelCode, modelName, h, s, now)
	if err != nil {
		return nil, fmt.Errorf("create binding: %w", err)
	}
	return &types.KeyBinding{ID: id, KeyID: keyID, ModelCode: modelCode, ModelName: modelName, KeyHash: h, Shared: shared, CreatedAt: now}, nil
}

// SharedBindings returns all bindings that are flagged as shared (for platform registration).
func SharedBindings() ([]types.KeyBinding, error) {
	rows, err := db.DB().Query("SELECT id, key_id, model_code, model_name, key_hash, shared, created_at FROM bindings WHERE shared = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.KeyBinding
	for rows.Next() {
		var b types.KeyBinding
		rows.Scan(&b.ID, &b.KeyID, &b.ModelCode, &b.ModelName, &b.KeyHash, &b.Shared, &b.CreatedAt)
		out = append(out, b)
	}
	return out, nil
}
