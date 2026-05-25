# LLM Ledger Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first-stage LLM closed loop: registration/auth reuse, recharge-to-credits, Virtual Key access, LLM resource management, OpenAI-compatible proxying, LLM pricing, unified settlement, ledger entries, and points issuance.

**Architecture:** Keep LLM, MCP, and Agent as separate product domains with separate pricing and usage records. LLM produces settlement inputs; a shared ledger service writes accounts, transactions, transaction entries, settlement records, and later snapshots. Follow the existing GoFrame service-registration pattern in `server/internal/service` and `server/internal/logic`, while using focused DTO/controller packages for LLM, gateway data-plane, and settlement.

**Tech Stack:** GoFrame v2, PostgreSQL migrations, Go service interfaces, `g.DB().Model(...).Ctx(ctx)` in current codebase style, JWT/API-key middleware, OpenAI-compatible HTTP JSON proxy.

---

## Scope and Delivery Order

This plan intentionally implements LLM in detail and leaves MCP/Agent as boundaries only.

Phase order:

1. Database schema and DTOs.
2. Shared ledger/settlement service.
3. LLM management service.
4. LLM runtime/proxy service.
5. Recharge and overdraft recovery integration.
6. Admin/Provider APIs.
7. Data-plane `/v1/*` APIs.
8. Smoke tests and docs alignment.

---

## File Structure

### Migrations

- Create `server/migrations/0011_llm_core.sql`
  - Adds `api_keys.disabled_reason`.
  - Creates `llm_channels`, `llm_model_specs`, `llm_model_prices`, `llm_model_keys`, `llm_model_key_models`, `llm_usage_records`.
- Create `server/migrations/0012_settlement_ledger.sql`
  - Creates `settlement_records`, `ledger_snapshots`.
  - Adds useful indexes for settlement lookup.

### DTOs

- Create `server/internal/model/dto/llm.go`
  - LLM channel/model/key/price/usage request and response DTOs.
- Create `server/internal/model/dto/settlement.go`
  - Settlement input, settlement record, ledger transaction DTOs.
- Modify `server/internal/model/dto/billing.go`
  - Add transaction type constants only if needed by handlers; avoid changing existing structs unless a task requires it.

### Service Interfaces

- Create `server/internal/service/llm.go`
  - Management methods for channels, model specs, prices, model keys, key-model bindings.
  - Runtime methods for model listing, chat/completions/embeddings proxy, key testing.
- Create `server/internal/service/settlement.go`
  - Unified settlement methods.
- Modify `server/internal/service/billing.go`
  - Add low-level ledger transaction method if reusing existing billing implementation is cleaner than a separate writer.

### Logic Packages

- Create `server/internal/logic/llm/llm.go`
  - Service registration and facade struct.
- Create `server/internal/logic/llm/channel.go`
  - Channel CRUD/review.
- Create `server/internal/logic/llm/model.go`
  - Model spec and price management.
- Create `server/internal/logic/llm/key.go`
  - Model key encryption, key CRUD, key-model binding, async test trigger.
- Create `server/internal/logic/llm/runtime.go`
  - Model resolution, random scheduling, quota checks, runtime usage normalization.
- Create `server/internal/logic/llm/openai.go`
  - OpenAI-compatible upstream request/response handling.
- Create `server/internal/logic/settlement/settlement.go`
  - Settlement record creation, idempotent settlement, transaction writing.
- Create `server/internal/logic/settlement/ledger.go`
  - Account update helpers and transaction entry creation.
- Modify `server/internal/logic/logic.go`
  - Blank import new `llm` and `settlement` packages.

### Controllers

- Create `server/internal/controller/api/llm/admin.go`
  - Admin management APIs for all LLM resources.
- Create `server/internal/controller/api/llm/provider.go`
  - Provider self-service APIs for own keys and key-models.
- Create `server/internal/controller/gateway/openai.go`
  - Data-plane `/v1/models`, `/v1/chat/completions`, `/v1/completions`, `/v1/embeddings`.
- Modify `server/internal/boot/boot.go`
  - Wire admin/provider LLM APIs on `:8080`.
  - Wire OpenAI-compatible gateway APIs on `:8081` with `middleware.APIKeyAuth`.

### Middleware

- Modify `server/internal/middleware/apikey.go`
  - Ensure it sets `user_id` and `api_key_id` context variables consistently for gateway data-plane.
  - Enforce `disabled_reason='overdraft'` through existing status checks if status already disables keys.
- No new middleware unless role checks cannot be expressed with existing `JWTAuth` and `AdminAuth`.

### Config

- Modify `server/manifest/config/config.yaml`
  - Add `security.modelKeyEncryptionKey` or document using `MODEL_KEY_ENCRYPTION_KEY` env var.

---

## Detailed Service Interfaces

### `service.ILLM`

Create `server/internal/service/llm.go`:

```go
package service

import (
    "context"

    "ai-platform/internal/model/dto"
)

type ILLM interface {
    CreateChannel(ctx context.Context, in dto.LLMCreateChannelIn) (*dto.LLMChannelInfo, error)
    ListChannels(ctx context.Context, in dto.LLMListChannelsIn) ([]*dto.LLMChannelInfo, int, error)
    ReviewChannel(ctx context.Context, in dto.LLMReviewChannelIn) error

    CreateModelSpec(ctx context.Context, in dto.LLMCreateModelSpecIn) (*dto.LLMModelSpecInfo, error)
    ListModelSpecs(ctx context.Context, in dto.LLMListModelSpecsIn) ([]*dto.LLMModelSpecInfo, int, error)
    ReviewModelSpec(ctx context.Context, in dto.LLMReviewModelSpecIn) error

    UpsertModelPrice(ctx context.Context, in dto.LLMUpsertModelPriceIn) (*dto.LLMModelPriceInfo, error)
    ListModelPrices(ctx context.Context, modelSpecID int64) ([]*dto.LLMModelPriceInfo, error)

    CreateModelKey(ctx context.Context, providerUserID int64, in dto.LLMCreateModelKeyIn) (*dto.LLMModelKeyInfo, error)
    ListModelKeys(ctx context.Context, in dto.LLMListModelKeysIn) ([]*dto.LLMModelKeyInfo, int, error)
    DisableModelKey(ctx context.Context, providerUserID, keyID int64, isAdmin bool) error

    BindKeyModel(ctx context.Context, providerUserID int64, in dto.LLMBindKeyModelIn) (*dto.LLMKeyModelInfo, error)
    ListKeyModels(ctx context.Context, in dto.LLMListKeyModelsIn) ([]*dto.LLMKeyModelInfo, int, error)
    TriggerKeyModelTest(ctx context.Context, keyModelID int64) error

    ListAvailableModels(ctx context.Context, apiKeyID, userID int64) ([]*dto.OpenAIModelInfo, error)
    ProxyChatCompletions(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error)
    ProxyCompletions(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error)
    ProxyEmbeddings(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error)
}

var localLLM ILLM

func RegisterLLM(i ILLM) { localLLM = i }

func LLM() ILLM {
    if localLLM == nil {
        panic("service.LLM not registered: missing import _ \"ai-platform/internal/logic\"")
    }
    return localLLM
}
```

### `service.ISettlement`

Create `server/internal/service/settlement.go`:

