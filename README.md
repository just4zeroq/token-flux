# Token Flux

AI 能力交易市场 —— 连接模型提供商与消费者的开放平台。

**核心价值：** Token (API Key) 在传统模式下各自绑定单一厂商和计费体系。Token Flux 将 Key 抽象为统一虚拟 Key，消费者只需一个 Key 即可访问多家 LLM 服务；Provider 可以托管自己的 API Key 或部署自托管网关，通过市场机制按服务质量、价格和信誉竞争流量，平台负责路由、计费、结算和纠纷仲裁。让 AI 能力像商品一样自由流通。

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    Web Frontend                      │
│              React SPA (Vite) :80                    │
└──────────────────────┬──────────────────────────────┘
                       │ HTTP
┌──────────────────────▼──────────────────────────────┐
│                   api-gateway                        │
│                 HTTP :8080                           │
└──────┬──────────────────────┬───────────────────────┘
       │ gRPC                 │ gRPC
┌──────▼──────────┐   ┌──────▼───────────────────────┐
│    user-svc     │   │         asset-svc              │
│   gRPC :8100    │   │       gRPC :8101               │
│                 │   │                                │
│ - register      │   │ - balance                      │
│ - login         │   │ - transactions                 │
│ - API keys      │   │ - usage records                │
│ - token/        │   │ - orders                       │
│   key validate  │   │                                │
└─────────────────┘   └───────────────────────────────┘
                              ▲
                              │ gRPC
                    ┌────────┴────────┐
                    │   market-svc    │
                    │  gRPC :8102     │
                    │                 │
                    │ - listings      │
                    │ - trades        │
                    └─────────────────┘

┌─────────────────────────────────────────────────────┐
│                   ai-gateway                         │
│                 HTTP :8081                           │
│                                                      │
│ - LLM API proxy                                      │
│ - API key auth                                       │
│ - Usage reporting                                    │
└─────────────────────────────────────────────────────┘
```

## Services

| Service | Port | Protocol | Description |
|---------|------|----------|-------------|
| user-svc | 8100 | gRPC | User auth, API key management |
| asset-svc | 8101 | gRPC | Balance, transactions, usage records, orders |
| market-svc | 8102 | gRPC | Listings, trades |
| api-gateway | 8080 | HTTP | REST API gateway → gRPC services |
| ai-gateway | 8081 | HTTP | LLM proxy with API key auth and usage tracking |
| web | 80 | HTTP | React SPA frontend |

## Tech Stack

- **Backend**: Go (GoFrame v2.7.1), gRPC, Protocol Buffers
- **Frontend**: React 19, TanStack Router, TanStack Query, Zustand, Tailwind CSS v4
- **Desktop**: Tauri (Rust)
- **Database**: PostgreSQL 16
- **Container**: Docker, docker-compose

## Quick Start

### One-click start (all services)

```bash
cd server/docker && bash start.sh
```

### Per-service (independent deployment)

```bash
cd server/user-svc    && docker compose up -d    # postgres + asset-svc + user-svc
cd server/asset-svc   && docker compose up -d    # postgres + asset-svc
cd server/market-svc  && docker compose up -d    # postgres + market-svc
cd server/api-gateway && docker compose up -d    # postgres + user-svc + asset-svc + api-gateway
cd server/ai-gateway  && docker compose up -d    # postgres + user-svc + asset-svc + ai-gateway
```

### Local development

```bash
# Start PostgreSQL
cd server/docker && docker compose up -d postgres

# Run migrations
cd server/docker && bash migrate.sh

# Start a Go service
cd server/user-svc && go run .

# Start web frontend
cd app/web && npm install && npm run dev
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | `localhost` | Database server hostname |
| `USER_SVC_ADDR` | `localhost:8100` | user-svc gRPC address |
| `ASSET_SVC_ADDR` | `localhost:8101` | asset-svc gRPC address |

Set in docker-compose `environment:` — entrypoint.sh substitutes at container startup.

## Database Migrations

Uses [Goose](https://github.com/pressly/goose) for SQL migrations:

```bash
# One-click all migrations
cd server/docker && bash migrate.sh

# Per service
goose -dir server/user-svc/migrations postgres \
  "postgres://aiplatform:aiplatform@localhost:5432/user_svc?sslmode=disable" up
```

## Project Structure

```
ai-platform/
├── server/
│   ├── api/              ← Shared proto module (.pb.go)
│   ├── user-svc/         ← gRPC :8100
│   ├── asset-svc/        ← gRPC :8101
│   ├── market-svc/       ← gRPC :8102
│   ├── api-gateway/      ← HTTP :8080
│   ├── ai-gateway/       ← HTTP :8081
│   ├── proto/            ← Proto source definitions
│   ├── scripts/          ← Database init scripts
│   └── docker/           ← Dockerfiles, compose, entrypoint, migrations
├── app/
│   ├── web/              ← React SPA (Vite)
│   └── desktop/          ← Tauri desktop app
└── CLAUDE.md
```
