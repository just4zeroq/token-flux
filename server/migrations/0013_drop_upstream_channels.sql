-- +goose Up
DROP TABLE IF EXISTS upstream_channels;

-- +goose Down
-- Intentionally not recreating: superseded by llm_channels (0011_llm_core.sql).
-- If rollback is needed, recover via `goose down` of 0011 then 0010, then re-apply 0009.
