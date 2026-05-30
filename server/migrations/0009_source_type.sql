-- +goose Up
-- Add source_type to llm_channels (built-in vs provider-added).
-- Provider-added channels need review (status=pending → active).
ALTER TABLE llm_channels ADD COLUMN source_type VARCHAR(32) NOT NULL DEFAULT 'admin';

-- +goose Down
ALTER TABLE llm_channels DROP COLUMN source_type;
