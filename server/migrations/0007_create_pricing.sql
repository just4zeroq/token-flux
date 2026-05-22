-- +goose Up
CREATE TABLE pricing_rules (
    id             BIGSERIAL PRIMARY KEY,
    item_id        BIGINT      NOT NULL REFERENCES catalog_items(id),
    strategy       VARCHAR(32) NOT NULL,   -- token | call | subscription
    params_json    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to   TIMESTAMPTZ
);

CREATE INDEX idx_pricing_rules_item ON pricing_rules(item_id);

CREATE TABLE exchange_rates (
    id            BIGSERIAL PRIMARY KEY,
    asset_from    VARCHAR(32) NOT NULL,
    asset_to      VARCHAR(32) NOT NULL,
    rate_micro    BIGINT      NOT NULL,
    fee_rate_bps  INT         NOT NULL DEFAULT 0,
    effective_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_exchange_rates_pair ON exchange_rates(asset_from, asset_to, effective_at DESC);

-- +goose Down
DROP TABLE IF EXISTS exchange_rates;
DROP TABLE IF EXISTS pricing_rules;
