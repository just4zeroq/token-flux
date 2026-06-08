# Node Network — Token Sharing Design

## Overview

Desktop node joins platform network via self-sovereign crypto identity (Ed25519 keypair). Shares LLM keys to platform. Platform routes external requests to connected nodes via WebSocket tunnel. Node owner controls which models shared, at what price. Earnings credited to node's wallet address.

No email, no password, no JWT.

---

## Crypto Identity

### Key generation — Ed25519

```go
import "crypto/ed25519"
pub, priv := ed25519.GenerateKey(nil)  // 32B seed, 32B pub
```

- `seed` — the actual secret. 32 bytes. Needed to restore keypair.
- `public_key` — 32 bytes. Used as node_id and wallet address.
- `wallet_address = hex(public_key)` — 64 hex chars. IS the node_id.
- `private_key = ed25519.NewKeyFromSeed(seed)` — derived from seed at runtime.

### Storage — `~/.flux/node.key`

Encrypted with AES-256-GCM. Key material derived from `SHA256(hostname + home_dir)`.

```json
{
  "ciphertext": "base64...",
  "nonce": "base64...",
  "salt": "hex..."
}
```

File permissions: `0600` (owner only).

### Backup — shown once on key generation

User shown:
```
Your wallet address: 2a1b3c4d5e6f7890abcdef1234567890abcdef12

⚠️  SAVE THIS — IT CONTROLS YOUR NODE & EARNINGS

Private key seed (hex):
  a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2

Options: [Copy to clipboard]  [Download backup.txt]
```

No mnemonic for MVP. User gets hex seed. Back up or lose access forever.

---

## Registration Flow

Node side:
1. Generate Ed25519 keypair
2. Save encrypted to `~/.flux/node.key`
3. Show wallet_address + seed backup
4. POST to platform: `{wallet_address, signature: ed25519.Sign(priv, "flux-node-register-"+wallet_address)}`
5. Platform: recover public_key from wallet_address (hex decode), verify signature
6. Platform: INSERT nodes table, return `{status: "registered"}`
7. Node: connect WS tunnel

---

## WebSocket Auth (challenge-response)

Existing HMAC auth replaced. New flow:

```
Client (node)                    Server (platform)
    │                                  │
    │  GET /ws                         │
    │─────────────────────────────────→│
    │                                  │
    │  {type:"auth",                   │
    │   wallet_address: "abc..."       │
    │  }                               │
    │←─────────────────────────────────│
    │                                  │
    │  {type:"challenge",              │
    │   nonce: "random-32-bytes-hex"   │
    │  }                               │
    │                                  │
    │  {type:"auth_response",          │
    │   signature: "ed25519-sig"       │
    │  }                               │
    │─────────────────────────────────→│  verify(public_key, nonce, signature)
    │                                  │
    │  {type:"auth_ok"}                │
    │←─────────────────────────────────│  or {type:"auth_fail", error:"..."}
```

Platform stores `last_heartbeat` after auth. If node reconnects within 60s, uses same nonce timeout guard.

---

## Architecture

```
node/desktop/
├── main.go                             ← NodeService methods for crypto + network
├── pkg/
│   ├── wallet/
│   │   └── wallet.go                   ← Ed25519 gen, sign, encrypt, decrypt, backup
│   └── tunnel/
│       └── tunnel.go                   ← WS client (challenge-response auth)
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   │   └── LoginWidget.tsx         ← becomes AuthWidget: wallet status + setup
│   │   ├── pages/
│   │   │   ├── Network.tsx             ← network page (join/leave/share params)
│   │   │   └── Settings.tsx            ← toggle "Share to Network"
│   │   ├── stores/
│   │   │   └── nodeStore.ts            ← + useNetworkStore, useWalletStore
│   │   └── App.tsx                     ← top bar with AuthWidget
│   └── styles.css

server/
├── internal/
│   ├── logic/node/
│   │   ├── node.go                     ← registry CRUD, challenge generation
│   │   └── ws.go                       ← WS handler, challenge-response auth
│   ├── controller/api/
│   │   └── node.go                     ← REST: register, count, list + WS handler
│   └── relay/channel/
│       └── bridge/                     ← (future) route request to connected node
├── cmd/api/internal/cmd.go             ← register routes
└── migrations/
    └── 0019_nodes.sql                  ← nodes + node_bindings table
```

---

## Platform Data Model

### Migration 0019: `nodes` + `node_bindings`

