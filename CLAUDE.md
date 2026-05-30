# AI Platform — CLAUDE.md

## Project Overview

AI capability platform (LLM → MCP → Agent). GoFrame v2 monolith serving three HTTP ports. Current phase: LLM API key hosting with double-entry settlement.

## Architecture

```
ai-platform/
├── server/                    ← GoFrame monolith
│   ├── main.go
│   ├── internal/
│   │   ├── boot/boot.go       ← starts 3 ghttp.Server instances
│   │   ├── controller/
│   │   │   ├── api/           ← :8080 handlers (identity, billing, wallet, payment, provider LLM)
│   │   │   │   ├── admin/     ← :8082 handlers (user, billing, settlement, invoice, llm, payment)
│   │   │   │   └── payment/   ← user-facing payment (recharge, orders, notify callbacks)
│   │   │   └── gateway/       ← :8081 handlers (OpenAI-compatible data-plane)
│   │   ├── logic/             ← business logic (one package per domain)
│   │   ├── service/           ← interface definitions + accessors
│   │   ├── model/dto/         ← request/response DTOs (8 files: billing, identity, invoice, llm, payment, settlement, wallet)
│   │   └── middleware/        ← JWTAuth, APIKeyAuth, AdminTokenAuth, CORS, Recover, RequestID
│   ├── migrations/            ← goose SQL migrations (7 files)
│   ├── manifest/config/       ← GoFrame config YAML
│   └── scripts/               ← smoke tests
├── app/
│   ├── web/                   ← React SPA (Vite)
│   └── desktop/               ← Tauri desktop app
└── docs/design/               ← design documents
```

## Three Servers

| Server | Port | Auth | Purpose |
|--------|------|------|---------|
| api | :8080 | JWT (HS256) | User + provider endpoints |
| gateway | :8081 | API Key (sk-xxx) | OpenAI-compatible LLM data-plane |
| admin | :8082 | Bearer token (env `ADMIN_API_TOKEN`) | Backend management, consumed by gfast UI |

### :8080 — API Server (JWT)

Public: `/api/v1/auth/register`, `/api/v1/auth/verify-email`, `/api/v1/auth/login`
Public callbacks: `/api/v1/payment/notify/alipay`, `/api/v1/payment/notify/wechat`
JWT-protected: `/users/me/*`, `/billing/*`, `/wallet/*`, `/payment/recharge`, `/payment/orders`, `/provider/llm/*`

Provider routes check `role == dto.RoleProvider` (1) per handler.

### :8081 — Gateway Server (API Key)

`/v1/models`, `/v1/chat/completions`, `/v1/completions`, `/v1/embeddings`

APIKeyAuth middleware validates `sk-xxx` keys, injects `user_id` + `api_key_id` into ctx.

### :8082 — Admin Server (Static Token)

AdminTokenAuth checks `Authorization: Bearer <ADMIN_API_TOKEN>`. No JWT, no session.

**Admin endpoints:**
| Category | Routes |
|----------|--------|
| Users | `GET/POST /users`, `GET /users/:id`, `PUT /users/:id/status`, `PUT /users/:id/role`, `DELETE /users/:id` |
| Billing | `GET /accounts`, `GET /balances`, `GET /transactions`, `POST /recharge`, `POST /adjust-balance` |
| Settlement | `GET /settlements` |
| Invoice | `GET/POST /invoices`, `GET /invoices/:id`, `PUT /invoices/:id/status` |
| LLM | `GET/POST /llm/channels`, `PUT /llm/channels/:id/review`, `DELETE /llm/channels/:id`, `GET/POST /llm/models`, `PUT /llm/models/:id/review`, `DELETE /llm/models/:id`, `GET/POST /llm/models/:id/prices`, `GET /llm/model-keys`, `GET /llm/key-models`, `POST /llm/key-models/:id/test` |
| Payment | `GET/POST /payment/channels`, `PUT /payment/channels/:id`, `GET /payment/orders` |

