# dnode — P2P Direct Communication Design

> Branch: `dnode` · Last updated: 2026-05-31

## Overview

dnode replaces the platform-centric forwarding model with an overlay P2P network. Consumers connect directly to providers over NetBird WireGuard mesh. The platform becomes a **node on equal footing** but retains **routing control** — the platform runs a recommendation algorithm that scores and ranks providers for each request. Consumers express preferences (cheap/fast/favorite), but the platform decides the final route.

---

## 1. Architecture Comparison

### Model A: Forwarding (WebSocket Tunnel) — 现有方案

```
Consumer ──HTTP──→ Platform (:8081) ──WS Tunnel──→ Provider Node ──→ upstream LLM
```

| Pros | Cons |
|------|------|
| Provider 无需公网 IP | 平台承载全部流量，瓶颈 |
| 平台全程可控（审计/限流/鉴权） | 带宽成本高 |
| 简单，一版实现 | 延迟多一跳 |
| | 单点故障 |

### Model B: P2P Direct — dnode 方案

```
Consumer ──WireGuard──→ Provider Node ──→ upstream LLM
     ↘                          ↙
        Platform Hub (meta only)
```

| Pros | Cons |
|------|------|
| 平台不承载数据流量，零带宽成本 | 需要双方有 NetBird 客户端 |
| 延迟最低（直连） | NAT 穿透依赖 NetBird Mgmt |
| 去中心化，无单点 | 对账依赖双方诚实上报 |
| 平台专注结算/对账/发现 | 新增 NetBird 依赖 |

### Comparison

| Dimension | Forwarding (WS) | P2P Direct (dnode) |
|-----------|-----------------|---------------------|
| **延迟** | 3 hops | 2 hops (consumer ↔ provider direct) |
| **平台带宽** | 承载全部 LLM stream 流量 | 仅承载 meta (注册/发现/上报) |
| **Provider 要求** | 主动 WS 连接平台即可 | NetBird agent + NAT 穿透 |
| **Consumer 要求** | 只需 API key | 需要客户端 + NetBird agent |
| **结算可靠性** | 平台记录一切 | 双方上报对账 |
| **可审计性** | 高（全在平台） | 依赖上报 |
| **扩展性** | 平台瓶颈 | 线性扩展 |
| **适用场景** | 供应商懒得装客户端 | 长期节点、桌面用户 |

---

## 2. Network Topology

