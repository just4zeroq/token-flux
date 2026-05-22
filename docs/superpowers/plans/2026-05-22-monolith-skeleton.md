# Monolith Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish the new monolith Go module under `server/`, replace the 5-microservice layout with a single GoFrame service/logic-pattern codebase, write all merged DB migrations, and produce a runnable `cmd/api` binary that starts two empty `ghttp.Server` instances on `:8080` and `:8081`.

**Architecture:** A single Go module `ai-platform` rooted at `server/`. Internal layout follows GoFrame conventions: `service/` (interfaces) + `logic/` (implementations registered via `init()`). Multi-`cmd/` entry-points share `internal/`. Single PostgreSQL database `ai_platform` with all tables. P1 produces only the skeleton — every domain interface has an empty stub implementation. Real logic lands in P2–P7.

**Tech Stack:** Go 1.25, GoFrame v2.10, PostgreSQL 16, goose migrations, `github.com/gogf/gf/contrib/drivers/pgsql/v2`.

---

## Scope

In:
- Worktree creation
- Nuke `server/{api,proto,api-gateway,ai-gateway,user-svc,asset-svc,market-svc,docker,scripts,ai-platform}`
- New `server/go.mod` + directory skeleton
- All merged migrations (8 domain × tables)
- 10 service interfaces with empty stub implementations
- `cmd/api/main.go` → two `ghttp.Server` with only `/health`
- `cmd/watcher/main.go`, `cmd/recorder/main.go`, `cmd/settlement/main.go` placeholders (no-op `g.Wait()`)
- Config `manifest/config/config.yaml`
- Verification: `go build ./...`, `go run ./cmd/api`, `goose up` succeeds, `curl :8080/health` and `curl :8081/health` return 200

Out (later plans):
- Any real business logic (identity register/login, billing accounting, gateway relay, wallet chain code, etc.)
- DAO code generation (`gf gen dao`)
- Docker compose updates
- Frontend changes
- MCP / Agent route handlers

---

## File Structure

```
server/
├── go.mod                                  (NEW, module ai-platform)
├── go.sum                                  (NEW, auto-generated)
├── main.go                                 (NEW, thin shim calling cmd/api)
├── manifest/config/config.yaml             (NEW)
├── migrations/                             (NEW, see Task 5 for list)
├── hack/config.yaml                        (NEW, gf gen dao config)
├── cmd/
│   ├── api/main.go                         (NEW)
│   ├── watcher/main.go                     (NEW, placeholder)
│   ├── recorder/main.go                    (NEW, placeholder)
│   └── settlement/main.go                  (NEW, placeholder)
├── internal/
│   ├── boot/boot.go                        (NEW, two-server setup)
│   ├── consts/consts.go                    (NEW, empty)
│   ├── middleware/common.go                (NEW, recover/cors/requestid)
│   ├── service/                            (NEW, 10 interfaces)
│   │   ├── identity.go
│   │   ├── catalog.go
│   │   ├── market.go
│   │   ├── wallet.go
│   │   ├── billing.go
│   │   ├── pricing.go
│   │   ├── usage.go
│   │   ├── gateway_llm.go
│   │   ├── gateway_mcp.go
│   │   └── gateway_agent.go
│   ├── logic/                              (NEW, stubs + init registration)
│   │   ├── logic.go                        (blank imports)
│   │   ├── identity/identity.go
│   │   ├── catalog/catalog.go
│   │   ├── market/market.go
│   │   ├── wallet/wallet.go
│   │   ├── billing/billing.go
│   │   ├── pricing/pricing.go
│   │   ├── usage/usage.go
│   │   └── gateway/
│   │       ├── gateway.go                  (blank imports llm/mcp/agent)
│   │       ├── llm/llm.go
│   │       ├── mcp/mcp.go
│   │       └── agent/agent.go
│   └── model/dto/                          (NEW, empty placeholder package)
│       └── dto.go
├── resource/                               (NEW, empty)
└── README.md                               (NEW, short pointer to spec)

DELETED entirely:
- server/api/
- server/proto/
- server/api-gateway/
- server/ai-gateway/
- server/user-svc/
- server/asset-svc/
- server/market-svc/
- server/docker/
- server/scripts/
- server/ai-platform/
```

---

## Task 1: Create isolation worktree

**Files:** (no source changes — worktree setup only)

- [ ] **Step 1: Invoke `superpowers:using-git-worktrees` skill**

