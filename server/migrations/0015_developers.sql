-- +goose Up
CREATE TABLE developers (
    id           BIGSERIAL PRIMARY KEY,
    name         VARCHAR(255) NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    website      VARCHAR(500) NOT NULL DEFAULT '',
    logo_url     VARCHAR(500) NOT NULL DEFAULT '',
    sort_order   INT NOT NULL DEFAULT 0,
    status       SMALLINT NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_developers_name ON developers(name);
CREATE INDEX idx_developers_status_sort ON developers(status, sort_order);

-- Seed: populate with unique developer names from existing model specs.
INSERT INTO developers (name, sort_order, status)
SELECT DISTINCT developer_name, 0, 1
FROM llm_model_specs
WHERE developer_name != ''
ON CONFLICT (name) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS developers;
