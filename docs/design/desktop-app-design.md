# Token Flux Node — Desktop App Design

> Based on 9Router feature analysis
> Tech: Go (Wails) + React (TanStack Router + Zustand + i18next)

---

## 1. Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    Wails Desktop App                       │
│                                                           │
│  ┌─ Go Backend (main.go) ──────────────────────────┐     │
│  │  NodeService (Wails Bind)                        │     │
│  │  ├── Start/Stop/Status node                      │     │
│  │  ├── Key management (AddKey, ListKeys, DeleteKey) │     │
│  │  ├── Model binding (BindModel, UnbindModel)      │     │
│  │  ├── Combo management (CreateCombo, ListCombos)  │     │
│  │  ├── RTK/Caveman config                          │     │
│  │  ├── Format translation (via pkg/translator)     │     │
│  │  └── Provider HTTP calls (via pkg/provider)      │     │
│  └─────────────────────────────────────────────────┘     │
│                                                           │
│  ┌─ React Frontend ────────────────────────────────┐     │
│  │  TanStack Router (tabs)                          │     │
│  │  ├── /dashboard   — Node status, key, toggles   │     │
│  │  ├── /providers   — Provider connections        │     │
│  │  ├── /combos      — Combo model lists           │     │
│  │  ├── /usage       — Usage stats + charts        │     │
│  │  ├── /keys        — API key management          │     │
│  │  ├── /proxy       — Proxy pool config           │     │
│  │  ├── /logs        — Live request log            │     │
│  │  └── /settings    — Config + i18n               │     │
│  │                                                   │     │
│  │  Zustand stores                                    │     │
│  │  ├── useNodeStore    — Node status                │     │
│  │  ├── useKeyStore     — API keys                   │     │
│  │  ├── useModelStore   — Model bindings + combos    │     │
│  │  ├── useUsageStore   — Usage stats                │     │
│  │  └── useSettingsStore — RTK/Caveman/config        │     │
│  └─────────────────────────────────────────────────┘     │
│                                                           │
└──────────────────────────────────────────────────────────┘
         │                        │
         ▼                        ▼
  pkg/translator            Local node :20129
  pkg/provider               (Go subprocess)
```

---

## 2. Pages & Routes

| Route | 9Router Equivalent | Purpose | Zustand Store | Wails Bind |
|-------|-------------------|---------|---------------|------------|
| `/dashboard` | Endpoint Page | Node status, API key display, toggles | useNodeStore | Status, Start/Stop |
| `/providers` | Providers | Provider grid, per-provider cards | useProviderStore | ListProviders |
| `/providers/:id` | Provider Detail | Per-provider connection detail | useProviderStore | ListKeys |
| `/combos` | Combos | Create/edit model combo lists | useComboStore | CreateCombo/ListCombos |
| `/usage` | Usage | Stats cards + charts + logs | useUsageStore | GetUsageStats |
| `/keys` | API Keys | CRUD API keys | useKeyStore | AddKey/ListKeys/DeleteKey |
| `/proxy` | Proxy Pools | Proxy pool management | useProxyStore | CreateProxy/ListProxies |
| `/logs` | Console Log | Live request log tail | useLogStore | GetLogs/StreamLogs |
| `/settings` | Profile/Settings | RTK/Caveman/language | useSettingsStore | SetConfig/GetConfig |
| `/chat` | Basic Chat | Simple chat playground | — | SendRequest |

---

## 3. Core Features — 9Router Feature Map

### 3.1 Dashboard (Endpoint Page)

```
┌──────────────────────────────────────────────────────────┐
│ Token Flux Node — Dashboard                               │
│                                                           │
│ Status: ● Running     Port: 20128     Uptime: 3h         │
│                                                           │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐ │
│ │ API Keys  │ │ Providers │ │ Models   │ │ Requests     │ │
│ │ 3 active  │ │ 1 online │ │ 5 bound  │ │ 1,234 today│ │
│ └──────────┘ └──────────┘ └──────────┘ └──────────────┘ │
│                                                           │
│ ┌────────────────────────────────────────────┐            │
│ │ API Key: sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx │           │
│ │ [Copy] [Create New]                        │            │
│ └────────────────────────────────────────────┘            │
│                                                           │
│ Toggles: [RTK Token Saver ●] [Caveman Mode ○] [Tunnel]  │
│                                                           │
│ Quick Links: [Chat Playground] [API Status] [Logs]       │
└──────────────────────────────────────────────────────────┘
```

### 3.2 Provider Management

**Data Model in SQLite:**

```sql
CREATE TABLE providers (
    id          TEXT PRIMARY KEY,
    label       TEXT NOT NULL,
    type        TEXT NOT NULL,         -- 'api_key' | 'oauth'
    api_key     TEXT NOT NULL DEFAULT '',
    base_url    TEXT NOT NULL DEFAULT '',
    models      TEXT NOT NULL DEFAULT '[]',  -- JSON array
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  INTEGER NOT NULL DEFAULT 0
);
```

**Auth Types:**

| Auth Type | Implementation |
|-----------|---------------|
| API Key | Simple text field (pkg/provider.DefaultHTTPProvider.Call) |
| OAuth | Browser popup → exchange → store refresh token |
| Custom Compatible | User sets base_url + api_key |

### 3.3 Combo Model Lists

```
┌─ Create Combo ──────────────────────────────────────┐
│ Name: "fast-chat"                                    │
│                                                      │
│ Rank │ Model                        │ Provider       │
│ ─────────────────────────────────────────────────── │
│ 1    │ deepseek-v4-flash            │ DeepSeek  [×] │
│ 2    │ gpt-4o-mini                  │ OpenAI    [×] │
│ 3    │ claude-haiku-3.5            │ Anthropic [×] │
│      │                              │                │
│      │              [↕ Drag to reorder]              │
│                                                      │
│ Strategy: [fallback] [round-robin (sticky: 3)]      │
│                                                      │
│ [Save] [Test All]                                     │
└──────────────────────────────────────────────────────┘
```

**Backend (Go):**

```go
type ComboConfig struct {
    Name     string   `json:"name"`
    Models   []string `json:"models"`  // ordered list
    Strategy string   `json:"strategy"` // "fallback" | "round-robin"
    Sticky   int      `json:"sticky"`   // consecutive requests before rotate
}

