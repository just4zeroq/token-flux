# AI Platform — Complete Development Roadmap

> Last updated: 2026-05-31
> 
> ## Progress
> | Batch | Status |
> |-------|--------|
> | 1 Backend | ✅ Done |
> | 2 Model Detail | ✅ Done |
> | 3 Providers | ✅ Done |
> | 4 Console | ✅ Done |
> | 5 i18n | ✅ Infrastructure done |
> | 6 Go Node | ✅ Done |
> | 7 Tauri Desktop | Pending |
> Covers: model detail page / console overview / providers page / i18n / Go local node / Tauri desktop client

---

## Table of Contents

1. [Execution Order](#1-execution-order)
2. [Batch 1: Backend Changes](#2-batch-1-backend-changes)
3. [Batch 2: Model Detail Page](#3-batch-2-model-detail-page)
4. [Batch 3: Providers Page](#4-batch-3-providers-page)
5. [Batch 4: Console Overview](#5-batch-4-console-overview)
6. [Batch 5: i18n Internationalization](#6-batch-5-i18n-internationalization)
7. [Batch 6: Go Local Node](#7-batch-6-go-local-node)
8. [Batch 7: Tauri Desktop Client](#8-batch-7-tauri-desktop-client)

---

## 1. Execution Order

| Batch | Description | Dependencies |
|-------|-------------|-------------|
| **1** | Backend changes (migration + APIs + routes) | None |
| **2** | Model detail page (`/models/:code`) | Batch 1 |
| **3** | Providers pages (`/providers` + `/providers/:name`) | Batch 1 |
| **4** | Console Overview optimization | Batch 1 |
| **5** | i18n (EN/ZH/JA/KO/VI) | None (can overlap) |
| **6** | Go local node (`cmd/node/`) | Batch 1 |
| **7** | Tauri desktop client (`app/desktop/`) | Batch 6 |

Each batch completes before the next begins. Backend restarts only once (end of Batch 1).

---

## 2. Batch 1: Backend Changes

### 2.1 Migration 0017 — developer reputation score

**File:** `server/migrations/0017_developers_reputation.sql`

```sql
ALTER TABLE developers ADD COLUMN IF NOT EXISTS reputation_score INT NOT NULL DEFAULT 0;

-- Seed: assign default scores based on usage volume
UPDATE developers d
SET reputation_score = COALESCE((
    SELECT COUNT(*) * 10
    FROM llm_usage_records ur
    JOIN llm_model_specs ms ON ur.model_spec_id = ms.id
    WHERE ms.developer_name = d.name
), 0);
```

### 2.2 Catalog API — model detail

**Endpoint:** `GET /api/v1/catalog/models/:code` (public, no auth)

**Handler:** `catalog.go` — `GetModelDetail` ✅ already implemented

**Response:**
```json
{
  "code": 0,
  "data": {
    "model": { "id", "developer_name", "model_name", "model_code", "display_name", "model_family", "description", "capabilities", "context_window", "status" },
    "providers": [
      { "channel_id", "channel_name", "channel_code", "upstream_model_name",
        "cache_hit_price_per_1k", "cache_miss_price_per_1k", "output_price_per_1k" }
    ]
  }
}
```

**Route registration:** ✅ already in `boot.go`

### 2.3 Catalog API — provider list

**Endpoint:** `GET /api/v1/catalog/providers?sort=&search=` (public, no auth)

**Handler:** `catalog.go` — `ListProviders` ✅ already implemented

**Sort options:** `model_count` (default), `calls`, `reputation`

**Response:**
```json
{
  "code": 0,
  "data": {
    "list": [
      { "id", "name", "description", "website", "logo_url", "sort_order", "reputation_score",
        "model_count", "total_calls", "total_tokens", "total_credits" }
    ]
  }
}
```

### 2.4 Catalog API — provider detail

**Endpoint:** `GET /api/v1/catalog/providers/:name/models` (public, no auth)

**Handler:** `catalog.go` — `GetProviderModels` ✅ already implemented

**Response:**
```json
{
  "code": 0,
  "data": {
    "developer": { "id", "name", "description", "website", "logo_url", "reputation_score" },
    "stats": { "model_count", "total_calls", "total_tokens" },
    "models": [
      { "id", "model_name", "model_code", "display_name", "model_family", "context_window",
        "capabilities", "input_price_per_1k", "output_price_per_1k", "cache_miss_price_per_1k", "channel_name" }
    ]
  }
}
```

### 2.5 Route registration

**File:** `server/internal/boot/boot.go`

Need to add:
```go
v1.GET("/catalog/models/:code", catalogapi.GetModelDetail)    // ✅ already in
v1.GET("/catalog/providers", catalogapi.ListProviders)         // need to add
v1.GET("/catalog/providers/:name/models", catalogapi.GetProviderModels) // need to add
```

### 2.6 Settlement stats

**Status:** ✅ Already done.
- `dto.ProviderSettlementStats` has `PendingRevenueCredits` + `PendingCount`
- `GetProviderStats` already queries both settled and pending aggregates

---

## 3. Batch 2: Model Detail Page

### 3.1 Route

**File:** `app/web/src/routes/models.$code.tsx` (new)

**Route:** `/models/:code`

### 3.2 Layout

```
← Models  /  {model_name}
┌──────────────────────────────────────────────┐
│  {display_name}               [Try in Console] │
│  {developer_name} · {model_family} · {ctx}K   │
│  {capabilities}                                │
│  {description}                                 │
├──────────────────────────────────────────────┤
│  Available Providers                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐     │
│  │ProviderA │  │ProviderB │  │ProviderC │     │
│  │ Input:0.50 │  │ Input:0.60 │  │ Input:0.55 │     │
│  │ Output:1.50│  │ Output:1.80│  │ Output:1.60│     │
│  │ Cache:0.25 │  │ Cache:0.30 │  │ Cache:0.28 │     │
│  └──────────┘  └──────────┘  └──────────┘     │
└──────────────────────────────────────────────┘
```

### 3.3 Data source

Fetch from `GET /api/v1/catalog/models/:code` via Vite proxy.

### 3.4 Models list page update

**File:** `app/web/src/routes/models.tsx`

Changes:
- Remove drawer (`selectedModel` state, prices state, drawer JSX)
- Card `onClick` → `navigate({ to: '/models/$code', params: { code: m.model_code } })`

---

## 4. Batch 3: Providers Page

### 4.1 Providers list page

**File:** `app/web/src/routes/providers.tsx` (rewrite)

Features:
- Search bar (by name)
- Sort dropdown: Model Count / Call Volume / Reputation
- Provider cards: logo (first letter), name, model count, total calls, total tokens, reputation score
- Card click → `navigate({ to: '/providers/$name', params: { name } })`

### 4.2 Provider detail page

**File:** `app/web/src/routes/providers.$name.tsx` (new)

Layout:
```
← Providers  /  {name}
┌─ Developer Info ──────────────────────────────┐
│ Logo + Name + Description + Website            │
│ Reputation: ★★★★☆                               │
├─ Stats ───────────────────────────────────────┐
│ Models: {n}  │  Calls: {n}  │  Tokens: {n}    │
├─ Models ──────────────────────────────────────┐
│ Each model with pricing (input/output/cache)   │
│ per channel                                    │
└───────────────────────────────────────────────┘
```

---

## 5. Batch 4: Console Overview

**File:** `app/web/src/routes/console.tsx` — `OverviewTab`

### 5.1 Provider view

When `user.role === 1`:

| Widget | Data |
|--------|------|
| **Total Revenue** | `GetProviderStats().total_revenue_credits` |
| **Pending Settlement** | `GetProviderStats().pending_revenue_credits` |
| **Settled** | `GetProviderStats().total_settlements` |
| **Recent Settlements** | `GetProviderStats().recent_settlements` → table |

### 5.2 User view

When `user.role !== 1`:

| Widget | Data |
|--------|------|
| **Credits / Wallet / Keys** | Existing stat cards |
| **Usage by Model** | `GET /api/v1/usage/stats-by-model` → table: model, calls, tokens, credits |
| **Recent Usage** | Existing recent list |

---

## 6. Batch 5: i18n Internationalization

### 6.1 Technology

- `react-i18next` + `i18next` + `i18next-browser-languagedetector`

### 6.2 File structure

```
app/web/src/i18n/
├── config.ts
└── locales/
    ├── en.json    (default fallback)
    ├── zh.json
    ├── ja.json
    ├── ko.json
    └── vi.json
```

### 6.3 Language file structure

```json
{
  "nav": {
    "home": "Home",
    "models": "Models",
    "providers": "Providers",
    "console": "Console"
  },
  "home": {
    "hero": {
      "title": "Build the Next Generation of AI Applications"
    }
  },
  "models": {
    "title": "Model Factory",
    "search": "Search by model name, code, or developer..."
  },
  "providers": {
    "title": "Providers",
    "search": "Search providers..."
  },
  "console": {
    "overview": "Overview",
    "credits": "Credits"
  },
  "common": {
    "loading": "Loading...",
    "no_results": "No results found",
    "search": "Search..."
  }
}
```

### 6.4 Components to update

| File | Scope |
|------|-------|
| `components/Navbar.tsx` | Navigation links |
| `components/Footer.tsx` | Footer text |
| `components/Sidebar.tsx` | Sidebar menu items |
| `components/LanguageSwitcher.tsx` | **New** — language dropdown |
| `routes/index.tsx` | Landing page |
| `routes/models.tsx` | Model list |
| `routes/models.$code.tsx` | Model detail |
| `routes/providers.tsx` | Provider list |
| `routes/providers.$name.tsx` | Provider detail |
| `routes/console.tsx` | All tabs |
| `routes/login.tsx` | Login form |
| `routes/register.tsx` | Register form |
| `routes/keys.tsx` | API keys page |
| `routes/transactions.tsx` | Transactions |
| `routes/usage.tsx` | Usage records |
| `routes/orders.tsx` | Payment orders |
| `routes/providers-apply.tsx` | Apply as provider |
| `routes/dashboard.tsx` | Dashboard |
| `routes/admin.tsx` | Admin panel (if applicable) |

---

## 7. Batch 6: Go Local Node

### 7.1 Design Decisions

| Decision | Conclusion |
|----------|------------|
| **Language** | Go |
| **HTTP framework** | `net/http` + `chi` |
| **Database** | SQLite (`modernc.org/sqlite`) |
| **Config** | YAML |
| **Deployment** | Independent binary, supplier-deployed |
| **Platform communication** | WebSocket tunnel (node connects actively — no fixed IP needed) |
| **Auth** | HMAC-SHA256 — platform generates `node_id` + `node_secret` |
| **Shared models** | Node fetches model list from platform, binds local keys → registers |
| **Private models** | Local keys not bound to platform models → node-only, not registered |
| **Key security** | Upload `HASH(local_key + channel + model_name)` to platform |

### 7.2 Project structure

```
cmd/node/
├── main.go                     # Entry point
├── config.yaml                 # Default config
├── internal/
│   ├── server/
│   │   └── server.go           # HTTP server (OpenAI-compatible /v1/* endpoints)
│   ├── tunnel/
│   │   ├── client.go           # WebSocket tunnel client (connect + auth + heartbeat)
│   │   └── handler.go          # WS message dispatch (request → process → response)
│   ├── router/
│   │   ├── router.go           # Model routing logic
│   │   ├── fallback.go         # Multi-account fallback chain
│   │   └── roundrobin.go       # Round-robin account selection
│   ├── provider/
│   │   ├── registry.go         # Provider executor registry
│   │   └── http.go             # Default HTTP executor (API-key based)
│   ├── keychain/
│   │   ├── store.go            # Local key storage + management
│   │   └── hash.go             # HASH(local_key + channel + model_name)
│   ├── db/
│   │   ├── sqlite.go           # SQLite init + migrations
│   │   ├── usage.go            # Usage record CRUD
│   │   └── config.go           # Config persistence
│   └── types/
│       └── types.go            # Shared types

pkg/translator/
├── translator.go               # Translator interface
├── registry.go                 # Registry for format translators
├── openai/
│   └── translator.go           # OpenAI ↔ OpenAI (passthrough)
├── anthropic/
│   ├── request.go              # OpenAI → Anthropic
│   └── response.go             # Anthropic → OpenAI
├── gemini/
│   ├── request.go
│   └── response.go
├── kiro/
│   ├── request.go
│   └── response.go
└── cursor/
    ├── request.go
    └── response.go
```

### 7.3 Communication protocol

#### Registration flow

```
Node                                Platform
  │                                     │
  │── WS connect ──────────────────────→│
  │── { type: "auth",                   │
  │     node_id,                        │
  │     signature: HMAC(node_id, secret) }──→│
  │← { type: "auth_ok", node_id } ──────│
  │                                     │
  │── { type: "register",               │
  │     models: [{ model_code,          │
  │                key_hash,            │
  │                cache_hit_price,     │
  │                cache_miss_price,    │
  │                output_price }]      │
  │   } ──────────────────────────────→│
  │← { type: "registered", ok: true } ──│
```

#### Request flow

```
Platform                            Node
  │                                     │
  │── { type: "request",                │
  │     request_id,                     │
  │     model,                          │
  │     messages,                       │
  │     stream: true,                   │
  │     key_hash } ───────────────────→│
  │                                     │
  │  (node processes via translator +   │
  │   executor, calls upstream API)     │
  │                                     │
  │← { type: "chunk",                   │
  │     request_id,                     │
  │     data: { choices: [...] } } ─────│  (repeated for stream)
  │                                     │
  │← { type: "done",                    │
  │     request_id,                     │
  │     usage: { ... } } ───────────────│
```

#### Heartbeat

```
Every 30 seconds:
Node → { type: "ping" }
Platform → { type: "pong" }
```

### 7.4 Provider Executor System

```go
// pkg/provider/executor.go
type Executor interface {
    Execute(ctx context.Context, req *Request) (*Response, error)
    ExecuteStream(ctx context.Context, req *Request) (<-chan *Chunk, error)
}

// DefaultExecutor: simple HTTP POST with API key
// OAuthExecutor: handles OAuth token refresh
// Custom executors for special providers (Claude, Gemini, Kiro...)
```

### 7.5 Keychain & Security

```go
// hash.go
func ComputeKeyHash(localKey, channelID, modelName string) string {
    data := localKey + "|" + channelID + "|" + modelName
    h := hmac.New(sha256.New, []byte(secretSalt))
    h.Write([]byte(data))
    return hex.EncodeToString(h.Sum(nil))
}
```

The platform never sees the raw API key. The hash is used to identify which key to use when routing requests back to the node.

---

## 8. Batch 7: Tauri Desktop Client

### 8.1 Architecture

```
┌─ Desktop App (Tauri) ──────────────────────────┐
│  ┌─ React UI (src/) ────────────────────────┐  │
│  │  Login page (platform account)            │  │
│  │  Node dashboard                           │  │
│  │    ├─ Node status (online/offline)        │  │
│  │    ├─ Key management                      │  │
│  │    ├─ Model binding                       │  │
│  │    └─ Usage stats / request log           │  │
│  │  Settings page                            │  │
│  └───────────────────────────────────────────┘  │
│  ┌─ Rust Backend (src-tauri/) ───────────────┐  │
│  │  Spawn/manage Go node subprocess          │  │
│  │  Local HTTP client → node admin API       │  │
│  │  Auto-update                             │  │
│  └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
                         │
                         ▼
┌─ Go Node (subprocess) ─────────────────────────┐
│  Local admin API (localhost:20129)              │
│  WS tunnel to platform                         │
│  Provider executors                            │
└─────────────────────────────────────────────────┘
```

### 8.2 Desktop UI pages

| Page | Description |
|------|-------------|
| **Login** | Login with platform account (email + password), fetch user's nodes |
| **Dashboard** | Show node status, view real-time request log, usage charts |
| **Keys** | Manage local API keys (add/remove/test) |
| **Models** | View platform models, bind local keys to models, set pricing |
| **Settings** | Node config (auto-start, WS endpoint, theme) |

### 8.3 Tauri commands (Rust → frontend)

```rust
#[tauri::command]
fn start_node(config: NodeConfig) -> Result<(), String>  // spawn Go binary

#[tauri::command]
fn stop_node() -> Result<(), String>                     // kill subprocess

#[tauri::command]
fn node_status() -> Result<NodeStatus, String>           // check subprocess health

#[tauri::command]
fn get_local_keys() -> Result<Vec<KeyInfo>, String>      // from node local API

#[tauri::command]
fn get_platform_models(token: String) -> Result<Vec<ModelInfo>, String>  // from platform API

#[tauri::command]
fn get_platform_nodes(token: String) -> Result<Vec<NodeInfo>, String>    // from platform API
```

### 8.4 Node local admin API

The Go node exposes a local HTTP API for the desktop client:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/node/status` | GET | Node status + uptime |
| `/api/node/keys` | GET/POST | List/add keys |
| `/api/node/keys/:id` | DELETE | Remove key |
| `/api/node/models` | GET | Bound models list |
| `/api/node/models/bind` | POST | Bind key to platform model |
| `/api/node/models/unbind` | POST | Unbind |
| `/api/node/usage` | GET | Usage stats |
| `/api/node/logs` | GET | Recent request log |
| `/api/node/restart` | POST | Restart tunnel connection |

---

## Appendix A: File Change Summary

### Backend (server/)

| File | Status |
|------|--------|
| `migrations/0017_developers_reputation.sql` | ✅ ready to write |
| `internal/controller/api/catalog/catalog.go` | ✅ ListProviders + GetProviderModels written |
| `internal/boot/boot.go` | Need to add 2 routes |
| `internal/model/dto/settlement.go` | ✅ already has pending fields |
| `internal/logic/settlement/settlement.go` | ✅ already has pending query |

### New code

| File | Status |
|------|--------|
| `pkg/translator/` | New package |
| `cmd/node/` | New binary |
| `app/web/src/routes/models.$code.tsx` | New page |
| `app/web/src/routes/providers.$name.tsx` | New page |
| `app/web/src/components/LanguageSwitcher.tsx` | New component |
| `app/web/src/i18n/` | New directory (config + 5 locales) |

### Frontend modifications

| File | Change |
|------|--------|
| `app/web/src/routes/models.tsx` | Remove drawer, add navigate |
| `app/web/src/routes/providers.tsx` | Rewrite: search + sort + cards |
| `app/web/src/routes/console.tsx` | Enhance OverviewTab |
| `app/web/src/components/Navbar.tsx` | Add LanguageSwitcher |
| `app/web/src/components/Footer.tsx` | i18n text |
| `app/web/src/components/Sidebar.tsx` | i18n text |
| All remaining route files | Replace text with `t()` |

### Desktop client

| File | Status |
|------|--------|
| `app/desktop/src/main.ts` | Extend |
| `app/desktop/src-tauri/src/main.rs` | Add Tauri commands |
| `app/desktop/package.json` | Add deps |
| `app/desktop/src/` | UI pages (login, dashboard, keys, models, settings) |
