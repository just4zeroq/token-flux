# Monolith Skeleton — Cmd Entries + Verification Plan (P1d)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the 4 `cmd/` entry-points and the `server/main.go` shim, then verify the full build compiles and both servers start with working `/health` endpoints.

**Architecture:** Each cmd binary blank-imports `internal/logic` (triggering all service registrations) and `github.com/gogf/gf/contrib/drivers/pgsql/v2` (registering the database driver with GoFrame). Only `cmd/api` does real work (`boot.RunAPI`). The other three are placeholders that call `g.Wait()`.

**Tech Stack:** Go 1.26, GoFrame v2.10.

---

## Scope

Pre-req: P1a + P1b + P1c completed — `server/internal/boot`, `server/internal/logic`, `server/internal/service`, and all middleware are in place and compiling.

In:
- `server/main.go` — thin shim calling `boot.RunAPI()` (allows `go run ./server`)
- `server/cmd/api/main.go` — production entry, calls `boot.RunAPI()`
- `server/cmd/watcher/main.go` — placeholder, logs + `g.Wait()`
- `server/cmd/recorder/main.go` — placeholder, logs + `g.Wait()`
- `server/cmd/settlement/main.go` — placeholder, logs + `g.Wait()`
- Verification: `go build ./...`, binary start, `curl :8080/health` + `curl :8081/health`

Out (later plans):
- Real watcher/recorder/settlement implementations
- Docker compose
- goose migration run against a live database

---

## Task 1: Create cmd entry-points + update main.go

**Files:**
- Create: `server/cmd/api/main.go`
- Create: `server/cmd/watcher/main.go`
- Create: `server/cmd/recorder/main.go`
- Create: `server/cmd/settlement/main.go`
- Modify: `server/main.go` (add pgsql driver import + comment)

- [ ] **Step 1: Write `server/cmd/api/main.go`**

```go
// cmd/api is the main HTTP entry-point. It starts the api ghttp.Server (:8080)
// for business REST API and the gateway ghttp.Server (:8081) for LLM/MCP/Agent
// runtime proxying. Both servers share the same process, same DB pool, same
// in-memory service registry.
package main

import (
	_ "ai-platform/internal/logic"

	"ai-platform/internal/boot"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
)

func main() { boot.RunAPI() }
```

- [ ] **Step 2: Write `server/cmd/watcher/main.go`**

```go
// cmd/watcher will host on-chain deposit observers for BNB / Base / TON.
// Placeholder: only blank-imports logic so service interfaces are registered;
// real wallet.Watcher.Start lands in P6 (wallet plan).
package main

import (
	_ "ai-platform/internal/logic"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
)

func main() {
	ctx := gctx.New()
	g.Log().Info(ctx, "ai-platform-watcher: placeholder, no chain workers started yet")
	g.Wait()
}
```

- [ ] **Step 3: Write `server/cmd/recorder/main.go`**

```go
// cmd/recorder will batch-aggregate usage_records (hourly/daily roll-ups, archival).
// Placeholder: real aggregator lands in P8.
package main

import (
	_ "ai-platform/internal/logic"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
)

func main() {
	ctx := gctx.New()
	g.Log().Info(ctx, "ai-platform-recorder: placeholder, no aggregation started yet")
	g.Wait()
}
```

- [ ] **Step 4: Write `server/cmd/settlement/main.go`**

```go
// cmd/settlement will run the periodic provider revenue settlement cron
// (move provider credits -> provider balance once per day).
// Placeholder: real cron lands in P7.
package main

import (
	_ "ai-platform/internal/logic"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
)

func main() {
	ctx := gctx.New()
	g.Log().Info(ctx, "ai-platform-settlement: placeholder, no cron started yet")
	g.Wait()
}
```

- [ ] **Step 5: Update `server/main.go` to add pgsql driver import**

The shim at `server/main.go` allows `go run ./server` for quickstarts. It must also import the pgsql driver, since driver registration belongs to the binary (not the library).

```go
// Local-quickstart shim: `go run ./server` boots the API server. The real
// production entry-point is cmd/api, which this file mirrors.
package main

import (
	_ "ai-platform/internal/logic"

	"ai-platform/internal/boot"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
)

func main() { boot.RunAPI() }
```

- [ ] **Step 6: Commit**

```bash
git add server/cmd/ server/main.go
git commit -m "feat(cmd): add 4 cmd entry-points + add pgsql driver import"
```

---

## Task 2: Verification — build, start, and probe

- [ ] **Step 1: Full build**

Run: `cd server && go build ./...`
Expected: no output, exit 0.

- [ ] **Step 2: Vet**

Run: `cd server && go vet ./...`
Expected: no output, exit 0.

- [ ] **Step 3: Build binary**

Run: `cd server && go build -o /tmp/ai-platform-api.exe ./cmd/api`
Expected: binary created at `/tmp/ai-platform-api.exe` (or platform temp dir), ~25-30 MB.

- [ ] **Step 4: Start binary in background**

Run: `/tmp/ai-platform-api.exe &`
Wait ~3 seconds.

Expected log output includes:
```
http server started listening on [:8080]
http server started listening on [:8081]
ai-platform started: api :8080 + gateway :8081
```

If servers do not bind: check that `boot.go` calls `SetAddr(":8080")` / `SetAddr(":8081")` and `Start()` on each server. Without these, GoFrame v2.10 creates Server objects but never opens listeners.

- [ ] **Step 5: Probe health endpoints**

Run: `curl -s http://localhost:8080/health`
Expected: `{"status":"ok"}`

Run: `curl -s http://localhost:8081/health`
Expected: `{"status":"ok"}`

- [ ] **Step 6: Stop the server**

Kill the background process.

- [ ] **Step 7: If boot.go needed a fix (SetAddr + explicit Start), commit that fix**

```bash
git add server/internal/boot/boot.go
git commit -m "fix(boot): use SetAddr + explicit Start for each ghttp.Server"
```

P1 complete. The monolith skeleton compiles, starts, and serves health probes on both ports. All 8 domains have service interfaces and empty logic stubs wired via init(). Next plan: P2 (identity migration).