**gfast integration:** The admin frontend at `../gfast/` calls these endpoints. Admin response format must match gfast conventions: `{"code": 0, "message": "ok", "data": {...}}` on success, `{"code": -1, "message": "error"}` on failure. Pagination uses `pageNum`/`pageSize` query params, response includes `currentPage`/`total`.

## Domain Modules

| Domain | Logic Package | Service Interface | Tables |
|--------|--------------|-------------------|--------|
| identity | logic/identity/ | IIdentity | users, api_keys, email_verifications |
| billing | logic/billing/ | IBilling | accounts, transactions, transaction_entries |
| settlement | logic/settlement/ | ISettlement | settlement_records |
| llm | logic/llm/ | ILLM | llm_channels, llm_model_specs, llm_model_prices, llm_model_keys, llm_model_key_models, llm_usage_records |
| wallet | logic/wallet/ | IWallet | deposit_addresses, chain_deposits, withdraw_requests |
| gateway | logic/gateway/ | IGatewayAgent, IGatewayMcp | (no own tables) |
| invoice | logic/invoice/ | IInvoice | invoices |
| payment | logic/payment/ | IPayment | payment_channels, payment_orders |

## Service Registration Pattern

Each logic package registers itself via `init()`:

```go
// internal/logic/llm/llm.go
type sLLM struct{}
func init() { service.RegisterLLM(New()) }
func New() *sLLM { return &sLLM{} }
```

Service interfaces in `internal/service/` with package-level accessor:

```go
// internal/service/llm.go
type ILLM interface { ... }
var localLLM ILLM
func RegisterLLM(i ILLM) { localLLM = i }
func LLM() ILLM { return localLLM }  // panics if not registered
```

All logic packages are blank-imported in `logic/logic.go` so `init()` runs on startup.

## Database

