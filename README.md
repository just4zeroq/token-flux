# Token Flux

AI 能力交易市场 —— 连接模型提供商与消费者的开放平台。

**核心价值：** API Key 在传统模式下各自绑定单一厂商。Token Flux 将 Key 抽象为统一虚拟 Key，消费者只需一个 Key 即可访问多家 LLM 服务；Provider 托管 API Key 或部署本地节点，通过市场机制按价格和信誉竞争流量，平台负责路由、计费、结算。

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  Web Frontend                        │
│            React SPA (Vite) :5173 dev               │
│            Tauri Desktop (packaged)                  │
└──────────────────────┬──────────────────────────────┘
                       │ HTTP
┌──────────────────────▼──────────────────────────────┐
│                 GoFrame Monolith                     │
│                                                      │
│  :8080  API Server  (JWT auth, user + provider)     │
│  :8081  Gateway     (API Key auth, OpenAI-compat)   │
│  :8082  Admin       (Bearer token, management)      │
│                                                      │
│  internal/                                           │
│  ├── controller/api/     (user endpoints)            │
│  ├── controller/api/admin/ (management endpoints)    │
│  ├── controller/gateway/ (LLM proxy)                 │
│  ├── logic/              (business logic)            │
│  ├── service/            (interface definitions)     │
│  └── model/dto/          (request/response DTOs)     │
│                                                      │
│  pkg/translator/         (LLM format translation)    │
│  cmd/node/               (local node binary)         │
└─────────────────────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────┐
│                  PostgreSQL 16                       │
│                  Redis (settlement)                  │
└─────────────────────────────────────────────────────┘
```

## Servers

| Server | Port | Auth | Purpose |
|--------|------|------|---------|
| API | :8080 | JWT (HS256) | User + provider endpoints |
| Gateway | :8081 | API Key (sk-xxx) | OpenAI-compatible LLM data-plane |
| Admin | :8082 | Bearer token | Backend management |

## Tech Stack

- **Backend**: Go (GoFrame v2), PostgreSQL 16, Redis
- **Frontend**: React 19, TanStack Router, TanStack Query, Zustand, Tailwind CSS v4
- **Desktop**: Tauri (Rust)
- **i18n**: 5 languages (EN / ZH / JA / KO / VI)
- **Container**: Docker, docker-compose

## Quick Start

```bash
# Start PostgreSQL + Redis
docker compose up -d postgres redis

# Start backend
cd server && go run .

# Start frontend
cd app/web && npm run dev
```

## Config

`server/manifest/config/config.yaml` — database, JWT, wallet, redis settings.

## Database Migrations

Goose SQL migrations in `server/migrations/`:

```
0001_identity.sql
0002_wallet.sql
0003_billing.sql
0004_llm.sql
0005_settlement.sql
0006_invoices.sql
0007_payment.sql
0008-0014  provider applications, settlement cycle, system configs, etc.
0015_developers.sql
0016_system_configs_add_name.sql
0017_developers_reputation.sql
```

## Project Structure

```
├── server/                  ← GoFrame monolith
│   ├── internal/            ← controllers, logic, service, dto
│   ├── migrations/          ← goose SQL migrations
│   ├── pkg/translator/      ← LLM format translators
│   ├── cmd/node/            ← local node binary
│   └── manifest/config/     ← GoFrame config YAML
├── app/
│   ├── web/                 ← React SPA (Vite)
│   └── desktop/             ← Tauri desktop app
├── docs/design/             ← design documents
├── docker-compose.yml       ← dev environment
└── CLAUDE.md                ← AI coding assistant context
```

## License

Private.