```go
package service

import (
    "context"

    "ai-platform/internal/model/dto"
)

type ISettlement interface {
    Submit(ctx context.Context, in dto.SettlementSubmitIn) (*dto.SettlementRecordInfo, error)
    Settle(ctx context.Context, settlementID int64) (*dto.TransactionInfo, error)
    SubmitAndSettle(ctx context.Context, in dto.SettlementSubmitIn) (*dto.TransactionInfo, error)
    CreateRecharge(ctx context.Context, in dto.RechargeSettlementIn) (*dto.TransactionInfo, error)
    RestoreOverdraftKeys(ctx context.Context, userID int64) error
}

var localSettlement ISettlement

func RegisterSettlement(i ISettlement) { localSettlement = i }

func Settlement() ISettlement {
    if localSettlement == nil {
        panic("service.Settlement not registered: missing import _ \"ai-platform/internal/logic\"")
    }
    return localSettlement
}
```

---

## API Surface

### Admin APIs, `:8080 /api/v1/admin/llm/*`

All require `JWTAuth + AdminAuth`.

```text
POST   /api/v1/admin/llm/channels
GET    /api/v1/admin/llm/channels
PUT    /api/v1/admin/llm/channels/:id/review

POST   /api/v1/admin/llm/models
GET    /api/v1/admin/llm/models
PUT    /api/v1/admin/llm/models/:id/review

POST   /api/v1/admin/llm/models/:id/prices
GET    /api/v1/admin/llm/models/:id/prices

GET    /api/v1/admin/llm/model-keys
GET    /api/v1/admin/llm/key-models
POST   /api/v1/admin/llm/key-models/:id/test
```

### Provider APIs, `:8080 /api/v1/provider/llm/*`

Require `JWTAuth` and `role >= 10` check inside controller/service.

```text
POST   /api/v1/provider/llm/channels
POST   /api/v1/provider/llm/models
GET    /api/v1/provider/llm/models

POST   /api/v1/provider/llm/model-keys
GET    /api/v1/provider/llm/model-keys
DELETE /api/v1/provider/llm/model-keys/:id

POST   /api/v1/provider/llm/model-keys/:id/models
GET    /api/v1/provider/llm/model-keys/:id/models
POST   /api/v1/provider/llm/key-models/:id/test
```

### Gateway Data-Plane APIs, `:8081 /v1/*`

Require `APIKeyAuth`.

```text
GET  /v1/models
POST /v1/chat/completions
POST /v1/completions
POST /v1/embeddings
```

---

## Task 1: Add Database Migrations

**Files:**
- Create: `server/migrations/0011_llm_core.sql`
- Create: `server/migrations/0012_settlement_ledger.sql`

- [ ] **Step 1: Create LLM core migration**

Create `server/migrations/0011_llm_core.sql`:

```sql
-- +goose Up
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS disabled_reason VARCHAR(64) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS llm_channels (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    protocols_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_by_user_id BIGINT REFERENCES users(id),
    reviewed_by_user_id BIGINT REFERENCES users(id),
    reviewed_at TIMESTAMPTZ NULL,
    review_note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS llm_model_specs (
    id BIGSERIAL PRIMARY KEY,
    developer_name VARCHAR(255) NOT NULL,
    model_name VARCHAR(255) NOT NULL,
    model_code VARCHAR(255) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    model_family VARCHAR(128) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    capabilities_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    context_window INT NOT NULL DEFAULT 0,
    max_input_tokens INT NOT NULL DEFAULT 0,
    max_output_tokens INT NOT NULL DEFAULT 0,
    supports_stream BOOLEAN NOT NULL DEFAULT FALSE,
    supports_tools BOOLEAN NOT NULL DEFAULT FALSE,
    supports_vision BOOLEAN NOT NULL DEFAULT FALSE,
    supports_json_mode BOOLEAN NOT NULL DEFAULT FALSE,
    supports_reasoning BOOLEAN NOT NULL DEFAULT FALSE,
    supports_logprobs BOOLEAN NOT NULL DEFAULT FALSE,
    supported_params_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    default_params_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    param_limits_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_type VARCHAR(32) NOT NULL DEFAULT 'admin',
    created_by_user_id BIGINT REFERENCES users(id),
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    reviewed_by_user_id BIGINT REFERENCES users(id),
    reviewed_at TIMESTAMPTZ NULL,
    review_note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(developer_name, model_name)
);

CREATE TABLE IF NOT EXISTS llm_model_prices (
    id BIGSERIAL PRIMARY KEY,
    model_spec_id BIGINT NOT NULL REFERENCES llm_model_specs(id),
    capability VARCHAR(64) NOT NULL,
    currency_asset VARCHAR(32) NOT NULL DEFAULT 'credits',
    cache_hit_price_per_1k BIGINT NOT NULL DEFAULT 0,
    cache_miss_price_per_1k BIGINT NOT NULL DEFAULT 0,
    output_price_per_1k BIGINT NOT NULL DEFAULT 0,
    effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to TIMESTAMPTZ NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS llm_model_keys (
    id BIGSERIAL PRIMARY KEY,
    provider_user_id BIGINT NOT NULL REFERENCES users(id),
    channel_id BIGINT NOT NULL REFERENCES llm_channels(id),
    name VARCHAR(255) NOT NULL DEFAULT '',
    key_encrypted TEXT NOT NULL,
    key_masked VARCHAR(64) NOT NULL DEFAULT '',
    quota_limit_credits BIGINT NOT NULL DEFAULT 0,
    quota_used_credits BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    last_test_at TIMESTAMPTZ NULL,
    last_test_status VARCHAR(32) NOT NULL DEFAULT '',
    last_test_error TEXT NOT NULL DEFAULT '',
    test_attempts INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS llm_model_key_models (
    id BIGSERIAL PRIMARY KEY,
    model_key_id BIGINT NOT NULL REFERENCES llm_model_keys(id),
    model_spec_id BIGINT NOT NULL REFERENCES llm_model_specs(id),
    upstream_model_name VARCHAR(255) NOT NULL,
    quota_limit_credits BIGINT NOT NULL DEFAULT 0,
    quota_used_credits BIGINT NOT NULL DEFAULT 0,
    provider_share_bps INT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'pending_test',
    last_test_at TIMESTAMPTZ NULL,
    last_test_status VARCHAR(32) NOT NULL DEFAULT '',
    last_test_error TEXT NOT NULL DEFAULT '',
    test_attempts INT NOT NULL DEFAULT 0,
    consecutive_failures INT NOT NULL DEFAULT 0,
    last_error_code VARCHAR(64) NOT NULL DEFAULT '',
    last_error_message TEXT NOT NULL DEFAULT '',
    last_error_at TIMESTAMPTZ NULL,
    last_success_at TIMESTAMPTZ NULL,
    next_test_at TIMESTAMPTZ NULL,
    priority INT NOT NULL DEFAULT 0,
    weight INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(model_key_id, model_spec_id, upstream_model_name)
);

CREATE TABLE IF NOT EXISTS llm_usage_records (
    id BIGSERIAL PRIMARY KEY,
    consumer_user_id BIGINT NOT NULL REFERENCES users(id),
    virtual_key_id BIGINT REFERENCES api_keys(id),
    model_spec_id BIGINT NOT NULL REFERENCES llm_model_specs(id),
    model_key_id BIGINT NOT NULL REFERENCES llm_model_keys(id),
    key_model_id BIGINT NOT NULL REFERENCES llm_model_key_models(id),
    provider_user_id BIGINT NOT NULL REFERENCES users(id),
    channel_id BIGINT NOT NULL REFERENCES llm_channels(id),
    request_id VARCHAR(128) NOT NULL DEFAULT '',
    external_request_id VARCHAR(128) NOT NULL DEFAULT '',
    capability VARCHAR(64) NOT NULL,
    is_stream BOOLEAN NOT NULL DEFAULT FALSE,
    input_tokens INT NOT NULL DEFAULT 0,
    cache_hit_tokens INT NOT NULL DEFAULT 0,
    cache_miss_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    total_tokens INT NOT NULL DEFAULT 0,
    cost_credits BIGINT NOT NULL DEFAULT 0,
    provider_revenue_credits BIGINT NOT NULL DEFAULT 0,
    commission_credits BIGINT NOT NULL DEFAULT 0,
    latency_ms INT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'success',
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_llm_model_prices_model ON llm_model_prices(model_spec_id, capability, status);
CREATE INDEX IF NOT EXISTS idx_llm_model_keys_provider ON llm_model_keys(provider_user_id);
CREATE INDEX IF NOT EXISTS idx_llm_model_keys_channel ON llm_model_keys(channel_id);
CREATE INDEX IF NOT EXISTS idx_llm_model_keys_status ON llm_model_keys(status);
CREATE INDEX IF NOT EXISTS idx_llm_key_models_model ON llm_model_key_models(model_spec_id, status);
CREATE INDEX IF NOT EXISTS idx_llm_key_models_key ON llm_model_key_models(model_key_id);
CREATE INDEX IF NOT EXISTS idx_llm_key_models_next_test ON llm_model_key_models(status, next_test_at);
CREATE INDEX IF NOT EXISTS idx_llm_usage_consumer_created ON llm_usage_records(consumer_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_llm_usage_provider_created ON llm_usage_records(provider_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_llm_usage_model_created ON llm_usage_records(model_spec_id, created_at);
CREATE INDEX IF NOT EXISTS idx_llm_usage_key_model_created ON llm_usage_records(key_model_id, created_at);
CREATE INDEX IF NOT EXISTS idx_llm_usage_request_id ON llm_usage_records(request_id);

-- +goose Down
DROP TABLE IF EXISTS llm_usage_records;
DROP TABLE IF EXISTS llm_model_key_models;
DROP TABLE IF EXISTS llm_model_keys;
DROP TABLE IF EXISTS llm_model_prices;
DROP TABLE IF EXISTS llm_model_specs;
DROP TABLE IF EXISTS llm_channels;
ALTER TABLE api_keys DROP COLUMN IF EXISTS disabled_reason;
```

