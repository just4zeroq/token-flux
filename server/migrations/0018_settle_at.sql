-- +goose Up
-- Add settle_at column for delayed provider payout (default: now + 7 days).
ALTER TABLE settlement_records ADD COLUMN IF NOT EXISTS settle_at TIMESTAMPTZ NULL;
-- Backfill existing pending records so the query index works.
UPDATE settlement_records SET settle_at = created_at + INTERVAL '7 days' WHERE status = 'pending' AND settle_at IS NULL;
-- Index for the settlement ticker query: WHERE status='pending' AND settle_at <= NOW()
CREATE INDEX IF NOT EXISTS idx_settlement_records_settle ON settlement_records(status, settle_at) WHERE status = 'pending';

-- +goose Down
DROP INDEX IF EXISTS idx_settlement_records_settle;
ALTER TABLE settlement_records DROP COLUMN IF EXISTS settle_at;