```sql
CREATE TABLE nodes (
    id              BIGSERIAL PRIMARY KEY,
    wallet_address  TEXT NOT NULL UNIQUE,       -- hex(public_key), acts as node_id
    public_key      TEXT NOT NULL,              -- hex encoded 32B public key
    name            TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'offline',  -- offline | online | suspended
    ip_address      TEXT NOT NULL DEFAULT '',
    version         TEXT NOT NULL DEFAULT '',
    total_earned    BIGINT NOT NULL DEFAULT 0,  -- cumulative credits earned
    last_heartbeat  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE node_bindings (
    id                  BIGSERIAL PRIMARY KEY,
    wallet_address      TEXT NOT NULL REFERENCES nodes(wallet_address),
    model_code          TEXT NOT NULL,
    key_hash            TEXT NOT NULL,
    input_price_per_1k  BIGINT NOT NULL DEFAULT 0,
    output_price_per_1k BIGINT NOT NULL DEFAULT 0,
    cache_price_per_1k  BIGINT NOT NULL DEFAULT 0,
    status              TEXT NOT NULL DEFAULT 'active',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(wallet_address, model_code, key_hash)
);

CREATE INDEX idx_nodes_status ON nodes(status);
CREATE INDEX idx_node_bindings_model ON node_bindings(model_code);
```

### Node registration REST (no auth required)

`POST /api/v1/nodes/register`
```json
// Request
{
  "wallet_address": "a1b2c3d4...",
  "signature": "ed25519 sig of 'flux-node-register-'+wallet_address",
  "name": "my-home-node"
}
// Response
{ "status": "registered", "wallet_address": "a1b2c3..." }
```

`GET /api/v1/nodes/count`
```json
// Response
{ "count": 42 }
```

`GET /api/v1/nodes` — admin only
`GET /api/v1/nodes/:wallet_address` — admin only

---

## WebSocket Protocol (revised)

| Direction | Type | Payload | Description |
|-----------|------|---------|-------------|
| C→S | `auth` | `{wallet_address}` | Send node ID |
| S→C | `challenge` | `{nonce}` | Random 32B hex challenge |
| C→S | `auth_response` | `{signature}` | ed25519 signed nonce |
| S→C | `auth_ok` | `{}` | Auth success |
| S→C | `auth_fail` | `{error}` | Auth rejected |
| C→S | `register` | `{bindings: [{model_code, key_hash, input_price_per_1k, output_price_per_1k}]}` | Register shared models |
| C→S | `unregister` | `{bindings: [{model_code, key_hash}]}` | Remove shared models |
| S→C | `registered` | `{count: int}` | Platform confirmation |
| C→S | `ping` | none | Keepalive |
| S→C | `pong` | none | Keepalive |
| S→C | `node_list` | `{count: int}` | Broadcast peer count |
| C→S | `stats` | `{total_calls, total_tokens, uptime_sec}` | Periodic stats |
| S→C | `request` | `{request_id, model, key_hash, body}` | Relay LLM request |
| C→S | `chunk` / `done` / `error` | stream response | Stream LLM response |

---

## Node Side — Backend

### `pkg/wallet/wallet.go`

```go
type Wallet struct {
    Seed          []byte  // 32 bytes
    PublicKey     []byte  // 32 bytes
}

func Generate() (*Wallet, error)                          // Generate new Ed25519
func Load(path string) (*Wallet, error)                   // Decrypt + load from file
func (w *Wallet) Save(path string) error                  // Encrypt + write to file
func (w *Wallet) Sign(msg []byte) []byte                  // Sign message
func (w *Wallet) Address() string                         // hex(public_key)
func (w *Wallet) SeedHex() string                         // hex(seed) for backup
func EncryptMachineKey() []byte                            // derive AES key from hostname
func BackupWarning() string                                // "SAVE THIS SEED" message
```

Encryption: AES-256-GCM with key = SHA256(`hostname + ":" + homedir + "flux-node"`), random nonce stored alongside.

### `main.go` — NodeService new methods

```go
// WalletStatus returns wallet address + whether registered.
func (n *NodeService) WalletStatus() map[string]any {
    // Returns {wallet_address, has_seed, registered, registered_name}
}

// SetupWallet generates keypair, shows backup, returns wallet address.
func (n *NodeService) SetupWallet() map[string]any {
    // 1. Generate Ed25519 keypair
    // 2. Save encrypted to ~/.flux/node.key
    // 3. Return {wallet_address, seed_hex} for display
    //    (seed shown once, not stored in return value on subsequent calls)
}

// JoinNetwork registers node on platform + connects tunnel.
func (n *NodeService) JoinNetwork(name string) string {
    // 1. Load wallet
    // 2. Sign registration message
    // 3. POST /api/v1/nodes/register
    // 4. Connect WS tunnel
    // 5. Return "ok"
}

// LeaveNetwork disconnects tunnel.
func (n *NodeService) LeaveNetwork() string {
    // 1. Stop tunnel
    // 2. Return "ok"
}

// NetworkStatus returns current connectivity state.
func (n *NodeService) NetworkStatus() map[string]any {
    // Returns {connected, wallet_address, node_name, uptime_sec, peer_count, shared_count}
}

// UpdateShareParams updates prices per binding + re-registers.
func (n *NodeService) UpdateShareParams(params []ShareParam) string {
    // 1. UPDATE DB
    // 2. If connected, re-register via WS
}

// FetchPeerCount calls platform GET /api/v1/nodes/count.
func (n *NodeService) FetchPeerCount() int {
    return count or 0
}
```

