-- +goose Up
-- Add user_id column to nodes table for settlement.
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS user_id BIGINT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS node_models (
    id              BIGSERIAL PRIMARY KEY,
    wallet_address  TEXT NOT NULL REFERENCES nodes(wallet_address),
    user_id         BIGINT NOT NULL DEFAULT 0,
    model_code      TEXT NOT NULL,
    key_hash        TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'active',
    input_price     BIGINT NOT NULL DEFAULT 0,
    output_price    BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(wallet_address, model_code, key_hash)
);

CREATE INDEX IF NOT EXISTS idx_nm_model   ON node_models(model_code);
CREATE INDEX IF NOT EXISTS idx_nm_wallet  ON node_models(wallet_address);

-- +goose Down
DROP TABLE IF EXISTS node_models;
