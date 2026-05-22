# Monolith Skeleton — Domain Tables Plan (P1b)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the remaining 5 database migrations (wallet, billing, pricing, usage, gateway) on top of the skeleton created by `2026-05-22-monolith-skeleton.md`.

**Architecture:** Single PostgreSQL database `ai_platform`. Money uses BIGINT micro-units (value × 10^6) — never floats — to keep ledger arithmetic exact. Double-entry accounting lives in three linked tables (accounts / transactions / transaction_entries). 8 domain logical tables added in this plan, all referencing `users(id)` or `catalog_items(id)` from P1a.

**Tech Stack:** PostgreSQL 16, goose `-- +goose Up/Down` migration format.

---

## Scope

Pre-req: P1a (`2026-05-22-monolith-skeleton.md`) completed — `server/migrations/0001..0004.sql` exist, `users` and `catalog_items` tables already migrated.

In:
- `server/migrations/0005_create_wallet.sql` — `deposit_addresses`, `chain_deposits`, `withdraw_requests`
- `server/migrations/0006_create_billing.sql` — `accounts`, `transactions`, `transaction_entries`
- `server/migrations/0007_create_pricing.sql` — `pricing_rules`, `exchange_rates`
- `server/migrations/0008_create_usage.sql` — `usage_records`
- `server/migrations/0009_create_gateway.sql` — `upstream_channels`
- One git commit per file (or one combined commit covering all 5, at executor's discretion)

Out (later plans):
- DAO code generation (`gf gen dao`)
- Seed data inserts beyond schema
- Indexes added later as queries surface real workloads

---

## Task 1: Wallet migration (0005)

**Files:**
- Create: `server/migrations/0005_create_wallet.sql`

- [ ] **Step 1: Write the migration**

```sql
-- +goose Up
CREATE TABLE deposit_addresses (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT       NOT NULL REFERENCES users(id),
    chain      VARCHAR(32)  NOT NULL,
    address    VARCHAR(255) NOT NULL,
    hd_path    VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, chain),
    UNIQUE (chain, address)
);

CREATE TABLE chain_deposits (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT          NOT NULL REFERENCES users(id),
    chain              VARCHAR(32)     NOT NULL,
    tx_hash            VARCHAR(128)    NOT NULL,
    from_addr          VARCHAR(255)    NOT NULL,
    to_addr            VARCHAR(255)    NOT NULL,
    amount_native      NUMERIC(38, 18) NOT NULL,
    amount_usd_at_time BIGINT          NOT NULL,
    rate_at_time       NUMERIC(38, 18) NOT NULL,
    confirmations      INT             NOT NULL DEFAULT 0,
    status             VARCHAR(32)     NOT NULL DEFAULT 'observed',
    observed_at        TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    UNIQUE (chain, tx_hash)
);

CREATE INDEX idx_chain_deposits_user ON chain_deposits(user_id);
CREATE INDEX idx_chain_deposits_status ON chain_deposits(status);

CREATE TABLE withdraw_requests (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT       NOT NULL REFERENCES users(id),
    chain           VARCHAR(32)  NOT NULL,
    to_address      VARCHAR(255) NOT NULL,
    amount_balance  BIGINT       NOT NULL,
    fee             BIGINT       NOT NULL DEFAULT 0,
    status          VARCHAR(32)  NOT NULL DEFAULT 'pending',
    tx_hash         VARCHAR(128),
    broadcast_at    TIMESTAMPTZ,
    confirmed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_withdraw_requests_user ON withdraw_requests(user_id);
CREATE INDEX idx_withdraw_requests_status ON withdraw_requests(status);

-- +goose Down
DROP TABLE IF EXISTS withdraw_requests;
DROP TABLE IF EXISTS chain_deposits;
DROP TABLE IF EXISTS deposit_addresses;
```

- [ ] **Step 2: Stage the file**

```bash
git add server/migrations/0005_create_wallet.sql
```

---

## Task 2: Billing migration (0006) — double-entry ledger core

**Files:**
- Create: `server/migrations/0006_create_billing.sql`

- [ ] **Step 1: Write the migration**

```sql
-- +goose Up
-- Double-entry accounting core: accounts + transactions + transaction_entries.
-- All amounts in micro-units (BIGINT, value x 10^6) to avoid float drift.

CREATE TABLE accounts (
    id            BIGSERIAL PRIMARY KEY,
    owner_type    VARCHAR(32) NOT NULL,    -- user | platform
    owner_id      BIGINT      NOT NULL,
    asset         VARCHAR(32) NOT NULL,    -- credits | balance | points
    balance_micro BIGINT      NOT NULL DEFAULT 0,
    version       BIGINT      NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (owner_type, owner_id, asset)
);

CREATE INDEX idx_accounts_owner ON accounts(owner_type, owner_id);

CREATE TABLE transactions (
    id         BIGSERIAL PRIMARY KEY,
    tx_type    VARCHAR(32) NOT NULL,
    -- deposit_credits | exchange | consume | settle | withdraw | points_grant
    ref_type   VARCHAR(64),
    -- chain_deposit | order | usage_record | withdraw_request
    ref_id     BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transactions_type ON transactions(tx_type);
CREATE INDEX idx_transactions_ref ON transactions(ref_type, ref_id);

CREATE TABLE transaction_entries (
    id                  BIGSERIAL PRIMARY KEY,
    tx_id               BIGINT NOT NULL REFERENCES transactions(id),
    account_id          BIGINT NOT NULL REFERENCES accounts(id),
    delta_micro         BIGINT NOT NULL,
    balance_after_micro BIGINT NOT NULL
);

CREATE INDEX idx_transaction_entries_tx ON transaction_entries(tx_id);
CREATE INDEX idx_transaction_entries_account ON transaction_entries(account_id);

-- +goose Down
DROP TABLE IF EXISTS transaction_entries;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS accounts;
```

- [ ] **Step 2: Stage the file**

```bash
git add server/migrations/0006_create_billing.sql
```

---

## Task 3: Pricing migration (0007)

**Files:**
- Create: `server/migrations/0007_create_pricing.sql`

- [ ] **Step 1: Write the migration**

```sql
-- +goose Up
CREATE TABLE pricing_rules (
    id             BIGSERIAL PRIMARY KEY,
    item_id        BIGINT      NOT NULL REFERENCES catalog_items(id),
    strategy       VARCHAR(32) NOT NULL,   -- token | call | subscription
    params_json    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to   TIMESTAMPTZ
);

CREATE INDEX idx_pricing_rules_item ON pricing_rules(item_id);

CREATE TABLE exchange_rates (
    id            BIGSERIAL PRIMARY KEY,
    asset_from    VARCHAR(32) NOT NULL,
    asset_to      VARCHAR(32) NOT NULL,
    rate_micro    BIGINT      NOT NULL,
    fee_rate_bps  INT         NOT NULL DEFAULT 0,
    effective_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_exchange_rates_pair ON exchange_rates(asset_from, asset_to, effective_at DESC);

-- +goose Down
DROP TABLE IF EXISTS exchange_rates;
DROP TABLE IF EXISTS pricing_rules;
```

- [ ] **Step 2: Stage the file**

```bash
git add server/migrations/0007_create_pricing.sql
```

---

## Task 4: Usage migration (0008)

**Files:**
- Create: `server/migrations/0008_create_usage.sql`

- [ ] **Step 1: Write the migration**

Note on field naming: `cost_credits` / `commission_credits` / `provider_revenue_credits` / `points_to_consumer` / `points_to_provider` use the internal "credits/points" asset names. These match the design spec — only the public branding uses Flux/Karma.

```sql
-- +goose Up
CREATE TABLE usage_records (
    id                       BIGSERIAL PRIMARY KEY,
    user_id                  BIGINT       NOT NULL REFERENCES users(id),
    api_key_id               BIGINT       REFERENCES api_keys(id),
    item_id                  BIGINT       NOT NULL REFERENCES catalog_items(id),
    runtime                  VARCHAR(32)  NOT NULL,            -- llm | mcp | agent
    capability_kind          VARCHAR(64)  NOT NULL DEFAULT '', -- chat | embedding | mcp_call | agent_exec
    input_tokens             INT          NOT NULL DEFAULT 0,
    output_tokens            INT          NOT NULL DEFAULT 0,
    call_count               INT          NOT NULL DEFAULT 1,
    latency_ms               INT          NOT NULL DEFAULT 0,
    cost_credits             BIGINT       NOT NULL DEFAULT 0,
    commission_credits       BIGINT       NOT NULL DEFAULT 0,
    provider_revenue_credits BIGINT       NOT NULL DEFAULT 0,
    points_to_consumer       BIGINT       NOT NULL DEFAULT 0,
    points_to_provider       BIGINT       NOT NULL DEFAULT 0,
    request_id               VARCHAR(128) NOT NULL DEFAULT '',
    channel_id               BIGINT,
    ip                       VARCHAR(64)  NOT NULL DEFAULT '',
    is_stream                BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_usage_records_user_created ON usage_records(user_id, created_at);
CREATE INDEX idx_usage_records_item_created ON usage_records(item_id, created_at);
CREATE INDEX idx_usage_records_request_id ON usage_records(request_id);

-- +goose Down
DROP TABLE IF EXISTS usage_records;
```

- [ ] **Step 2: Stage the file**

```bash
git add server/migrations/0008_create_usage.sql
```

---

## Task 5: Gateway internal migration (0009)

**Files:**
- Create: `server/migrations/0009_create_gateway.sql`

- [ ] **Step 1: Write the migration**

```sql
-- +goose Up
-- gateway-internal: upstream channel routing for catalog_items of type=llm.
-- One catalog_item may map to multiple upstream channels for failover/priority.
CREATE TABLE upstream_channels (
    id            BIGSERIAL PRIMARY KEY,
    item_id       BIGINT       NOT NULL REFERENCES catalog_items(id),
    provider      VARCHAR(64)  NOT NULL,    -- openai | anthropic | azure | ...
    base_url      VARCHAR(512) NOT NULL,
    key_encrypted TEXT         NOT NULL,
    priority      INT          NOT NULL DEFAULT 0,
    status        VARCHAR(32)  NOT NULL DEFAULT 'active',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_upstream_channels_item ON upstream_channels(item_id);
CREATE INDEX idx_upstream_channels_status ON upstream_channels(status);

-- +goose Down
DROP TABLE IF EXISTS upstream_channels;
```

- [ ] **Step 2: Stage the file**

```bash
git add server/migrations/0009_create_gateway.sql
```

---

## Task 6: Commit all five migrations together

- [ ] **Step 1: Verify staged files**

Run: `git status --short`
Expected (order may vary):
```
A  server/migrations/0005_create_wallet.sql
A  server/migrations/0006_create_billing.sql
A  server/migrations/0007_create_pricing.sql
A  server/migrations/0008_create_usage.sql
A  server/migrations/0009_create_gateway.sql
```

- [ ] **Step 2: Commit**

```bash
git commit -m "feat(migrations): wallet, billing, pricing, usage, gateway tables

0005 deposit_addresses + chain_deposits + withdraw_requests
     HD-wallet derived per-user-per-chain addresses; deposits tracked
     observed -> confirmed -> credited; withdrawals pending -> confirmed.
0006 accounts + transactions + transaction_entries
     Double-entry ledger. All amounts BIGINT micro-units.
     accounts unique on (owner_type, owner_id, asset).
0007 pricing_rules + exchange_rates
     Per-item pricing strategies (token/call/subscription) + asset
     conversion ratios (credits <-> balance) with effective windows.
0008 usage_records
     Single table across LLM/MCP/Agent runtimes. Records cost split
     into credits / commission / provider_revenue plus points granted
     to consumer and provider for each call.
0009 upstream_channels
     gateway-internal upstream LLM channel routing (provider, base_url,
     encrypted key, priority)."
```
