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

CREATE TABLE IF NOT EXISTS keys (
    id         TEXT PRIMARY KEY,
    label      TEXT NOT NULL DEFAULT '',
    key_value  TEXT NOT NULL,
    channel_id TEXT NOT NULL DEFAULT '',
    base_url   TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'active',
    created_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS bindings (
    id          TEXT PRIMARY KEY,
    key_id      TEXT NOT NULL REFERENCES keys(id),
    model_code  TEXT NOT NULL,
    model_name  TEXT NOT NULL DEFAULT '',
    key_hash    TEXT NOT NULL UNIQUE,
    shared      INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS usage_log (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id   TEXT NOT NULL,
    model        TEXT NOT NULL,
    provider     TEXT NOT NULL DEFAULT '',
    tokens       INTEGER NOT NULL DEFAULT 0,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cost_credits INTEGER NOT NULL DEFAULT 0,
    latency_ms   INTEGER NOT NULL DEFAULT 0,
    success      INTEGER NOT NULL DEFAULT 1,
    created_at   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS combos (
    name        TEXT PRIMARY KEY,
    models      TEXT NOT NULL DEFAULT '[]',
    strategy    TEXT NOT NULL DEFAULT 'fallback',
    sticky      INTEGER NOT NULL DEFAULT 1,
    created_at  INTEGER NOT NULL DEFAULT 0
);
`

func Open(path string) error {
	var err error
	db, err = sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	_, err = db.Exec(schema)
	return err
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