type ComboState struct {
    ComboName    string
    CurrentIndex int
    StickCount   int
}
```

### 3.4 RTK Token Saver

**What it does:** Compresses `tool_result` content (git diffs, grep output, ls -la trees, etc.) before sending to the LLM.

**Detection logic:**
- Scans message content for patterns: `diff --git`, `Index:`, file paths
- Applies bucket compression, line counting, or summarization

**Config:**
```go
type RTKConfig struct {
    Enabled bool  `json:"enabled"`
    Mode    string `json:"mode"` // "auto" | "compress" | "summary"
}
```

**Implementation:** Applied as a middleware in `translator.Normalize()`, before the request goes to the provider.

### 3.5 Caveman Mode

Injects a terse-speak system prompt to reduce output tokens.
Not a high priority — simple system prompt injection.

### 3.6 Usage Tracking

| Metric | Collection | Storage |
|--------|-----------|---------|
| Request count | Per-request counter | SQLite usage_log |
| Token usage | From upstream response | SQLite usage_log |
| Latency | Per-request timer | SQLite usage_log |
| Cost | Computed from token × price | Derived from pricing config |
| Provider stats | Aggregated by provider | SQLite query |

### 3.7 Format Translation

Already done in `pkg/translator`. Desktop app uses it via Go backend.

### 3.8 Proxy Support

Not needed (removed in earlier discussion). But if needed in future:

```go
type ProxyPool struct {
    Name     string `json:"name"`
    Type     string `json:"type"`  // "http" | "socks5"
    Address  string `json:"address"`
    Username string `json:"username,omitempty"`
    Password string `json:"password,omitempty"`
}
```

### 3.9 i18n

Reuse existing `app/web/src/i18n/` locale files directly.
5 languages: EN / ZH / JA / KO / VI. Language switcher in settings.

---

## 4. Wails Go Backend — NodeService Bind API

```go
// NodeService exposes all node capabilities to the React frontend.
type NodeService struct{}

// === Node Lifecycle ===
func (n *NodeService) Start()  string          // Start node
func (n *NodeService) Stop()   string          // Stop node
func (n *NodeService) Status() map[string]any  // Running/PID/uptime

// === Key Management ===
func (n *NodeService) AddKey(label, key, channelID, baseURL string) map[string]any
func (n *NodeService) ListKeys() []map[string]any
func (n *NodeService) DeleteKey(id string) string

// === Model Binding ===
func (n *NodeService) BindModel(keyID, modelCode, modelName string, shared bool) map[string]any
func (n *NodeService) UnbindModel(id string) string
func (n *NodeService) ListBindings() []map[string]any

// === Combo ===
func (n *NodeService) CreateCombo(name string, models []string, strategy string, sticky int) map[string]any
func (n *NodeService) ListCombos() []map[string]any
func (n *NodeService) DeleteCombo(name string) string

// === RTK / Caveman ===
func (n *NodeService) SetRTK(enabled bool, mode string) string
func (n *NodeService) SetCaveman(enabled bool) string
func (n *NodeService) GetConfig() map[string]any

// === Usage ===
func (n *NodeService) GetUsageStats() map[string]any
func (n *NodeService) GetUsageLogs(limit int) []map[string]any

// === Provider ===
func (n *NodeService) ListProviders() []map[string]any
func (n *NodeService) RegisterProvider(config map[string]any) map[string]any
func (n *NodeService) TestProvider(providerID string) map[string]any
```

---

## 5. Frontend Zustand Stores

### useNodeStore
```ts
interface NodeState {
  status: 'running' | 'stopped'
  pid: number | null
  uptime: number
  port: number
  keysCount: number
  modelsCount: number
  requestsToday: number
}
```

### useComboStore
```ts
interface ComboStore {
  combos: Combo[]
  createCombo: (name, models, strategy, sticky) => Promise<void>
  deleteCombo: (name) => Promise<void>
  reorderModels: (comboName, fromIndex, toIndex) => void
}
```

### useUsageStore
```ts
interface UsageStore {
  totalRequests: number
  totalTokens: number
  totalCost: number
  byModel: { model: string; calls: number; tokens: number }[]
  chartData: { time: string; tokens: number }[]
}
```

---

## 6. Implementation Priority

| Feature | Priority | Effort | Dependency |
|---------|----------|--------|-----------|
| Node lifecycle (start/stop/status) | P0 | Small | Existing cmd/node |
| API Key CRUD | P0 | Small | Existing keychain |
| Model binding | P0 | Medium | Existing keychain + router |
| Dashboard page | P0 | Medium | Wails bind |
| Usage tracking | P1 | Medium | Existing db |
| Combo model lists | P1 | Medium | New feature |
| RTK Token Saver | P2 | Large | New pkg/rtk |
| Provider management | P2 | Medium | New feature |
| Chat playground | P2 | Small | pkg/provider |
| Caveman Mode | P3 | Small | Prompt injection |
| Proxy pools | P3 | Small | New feature |
| OAuth | P3 | Large | New feature |