Tell the skill the feature name is `monolith-skeleton`. The skill will create a worktree at `.claude/worktrees/monolith-skeleton/` (or platform equivalent) branched from current HEAD, then switch the session into it.

Expected: prompt confirming worktree created and session CWD switched.

- [ ] **Step 2: Confirm worktree is active**

Run: `git rev-parse --abbrev-ref HEAD`
Expected: branch name containing `monolith-skeleton` (not `master`).

Run: `pwd`
Expected: path under `.claude/worktrees/` (or platform-specific worktree root).

- [ ] **Step 3: Confirm clean working tree in worktree**

Run: `git status --short`
Expected: empty output (worktree starts clean from base ref).

If output is not empty, stop and ask the user — the worktree base ref may have been wrong.

---

## Task 2: Delete all old server subdirectories

**Files:**
- Delete: `server/api/`
- Delete: `server/proto/`
- Delete: `server/api-gateway/`
- Delete: `server/ai-gateway/`
- Delete: `server/user-svc/`
- Delete: `server/asset-svc/`
- Delete: `server/market-svc/`
- Delete: `server/docker/`
- Delete: `server/scripts/`
- Delete: `server/ai-platform/`

- [ ] **Step 1: Verify these are the only top-level dirs under server/**

Run: `ls server/`
Expected output (order may vary):
```
ai-gateway/
ai-platform/
api/
api-gateway/
asset-svc/
docker/
market-svc/
proto/
scripts/
user-svc/
```
If anything else appears (e.g. existing `internal/`, `cmd/`, `go.mod`), STOP and inspect — the worktree may not be a clean base.

- [ ] **Step 2: Remove the 10 subdirectories**

Run:
```bash
rm -rf server/api server/proto server/api-gateway server/ai-gateway \
       server/user-svc server/asset-svc server/market-svc \
       server/docker server/scripts server/ai-platform
```

- [ ] **Step 3: Verify server/ is now empty**

Run: `ls server/ 2>/dev/null; echo "exit=$?"`
Expected: nothing before the `exit=0` line (the directory exists but is empty).

If `server/` was accidentally deleted: `mkdir server`.

- [ ] **Step 4: Commit the deletion**

```bash
git add -A
git commit -m "chore: nuke old microservice directories for monolith refactor"
```

Expected: `git status` shows clean tree after commit.

---

## Task 3: Initialize new Go module

**Files:**
- Create: `server/go.mod`
- Create: `server/main.go`

- [ ] **Step 1: Create go.mod via `go mod init`**

```bash
cd server
go mod init ai-platform
cd ..
```

Verify `server/go.mod` contains exactly:
```
module ai-platform

go 1.25
```

(Go may write `go 1.25.0` — that's fine.)

- [ ] **Step 2: Add dependencies**

```bash
cd server
go get github.com/gogf/gf/v2@v2.10.0
go get github.com/gogf/gf/contrib/drivers/pgsql/v2@v2.10.0
go get github.com/golang-jwt/jwt/v5@v5.3.1
go get golang.org/x/crypto@v0.51.0
cd ..
```

Expected: `server/go.mod` gains `require` entries for each; `server/go.sum` created.

- [ ] **Step 3: Create the thin `server/main.go` shim**

Write to `server/main.go`:
```go
package main

import (
	_ "ai-platform/internal/logic"

	"ai-platform/internal/boot"
)

func main() { boot.RunAPI() }
```

(This file lets `go run ./server` work for local quickstart; `cmd/api/main.go` is the real production entry.)

- [ ] **Step 4: Verify module structure**

Run: `cd server && go list -m && cd ..`
Expected: `ai-platform`

- [ ] **Step 5: Commit**

```bash
git add server/go.mod server/go.sum server/main.go
git commit -m "feat(server): initialize ai-platform go module"
```

---

## Task 4: Write config and minimal directory layout

**Files:**
- Create: `server/manifest/config/config.yaml`
- Create: `server/resource/.gitkeep`
- Create: `server/README.md`
- Create: `server/hack/config.yaml`

- [ ] **Step 1: Create directories**

```bash
mkdir -p server/manifest/config server/resource server/hack server/migrations \
         server/cmd/api server/cmd/watcher server/cmd/recorder server/cmd/settlement \
         server/internal/boot server/internal/consts server/internal/middleware \
         server/internal/service server/internal/logic server/internal/model/dto \
         server/internal/logic/identity server/internal/logic/catalog \
         server/internal/logic/market server/internal/logic/wallet \
         server/internal/logic/billing server/internal/logic/pricing \
         server/internal/logic/usage server/internal/logic/gateway \
         server/internal/logic/gateway/llm server/internal/logic/gateway/mcp \
         server/internal/logic/gateway/agent
touch server/resource/.gitkeep
```

- [ ] **Step 2: Write `server/manifest/config/config.yaml`**

```yaml
server:
  api:
    address: ":8080"
  gateway:
    address: ":8081"

database:
  default:
    type: "pgsql"
    host: "127.0.0.1"
    port: "5432"
    user: "aiplatform"
    pass: "aiplatform"
    name: "ai_platform"
    debug: false

logger:
  level: "info"
  stdout: true

jwt:
  secret: "change-me-in-prod"
  expireHours: 24

wallet:
  hd:
    mnemonic_env: "WALLET_MNEMONIC"
  chains:
    bnb:
      rpc: "https://bsc-dataseed.binance.org"
      confirmations: 12
      poll_interval_seconds: 5
    base:
      rpc: "https://mainnet.base.org"
      confirmations: 12
      poll_interval_seconds: 5
    ton:
      api: "https://toncenter.com/api/v2"
      confirmations: 1
      poll_interval_seconds: 5

usage:
  recorder:
    batch_size: 50
    flush_interval_ms: 100

settlement:
  cron: "0 0 3 * * *"
```

- [ ] **Step 3: Write `server/hack/config.yaml` (gf gen dao config)**

```yaml
gfcli:
  gen:
    dao:
      - link: "pgsql:aiplatform:aiplatform@tcp(127.0.0.1:5432)/ai_platform"
        path: "./internal"
        descriptionTag: true
        noJsonTag: false
        noModelComment: false
        clear: false
        prefix: ""
        removePrefix: ""
        jsonCase: "CamelLower"
```

- [ ] **Step 4: Write `server/README.md`**

```markdown
# ai-platform server

Monolith Go service for the AI capability platform. See
[../docs/superpowers/specs/2026-05-22-monolith-refactor-design.md](../docs/superpowers/specs/2026-05-22-monolith-refactor-design.md)
for architecture and design decisions.

## Layout

- `cmd/api`        — main HTTP server (`:8080` API + `:8081` gateway)
- `cmd/watcher`    — wallet on-chain deposit watcher
- `cmd/recorder`   — usage record aggregator / archiver
- `cmd/settlement` — periodic provider revenue settlement cron
- `internal/`      — shared library (service interfaces + logic implementations)
- `migrations/`    — goose SQL migrations against `ai_platform` database

## Local quickstart

```
cd server
go run ./cmd/api
```
```

- [ ] **Step 5: Commit**

```bash
git add server/manifest server/resource server/hack server/README.md
git commit -m "feat(server): add config, hack, resource directories"
```

---

## Task 5: Write merged migrations (identity + catalog + market)

**Files:**
- Create: `server/migrations/0001_create_users.sql`
- Create: `server/migrations/0002_create_api_keys.sql`
- Create: `server/migrations/0003_create_catalog.sql`
- Create: `server/migrations/0004_create_market.sql`

All migrations use goose format with `-- +goose Up` / `-- +goose Down` markers.

- [ ] **Step 1: Write `server/migrations/0001_create_users.sql`**

```sql
-- +goose Up
CREATE TABLE users (
    id           BIGSERIAL PRIMARY KEY,
    username     VARCHAR(255) NOT NULL UNIQUE,
    password     VARCHAR(255) NOT NULL,
    email        VARCHAR(255) NOT NULL DEFAULT '',
    phone        VARCHAR(64)  NOT NULL DEFAULT '',
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    avatar       VARCHAR(512) NOT NULL DEFAULT '',
    source       VARCHAR(64)  NOT NULL DEFAULT 'email',
    role         INT          NOT NULL DEFAULT 1,
    status       INT          NOT NULL DEFAULT 1,
    group_name   VARCHAR(64)  NOT NULL DEFAULT 'default',
    kyc_status   VARCHAR(32)  NOT NULL DEFAULT 'none',
    remark       TEXT         NOT NULL DEFAULT '',
    tenant_id    BIGINT       NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS users;
```

- [ ] **Step 2: Write `server/migrations/0002_create_api_keys.sql`**

```sql
-- +goose Up
CREATE TABLE api_keys (
    id                   BIGSERIAL PRIMARY KEY,
    user_id              BIGINT       NOT NULL REFERENCES users(id),
    key                  VARCHAR(255) NOT NULL UNIQUE,
    name                 VARCHAR(255) NOT NULL DEFAULT '',
    status               INT          NOT NULL DEFAULT 1,
    expire_time          TIMESTAMPTZ,
    model_limits         TEXT         NOT NULL DEFAULT '',
    model_limits_enabled BOOLEAN      NOT NULL DEFAULT FALSE,
    group_name           VARCHAR(64)  NOT NULL DEFAULT 'default',
    quota_credits        BIGINT,
    remain_quota         BIGINT       NOT NULL DEFAULT 0,
    used_quota           BIGINT       NOT NULL DEFAULT 0,
    unlimited_quota      BOOLEAN      NOT NULL DEFAULT FALSE,
    allow_ips            TEXT         NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_key ON api_keys(key);

-- +goose Down
DROP TABLE IF EXISTS api_keys;
```

- [ ] **Step 3: Write `server/migrations/0003_create_catalog.sql`**

```sql
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
```

- [ ] **Step 4: Write `server/migrations/0004_create_market.sql`**

```sql
-- +goose Up
CREATE TABLE orders (
    id             BIGSERIAL PRIMARY KEY,
    order_no       VARCHAR(64)  NOT NULL UNIQUE,
    buyer_user_id  BIGINT       NOT NULL REFERENCES users(id),
    item_id        BIGINT       NOT NULL REFERENCES catalog_items(id),
    plan_type      VARCHAR(32)  NOT NULL DEFAULT 'one_off',
    amount_credits BIGINT       NOT NULL,
    payment_method VARCHAR(64)  NOT NULL DEFAULT '',
    status         VARCHAR(32)  NOT NULL DEFAULT 'pending',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_buyer ON orders(buyer_user_id);
CREATE INDEX idx_orders_status ON orders(status);

CREATE TABLE reviews (
    id            BIGSERIAL PRIMARY KEY,
    buyer_user_id BIGINT       NOT NULL REFERENCES users(id),
    item_id       BIGINT       NOT NULL REFERENCES catalog_items(id),
    rating        SMALLINT     NOT NULL,
    body          TEXT         NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reviews_item ON reviews(item_id);

-- +goose Down
DROP TABLE IF EXISTS reviews;
DROP TABLE IF EXISTS orders;
```

- [ ] **Step 5: Commit**

```bash
git add server/migrations/0001_create_users.sql server/migrations/0002_create_api_keys.sql \
        server/migrations/0003_create_catalog.sql server/migrations/0004_create_market.sql
git commit -m "feat(migrations): identity + catalog + market tables"
```

---

---

## TASKS 6-12: NOT YET WRITTEN

This plan stops after Task 5. Tasks 6-12 will be authored after Tasks 1-5 are executed and the basic skeleton is verified to build, since execution will surface concrete decisions (e.g. whether `gf gen dao` runs cleanly against the schemas in Task 5).

**Remaining scope to be planned later:**

- Task 6: migrations for `wallet` (deposit_addresses, chain_deposits, withdraw_requests), `billing` (accounts, transactions, transaction_entries), `pricing` (pricing_rules, exchange_rates), `usage` (usage_records), `gateway` (upstream_channels)
- Task 7: `internal/middleware/common.go` (CORS / Recover / RequestID) and `internal/boot/boot.go` (two `ghttp.Server` setup with `/health`)
- Task 8: 10 `internal/service/*.go` interface files (IIdentity, ICatalog, IMarket, IWallet, IBilling, IPricing, IUsage, ILLMRuntime, IMCPRuntime, IAgentRuntime) with `RegisterXxx` / `Xxx()` accessors
- Task 9: 8 `internal/logic/<domain>/<domain>.go` empty stubs with `init() { service.RegisterXxx(New()) }`, plus `internal/logic/logic.go` blank-imports aggregator, plus `internal/logic/gateway/gateway.go` aggregator for llm/mcp/agent
- Task 10: `cmd/api/main.go` + placeholder `cmd/{watcher,recorder,settlement}/main.go`
- Task 11: Verification — `go build ./...`, run goose migrations against a fresh `ai_platform` database, `go run ./cmd/api`, `curl :8080/health` and `curl :8081/health`
- Task 12: Spec self-review against the design doc, final commit summary
