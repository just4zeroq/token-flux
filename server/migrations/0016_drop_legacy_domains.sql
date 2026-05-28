-- +goose Up
DROP TABLE IF EXISTS usage_records;
DROP TABLE IF EXISTS exchange_rates;
DROP TABLE IF EXISTS pricing_rules;
DROP TABLE IF EXISTS reviews;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS catalog_item_tags;
DROP TABLE IF EXISTS catalog_items;
DROP TABLE IF EXISTS catalog_categories;

-- Role semantics: 0=user (default), 1=provider
UPDATE users SET role = 0;
ALTER TABLE users ALTER COLUMN role SET DEFAULT 0;

-- +goose Down
-- Intentionally not restoring legacy tables; they had no active consumers.
-- Re-apply 0003/0004/0007/0008 if restore is needed.
