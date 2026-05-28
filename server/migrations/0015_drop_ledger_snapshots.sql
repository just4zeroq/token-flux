-- +goose Up
DROP TABLE IF EXISTS ledger_snapshots;

-- +goose Down
-- ledger_snapshots was never written to or queried. If restoring, re-apply 0012_settlement_ledger.sql.
