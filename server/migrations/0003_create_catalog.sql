-- +goose Up
CREATE TABLE catalog_items (
    id            BIGSERIAL PRIMARY KEY,
    owner_user_id BIGINT       NOT NULL REFERENCES users(id),
    type          VARCHAR(32)  NOT NULL,
    name          VARCHAR(255) NOT NULL,
    description   TEXT         NOT NULL DEFAULT '',
    status        VARCHAR(32)  NOT NULL DEFAULT 'draft',
    review_status VARCHAR(32)  NOT NULL DEFAULT 'pending',
    version       INT          NOT NULL DEFAULT 1,
    config_json   JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_catalog_items_owner ON catalog_items(owner_user_id);
CREATE INDEX idx_catalog_items_type_status ON catalog_items(type, status);

CREATE TABLE catalog_categories (
    id        BIGSERIAL PRIMARY KEY,
    name      VARCHAR(255) NOT NULL,
    parent_id BIGINT REFERENCES catalog_categories(id)
);

CREATE TABLE catalog_item_tags (
    item_id BIGINT       NOT NULL REFERENCES catalog_items(id),
    tag     VARCHAR(128) NOT NULL,
    PRIMARY KEY (item_id, tag)
);

-- +goose Down
DROP TABLE IF EXISTS catalog_item_tags;
DROP TABLE IF EXISTS catalog_categories;
DROP TABLE IF EXISTS catalog_items;
