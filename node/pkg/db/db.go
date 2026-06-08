package db

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

var db *sql.DB

const schema = `
CREATE TABLE IF NOT EXISTS node_config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS llm_channels (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    code              TEXT NOT NULL UNIQUE,
    name              TEXT NOT NULL,
    description       TEXT NOT NULL DEFAULT '',
    provider_type     INTEGER NOT NULL DEFAULT 1,
    protocols_json    TEXT NOT NULL DEFAULT '[]',
    status            TEXT NOT NULL DEFAULT 'active',
    source            TEXT NOT NULL DEFAULT 'local',
    created_at        INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS llm_model_specs (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    developer_name      TEXT NOT NULL DEFAULT '',
    model_name          TEXT NOT NULL,
    model_code          TEXT NOT NULL UNIQUE,
    display_name        TEXT NOT NULL DEFAULT '',
    model_family        TEXT NOT NULL DEFAULT '',
    description         TEXT NOT NULL DEFAULT '',
    capabilities_json   TEXT NOT NULL DEFAULT '[]',
    context_window      INTEGER NOT NULL DEFAULT 0,
    max_input_tokens    INTEGER NOT NULL DEFAULT 0,
    max_output_tokens   INTEGER NOT NULL DEFAULT 0,
    supports_stream     INTEGER NOT NULL DEFAULT 0,
    supports_tools      INTEGER NOT NULL DEFAULT 0,
    supports_vision     INTEGER NOT NULL DEFAULT 0,
    supports_json_mode  INTEGER NOT NULL DEFAULT 0,
    supports_reasoning  INTEGER NOT NULL DEFAULT 0,
    supports_logprobs   INTEGER NOT NULL DEFAULT 0,
    status              TEXT NOT NULL DEFAULT 'active',
    source              TEXT NOT NULL DEFAULT 'local',
    created_at          INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS llm_model_prices (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    model_spec_id       INTEGER NOT NULL,
    capability          TEXT NOT NULL DEFAULT 'chat',
    input_price_per_1k  INTEGER NOT NULL DEFAULT 0,
    output_price_per_1k INTEGER NOT NULL DEFAULT 0,
    status              TEXT NOT NULL DEFAULT 'active',
    created_at          INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS llm_channel_models (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id          INTEGER NOT NULL,
    model_spec_id       INTEGER NOT NULL,
    upstream_model_name TEXT NOT NULL DEFAULT '',
    source              TEXT NOT NULL DEFAULT 'local',
    created_at          INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS llm_model_keys (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id          INTEGER NOT NULL DEFAULT 0,
    name                TEXT NOT NULL DEFAULT '',
    key_encrypted       TEXT NOT NULL,
    key_masked          TEXT NOT NULL DEFAULT '',
    base_url            TEXT NOT NULL DEFAULT '',
    quota_limit_credits INTEGER NOT NULL DEFAULT 0,
    quota_used_credits  INTEGER NOT NULL DEFAULT 0,
    status              TEXT NOT NULL DEFAULT 'active',
    created_at          INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS llm_model_key_models (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    model_key_id        INTEGER NOT NULL,
    model_spec_id       INTEGER NOT NULL DEFAULT 0,
    upstream_model_name TEXT NOT NULL,
    model_code          TEXT NOT NULL DEFAULT '',
    key_hash            TEXT NOT NULL DEFAULT '',
    priority            INTEGER NOT NULL DEFAULT 0,
    weight              INTEGER NOT NULL DEFAULT 1,
    status              TEXT NOT NULL DEFAULT 'active',
    last_error          TEXT NOT NULL DEFAULT '',
    created_at          INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL,
    UNIQUE(model_key_id, model_spec_id, upstream_model_name)
);

CREATE TABLE IF NOT EXISTS llm_usage_records (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id        TEXT NOT NULL DEFAULT '',
    model_spec_id     INTEGER NOT NULL DEFAULT 0,
    model_key_id      INTEGER NOT NULL DEFAULT 0,
    channel_id        INTEGER NOT NULL DEFAULT 0,
    capability        TEXT NOT NULL DEFAULT 'chat',
    is_stream         INTEGER NOT NULL DEFAULT 0,
    input_tokens      INTEGER NOT NULL DEFAULT 0,
    output_tokens     INTEGER NOT NULL DEFAULT 0,
    total_tokens      INTEGER NOT NULL DEFAULT 0,
    cost_credits      INTEGER NOT NULL DEFAULT 0,
    latency_ms        INTEGER NOT NULL DEFAULT 0,
    status            TEXT NOT NULL DEFAULT 'success',
    error_code        TEXT NOT NULL DEFAULT '',
    error_message     TEXT NOT NULL DEFAULT '',
    source            TEXT NOT NULL DEFAULT 'local',
    created_at        INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS node_api_keys (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    key_hash      TEXT NOT NULL UNIQUE,
    key_prefix    TEXT NOT NULL,
    key_value     TEXT NOT NULL DEFAULT '',
    label         TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'active',
    last_used_at  INTEGER NOT NULL DEFAULT 0,
    created_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS node_combos (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    models      TEXT NOT NULL DEFAULT '[]',
    strategy    TEXT NOT NULL DEFAULT 'fallback',
    sticky      INTEGER NOT NULL DEFAULT 1,
    created_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS wallet (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    public_key     TEXT NOT NULL,
    seed_encrypted BLOB NOT NULL,
    nonce          BLOB NOT NULL,
    created_at     INTEGER NOT NULL
);
`

func Open(path string) error {
	var err error
	db, err = sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	_, err = db.Exec(schema)
	if err != nil {
		return err
	}

	// Migration: add source column to existing tables (ignore errors if already exist)
	db.Exec("ALTER TABLE llm_channels ADD COLUMN source TEXT NOT NULL DEFAULT 'local'")
	db.Exec("ALTER TABLE llm_model_specs ADD COLUMN source TEXT NOT NULL DEFAULT 'local'")
	db.Exec("ALTER TABLE llm_usage_records ADD COLUMN source TEXT NOT NULL DEFAULT 'local'")

	// Migration: add name and status columns to wallet
	db.Exec("ALTER TABLE wallet ADD COLUMN name TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE wallet ADD COLUMN status TEXT NOT NULL DEFAULT 'active'")

	return nil
}

func DB() *sql.DB { return db }

func GetConfig(key string) (string, error) {
	var v string
	err := db.QueryRow("SELECT value FROM node_config WHERE key = ?", key).Scan(&v)
	return v, err
}

func SetConfig(key, value string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO node_config(key, value) VALUES (?, ?)", key, value)
	return err
}
