package model

// ModelKey maps to llm_model_keys table.
// Stores an encrypted upstream API key owned by a provider user.
type ModelKey struct {
	ID                int64  `json:"id"`
	ProviderUserID    int64  `json:"provider_user_id"`    // user who owns this key
	ChannelID         int64  `json:"channel_id"`           // parent channel
	Name              string `json:"name"`
	KeyEncrypted      string `json:"-"`                    // AES-GCM encrypted, never serialized
	KeyMasked         string `json:"key_masked"`           // visible prefix for UI (sk-****)
	QuotaLimitCredits int64  `json:"quota_limit_credits"`  // max credits allowed
	QuotaUsedCredits  int64  `json:"quota_used_credits"`   // credits consumed
	Status            string `json:"status"`               // active / disabled / expired
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
}

// KeyModelBinding maps to llm_model_key_models table.
// Links a model key to a model spec for a specific upstream model.
type KeyModelBinding struct {
	ID                int64  `json:"id"`
	ModelKeyID        int64  `json:"model_key_id"`
	ModelSpecID       int64  `json:"model_spec_id"`
	UpstreamModelName string `json:"upstream_model_name"`  // actual model name on upstream
	Priority          int    `json:"priority"`             // lower = higher priority
	Weight            int    `json:"weight"`               // for weighted selection
	Status            string `json:"status"`               // active / pending_test / disabled
	LastError         string `json:"last_error,omitempty"`
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
}
