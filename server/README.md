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