### `server.go` — new REST endpoints

```go
mux.HandleFunc("/api/node/wallet", adminOnly(s.handleWallet))   // GET wallet status, POST setup
mux.HandleFunc("/api/node/network", adminOnly(s.handleNetwork)) // GET status, POST join, DELETE leave
mux.HandleFunc("/api/node/network/params", adminOnly(s.handleNetworkParams)) // PUT update share params
```

---

## Node Side — Frontend

### Layout change

```
┌─────────────────────────────────────────────┐
│ Token Flux Node    v0.1     [AuthWidget  ]  │ ← top bar (40px)
├────────────────────┬────────────────────────┤
│ Sidebar            │ Main Content           │
└────────────────────┴────────────────────────┘
```

### `AuthWidget.tsx` — top-right component

States:
| State | Display |
|-------|---------|
| No wallet | `[Setup Node Wallet]` → opens setup modal |
| Wallet, not registered | `flux_a1b2... [Join Network]` |
| Connected | `flux_a1b2... ● Online [Leave]` |

No email, no password, no login form.

### Setup wizard (modal)

1. Generate keypair → show wallet address
2. ⚠️ Backup seed screen — hex seed, copy/download. User must confirm saved.
3. (Optional) Name your node
4. Done → auto-join

### `Settings.tsx`

```
Features card:
  [RTK toggle]
  [Caveman toggle]
  [Share to Network toggle]  ← same pattern as RTK
    (disabled if no wallet → "Setup wallet first" tooltip)
```

Toggle ON + wallet ready → auto-join. Toggle OFF → leave network.

### Sidebar nav

When toggle ON → show `Network` nav item.

### `Network.tsx`

```
Disconnected:
  - Wallet address
  - [Join Network] button (or auto-connected if toggle was ON)
  - Node name input

Connected:
  - Status bar: ● Connected | wallet: 0xa1b2... | uptime: 12m
  - [Leave Network] button
  - Global nodes: 42 online
  - Shared Models table:
      Model        Key         Toggle  Input/Output Price
      gpt-4o       sk-***...   [x]     [5]    [15]
      claude-4     sk-***...   [x]     [3]    [9]
  - [Sync to Platform] button
```

---

## Implementation Order

### Phase 1 (current)

| Step | Files | Est. |
|------|-------|------|
| 1. Wallet pkg | `node/pkg/wallet/wallet.go` | 2h |
| 2. Platform: migration | `server/migrations/0019_nodes.sql` | 1h |
| 3. Platform: node logic | `server/internal/logic/node/node.go`, `ws.go` | 3h |
| 4. Platform: controller | `server/internal/controller/api/node.go` | 2h |
| 5. Platform: route reg | `server/internal/boot/boot.go` | 0.5h |
| 6. Node: NodeService methods | `node/desktop/main.go` (6 methods) | 2h |
| 7. Node: REST handlers | `node/pkg/server/server.go` | 1h |
| 8. Node: backend API | `frontend/src/api/backend.ts` | 0.5h |
| 9. AuthWidget + top bar | `AuthWidget.tsx`, `App.tsx` layout | 1.5h |
| 10. Wallet setup modal | `SetupWallet.tsx` | 1.5h |
| 11. Network page | `Network.tsx` | 2h |
| 12. Settings toggle | `Settings.tsx` | 0.5h |
| 13. Nav item | `App.tsx` | 0.25h |
| | **Total** | **~18h** |

### Phase 2 (future)

- Gateway routing via connected nodes
- Settlement to wallet_address
- Web UI for wallet-linked account management

---

## File Manifest (Phase 1)

### New files
```
node/pkg/wallet/wallet.go
server/migrations/0019_nodes.sql
server/internal/logic/node/node.go
server/internal/logic/node/ws.go
server/internal/controller/api/node.go
node/desktop/frontend/src/components/AuthWidget.tsx
node/desktop/frontend/src/components/SetupWallet.tsx
node/desktop/frontend/src/pages/Network.tsx
```

### Modified files
```
node/pkg/tunnel/tunnel.go           — challenge-response auth (was HMAC)
server/internal/boot/boot.go         — register node routes
server/internal/logic/logic.go       — blank import node
server/internal/service/node.go      — new service interface
node/pkg/db/db.go                    — add shared + price columns on model_key_models
node/desktop/main.go                 — 6 new NodeService methods
node/pkg/server/server.go            — /api/node/wallet + /api/node/network endpoints
node/desktop/frontend/src/App.tsx    — top bar, AuthWidget, conditional nav
node/desktop/frontend/src/pages/Settings.tsx — Share to Network toggle
node/desktop/frontend/src/stores/nodeStore.ts — useNetworkStore
node/desktop/frontend/src/styles.css — top bar styles
node/desktop/frontend/src/api/backend.ts — wallet + network API functions
```
