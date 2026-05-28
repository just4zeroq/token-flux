-- +goose Up
-- LLM channel: upstream API provider configuration.
CREATE TABLE llm_channels (
    id                  BIGSERIAL PRIMARY KEY,
    code                VARCHAR(64) NOT NULL UNIQUE,
    name                VARCHAR(255) NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    protocols_json      JSONB NOT NULL DEFAULT '{}'::jsonb,
    status              VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_by_user_id  BIGINT NOT NULL DEFAULT 0,
    reviewed_by_user_id BIGINT NOT NULL DEFAULT 0,
    reviewed_at         TIMESTAMPTZ NULL,
    review_note         TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- LLM model spec: registered model definition.
CREATE TABLE llm_model_specs (
    id                   BIGSERIAL PRIMARY KEY,
    developer_name       VARCHAR(255) NOT NULL,
    model_name           VARCHAR(255) NOT NULL,
    model_code           VARCHAR(255) NOT NULL UNIQUE,
    display_name         VARCHAR(255) NOT NULL DEFAULT '',
    model_family         VARCHAR(128) NOT NULL DEFAULT '',
    description          TEXT NOT NULL DEFAULT '',
    capabilities_json    JSONB NOT NULL DEFAULT '[]'::jsonb,
    context_window       INT NOT NULL DEFAULT 0,
    max_input_tokens     INT NOT NULL DEFAULT 0,
    max_output_tokens    INT NOT NULL DEFAULT 0,
    supports_stream      BOOLEAN NOT NULL DEFAULT FALSE,
    supports_tools       BOOLEAN NOT NULL DEFAULT FALSE,
    supports_vision      BOOLEAN NOT NULL DEFAULT FALSE,
    supports_json_mode   BOOLEAN NOT NULL DEFAULT FALSE,
    supports_reasoning   BOOLEAN NOT NULL DEFAULT FALSE,
    supports_logprobs    BOOLEAN NOT NULL DEFAULT FALSE,
    supported_params_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    default_params_json  JSONB NOT NULL DEFAULT '{}'::jsonb,
    param_limits_json    JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_type          VARCHAR(32) NOT NULL DEFAULT 'admin',
    created_by_user_id   BIGINT NOT NULL DEFAULT 0,
    status               VARCHAR(32) NOT NULL DEFAULT 'pending',
    reviewed_by_user_id  BIGINT NOT NULL DEFAULT 0,
    reviewed_at          TIMESTAMPTZ NULL,
    review_note          TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(developer_name, model_name)
);

-- LLM model pricing: per-capability pricing for a model spec.
CREATE TABLE llm_model_prices (
    id                     BIGSERIAL PRIMARY KEY,
    model_spec_id          BIGINT NOT NULL,
    capability             VARCHAR(64) NOT NULL,
    currency_asset         VARCHAR(32) NOT NULL DEFAULT 'credits',
    cache_hit_price_per_1k  BIGINT NOT NULL DEFAULT 0,
    cache_miss_price_per_1k BIGINT NOT NULL DEFAULT 0,
    output_price_per_1k    BIGINT NOT NULL DEFAULT 0,
    effective_from         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to           TIMESTAMPTZ NULL,
    status                 VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_llm_model_prices_model ON llm_model_prices(model_spec_id, capability, status);

-- LLM model key: provider-owned upstream API key (encrypted at rest).
CREATE TABLE llm_model_keys (
    id                  BIGSERIAL PRIMARY KEY,
    provider_user_id    BIGINT NOT NULL,
    channel_id          BIGINT NOT NULL,
    name                VARCHAR(255) NOT NULL DEFAULT '',
    key_encrypted       TEXT NOT NULL,
    key_masked          VARCHAR(64) NOT NULL DEFAULT '',
    quota_limit_credits BIGINT NOT NULL DEFAULT 0,
    quota_used_credits  BIGINT NOT NULL DEFAULT 0,
    status              VARCHAR(32) NOT NULL DEFAULT 'pending',
    last_test_at        TIMESTAMPTZ NULL,
    last_test_status    VARCHAR(32) NOT NULL DEFAULT '',
    last_test_error     TEXT NOT NULL DEFAULT '',
    test_attempts       INT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_llm_model_keys_provider ON llm_model_keys(provider_user_id);
CREATE INDEX idx_llm_model_keys_channel ON llm_model_keys(channel_id);
CREATE INDEX idx_llm_model_keys_status ON llm_model_keys(status);

-- LLM key-model binding: links a model key to a model spec for a specific upstream model.
CREATE TABLE llm_model_key_models (
    id                    BIGSERIAL PRIMARY KEY,
    model_key_id          BIGINT NOT NULL,
    model_spec_id         BIGINT NOT NULL,
    upstream_model_name   VARCHAR(255) NOT NULL,
    quota_limit_credits   BIGINT NOT NULL DEFAULT 0,
    quota_used_credits    BIGINT NOT NULL DEFAULT 0,
    provider_share_bps    INT NOT NULL DEFAULT 0,
    status                VARCHAR(32) NOT NULL DEFAULT 'pending_test',
    last_test_at          TIMESTAMPTZ NULL,
    last_test_status      VARCHAR(32) NOT NULL DEFAULT '',
    last_test_error       TEXT NOT NULL DEFAULT '',
    test_attempts         INT NOT NULL DEFAULT 0,
    consecutive_failures  INT NOT NULL DEFAULT 0,
    last_error_code       VARCHAR(64) NOT NULL DEFAULT '',
    last_error_message    TEXT NOT NULL DEFAULT '',
    last_error_at         TIMESTAMPTZ NULL,
    last_success_at       TIMESTAMPTZ NULL,
    next_test_at          TIMESTAMPTZ NULL,
    priority              INT NOT NULL DEFAULT 0,
    weight                INT NOT NULL DEFAULT 1,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(model_key_id, model_spec_id, upstream_model_name)
);
CREATE INDEX idx_llm_key_models_model ON llm_model_key_models(model_spec_id, status);
CREATE INDEX idx_llm_key_models_key ON llm_model_key_models(model_key_id);
CREATE INDEX idx_llm_key_models_next_test ON llm_model_key_models(status, next_test_at);

-- LLM usage record: per-request billing detail, written before settlement.
CREATE TABLE llm_usage_records (
    id                       BIGSERIAL PRIMARY KEY,
    consumer_user_id         BIGINT NOT NULL,
    virtual_key_id           BIGINT NOT NULL DEFAULT 0,
    model_spec_id            BIGINT NOT NULL,
    model_key_id             BIGINT NOT NULL,
    key_model_id             BIGINT NOT NULL,
    provider_user_id         BIGINT NOT NULL,
    channel_id               BIGINT NOT NULL,
    request_id               VARCHAR(128) NOT NULL DEFAULT '',
    external_request_id      VARCHAR(128) NOT NULL DEFAULT '',
    capability               VARCHAR(64) NOT NULL,
    is_stream                BOOLEAN NOT NULL DEFAULT FALSE,
    input_tokens             INT NOT NULL DEFAULT 0,
    cache_hit_tokens         INT NOT NULL DEFAULT 0,
    cache_miss_tokens        INT NOT NULL DEFAULT 0,
    output_tokens            INT NOT NULL DEFAULT 0,
    total_tokens             INT NOT NULL DEFAULT 0,
    cost_credits             BIGINT NOT NULL DEFAULT 0,
    provider_revenue_credits BIGINT NOT NULL DEFAULT 0,
    commission_credits       BIGINT NOT NULL DEFAULT 0,
    latency_ms               INT NOT NULL DEFAULT 0,
    status                   VARCHAR(32) NOT NULL DEFAULT 'success',
    error_code               VARCHAR(64) NOT NULL DEFAULT '',
    error_message            TEXT NOT NULL DEFAULT '',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_llm_usage_consumer_created ON llm_usage_records(consumer_user_id, created_at);
CREATE INDEX idx_llm_usage_provider_created ON llm_usage_records(provider_user_id, created_at);
CREATE INDEX idx_llm_usage_model_created ON llm_usage_records(model_spec_id, created_at);
CREATE INDEX idx_llm_usage_key_model_created ON llm_usage_records(key_model_id, created_at);
CREATE INDEX idx_llm_usage_request_id ON llm_usage_records(request_id);

-- +goose Down
DROP TABLE IF EXISTS llm_usage_records;
DROP TABLE IF EXISTS llm_model_key_models;
DROP TABLE IF EXISTS llm_model_keys;
DROP TABLE IF EXISTS llm_model_prices;
DROP TABLE IF EXISTS llm_model_specs;
DROP TABLE IF EXISTS llm_channels;
