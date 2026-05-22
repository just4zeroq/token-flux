# Monolith Skeleton — Service/Logic Wiring Plan (P1c)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the GoFrame `service` (interface) + `logic` (implementation registered via `init()`) pattern across all 8 domains, plus add shared middleware and the two-server `boot` package. After this plan the API compiles end-to-end but contains no business logic — every domain implementation is an empty stub.

**Architecture:**
- `internal/service/<domain>.go` defines `I<Domain>` interface plus `Register<Domain>(i)` and `<Domain>()` accessor. Accessor panics if implementation wasn't registered, surfacing missing imports loudly.
- `internal/logic/<domain>/<domain>.go` implements the interface with an empty `s<Domain>{}` struct. `init()` calls `service.Register<Domain>(New())`.
- `internal/logic/logic.go` is the top-level aggregator: blank-imports every domain subpackage so a single `_ "ai-platform/internal/logic"` from a `main.go` triggers every registration.
- `internal/logic/gateway/gateway.go` is the sub-aggregator for the three gateway runtime packages (llm/mcp/agent).
- `internal/middleware/common.go` provides Recover / RequestID / CORS — used by both servers.
- `internal/boot/boot.go::RunAPI` creates `g.Server("api")` and `g.Server("gateway")`, explicitly calls `SetAddr` then `Start` on each, then `g.Wait` blocks. (GoFrame v2.10 requires explicit SetAddr — see gotcha note in Task 1.)

**Tech Stack:** Go 1.26, GoFrame v2.10 (`github.com/gogf/gf/v2`, `net/ghttp`, `frame/g`, `os/gctx`, `util/guid`).

---

## Scope

Pre-req: P1a + P1b completed — `server/internal/` directory tree exists with empty packages, `server/go.mod` has GoFrame dependencies.

In:
- `internal/middleware/common.go` — Recover, RequestID, CORS handlers
- `internal/boot/boot.go` — `RunAPI()` that starts both servers with a `/health` probe
- `internal/model/dto/dto.go` — empty placeholder so import path is valid
- `internal/service/service.go` — package doc comment
- 10 × `internal/service/<domain>.go` — empty `I<Domain>` interface + `Register<Domain>` + `<Domain>()` accessor
- `internal/logic/logic.go` — top aggregator with 8 blank imports
- `internal/logic/gateway/gateway.go` — sub-aggregator with 3 blank imports
- 8 × `internal/logic/<domain>/<domain>.go` — `s<Domain>{}` stub + `init()` registration + `New()` constructor

Out (later plans):
- Real method signatures on interfaces (added per-domain in P2-P7)
- Real method implementations (per-domain)
- DAO code generation (`gf gen dao`)
- HTTP route registration on the servers (currently only `/health`)
- cmd entry-points (covered by P1d)

---

## Task 1: Middleware + boot wiring

**Files:**
- Create: `server/internal/middleware/common.go`
- Create: `server/internal/boot/boot.go`

- [ ] **Step 1: Write `internal/middleware/common.go`**

```go
package middleware

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
)

const RequestIDHeader = "X-Request-Id"

// CORS allows any origin for now; tighten in production.
func CORS(r *ghttp.Request) {
	corsOpts := r.Response.DefaultCORSOptions()
	r.Response.CORS(corsOpts)
	r.Middleware.Next()
}

// RequestID injects a request id into the response header and gctx for log correlation.
func RequestID(r *ghttp.Request) {
	id := r.Header.Get(RequestIDHeader)
	if id == "" {
		id = guid.S()
	}
	r.SetCtxVar("request_id", id)
	r.Response.Header().Set(RequestIDHeader, id)
	r.Middleware.Next()
}

// Recover converts panics into 500 responses without crashing the server.
func Recover(r *ghttp.Request) {
	defer func() {
		if err := recover(); err != nil {
			g.Log().Errorf(r.Context(), "panic recovered: %v", err)
			r.Response.ClearBuffer()
			r.Response.WriteStatus(500, g.Map{
				"code":    500,
				"message": "internal server error",
			})
		}
	}()
	r.Middleware.Next()
}
```

- [ ] **Step 2: Write `internal/boot/boot.go`**

GoFrame v2.10 gotcha: `g.Server(name)` alone does not bind to an address — you must call `SetAddr` before `Start`. Without `SetAddr` both servers stay unstarted and `g.Wait` blocks forever with no error.

