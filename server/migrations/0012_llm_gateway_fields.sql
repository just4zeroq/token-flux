-- +goose Up
-- +goose StatementBegin

-- Scheduler: priority & weight on channels for channel selection ordering
ALTER TABLE llm_channels ADD COLUMN IF NOT EXISTS priority INT NOT NULL DEFAULT 0;
ALTER TABLE llm_channels ADD COLUMN IF NOT EXISTS weight INT NOT NULL DEFAULT 100;
COMMENT ON COLUMN llm_channels.priority IS 'Scheduler priority (higher = preferred)';
COMMENT ON COLUMN llm_channels.weight IS 'Weighted random selection weight within same priority group';

-- Per-consumer routing strategy: routing_config JSONB on api_keys
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS routing_config JSONB NOT NULL DEFAULT '{}'::jsonb;
COMMENT ON COLUMN api_keys.routing_config IS 'Routing strategy config: {mode: "price_first"|"speed_first"|"balanced", ...}';

-- Price columns moved from llm_model_prices to llm_model_key_models (per-binding pricing)
ALTER TABLE llm_model_key_models ADD COLUMN IF NOT EXISTS cache_hit_price_per_1k BIGINT NOT NULL DEFAULT 0;
ALTER TABLE llm_model_key_models ADD COLUMN IF NOT EXISTS cache_miss_price_per_1k BIGINT NOT NULL DEFAULT 0;
ALTER TABLE llm_model_key_models ADD COLUMN IF NOT EXISTS output_price_per_1k BIGINT NOT NULL DEFAULT 0;
COMMENT ON COLUMN llm_model_key_models.cache_hit_price_per_1k IS 'Price per 1K cached input tokens';
COMMENT ON COLUMN llm_model_key_models.cache_miss_price_per_1k IS 'Price per 1K uncached input tokens';
COMMENT ON COLUMN llm_model_key_models.output_price_per_1k IS 'Price per 1K output tokens';

-- Drop llm_model_prices table — pricing is now per key-model binding
DROP TABLE IF EXISTS llm_model_prices;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Recreate llm_model_prices table
CREATE TABLE llm_model_prices (
    id                     BIGSERIAL PRIMARY KEY,
    model_spec_id          BIGINT NOT NULL,
    capability             VARCHAR(64) NOT NULL,
    currency_asset         VARCHAR(32) NOT NULL DEFAULT 'credits',
    cache_hit_price_per_1k BIGINT NOT NULL DEFAULT 0,
    cache_miss_price_per_1k BIGINT NOT NULL DEFAULT 0,
    output_price_per_1k    BIGINT NOT NULL DEFAULT 0,
    effective_from         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to           TIMESTAMPTZ NULL,
    status                 VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_llm_model_prices_model ON llm_model_prices(model_spec_id, capability, status);

-- Remove price columns from llm_model_key_models
ALTER TABLE llm_model_key_models DROP COLUMN IF EXISTS output_price_per_1k;
ALTER TABLE llm_model_key_models DROP COLUMN IF EXISTS cache_miss_price_per_1k;
ALTER TABLE llm_model_key_models DROP COLUMN IF EXISTS cache_hit_price_per_1k;

-- Remove routing_config from api_keys
ALTER TABLE api_keys DROP COLUMN IF EXISTS routing_config;

-- Remove scheduler fields from llm_channels
ALTER TABLE llm_channels DROP COLUMN IF EXISTS weight;
ALTER TABLE llm_channels DROP COLUMN IF EXISTS priority;

-- +goose StatementEnd
