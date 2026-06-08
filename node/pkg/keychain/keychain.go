package keychain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"ai-platform-node/pkg/db"
	"ai-platform-node/pkg/types"
)

// ComputeHash returns HASH(local_key + channel_id + upstream_model_name).
func ComputeHash(localKey string, channelID int64, upstreamModelName string) string {
	h := hmac.New(sha256.New, []byte(localKey))
	h.Write([]byte(strconv.FormatInt(channelID, 10) + "|" + upstreamModelName))
	return hex.EncodeToString(h.Sum(nil))
}

// FindKeyByHash looks up a key entry + binding by key hash.
// Returns the key entry and binding for use by the provider executor.
func FindKeyByHash(hash string) (*types.KeyEntry, *types.KeyBinding, error) {
	r := db.DB().QueryRow(
		`SELECT k.id, k.name, k.key_encrypted, k.key_masked, k.base_url, k.channel_id,
		        k.status, k.quota_limit_credits, k.quota_used_credits, k.created_at,
		        b.id, b.model_key_id, b.model_spec_id, b.upstream_model_name,
		        b.model_code, b.priority, b.weight, b.status, b.last_error, b.created_at
		 FROM llm_model_key_models b
		 JOIN llm_model_keys k ON b.model_key_id = k.id
		 WHERE b.key_hash = ? AND k.status = 'active' AND b.status = 'active'`, hash)

	var e types.KeyEntry
	var b types.KeyBinding
	err := r.Scan(
		&e.ID, &e.Name, &e.Key, &e.KeyMasked, &e.BaseURL, &e.ChannelID,
		&e.Status, &e.QuotaLimitCredits, &e.QuotaUsedCredits, &e.CreatedAt,
		&b.ID, &b.ModelKeyID, &b.ModelSpecID, &b.UpstreamModelName,
		&b.ModelCode, &b.Priority, &b.Weight, &b.Status, &b.LastError, &b.CreatedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("key not found for hash %s: %w", hash, err)
	}
	b.KeyHash = hash
	b.Shared = (b.Status == "active")
	return &e, &b, nil
}

// AddKey inserts a new upstream API key into llm_model_keys.
func AddKey(name, keyValue, baseURL string, channelID int64) (*types.KeyEntry, error) {
	now := time.Now().Unix()
	masked := maskKey(keyValue)

	result, err := db.DB().Exec(
		`INSERT INTO llm_model_keys(name, key_encrypted, key_masked, channel_id, base_url,
		 status, quota_limit_credits, quota_used_credits, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 'active', 0, 0, ?, ?)`,
		name, keyValue, masked, channelID, baseURL, now, now)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &types.KeyEntry{
		ID: id, Name: name, Key: keyValue, KeyMasked: masked,
		ChannelID: channelID, BaseURL: baseURL, Status: "active", CreatedAt: now,
	}, nil
}

// GenerateAPIKey creates a new sk-xxx API key and stores its hash in node_api_keys.
// Returns the full key (shown once) and an error.
func GenerateAPIKey(label string) (*types.APIKey, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	secret := hex.EncodeToString(b)
	fullKey := "sk-" + secret[:48]
	prefix := fullKey[:12] + "..."

	h := sha256.Sum256([]byte(fullKey))
	hash := hex.EncodeToString(h[:])
	now := time.Now().Unix()

	_, err := db.DB().Exec(
		"INSERT INTO node_api_keys(key_hash, key_prefix, key_value, label, status, last_used_at, created_at) VALUES (?, ?, ?, ?, 'active', 0, ?)",
		hash, prefix, fullKey, label, now)
	if err != nil {
		return nil, fmt.Errorf("store key: %w", err)
	}

	var id int64
	db.DB().QueryRow("SELECT last_insert_rowid()").Scan(&id)

	return &types.APIKey{
		ID:        id,
		Key:       fullKey, // full key returned once
		KeyPrefix: prefix,
		Label:     label,
		Status:    "active",
		CreatedAt: now,
	}, nil
}

// maskKey returns a masked version of the key for display.
func maskKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:8] + "****"
}

// ListKeys returns all stored keys (without raw key value).
func ListKeys() ([]types.KeyEntry, error) {
	rows, err := db.DB().Query(
		`SELECT id, name, key_masked, channel_id, base_url,
		 status, quota_limit_credits, quota_used_credits, created_at
		 FROM llm_model_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.KeyEntry
	for rows.Next() {
		var e types.KeyEntry
		if err := rows.Scan(&e.ID, &e.Name, &e.KeyMasked, &e.ChannelID, &e.BaseURL,
			&e.Status, &e.QuotaLimitCredits, &e.QuotaUsedCredits, &e.CreatedAt); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// AddBinding binds a model key to a model code/spec and computes the hash.
func AddBinding(modelKeyID int64, modelCode, upstreamModelName string, modelSpecID int64, shared bool, priority, weight int) (*types.KeyBinding, error) {
	// Look up the key to compute hash
	var kv string
	var channelID int64
	err := db.DB().QueryRow("SELECT key_encrypted, channel_id FROM llm_model_keys WHERE id = ?", modelKeyID).Scan(&kv, &channelID)
	if err != nil {
		return nil, fmt.Errorf("key not found: %w", err)
	}

	h := ComputeHash(kv, channelID, upstreamModelName)
	now := time.Now().Unix()

	status := "active"
	if !shared {
		status = "private"
	}

	_, err = db.DB().Exec(
		`INSERT OR REPLACE INTO llm_model_key_models
		 (model_key_id, model_spec_id, upstream_model_name, model_code, key_hash,
		  priority, weight, status, last_error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?)`,
		modelKeyID, modelSpecID, upstreamModelName, modelCode, h,
		priority, weight, status, now, now)
	if err != nil {
		return nil, fmt.Errorf("create binding: %w", err)
	}

	var id int64
	db.DB().QueryRow("SELECT last_insert_rowid()").Scan(&id)

	return &types.KeyBinding{
		ID: id, ModelKeyID: modelKeyID, ModelSpecID: modelSpecID,
		UpstreamModelName: upstreamModelName, ModelCode: modelCode,
		KeyHash: h, Shared: shared, Priority: priority, Weight: weight,
		Status: status, CreatedAt: now,
	}, nil
}

// SharedBindings returns all bindings with status='active' (shared to router/platform).
func SharedBindings() ([]types.KeyBinding, error) {
	rows, err := db.DB().Query(
		`SELECT id, model_key_id, model_spec_id, upstream_model_name, model_code,
		 key_hash, priority, weight, status, last_error, created_at
		 FROM llm_model_key_models WHERE status = 'active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.KeyBinding
	for rows.Next() {
		var b types.KeyBinding
		if err := rows.Scan(&b.ID, &b.ModelKeyID, &b.ModelSpecID, &b.UpstreamModelName,
			&b.ModelCode, &b.KeyHash, &b.Priority, &b.Weight, &b.Status, &b.LastError, &b.CreatedAt); err != nil {
			continue
		}
		b.Shared = true
		out = append(out, b)
	}
	return out, nil
}