```
Token Flux P2P Network (NetBird WireGuard Mesh · 100.64.0.0/16)

┌──────────────────────────────────────────────────────────────┐
│                                                               │
│  ┌──────────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │   Platform        │  │ Provider A    │  │ Provider B    │  │
│  │   (官方Provider    │  │ (第三方节点)   │  │ (第三方节点)   │  │
│  │    + Hub 注册中心) │  │               │  │               │  │
│  │                   │  │               │  │               │  │
│  │  :8081 Gateway    │  │ :20128 /v1/*  │  │ :20128 /v1/*  │  │
│  │  :8080 Hub API    │  │               │  │               │  │
│  │   100.64.0.2      │  │ 100.64.0.3    │  │ 100.64.0.4    │  │
│  └───────┬──────────┘  └───────┬───────┘  └───────┬───────┘  │
│          │                     │                    │          │
│          └─────────────────────┼────────────────────┘          │
│                                │                               │
│                 NetBird WireGuard Mesh                         │
│                                │                               │
│   ┌────────────────────────────┼───────────────────────────┐  │
│   │                     Consumer Clients                   │  │
│   │    100.64.0.10      100.64.0.11      100.64.0.12       │  │
│   └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

**Platform dual role:**

| Role | Endpoints |
|------|-----------|
| **Official Provider** | `:8081/v1/*` — OpenAI-compatible, platform's own API keys/channels |
| **Hub Registry** | `:8080/api/v1/nodes/*` — registration, discovery, usage reporting |

---

## 3. Routing Control Model

**Platform controls routing. Consumers express preferences.**

```
Consumer preferences { "cheap", "fast", "favorite_node_X" }
         ↓
Platform Recommendation Engine
  ├── Consumer preference signal (20%)
  ├── Provider quality metrics (30%)
  ├── Platform operational strategy (50%)  ← 扶持新节点、打压违规、商业推广
  ↓
Scored & ranked provider list
  ↓
Consumer client → try nodes in order → fallback on failure
```

### 3.1 Routing Algorithm

**Scoring formula (platform-side):**

```
score = W_price × (1 - normalized_price)
      + W_reputation × normalized_reputation
      + W_latency × (1 - normalized_latency)
      + platform_boost
      - platform_penalty
```

**Platform operational levers:**

| Lever | Effect | Example |
|-------|--------|---------|
| `platform_boost` | 冷启动：新供应商自动加分 | 新注册Provider +0.15 持续7天 |
| `platform_penalty` | 违规降权 | 虚报用量 -0.30 |
| `blacklist` | 下架 | 严重违规直接不在结果中出现 |
| `min_reputation` | 质量门槛 | 信誉分<60 不推荐 |
| `max_latency` | 延迟门槛 | 延迟>500ms 不推荐 |
| `promoted_slot` | 商业化 | 竞价排名置顶 |

### 3.2 Consumer Preferences

```json
// Consumer sets in client settings
{
  "preference": "cheap",       // "cheap" | "fast" | "quality" | "balanced"
  "favorites": ["node-X"],     // personal favorite providers (soft boost)
  "blocked": ["node-Y"]        // blacklisted providers (forced exclude)
}
```

**How preferences affect weights:**

| Preference | W_price | W_reputation | W_latency |
|------------|---------|-------------|-----------|
| `cheap` | 0.6 | 0.2 | 0.2 |
| `fast` | 0.2 | 0.2 | 0.6 |
| `quality` | 0.2 | 0.6 | 0.2 |
| `balanced` | 0.33 | 0.33 | 0.33 |

**Advanced mode:** Expert users can pin a specific provider (bypass algorithm), but the provider must still be in the platform's approved list.

---

## 4. Message Flows

### 4.1 Registration

```
Provider Client                    Platform Hub
      │                                │
      │── POST /api/v1/nodes/register ──→│
      │   { node_id,                     │
      │     netbird_ip: "100.64.0.3",   │
      │     models: [                    │
      │       { model_code: "deepseek-v4-flash",    │
      │         input_price: 50,         │  (micro-credits/1K)
      │         output_price: 150,       │
      │         key_hash: "7fa70acb..." }│
      │     ]                            │
      │   }                              │
      │← { ok: true, registered_at } ────│
```

### 4.2 Routing Request (Platform Decides)

```
Consumer Client                               Platform Hub
      │                                            │
      │── POST /api/v1/nodes/route                │
      │   { model: "deepseek-v4-flash",           │
      │     preference: "cheap",                   │  ← consumer expresses intent
      │     favorites: ["node-X"],                 │
      │     blocked: ["node-Y"] }                  │
      │──────────────────────────────────────────→│
      │                                            │  Platform runs recommendation:
      │                                            │  1. Query eligible providers
      │                                            │  2. Apply consumer preference weights
      │                                            │  3. Apply platform operational levers
      │                                            │  4. Score + rank + return top N
      │                                            │
      │← {                                        │
      │     nodes: [                               │
      │       { rank: 1, node_id: "p-a",             │
      │         netbird_ip: "100.64.0.3",         │
      │         score: 0.87 },                     │
      │       { rank: 2, node_id: "p-b",           │
      │         netbird_ip: "100.64.0.4",         │
      │         score: 0.74 },                     │
      │       { rank: 3, node_id: "platform",      │
      │         netbird_ip: "100.64.0.2",         │
      │         score: 0.62 }                      │
      │     ]                                      │
      │   }                                        │
      │                                            │
      │  (Consumer client tries nodes in order)    │
```

### 4.3 Direct Chat (P2P)

```
Consumer Client                              Provider Client
      │                                            │
      │── POST http://100.64.0.3:20128/v1/        │
      │   chat/completions                        │
      │   { model: "deepseek-v4-flash",           │
      │     messages: [...],                       │
      │     stream: true }                         │
      │──────────────────────────────────────────→│
      │                                            │  Provider Node executes:
      │                                            │  · Resolve key_hash → real key
      │                                            │  · Translate format
      │                                            │  · Call upstream LLM
      │                                            │
      │← SSE stream (chunks...) ─────────────────│
      │                                            │
      │← { "done", usage: { tokens: 1500, ... } } │
```

### 4.4 Dual Usage Reporting

```
                    Platform Hub
                   ↗            ↖
Consumer reports   │              │  Provider reports
{ request_id,      │              │  { request_id,
  provider_node_id,│              │    consumer_user_id,
  model,           │              │    model,
  tokens_in,       │              │    tokens_in,
  tokens_out,      │              │    tokens_out,
  cost,            │              │    cost,
  latency_ms }     │              │    latency_ms }

         Platform compares request_id → both match → settlement
```

**Conflict resolution:**
- Only `request_id` matches → settle with the lower cost
- One side missing → trust the present one (gap fill with retry)
- Both missing → no settlement, gap in usage logs

---

## 5. Platform Hub API

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/v1/nodes/register` | POST | JWT (provider) | Register/update provider node |
| `/api/v1/nodes/unregister` | POST | JWT (provider) | Remove node |
| `/api/v1/nodes/route` | POST | JWT | **Platform routes consumer to providers** (replaces discovery) |
| `/api/v1/nodes/routing-config` | GET | JWT | Get platform routing config (weights, thresholds, blacklist) |
| `/api/v1/nodes/heartbeat` | POST | JWT | Node alive check |
| `/api/v1/nodes/report-usage` | POST | JWT | Submit usage record |
| `/api/v1/nodes/{id}` | GET | JWT | Get node detail |

### 5.1 Route Request

```
POST /api/v1/nodes/route
{
  "model": "deepseek-v4-flash",
  "preference": "cheap",
  "favorites": ["node-X"],
  "blocked": ["node-Y"]
}
→ { nodes: [{ rank, node_id, netbird_ip, score }] }
```

### 5.2 Routing Config (platform-pushed)

```
GET /api/v1/nodes/routing-config
→ {
  "global": {
    "weights": { "price": 0.33, "reputation": 0.33, "latency": 0.33 },
    "thresholds": { "min_reputation": 60, "max_latency_ms": 500 },
    "blacklist": ["node-bad"],
    "boost_map": { "node-new": 0.15 }
  },
  "per_model_overrides": {
    "deepseek-v4-pro": { "min_reputation": 80 }
  }
}
```

### Node table (in platform DB)

```sql
CREATE TABLE p2p_nodes (
    id              BIGSERIAL PRIMARY KEY,
    node_id         VARCHAR(64) NOT NULL UNIQUE,    -- unique node identifier
    owner_user_id   BIGINT NOT NULL,                 -- platform user
    role            VARCHAR(16) NOT NULL DEFAULT 'provider',  -- provider | consumer
    netbird_ip      VARCHAR(45) NOT NULL,            -- WireGuard IP
    status          VARCHAR(16) NOT NULL DEFAULT 'online',  -- online | offline
    last_heartbeat  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE p2p_node_models (
    id              BIGSERIAL PRIMARY KEY,
    node_id         VARCHAR(64) NOT NULL REFERENCES p2p_nodes(node_id),
    model_code      VARCHAR(255) NOT NULL,
    key_hash        VARCHAR(64) NOT NULL,
    input_price     BIGINT NOT NULL DEFAULT 0,       -- micro-credits/1K tokens
    output_price    BIGINT NOT NULL DEFAULT 0,
    cache_hit_price BIGINT NOT NULL DEFAULT 0,
    status          VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE p2p_usage_reports (
    id              BIGSERIAL PRIMARY KEY,
    request_id      VARCHAR(128) NOT NULL,
    reporter_node_id VARCHAR(64) NOT NULL,            -- who reported
    counterpart_node_id VARCHAR(64),                   -- the other side
    reporter_role   VARCHAR(16) NOT NULL,             -- consumer | provider
    model_code      VARCHAR(255) NOT NULL,
    input_tokens    INT NOT NULL DEFAULT 0,
    output_tokens   INT NOT NULL DEFAULT 0,
    cost_credits    BIGINT NOT NULL DEFAULT 0,
    latency_ms      INT NOT NULL DEFAULT 0,
    reported_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_p2p_usage_request ON p2p_usage_reports(request_id);
```

---

## 5. Client Architecture

```
Tauri Desktop App
├── src-tauri/ (Rust)
│   ├── spawn Go Node subprocess
│   └── Tauri commands (start/stop/status)
├── Go Node (subprocess)
│   ├── netbird SDK (embedded — connects to Mgmt, gets mesh IP)
│   ├── provider/http.go — upstream LLM caller
│   ├── server/server.go — local :20128 OpenAI-compatible + :20129 admin API
│   ├── router/router.go — multi-account round-robin + fallback
│   └── tunnel/tunnel.go — WS tunnel (fallback when P2P unavailable)
└── React UI
    ├── /desktop — login, dashboard, keys, models, settings
    └── communicates with node via localhost:20129/api/node/*
```

---

## 6. Admin Node Management

### 6.1 Admin API (platform :8082)

| Endpoint | Description |
|----------|-------------|
| `GET /api/admin/nodes` | List all registered nodes |
| `PUT /api/admin/nodes/:id/review` | Approve/reject node |
| `PUT /api/admin/nodes/:id/boost` | Set platform boost/penalty |
| `PUT /api/admin/nodes/:id/blacklist` | Blacklist/unblacklist node |
| `GET /api/admin/nodes/:id/stats` | Node statistics |
| `PUT /api/admin/routing-config` | Update global routing config |

### 6.2 Admin UI

- Node list with status, uptime, reputation
- Batch boost/penalty controls
- Routing config editor (weights, thresholds)
- Promoted slot management
#### 6.2.1 Route Visibility

平台通过以下方式知道当前 P2P 通信状态：

| 信息 | 来源 | 内容 |
|------|------|------|
| **谁请求了路由** | `POST /api/v1/nodes/route` | consumer_user_id, model, preference |
| **推荐了谁** | 平台算法 | 返回的 ranked node list |
| **实际连了谁** | 双上报 | `request_id` 关联 → 确认 consumer ↔ provider |
| **通信量多大** | 双上报 | tokens, cost, latency_ms |

数据流量（chat content）不经平台，但平台知道**谁和谁在通信、什么模型、用了多少**。

---

## 7. Migration Path

```
Phase 1 (current): Forwarding WS Tunnel  ← 已完成，平台集中转发
Phase 2 (dnode):   P2P Direct            ← 本设计
  · Consumer + Provider 都装客户端
  · NetBird mesh 建立
  · 双上报对账

Hybrid Mode: both can coexist
  · Provider 在线 → P2P
  · Provider 离线 / Consumer 无客户端 → Gateway fallback (:8081)
  · Consumer 优先 P2P，不可用时降级到平台转发
```

---

## 8. Security

| Concern | Mitigation |
|---------|------------|
| Provider key leak | Local key hash uploaded, not raw key |
| Usage fraud | Dual reporting with request_id cross-check |
| Man-in-middle | WireGuard encryption + NetBird mesh |
| Unauthorized access | Platform JWT auth for Hub API |
| Provider false model list | Platform reviews registered models |

---

## 9. Recommendation

**P2P Direct is the better long-term architecture:**

- Zero platform bandwidth cost for data-plane traffic
- Lower latency (no relay hop)
- Platform scales independently from data traffic
- Natural fit for a "marketplace" model — platform is the market, not the courier

The forwarding WS tunnel stays as a **fallback** for providers who can't/won't run a full client.