```go
// Package boot wires together the HTTP servers and starts the application.
package boot

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"

	"ai-platform/internal/middleware"
)

// RunAPI starts both the api (:8080) and gateway (:8081) ghttp.Servers and blocks.
// All domain logic is registered via service interfaces during package init();
// this function only wires routes and starts servers.
func RunAPI() {
	ctx := gctx.New()

	apiSrv := g.Server("api")
	apiSrv.SetAddr(":8080")
	apiSrv.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.Recover, middleware.RequestID, middleware.CORS)
		group.GET("/health", health)
	})

	gwSrv := g.Server("gateway")
	gwSrv.SetAddr(":8081")
	gwSrv.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.Recover, middleware.RequestID, middleware.CORS)
		group.GET("/health", health)
	})

	if err := apiSrv.Start(); err != nil {
		g.Log().Fatalf(ctx, "api server start failed: %v", err)
	}
	if err := gwSrv.Start(); err != nil {
		g.Log().Fatalf(ctx, "gateway server start failed: %v", err)
	}

	g.Log().Info(ctx, "ai-platform started: api :8080 + gateway :8081")
	g.Wait()
}

func health(r *ghttp.Request) {
	r.Response.WriteJson(g.Map{"status": "ok"})
}
```

- [ ] **Step 3: Verify compile**

Run: `cd server && go build ./internal/middleware/ ./internal/boot/`
Expected: no output, exit 0.

- [ ] **Step 4: Commit**

```bash
git add server/internal/middleware/common.go server/internal/boot/boot.go
git commit -m "feat(server): add common middleware and boot package"
```

---

## Task 2: Service interfaces (10 files)

**Files:**
- Create: `server/internal/model/dto/dto.go`
- Create: `server/internal/service/service.go`
- Create: `server/internal/service/identity.go`
- Create: `server/internal/service/catalog.go`
- Create: `server/internal/service/market.go`
- Create: `server/internal/service/wallet.go`
- Create: `server/internal/service/billing.go`
- Create: `server/internal/service/pricing.go`
- Create: `server/internal/service/usage.go`
- Create: `server/internal/service/gateway_llm.go`
- Create: `server/internal/service/gateway_mcp.go`
- Create: `server/internal/service/gateway_agent.go`

Pattern (repeated 10 times with the relevant domain name): empty interface, package-level variable, `RegisterXxx` setter, `Xxx()` getter that panics on missing registration.

- [ ] **Step 1: Write `internal/model/dto/dto.go`**

```go
// Package dto holds cross-module data transfer types shared across service
// interfaces. Concrete type definitions are added per-domain in later plans;
// this placeholder exists so the import path is valid from the skeleton stage.
package dto
```

- [ ] **Step 2: Write `internal/service/service.go`**

```go
// Package service defines the cross-module domain interfaces. Each domain
// (identity, catalog, market, wallet, billing, pricing, usage, and the three
// gateway runtimes) exposes a single interface plus a package-level accessor.
//
// Domain implementations live under internal/logic/<domain>/ and register
// themselves via init() calls to RegisterXxx. This keeps logic packages
// referenced only by their concrete interface, eliminating import cycles
// between domains.
//
// Usage:
//
//	// In a controller or another logic package:
//	user, err := service.Identity().GetUser(ctx, userID)
package service
```

- [ ] **Step 3: Write `internal/service/identity.go`**

```go
package service

// IIdentity is the contract for the identity domain (users, JWT, API keys).
// Methods will be filled in during the identity migration plan; this skeleton
// only establishes the registration mechanism.
type IIdentity interface{}

var localIdentity IIdentity

// RegisterIdentity installs the identity implementation. Called once during
// init() from internal/logic/identity.
func RegisterIdentity(i IIdentity) { localIdentity = i }

// Identity returns the registered identity service. Panics if no implementation
// has been registered, which indicates a missing blank import of internal/logic.
func Identity() IIdentity {
	if localIdentity == nil {
		panic("service.Identity not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localIdentity
}
```

- [ ] **Step 4: Write the other 9 service files**

Each follows the same pattern as `identity.go`. The full set:

| File | Type | Register | Accessor |
|---|---|---|---|
| `catalog.go` | `ICatalog` | `RegisterCatalog` | `Catalog()` |
| `market.go` | `IMarket` | `RegisterMarket` | `Market()` |
| `wallet.go` | `IWallet` | `RegisterWallet` | `Wallet()` |
| `billing.go` | `IBilling` | `RegisterBilling` | `Billing()` |
| `pricing.go` | `IPricing` | `RegisterPricing` | `Pricing()` |
| `usage.go` | `IUsage` | `RegisterUsage` | `Usage()` |
| `gateway_llm.go` | `ILLMRuntime` | `RegisterLLMRuntime` | `LLMRuntime()` |
| `gateway_mcp.go` | `IMCPRuntime` | `RegisterMCPRuntime` | `MCPRuntime()` |
| `gateway_agent.go` | `IAgentRuntime` | `RegisterAgentRuntime` | `AgentRuntime()` |

Template for each (substituting Xxx and IXxx accordingly):