- [ ] **Step 2: Create settlement migration**

Create `server/migrations/0012_settlement_ledger.sql`:

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS settlement_records (
    id BIGSERIAL PRIMARY KEY,
    product_type VARCHAR(32) NOT NULL,
    ref_type VARCHAR(64) NOT NULL,
    ref_id BIGINT NOT NULL,
    consumer_user_id BIGINT NOT NULL REFERENCES users(id),
    provider_user_id BIGINT REFERENCES users(id),
    cost_credits BIGINT NOT NULL DEFAULT 0,
    provider_revenue_credits BIGINT NOT NULL DEFAULT 0,
    commission_credits BIGINT NOT NULL DEFAULT 0,
    points_to_consumer BIGINT NOT NULL DEFAULT 0,
    points_to_provider BIGINT NOT NULL DEFAULT 0,
    transaction_id BIGINT REFERENCES transactions(id),
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at TIMESTAMPTZ NULL,
    UNIQUE(ref_type, ref_id)
);

CREATE TABLE IF NOT EXISTS ledger_snapshots (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id),
    asset VARCHAR(32) NOT NULL,
    balance_micro BIGINT NOT NULL,
    snapshot_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_settlement_records_consumer ON settlement_records(consumer_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_settlement_records_provider ON settlement_records(provider_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_settlement_records_status ON settlement_records(status, created_at);
CREATE INDEX IF NOT EXISTS idx_ledger_snapshots_account_time ON ledger_snapshots(account_id, snapshot_at);

-- +goose Down
DROP TABLE IF EXISTS ledger_snapshots;
DROP TABLE IF EXISTS settlement_records;
```

- [ ] **Step 3: Run migrations**

Run:

```bash
cd server/docker && bash migrate.sh
```

Expected: goose applies `0011_llm_core.sql` and `0012_settlement_ledger.sql` without errors.

- [ ] **Step 4: Commit**

```bash
git add server/migrations/0011_llm_core.sql server/migrations/0012_settlement_ledger.sql
git commit -m "feat(db): add llm and settlement ledger schema"
```

---

## Task 2: Add DTOs and Service Interfaces

**Files:**
- Create: `server/internal/model/dto/llm.go`
- Create: `server/internal/model/dto/settlement.go`
- Create: `server/internal/service/llm.go`
- Create: `server/internal/service/settlement.go`

- [ ] **Step 1: Create LLM DTOs**

Create `server/internal/model/dto/llm.go` with structs for:

```go
package dto

import "time"

type LLMChannelInfo struct {
    ID              int64     `json:"id"`
    Code            string    `json:"code"`
    Name            string    `json:"name"`
    Description     string    `json:"description"`
    ProtocolsJson   string    `json:"protocols_json"`
    Status          string    `json:"status"`
    CreatedByUserID int64     `json:"created_by_user_id"`
    ReviewedByUserID int64    `json:"reviewed_by_user_id"`
    ReviewedAt      time.Time `json:"reviewed_at,omitempty"`
    ReviewNote      string    `json:"review_note"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}

type LLMCreateChannelIn struct {
    Code          string `json:"code" v:"required"`
    Name          string `json:"name" v:"required"`
    Description   string `json:"description"`
    ProtocolsJson string `json:"protocols_json" v:"required"`
    CreatedByUserID int64 `json:"-"`
}

type LLMListChannelsIn struct {
    Status string `json:"status"`
    Page   int    `json:"page"`
    Size   int    `json:"size"`
}

type LLMReviewChannelIn struct {
    ID         int64  `json:"-"`
    Status     string `json:"status" v:"required|in:active,disabled,rejected"`
    ReviewNote string `json:"review_note"`
    ReviewerID int64  `json:"-"`
}

type LLMModelSpecInfo struct {
    ID                  int64     `json:"id"`
    DeveloperName       string    `json:"developer_name"`
    ModelName           string    `json:"model_name"`
    ModelCode           string    `json:"model_code"`
    DisplayName         string    `json:"display_name"`
    ModelFamily         string    `json:"model_family"`
    Description         string    `json:"description"`
    CapabilitiesJson    string    `json:"capabilities_json"`
    ContextWindow       int       `json:"context_window"`
    MaxInputTokens      int       `json:"max_input_tokens"`
    MaxOutputTokens     int       `json:"max_output_tokens"`
    SupportsStream      bool      `json:"supports_stream"`
    SupportsTools       bool      `json:"supports_tools"`
    SupportsVision      bool      `json:"supports_vision"`
    SupportsJsonMode    bool      `json:"supports_json_mode"`
    SupportsReasoning   bool      `json:"supports_reasoning"`
    SupportsLogprobs    bool      `json:"supports_logprobs"`
    SupportedParamsJson string    `json:"supported_params_json"`
    DefaultParamsJson   string    `json:"default_params_json"`
    ParamLimitsJson     string    `json:"param_limits_json"`
    SourceType          string    `json:"source_type"`
    CreatedByUserID     int64     `json:"created_by_user_id"`
    Status              string    `json:"status"`
    CreatedAt           time.Time `json:"created_at"`
    UpdatedAt           time.Time `json:"updated_at"`
}

type LLMCreateModelSpecIn struct {
    DeveloperName       string `json:"developer_name" v:"required"`
    ModelName           string `json:"model_name" v:"required"`
    ModelCode           string `json:"model_code"`
    DisplayName         string `json:"display_name"`
    ModelFamily         string `json:"model_family"`
    Description         string `json:"description"`
    CapabilitiesJson    string `json:"capabilities_json" v:"required"`
    ContextWindow       int    `json:"context_window"`
    MaxInputTokens      int    `json:"max_input_tokens"`
    MaxOutputTokens     int    `json:"max_output_tokens"`
    SupportsStream      bool   `json:"supports_stream"`
    SupportsTools       bool   `json:"supports_tools"`
    SupportsVision      bool   `json:"supports_vision"`
    SupportsJsonMode    bool   `json:"supports_json_mode"`
    SupportsReasoning   bool   `json:"supports_reasoning"`
    SupportsLogprobs    bool   `json:"supports_logprobs"`
    SupportedParamsJson string `json:"supported_params_json"`
    DefaultParamsJson   string `json:"default_params_json"`
    ParamLimitsJson     string `json:"param_limits_json"`
    SourceType          string `json:"-"`
    CreatedByUserID     int64  `json:"-"`
}

type LLMListModelSpecsIn struct {
    Status string `json:"status"`
    Page   int    `json:"page"`
    Size   int    `json:"size"`
}

type LLMReviewModelSpecIn struct {
    ID         int64  `json:"-"`
    Status     string `json:"status" v:"required|in:active,disabled,rejected"`
    ReviewNote string `json:"review_note"`
    ReviewerID int64  `json:"-"`
}

type LLMUpsertModelPriceIn struct {
    ModelSpecID          int64  `json:"-"`
    Capability           string `json:"capability" v:"required"`
    CacheHitPricePer1K   int64  `json:"cache_hit_price_per_1k"`
    CacheMissPricePer1K  int64  `json:"cache_miss_price_per_1k"`
    OutputPricePer1K     int64  `json:"output_price_per_1k"`
    Status               string `json:"status"`
}

type LLMModelPriceInfo struct {
    ID                  int64     `json:"id"`
    ModelSpecID         int64     `json:"model_spec_id"`
    Capability          string    `json:"capability"`
    CurrencyAsset       string    `json:"currency_asset"`
    CacheHitPricePer1K  int64     `json:"cache_hit_price_per_1k"`
    CacheMissPricePer1K int64     `json:"cache_miss_price_per_1k"`
    OutputPricePer1K    int64     `json:"output_price_per_1k"`
    Status              string    `json:"status"`
    CreatedAt           time.Time `json:"created_at"`
    UpdatedAt           time.Time `json:"updated_at"`
}

type LLMCreateModelKeyIn struct {
    ChannelID          int64  `json:"channel_id" v:"required"`
    Name               string `json:"name"`
    Key                string `json:"key" v:"required"`
    QuotaLimitCredits  int64  `json:"quota_limit_credits"`
}

type LLMModelKeyInfo struct {
    ID                 int64     `json:"id"`
    ProviderUserID     int64     `json:"provider_user_id"`
    ChannelID          int64     `json:"channel_id"`
    Name               string    `json:"name"`
    KeyMasked          string    `json:"key_masked"`
    QuotaLimitCredits  int64     `json:"quota_limit_credits"`
    QuotaUsedCredits   int64     `json:"quota_used_credits"`
    Status             string    `json:"status"`
    LastTestAt         time.Time `json:"last_test_at,omitempty"`
    LastTestStatus     string    `json:"last_test_status"`
    LastTestError      string    `json:"last_test_error"`
    TestAttempts       int       `json:"test_attempts"`
    CreatedAt          time.Time `json:"created_at"`
    UpdatedAt          time.Time `json:"updated_at"`
}

type LLMListModelKeysIn struct {
    ProviderUserID int64
    IsAdmin        bool
    Status         string
    Page           int
    Size           int
}

type LLMBindKeyModelIn struct {
    ModelKeyID          int64  `json:"-"`
    ModelSpecID         int64  `json:"model_spec_id" v:"required"`
    UpstreamModelName   string `json:"upstream_model_name" v:"required"`
    QuotaLimitCredits   int64  `json:"quota_limit_credits"`
    ProviderShareBps    int    `json:"provider_share_bps"`
}

type LLMKeyModelInfo struct {
    ID                 int64     `json:"id"`
    ModelKeyID         int64     `json:"model_key_id"`
    ModelSpecID        int64     `json:"model_spec_id"`
    UpstreamModelName  string    `json:"upstream_model_name"`
    QuotaLimitCredits  int64     `json:"quota_limit_credits"`
    QuotaUsedCredits   int64     `json:"quota_used_credits"`
    ProviderShareBps   int       `json:"provider_share_bps"`
    Status             string    `json:"status"`
    ConsecutiveFailures int      `json:"consecutive_failures"`
    LastErrorCode      string    `json:"last_error_code"`
    LastErrorMessage   string    `json:"last_error_message"`
    CreatedAt          time.Time `json:"created_at"`
    UpdatedAt          time.Time `json:"updated_at"`
}

type LLMListKeyModelsIn struct {
    ProviderUserID int64
    IsAdmin        bool
    ModelKeyID     int64
    ModelSpecID    int64
    Status         string
    Page           int
    Size           int
}

type OpenAIModelInfo struct {
    ID      string `json:"id"`
    Object  string `json:"object"`
    Created int64  `json:"created"`
    OwnedBy string `json:"owned_by"`
}

type OpenAIProxyRequest struct {
    UserID      int64
    ApiKeyID    int64
    RequestID   string
    Capability  string
    RawBody     []byte
    IsStream    bool
}

type OpenAIProxyResponse struct {
    StatusCode int
    Body       []byte
    Headers    map[string]string
}
```

- [ ] **Step 2: Create settlement DTOs**

Create `server/internal/model/dto/settlement.go`:

```go
package dto

import "time"

type SettlementSubmitIn struct {
    ProductType             string `json:"product_type"`
    RefType                 string `json:"ref_type"`
    RefID                   int64  `json:"ref_id"`
    ConsumerUserID          int64  `json:"consumer_user_id"`
    ProviderUserID          int64  `json:"provider_user_id"`
    CostCredits             int64  `json:"cost_credits"`
    ProviderRevenueCredits  int64  `json:"provider_revenue_credits"`
    CommissionCredits       int64  `json:"commission_credits"`
    PointsToConsumer        int64  `json:"points_to_consumer"`
    PointsToProvider        int64  `json:"points_to_provider"`
}

type RechargeSettlementIn struct {
    UserID          int64  `json:"user_id"`
    RefType         string `json:"ref_type"`
    RefID           int64  `json:"ref_id"`
    RechargeCredits int64  `json:"recharge_credits"`
}

type SettlementRecordInfo struct {
    ID                      int64     `json:"id"`
    ProductType             string    `json:"product_type"`
    RefType                 string    `json:"ref_type"`
    RefID                   int64     `json:"ref_id"`
    ConsumerUserID          int64     `json:"consumer_user_id"`
    ProviderUserID          int64     `json:"provider_user_id"`
    CostCredits             int64     `json:"cost_credits"`
    ProviderRevenueCredits  int64     `json:"provider_revenue_credits"`
    CommissionCredits       int64     `json:"commission_credits"`
    PointsToConsumer        int64     `json:"points_to_consumer"`
    PointsToProvider        int64     `json:"points_to_provider"`
    TransactionID           int64     `json:"transaction_id"`
    Status                  string    `json:"status"`
    ErrorMessage            string    `json:"error_message"`
    CreatedAt               time.Time `json:"created_at"`
    SettledAt               time.Time `json:"settled_at,omitempty"`
}
```

- [ ] **Step 3: Create service interfaces**

Create `server/internal/service/llm.go` and `server/internal/service/settlement.go` using the exact interface definitions from the “Detailed Service Interfaces” section above.

- [ ] **Step 4: Build**

Run:

```bash
cd server && go build ./...
```

Expected: build passes or only fails because implementation packages have not been added yet. If build fails due to unused imports or syntax, fix DTO/interface files.

- [ ] **Step 5: Commit**

```bash
git add server/internal/model/dto/llm.go server/internal/model/dto/settlement.go server/internal/service/llm.go server/internal/service/settlement.go
git commit -m "feat(llm): add dto and service interfaces"
```

---

## Task 3: Implement Settlement and Ledger Service

**Files:**
- Create: `server/internal/logic/settlement/settlement.go`
- Create: `server/internal/logic/settlement/ledger.go`
- Modify: `server/internal/logic/logic.go`

- [ ] **Step 1: Implement registration**

Create `server/internal/logic/settlement/settlement.go`:

```go
package settlement

import (
    "context"

    "ai-platform/internal/model/dto"
    "ai-platform/internal/service"
)

type sSettlement struct{}

func init() { service.RegisterSettlement(New()) }

func New() *sSettlement { return &sSettlement{} }

func (s *sSettlement) Submit(ctx context.Context, in dto.SettlementSubmitIn) (*dto.SettlementRecordInfo, error) {
    return s.submit(ctx, in)
}

func (s *sSettlement) Settle(ctx context.Context, settlementID int64) (*dto.TransactionInfo, error) {
    return s.settle(ctx, settlementID)
}

func (s *sSettlement) SubmitAndSettle(ctx context.Context, in dto.SettlementSubmitIn) (*dto.TransactionInfo, error) {
    record, err := s.submit(ctx, in)
    if err != nil {
        return nil, err
    }
    return s.settle(ctx, record.ID)
}

func (s *sSettlement) CreateRecharge(ctx context.Context, in dto.RechargeSettlementIn) (*dto.TransactionInfo, error) {
    return s.createRecharge(ctx, in)
}

func (s *sSettlement) RestoreOverdraftKeys(ctx context.Context, userID int64) error {
    return s.restoreOverdraftKeys(ctx, userID)
}
```

- [ ] **Step 2: Implement ledger helpers**

Create `server/internal/logic/settlement/ledger.go` with helper functions:

```go
package settlement

import (
    "context"
    "time"

    "ai-platform/internal/model/dto"

    "github.com/gogf/gf/v2/database/gdb"
    "github.com/gogf/gf/v2/errors/gerror"
    "github.com/gogf/gf/v2/frame/g"
)

const (
    ownerTypeUser     = "user"
    ownerTypePlatform = "platform"
    assetCredits      = "credits"
    assetPoints       = "points"
)

func (s *sSettlement) submit(ctx context.Context, in dto.SettlementSubmitIn) (*dto.SettlementRecordInfo, error) {
    id, err := g.DB().Model("settlement_records").Ctx(ctx).InsertAndGetId(g.Map{
        "product_type":              in.ProductType,
        "ref_type":                  in.RefType,
        "ref_id":                    in.RefID,
        "consumer_user_id":          in.ConsumerUserID,
        "provider_user_id":          in.ProviderUserID,
        "cost_credits":              in.CostCredits,
        "provider_revenue_credits":  in.ProviderRevenueCredits,
        "commission_credits":        in.CommissionCredits,
        "points_to_consumer":        in.PointsToConsumer,
        "points_to_provider":        in.PointsToProvider,
        "status":                    "pending",
    })
    if err != nil {
        existing, getErr := g.DB().Model("settlement_records").Ctx(ctx).
            Where("ref_type = ? AND ref_id = ?", in.RefType, in.RefID).One()
        if getErr != nil || existing.IsEmpty() {
            return nil, gerror.Wrap(err, "create settlement record failed")
        }
        return mapSettlementRecord(existing), nil
    }

    record, err := g.DB().Model("settlement_records").Ctx(ctx).Where("id", id).One()
    if err != nil {
        return nil, gerror.Wrap(err, "query settlement record failed")
    }
    return mapSettlementRecord(record), nil
}

func (s *sSettlement) settle(ctx context.Context, settlementID int64) (*dto.TransactionInfo, error) {
    var out *dto.TransactionInfo
    err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
        record, err := tx.Model("settlement_records").Ctx(ctx).Where("id", settlementID).One()
        if err != nil {
            return err
        }
        if record.IsEmpty() {
            return gerror.New("settlement record not found")
        }
        if record["status"].String() == "settled" {
            txID := record["transaction_id"].Int64()
            txInfo, err := loadTransaction(ctx, tx, txID)
            if err != nil {
                return err
            }
            out = txInfo
            return nil
        }

        cost := record["cost_credits"].Int64()
        providerRevenue := record["provider_revenue_credits"].Int64()
        commission := record["commission_credits"].Int64()
        if cost != providerRevenue+commission {
            return gerror.New("settlement credits are not balanced")
        }

        txID, err := tx.Model("transactions").Ctx(ctx).InsertAndGetId(g.Map{
            "tx_type":  record["product_type"].String() + "_usage_settle",
            "ref_type": record["ref_type"].String(),
            "ref_id":   record["ref_id"].Int64(),
        })
        if err != nil {
            return err
        }

        entries := []entryDelta{
            {OwnerType: ownerTypeUser, OwnerID: record["consumer_user_id"].Int64(), Asset: assetCredits, Delta: -cost},
            {OwnerType: ownerTypeUser, OwnerID: record["provider_user_id"].Int64(), Asset: assetCredits, Delta: providerRevenue},
            {OwnerType: ownerTypePlatform, OwnerID: 0, Asset: assetCredits, Delta: commission},
        }
        if points := record["points_to_consumer"].Int64(); points > 0 {
            entries = append(entries, entryDelta{OwnerType: ownerTypeUser, OwnerID: record["consumer_user_id"].Int64(), Asset: assetPoints, Delta: points})
        }
        if points := record["points_to_provider"].Int64(); points > 0 {
            entries = append(entries, entryDelta{OwnerType: ownerTypeUser, OwnerID: record["provider_user_id"].Int64(), Asset: assetPoints, Delta: points})
        }

        for _, entry := range entries {
            if err := applyEntry(ctx, tx, txID, entry); err != nil {
                return err
            }
        }

        _, err = tx.Model("settlement_records").Ctx(ctx).Where("id", settlementID).Data(g.Map{
            "transaction_id": txID,
            "status":         "settled",
            "settled_at":     time.Now(),
        }).Update()
        if err != nil {
            return err
        }

        txInfo, err := loadTransaction(ctx, tx, txID)
        if err != nil {
            return err
        }
        out = txInfo
        return nil
    })
    if err != nil {
        return nil, gerror.Wrap(err, "settle failed")
    }
    return out, nil
}
```

Continue `ledger.go` with `entryDelta`, `applyEntry`, `ensureAccount`, `loadTransaction`, `createRecharge`, `restoreOverdraftKeys`, and mapping helpers. Use the current `billing.go` implementation as reference for transaction entry mapping.

- [ ] **Step 3: Wire logic import**

Modify `server/internal/logic/logic.go` and add:

```go
_ "ai-platform/internal/logic/settlement"
```

- [ ] **Step 4: Build**

Run:

```bash
cd server && go build ./...
```

Expected: build passes.

- [ ] **Step 5: Commit**

```bash
git add server/internal/logic/settlement server/internal/logic/logic.go
git commit -m "feat(settlement): add unified ledger settlement service"
```

---

## Task 4: Implement LLM Management Service

**Files:**
- Create: `server/internal/logic/llm/llm.go`
- Create: `server/internal/logic/llm/channel.go`
- Create: `server/internal/logic/llm/model.go`
- Create: `server/internal/logic/llm/key.go`
- Modify: `server/internal/logic/logic.go`

- [ ] **Step 1: Create LLM service registration**

Create `server/internal/logic/llm/llm.go`:

```go
package llm

import "ai-platform/internal/service"

type sLLM struct{}

func init() { service.RegisterLLM(New()) }

func New() *sLLM { return &sLLM{} }
```

- [ ] **Step 2: Implement channel methods**

Create `server/internal/logic/llm/channel.go` implementing:

```go
func (s *sLLM) CreateChannel(ctx context.Context, in dto.LLMCreateChannelIn) (*dto.LLMChannelInfo, error)
func (s *sLLM) ListChannels(ctx context.Context, in dto.LLMListChannelsIn) ([]*dto.LLMChannelInfo, int, error)
func (s *sLLM) ReviewChannel(ctx context.Context, in dto.LLMReviewChannelIn) error
```

Rules:

- Validate `protocols_json` is valid JSON.
- Validate protocol keys are in allowed set: `openai-compatible`, `anthropic-compatible`, `gemini-compatible`, `azure-openai`.
- Provider-created channels start `pending`.
- Admin review can set `active`, `disabled`, `rejected`.

- [ ] **Step 3: Implement model and price methods**

Create `server/internal/logic/llm/model.go` implementing:

```go
func (s *sLLM) CreateModelSpec(ctx context.Context, in dto.LLMCreateModelSpecIn) (*dto.LLMModelSpecInfo, error)
func (s *sLLM) ListModelSpecs(ctx context.Context, in dto.LLMListModelSpecsIn) ([]*dto.LLMModelSpecInfo, int, error)
func (s *sLLM) ReviewModelSpec(ctx context.Context, in dto.LLMReviewModelSpecIn) error
func (s *sLLM) UpsertModelPrice(ctx context.Context, in dto.LLMUpsertModelPriceIn) (*dto.LLMModelPriceInfo, error)
func (s *sLLM) ListModelPrices(ctx context.Context, modelSpecID int64) ([]*dto.LLMModelPriceInfo, error)
```

Rules:

- Generate `model_code` when empty: lowercased normalized `developer_name + "/" + model_name`.
- Admin-created specs may be set active by review endpoint; provider-created specs start pending.
- No aliases.

- [ ] **Step 4: Implement encryption helpers and key methods**

Create `server/internal/logic/llm/key.go` implementing:

```go
func (s *sLLM) CreateModelKey(ctx context.Context, providerUserID int64, in dto.LLMCreateModelKeyIn) (*dto.LLMModelKeyInfo, error)
func (s *sLLM) ListModelKeys(ctx context.Context, in dto.LLMListModelKeysIn) ([]*dto.LLMModelKeyInfo, int, error)
func (s *sLLM) DisableModelKey(ctx context.Context, providerUserID, keyID int64, isAdmin bool) error
func (s *sLLM) BindKeyModel(ctx context.Context, providerUserID int64, in dto.LLMBindKeyModelIn) (*dto.LLMKeyModelInfo, error)
func (s *sLLM) ListKeyModels(ctx context.Context, in dto.LLMListKeyModelsIn) ([]*dto.LLMKeyModelInfo, int, error)
func (s *sLLM) TriggerKeyModelTest(ctx context.Context, keyModelID int64) error
```

Rules:

- Encrypt raw key with AES-GCM and `MODEL_KEY_ENCRYPTION_KEY`.
- Store masked key only for display.
- Provider can only access own keys unless `isAdmin` is true.
- Key-model status is `pending_model_review` when model spec is not active, otherwise `pending_test`.
- Trigger test asynchronously after binding when model and channel are active.

- [ ] **Step 5: Wire logic import**

Modify `server/internal/logic/logic.go` and add:

```go
_ "ai-platform/internal/logic/llm"
```

- [ ] **Step 6: Build**

Run:

```bash
cd server && go build ./...
```

Expected: build passes.

- [ ] **Step 7: Commit**

```bash
git add server/internal/logic/llm server/internal/logic/logic.go
git commit -m "feat(llm): add llm management service"
```

---

## Task 5: Implement LLM Runtime and OpenAI-Compatible Adapter

**Files:**
- Create: `server/internal/logic/llm/runtime.go`
- Create: `server/internal/logic/llm/openai.go`

- [ ] **Step 1: Implement available model listing**

In `runtime.go`, implement:

```go
func (s *sLLM) ListAvailableModels(ctx context.Context, apiKeyID, userID int64) ([]*dto.OpenAIModelInfo, error)
```

Rules:

- Only return active model specs.
- Require at least one active key-model candidate.
- Respect `api_keys.model_limits_enabled` and `model_limits`.
- Return OpenAI-compatible model objects.

- [ ] **Step 2: Implement scheduler**

In `runtime.go`, add an unexported helper:

```go
func pickRandomKeyModel(ctx context.Context, modelCode string, capability string) (scheduledKeyModel, error)
```

`scheduleKeyModel` includes:

```go
type scheduledKeyModel struct {
    ModelSpecID        int64
    ModelCode          string
    ModelKeyID         int64
    KeyModelID         int64
    ProviderUserID     int64
    ChannelID          int64
    UpstreamModelName  string
    KeyEncrypted       string
    ProtocolsJson      string
    ProviderShareBps   int
}
```

Rules:

- MVP uses random selection among candidates.
- Candidate must have active model, channel, model key, key-model.
- Candidate must have remaining key and key-model quota unless quota limit is 0.
- Candidate channel must support `openai-compatible`.

- [ ] **Step 3: Implement OpenAI-compatible proxy helpers**

In `openai.go`, implement:

```go
func proxyOpenAICompatible(ctx context.Context, baseURL string, apiKey string, body []byte) (status int, responseBody []byte, headers map[string]string, err error)
func extractRequestedModel(body []byte) (string, error)
func replaceRequestModel(body []byte, upstreamModel string) ([]byte, bool, error)
func normalizeOpenAIUsage(body []byte) normalizedUsage
```

`normalizedUsage`:

```go
type normalizedUsage struct {
    InputTokens     int
    CacheHitTokens  int
    CacheMissTokens int
    OutputTokens    int
    TotalTokens     int
}
```

Rules:

- If cache details missing, set `cache_hit_tokens=0`, `cache_miss_tokens=input_tokens`.
- Use request timeout.
- Do not log raw API key.

- [ ] **Step 4: Implement proxy methods**

Implement:

```go
func (s *sLLM) ProxyChatCompletions(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error)
func (s *sLLM) ProxyCompletions(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error)
func (s *sLLM) ProxyEmbeddings(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error)
```

Rules:

- Check user credits balance > 0 before upstream call.
- Resolve requested model.
- Schedule key-model.
- Decrypt model key.
- Replace request `model` with `upstream_model_name`.
- Call upstream.
- If upstream success, calculate cost from `llm_model_prices`.
- Write `llm_usage_records`.
- Submit and settle via `service.Settlement().SubmitAndSettle`.
- Update key and key-model quota used.
- If post-settlement balance < 0, disable all user api keys with `disabled_reason='overdraft'`.
- Return upstream response body.

- [ ] **Step 5: Build**

Run:

```bash
cd server && go build ./...
```

Expected: build passes.

- [ ] **Step 6: Commit**

```bash
git add server/internal/logic/llm/runtime.go server/internal/logic/llm/openai.go
git commit -m "feat(llm): add openai-compatible runtime proxy"
```

---

## Task 6: Add Admin and Provider Controllers

**Files:**
- Create: `server/internal/controller/api/llm/admin.go`
- Create: `server/internal/controller/api/llm/provider.go`
- Modify: `server/internal/boot/boot.go`

- [ ] **Step 1: Create admin controller**

Create `server/internal/controller/api/llm/admin.go` with handlers:

```go
CreateChannel
ListChannels
ReviewChannel
CreateModel
ListModels
ReviewModel
UpsertModelPrice
ListModelPrices
ListAllModelKeys
ListAllKeyModels
TriggerKeyModelTest
```

Each handler:

- Parses request DTO.
- Pulls reviewer user ID from context for review actions.
- Calls `service.LLM()`.
- Returns JSON or status error.

- [ ] **Step 2: Create provider controller**

Create `server/internal/controller/api/llm/provider.go` with handlers:

```go
ProviderCreateChannel
ProviderCreateModel
ProviderListModels
ProviderCreateModelKey
ProviderListModelKeys
ProviderDisableModelKey
ProviderBindKeyModel
ProviderListKeyModels
ProviderTriggerKeyModelTest
```

Rules:

- Read `user_id` and `role` from JWT context.
- Reject when `role < 10`.
- Provider handlers pass `providerUserID=current user` and `isAdmin=false`.

- [ ] **Step 3: Wire API routes**

Modify `server/internal/boot/boot.go`:

- Add import:

```go
llmapi "ai-platform/internal/controller/api/llm"
```

- Under admin group add:

```go
ag.POST("/llm/channels", llmapi.CreateChannel)
ag.GET("/llm/channels", llmapi.ListChannels)
ag.PUT("/llm/channels/:id/review", llmapi.ReviewChannel)
ag.POST("/llm/models", llmapi.CreateModel)
ag.GET("/llm/models", llmapi.ListModels)
ag.PUT("/llm/models/:id/review", llmapi.ReviewModel)
ag.POST("/llm/models/:id/prices", llmapi.UpsertModelPrice)
ag.GET("/llm/models/:id/prices", llmapi.ListModelPrices)
ag.GET("/llm/model-keys", llmapi.ListAllModelKeys)
ag.GET("/llm/key-models", llmapi.ListAllKeyModels)
ag.POST("/llm/key-models/:id/test", llmapi.TriggerKeyModelTest)
```

- Add provider group under `/api/v1`:

```go
v1.Group("/provider/llm", func(pg *ghttp.RouterGroup) {
    pg.Middleware(middleware.JWTAuth)
    pg.POST("/channels", llmapi.ProviderCreateChannel)
    pg.POST("/models", llmapi.ProviderCreateModel)
    pg.GET("/models", llmapi.ProviderListModels)
    pg.POST("/model-keys", llmapi.ProviderCreateModelKey)
    pg.GET("/model-keys", llmapi.ProviderListModelKeys)
    pg.DELETE("/model-keys/:id", llmapi.ProviderDisableModelKey)
    pg.POST("/model-keys/:id/models", llmapi.ProviderBindKeyModel)
    pg.GET("/model-keys/:id/models", llmapi.ProviderListKeyModels)
    pg.POST("/key-models/:id/test", llmapi.ProviderTriggerKeyModelTest)
})
```

- [ ] **Step 4: Build**

Run:

```bash
cd server && go build ./...
```

Expected: build passes.

- [ ] **Step 5: Commit**

```bash
git add server/internal/controller/api/llm server/internal/boot/boot.go
git commit -m "feat(llm): add admin and provider management APIs"
```

---

## Task 7: Add Gateway Data-Plane Controllers

**Files:**
- Create: `server/internal/controller/gateway/openai.go`
- Modify: `server/internal/boot/boot.go`

- [ ] **Step 1: Create gateway controller**

Create `server/internal/controller/gateway/openai.go`:

```go
package gateway

import (
    "io"

    "ai-platform/internal/model/dto"
    "ai-platform/internal/service"

    "github.com/gogf/gf/v2/net/ghttp"
)

func Models(r *ghttp.Request) {
    userID := r.GetCtxVar("user_id").Int64()
    apiKeyID := r.GetCtxVar("api_key_id").Int64()
    models, err := service.LLM().ListAvailableModels(r.Context(), apiKeyID, userID)
    if err != nil {
        r.Response.WriteStatusExit(500, map[string]any{"error": err.Error()})
        return
    }
    r.Response.WriteJson(map[string]any{"object": "list", "data": models})
}

func ChatCompletions(r *ghttp.Request) {
    proxy(r, "chat", service.LLM().ProxyChatCompletions)
}

func Completions(r *ghttp.Request) {
    proxy(r, "completion", service.LLM().ProxyCompletions)
}

func Embeddings(r *ghttp.Request) {
    proxy(r, "embedding", service.LLM().ProxyEmbeddings)
}

func proxy(r *ghttp.Request, capability string, fn func(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error)) {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        r.Response.WriteStatusExit(400, map[string]any{"error": "invalid request body"})
        return
    }
    res, err := fn(r.Context(), dto.OpenAIProxyRequest{
        UserID:     r.GetCtxVar("user_id").Int64(),
        ApiKeyID:   r.GetCtxVar("api_key_id").Int64(),
        RequestID:  r.GetCtxVar("request_id").String(),
        Capability: capability,
        RawBody:    body,
        IsStream:   false,
    })
    if err != nil {
        r.Response.WriteStatusExit(500, map[string]any{"error": err.Error()})
        return
    }
    for k, v := range res.Headers {
        r.Response.Header().Set(k, v)
    }
    r.Response.WriteStatus(res.StatusCode)
    r.Response.Write(res.Body)
}
```

Add missing `context` import when implementing.

- [ ] **Step 2: Wire gateway routes**

Modify gateway server group in `server/internal/boot/boot.go`:

```go
group.Group("/v1", func(v1 *ghttp.RouterGroup) {
    v1.Middleware(middleware.APIKeyAuth)
    v1.GET("/models", gateway.Models)
    v1.POST("/chat/completions", gateway.ChatCompletions)
    v1.POST("/completions", gateway.Completions)
    v1.POST("/embeddings", gateway.Embeddings)
})
```

- [ ] **Step 3: Build**

Run:

```bash
cd server && go build ./...
```

Expected: build passes.

- [ ] **Step 4: Commit**

```bash
git add server/internal/controller/gateway/openai.go server/internal/boot/boot.go
git commit -m "feat(gateway): add openai-compatible data-plane routes"
```

---

## Task 8: Add Recharge and Overdraft Recovery Flow

**Files:**
- Modify: `server/internal/logic/billing.go` or existing billing implementation path `server/internal/logic/billing/billing.go`
- Modify: `server/internal/controller/api/billing/billing.go`
- Modify: `server/internal/service/billing.go` if adding explicit recharge method

- [ ] **Step 1: Add recharge service method**

Modify `server/internal/service/billing.go` to add:

```go
RechargeCredits(ctx context.Context, userID int64, amountCredits int64, refType string, refID int64) (*dto.TransactionInfo, error)
```

- [ ] **Step 2: Implement recharge through settlement**

In billing logic, implement `RechargeCredits` by calling:

```go
service.Settlement().CreateRecharge(ctx, dto.RechargeSettlementIn{
    UserID: userID,
    RefType: refType,
    RefID: refID,
    RechargeCredits: amountCredits,
})
```

Then call:

```go
service.Settlement().RestoreOverdraftKeys(ctx, userID)
```

- [ ] **Step 3: Add controller endpoint**

For MVP/admin smoke testing, add an authenticated endpoint:

```text
POST /api/v1/billing/recharge
```

Request:

```json
{
  "amount_credits": 1000000,
  "ref_type": "manual_recharge",
  "ref_id": 0
}
```

- [ ] **Step 4: Build**

Run:

```bash
cd server && go build ./...
```

Expected: build passes.

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/billing.go server/internal/logic/billing/billing.go server/internal/controller/api/billing/billing.go
git commit -m "feat(billing): add recharge through ledger settlement"
```

---

## Task 9: Seed Minimal LLM Data for Smoke Testing

**Files:**
- Create: `server/migrations/0013_seed_llm_platform.sql` or add admin-only seed endpoint if migrations must remain data-free

- [ ] **Step 1: Create platform user seed strategy**

If no platform user exists, create one using a migration or documented manual command. Preferred migration:

```sql
INSERT INTO users (username, password, email, role, status)
VALUES ('platform', '', 'platform@local', 100, 1)
ON CONFLICT (username) DO NOTHING;
```

- [ ] **Step 2: Seed one channel/model/price only for local testing**

Use admin APIs for real data. If adding SQL seed, keep it local/test-oriented and do not insert real keys.

- [ ] **Step 3: Commit**

```bash
git add server/migrations/0013_seed_llm_platform.sql
git commit -m "chore(db): seed platform user for llm resources"
```

---

## Task 10: End-to-End Smoke Test

**Files:**
- Create: `server/scripts/smoke-llm.sh` or use existing smoke test style if present

- [ ] **Step 1: Start server**

Run:

```bash
cd server && go build -o /tmp/api-server.exe . && /tmp/api-server.exe
```

Expected: API server on `:8080`, gateway server on `:8081`.

- [ ] **Step 2: Register and login user**

Use existing auth endpoints:

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"llmuser","email":"llmuser@example.com","password":"pass123456"}'

curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"llmuser@example.com","password":"pass123456"}'
```

Expected: JWT token returned.

- [ ] **Step 3: Recharge credits**

```bash
curl -s -X POST http://localhost:8080/api/v1/billing/recharge \
  -H "Authorization: Bearer $JWT" \
  -H 'Content-Type: application/json' \
  -d '{"amount_credits":1000000,"ref_type":"manual_recharge","ref_id":0}'
```

Expected: transaction returned and balance increases.

- [ ] **Step 4: Create Virtual Key**

```bash
curl -s -X POST http://localhost:8080/api/v1/users/me/keys \
  -H "Authorization: Bearer $JWT" \
  -H 'Content-Type: application/json' \
  -d '{"name":"llm smoke"}'
```

Expected: `sk-...` key returned.

- [ ] **Step 5: Configure LLM resources**

Use admin/provider endpoints to create:

1. Active channel with `openai-compatible` base_url pointing to a local mock OpenAI server.
2. Active model spec `test/mock-chat`.
3. Active model price.
4. Provider model key.
5. Active key-model binding.

Expected: `/v1/models` returns `test/mock-chat`.

- [ ] **Step 6: Call chat completions**

```bash
curl -s -X POST http://localhost:8081/v1/chat/completions \
  -H "Authorization: Bearer $VIRTUAL_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"test/mock-chat","messages":[{"role":"user","content":"hello"}],"max_tokens":8}'
```

Expected:

- Upstream-compatible response returned.
- `llm_usage_records` row created.
- `settlement_records` row settled.
- `transactions` and `transaction_entries` rows created.
- Consumer credits decrease.
- Provider/platform credits increase.
- Points entries created if points rule enabled.

- [ ] **Step 7: Commit smoke script**

```bash
git add server/scripts/smoke-llm.sh
git commit -m "test(llm): add llm settlement smoke test"
```

---

## Implementation Review Checklist

- [ ] `go build ./...` passes under `server/`.
- [ ] Migrations apply on a clean database.
- [ ] Model keys are encrypted at rest and raw keys never appear in API responses.
- [ ] Provider users cannot list or disable other providers' keys.
- [ ] `/v1/models` only returns models usable by the current Virtual Key.
- [ ] LLM calls create `llm_usage_records` before settlement.
- [ ] Settlement is idempotent through `UNIQUE(ref_type, ref_id)`.
- [ ] Ledger writes happen in a DB transaction.
- [ ] Consumer debit equals provider credit plus platform commission.
- [ ] Overdraft disables all user Virtual Keys with `disabled_reason='overdraft'`.
- [ ] Recharge restores only overdraft-disabled keys.

---

## Plan Self-Review

Spec coverage:

- LLM schema: covered by Task 1.
- Service directory planning: covered in File Structure and Tasks 2-7.
- Service interfaces: covered in Detailed Service Interfaces and Task 2.
- Trading/settlement/ledger system: covered by Task 3 and migration Task 1.
- LLM management APIs: covered by Task 6.
- OpenAI-compatible data-plane: covered by Task 7.
- Recharge and overdraft recovery: covered by Task 8.
- MCP/Agent boundary: documented as out of implementation scope.

Placeholder scan:

- No implementation task uses TBD/TODO placeholders.
- Some implementation details are delegated within named files and exact methods because code size is large; each task names concrete functions and rules.

Type consistency:

- DTO names match service interface names.
- Route names match controller handler names.
- Settlement DTO names match settlement service interface.
