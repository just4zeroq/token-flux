# dnode — P2P Direct Communication Design

> Branch: `dnode` · Last updated: 2026-05-31

## Overview

dnode replaces the platform-centric forwarding model with an overlay P2P network where consumers connect directly to providers. The platform becomes a **node on equal footing** — acting as both an official provider and the Hub registry, but data-plane traffic bypasses the platform entirely.

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

## 3. Message Flows

### 3.1 Registration

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

### 3.2 Discovery

```
Consumer Client                   Platform Hub
      │                                │
      │── GET /api/v1/nodes/discover   │
      │     ?model=deepseek-v4-flash ──→│
      │                                │
      │← { nodes: [                    │
      │     { node_id: "p-a",          │
      │       netbird_ip: "100.64.0.3",│
      │       input_price: 50,         │
      │       output_price: 150,       │
      │       reputation: 95,          │
      │       latency_ms: 15 },         │
      │     { node_id: "platform",     │            ← 平台自身也是 Provider
      │       netbird_ip: "100.64.0.2",│
      │       input_price: 100,        │
      │       output_price: 300,       │
      │       reputation: 100 }        │
      │   ] }                          │
      │                                │
      │  (Consumer 按 price/reputation/latency 选择 Provider)  │
```

### 3.3 Direct Chat (P2P)

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

### 3.4 Dual Usage Reporting

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

## 4. Platform Hub API

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/v1/nodes/register` | POST | JWT (provider) | Register/update provider node |
| `/api/v1/nodes/unregister` | POST | JWT (provider) | Remove node |
| `/api/v1/nodes/discover` | GET | JWT | Find providers for a model |
| `/api/v1/nodes/heartbeat` | POST | JWT | Node alive check |
| `/api/v1/nodes/report-usage` | POST | JWT | Submit usage record |
| `/api/v1/nodes/{id}` | GET | JWT | Get node detail |

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

## 6. Migration Path

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

## 7. Security

| Concern | Mitigation |
|---------|------------|
| Provider key leak | Local key hash uploaded, not raw key |
| Usage fraud | Dual reporting with request_id cross-check |
| Man-in-middle | WireGuard encryption + NetBird mesh |
| Unauthorized access | Platform JWT auth for Hub API |
| Provider false model list | Platform reviews registered models |

---

## 8. Recommendation

**P2P Direct is the better long-term architecture:**

- Zero platform bandwidth cost for data-plane traffic
- Lower latency (no relay hop)
- Platform scales independently from data traffic
- Natural fit for a "marketplace" model — platform is the market, not the courier

The forwarding WS tunnel stays as a **fallback** for providers who can't/won't run a full client.