```go
package service

type IXxx interface{}

var localXxx IXxx

func RegisterXxx(i IXxx) { localXxx = i }

func Xxx() IXxx {
	if localXxx == nil {
		panic("service.Xxx not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localXxx
}
```

- [ ] **Step 5: Verify compile**

Run: `cd server && go build ./internal/service/ ./internal/model/dto/`
Expected: no output, exit 0.

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/ server/internal/model/dto/
git commit -m "feat(service): add 10 domain interfaces with register/accessor"
```

---

## Task 3: Logic stubs + registration aggregators

**Files:**
- Create: `server/internal/logic/logic.go`
- Create: `server/internal/logic/identity/identity.go`
- Create: `server/internal/logic/catalog/catalog.go`
- Create: `server/internal/logic/market/market.go`
- Create: `server/internal/logic/wallet/wallet.go`
- Create: `server/internal/logic/billing/billing.go`
- Create: `server/internal/logic/pricing/pricing.go`
- Create: `server/internal/logic/usage/usage.go`
- Create: `server/internal/logic/gateway/gateway.go`
- Create: `server/internal/logic/gateway/llm/llm.go`
- Create: `server/internal/logic/gateway/mcp/mcp.go`
- Create: `server/internal/logic/gateway/agent/agent.go`

- [ ] **Step 1: Write `internal/logic/logic.go` (top aggregator)**

```go
// Package logic is the aggregator: blank-importing this package triggers init()
// in every domain logic subpackage, which in turn registers each domain's
// concrete implementation with the service interface layer.
//
// Always import as:  _ "ai-platform/internal/logic"
package logic

import (
	_ "ai-platform/internal/logic/billing"
	_ "ai-platform/internal/logic/catalog"
	_ "ai-platform/internal/logic/gateway"
	_ "ai-platform/internal/logic/identity"
	_ "ai-platform/internal/logic/market"
	_ "ai-platform/internal/logic/pricing"
	_ "ai-platform/internal/logic/usage"
	_ "ai-platform/internal/logic/wallet"
)
```

- [ ] **Step 2: Write `internal/logic/gateway/gateway.go` (sub-aggregator)**

```go
// Package gateway aggregates the three runtime subpackages (llm, mcp, agent).
// Blank-importing this package triggers init() in each subpackage which in
// turn registers itself with the service layer.
package gateway

import (
	_ "ai-platform/internal/logic/gateway/agent"
	_ "ai-platform/internal/logic/gateway/llm"
	_ "ai-platform/internal/logic/gateway/mcp"
)
```

- [ ] **Step 3: Write each domain stub**

Template (substituting `<pkg>`, `s<Domain>`, `Register<Domain>`):

```go
package <pkg>

import "ai-platform/internal/service"

type s<Domain> struct{}

func init() { service.Register<Domain>(New()) }

// New returns the <domain> implementation. Empty in the skeleton; methods
// are added during the <domain> migration plan.
func New() *s<Domain> { return &s<Domain>{} }
```

Concrete files (each ~10 lines):

| File | Package | Struct | Register call |
|---|---|---|---|
| `internal/logic/identity/identity.go` | `identity` | `sIdentity` | `RegisterIdentity` |
| `internal/logic/catalog/catalog.go` | `catalog` | `sCatalog` | `RegisterCatalog` |
| `internal/logic/market/market.go` | `market` | `sMarket` | `RegisterMarket` |
| `internal/logic/wallet/wallet.go` | `wallet` | `sWallet` | `RegisterWallet` |
| `internal/logic/billing/billing.go` | `billing` | `sBilling` | `RegisterBilling` |
| `internal/logic/pricing/pricing.go` | `pricing` | `sPricing` | `RegisterPricing` |
| `internal/logic/usage/usage.go` | `usage` | `sUsage` | `RegisterUsage` |
| `internal/logic/gateway/llm/llm.go` | `llm` | `sLLM` | `RegisterLLMRuntime` |
| `internal/logic/gateway/mcp/mcp.go` | `mcp` | `sMCP` | `RegisterMCPRuntime` |
| `internal/logic/gateway/agent/agent.go` | `agent` | `sAgent` | `RegisterAgentRuntime` |

Example for `internal/logic/identity/identity.go`:

```go
package identity

import "ai-platform/internal/service"

type sIdentity struct{}

func init() { service.RegisterIdentity(New()) }

// New returns the identity domain implementation. Empty in the skeleton;
// methods are added during the identity migration plan.
func New() *sIdentity { return &sIdentity{} }
```

The other 9 follow the same template — only package name, struct name, and Register call change.

- [ ] **Step 4: Verify compile**

Run: `cd server && go build ./internal/logic/...`
Expected: no output, exit 0.

- [ ] **Step 5: Commit**

```bash
git add server/internal/logic/
git commit -m "feat(logic): add 8 domain stubs with init-time service registration"
```