PostgreSQL 16. Single database. Migrations managed by [Goose](https://github.com/pressly/goose).

### Migrations (7 files)

| File | Tables |
|------|--------|
| 0001_identity.sql | users, api_keys, email_verifications |
| 0002_wallet.sql | deposit_addresses, chain_deposits, withdraw_requests |
| 0003_billing.sql | accounts, transactions, transaction_entries |
| 0004_llm.sql | llm_channels, llm_model_specs, llm_model_prices, llm_model_keys, llm_model_key_models, llm_usage_records |
| 0005_settlement.sql | settlement_records |
| 0006_invoices.sql | invoices |
| 0007_payment.sql | payment_channels, payment_orders |

### Key Design Decisions

- **No FK constraints** — all reference columns are plain BIGINT, defaults to 0
- **Soft delete** — api_keys uses `deleted_at` (GoFrame auto-filters)
- **Role in users table** — `role` INT, `0` = user, `1` = provider (constants in `dto/identity.go`)
- **No admin role** — admin is a separate server with static token, not a user role
- **Optimistic locking** — `accounts.version` column, no retry loop (conflicts propagate to caller)
- **Idempotent settlement** — `settlement_records` UNIQUE(ref_type, ref_id); INSERT first, SELECT on conflict
- **AES-GCM key encryption** — `llm_model_keys.key_encrypted`, env var `MODEL_KEY_ENCRYPTION_KEY` (hex-encoded 32-byte)

### Account Assets

| Asset | Meaning | Accounting |
|-------|---------|------------|
| credits | Paid credit balance | Double-entry, balanced per tx |
| points | Loyalty/reward points | One-sided grant, not balanced |
| balance | Deposit balance (wallet) | Double-entry |

## LLM Management Chain

```
provider (user, role=1)
  → llm_channels        (protocol config, e.g. openai-compatible base_url)
    → llm_model_keys      (encrypted upstream API key)
      → llm_model_key_models (key ↔ model_spec binding, upstream_model_name, provider_share_bps)
```

```
admin (via :8082)
  → llm_channels        (create + review → active)
    → llm_model_specs     (create + review → active)
      → llm_model_prices   (per-capability pricing)
```

## LLM Settlement Flow

```
User request → /v1/chat/completions (sk-xxx key)
  → APIKeyAuth: resolve user_id + api_key_id
  → pickRandomKeyModel: join key_models + keys + channels, filter active, random select
  → decryptKey: AES-GCM decrypt upstream key
  → extractBaseURL: parse protocols_json
  → proxyOpenAICompatible: call upstream OpenAI API
  → on success:
    → INSERT llm_usage_records (tokens, cost, revenue, commission)
    → SubmitAndSettle: settlement_records INSERT (idempotent)
      → postLedger: accounts UPDATE (optimistic lock) + transactions + transaction_entries
    → Quota tracking: UPDATE llm_model_keys.quota_used_credits, llm_model_key_models.quota_used_credits
    → Overdraft check: if balance < 0, UPDATE api_keys SET status=0, disabled_reason='overdraft'
  → on failure:
    → INSERT llm_usage_records (status='failed', error_code, error_message)
    → No settlement attempted
  → on settlement failure:
    → Mark usage record status='settlement_pending' for recovery
    → Still bump quotas (provider did the work)
    → Skip overdraft check (balance wasn't actually changed)
```

## Payment Flow

```
User request → POST /api/v1/payment/recharge (JWT, sk-xxx key)
  → CreateRecharge: INSERT payment_orders (status=pending)
  → gopay API call (Alipay TradePagePay / WeChat V3TransactionNative)
  → Return pay_url/qrcode to user
  → User pays on Alipay/WeChat page
  → Gateway callback → POST /api/v1/payment/notify/{alipay,wechat} (public)
  → HandleNotify: parse + verify sign + decrypt (wechat)
  → confirmOrder (idempotent — skips if status != "pending"):
    → UPDATE payment_orders SET status=paid, trade_no, paid_at
    → service.Billing().RechargeCredits() — credits the user account
    → UPDATE payment_orders SET status=credited, credited_at
```

**Pricing:** 1 CNY = 10 credits (MVP).
**Idempotency:** confirmOrder checks `status != "pending"` → returns nil.
**gopay SDK:** github.com/go-pay/gopay v1.5.118 — Alipay V1 + WeChat Pay V3.

## Code Conventions

- **Framework**: GoFrame v2 (`github.com/gogf/gf/v2`)
- **DB access**: `g.DB().Model("table").Ctx(ctx)` — no generated DAO layer
- **Transactions**: `g.DB().Transaction(ctx, func(ctx, tx gdb.TX) error {...})`
- **Errors**: `gerror.Wrap(err, "context")` always
- **Controller**: Thin — parse request, delegate to service, write response
- **DTOs**: `internal/model/dto/` — one file per domain, GoFrame validation tags (`v:"required|min:1"`)
- **No g.Map for DB** — use `g.Map` for data maps (no DO/entity layer; project convention differs from gfast)
- **Response format for api gateway**: raw JSON objects (not `{code, message, data}` wrapped)
- **Response format for admin :8082**: gfast-compatible `{code, message, data}` wrapper

## Env Vars

| Variable | Used By | Purpose |
|----------|---------|---------|
| `ADMIN_API_TOKEN` | middleware.AdminTokenAuth | Static token for :8082 admin access |
| `MODEL_KEY_ENCRYPTION_KEY` | logic/llm/key.go | AES-GCM 32-byte hex key for encrypting upstream API keys |
| `JWT_SECRET` | logic/identity/jwt.go | HS256 signing key |

## Related Projects

- **gfast** (`../gfast/`) — GoFrame v3 admin framework. Frontend (Vue3) for :8082 admin server. Uses `{code, message, data}` response format, `pageNum`/`pageSize` pagination, `gftoken` auth with Redis. Port :8808.
- **gfast-ui** — Vue3 admin SPA (separate repo, github.com/tiger1103/gfast-ui)

## Key Commands

```bash
cd server && go build ./...          # build check
cd server && go run .                # start all 3 servers
cd server && goose up                # run migrations (7 files)
bash server/scripts/smoke-llm.sh    # LLM end-to-end smoke test
```
