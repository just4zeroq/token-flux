-- +goose Up
ALTER TABLE developers ADD COLUMN IF NOT EXISTS reputation_score INT NOT NULL DEFAULT 0;

-- Assign default reputation scores based on total usage (0 for seed data).
UPDATE developers d
SET reputation_score = COALESCE((
    SELECT COUNT(*) * 10
    FROM llm_usage_records ur
    JOIN llm_model_specs ms ON ur.model_spec_id = ms.id
    WHERE ms.developer_name = d.name
), 0);

-- +goose Down
ALTER TABLE developers DROP COLUMN IF EXISTS reputation_score;
