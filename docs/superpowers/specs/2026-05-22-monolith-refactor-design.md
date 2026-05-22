# 服务端单体化重构设计

**日期**：2026-05-22
**作者**：brainstorming session
**状态**：待 review

## 1. 背景与目标

### 1.1 现状

当前 `server/` 目录是 5 服务微服务架构：
- `user-svc` (gRPC :8100) — 用户、API Key
- `asset-svc` (gRPC :8101) — 余额、订单、用量
- `market-svc` (gRPC :8102) — 商品、评价
- `api-gateway` (HTTP :8080) — 业务 REST API，调用上述 3 个 gRPC
- `ai-gateway` (HTTP :8081) — LLM OpenAI 兼容中转 + admin 后台

外加一个共享的 `server/api/` proto 模块（编译后的 `.pb.go` 文件），每服务一个独立 `go.mod`，通过 `replace api => ../api` 互引。

数据库：4 个独立 PostgreSQL database（`user_svc` / `asset_svc` / `market_svc` / `ai_gateway`）。

### 1.2 目标定位

把服务端重构为 **单体化、模块化、双边 AI 能力市场平台**：

- **平台定位**：双边市场。供给方（LLM 供应商 / Skill+MCP+Agent 开发者）托管 AI 能力，消费方（终端用户）按需调用，平台从每次调用中抽佣。
- **资产体系**：三种资产
  - `credits`（信用分）— 加密充值后按当时汇率折算获得，能力调用从此扣费
  - `balance`（钱包余额，USDT 计价）— 可提现回加密链
  - `points`（积分）— 每次能力调用消费方+供给方各获得一份，未来用于兑换代币/质押
- **充值通道**：BNB Smart Chain、Base、TON 三条链，每用户每条链一个独立充值地址（HD 钱包派生）

### 1.3 决策回顾

| 编号 | 决策 |
|---|---|
| B | 模块化单体（保留 user/asset/market/gateway 等模块边界） |
| B1 | 合并为一个 PostgreSQL database `ai_platform` |
| C2 | 单进程内多 ghttp.Server：api `:8080` + gateway `:8081` |
| D1 | 彻底删除 proto 模块和 gRPC 代码，模块间用 Go interface |
| GF | 采用 GoFrame `service`（接口）+ `logic`（实现）+ `init()` 注册模式 |
| F2 | 删除 admin 的 HTTP 层（controller/router），保留底层 service 和数据库表 |
| G1 | 前端 `app/web` 不改动，保持对外 HTTP 路径兼容 |
| H1 | 一次性大改（big bang），不做 gRPC adapter 渐进迁移 |
| Multi-bin | 同一 `go.mod` 下多个 `cmd/<name>/main.go` 入口，分别可独立编译为：api / wallet-watcher / usage-recorder / settlement |

## 2. 顶层架构

### 2.1 进程拓扑（生产）

```
┌──────────────────────────────────────────────────────┐
│ ai-platform-api          (可水平扩展 N 实例)         │
│   ├── ghttp.Server "api"     → :8080 → /api/v1/*    │
│   └── ghttp.Server "gateway" → :8081 → /v1/*, /mcp/*,│
│                                          /agent/*    │
├──────────────────────────────────────────────────────┤
│ ai-platform-watcher      (单实例)                    │
│   └── BNB / Base / TON 三个监听 goroutine            │
├──────────────────────────────────────────────────────┤
│ ai-platform-recorder     (可扩展 N 实例)             │
│   └── usage_records 批量写入 worker                  │
├──────────────────────────────────────────────────────┤
│ ai-platform-settlement   (单实例)                    │
│   └── 周期性供给方收入结算 cron                      │
└──────────────────────────────────────────────────────┘
        ↓ 共享同一个 PostgreSQL: ai_platform
```

所有可执行文件共享同一份 `internal/`（service / logic / dao / model / worker），通过同一个 `go.mod` 编译。

### 2.2 开发模式

开发时通常只需 `go run ./cmd/api`，依赖 watcher 时单独跑 `go run ./cmd/watcher`。

### 2.3 领域划分（8 个）

| 领域 | 职责 |
|---|---|
| **identity** | 用户、角色、API Key、JWT |
| **catalog** | 能力上架/审核/版本/分类（LLM / MCP / Agent / Skill 共享生命周期） |
| **market** | 订单、购买、评价 |
| **wallet** | 加密钱包：充值地址生成、链上监听、提现签名广播 |
| **billing** | 复式记账：账户、流水、credits/balance/points 三资产、分账 |
| **pricing** | 定价策略：按 token / 按调用 / 订阅 |
| **usage** | 跨能力类型的统一用量记录与统计 |
| **gateway** | LLM / MCP / Agent 三种运行时代理 |

## 3. 目录结构

```
server/
├── go.mod                                  module ai-platform
├── main.go                                 (仅用于本地快速试跑，调 cmd/api)
├── manifest/config/config.yaml
├── migrations/                             单库统一 SQL（goose）
├── hack/config.yaml                        gf gen dao 配置
│
├── cmd/                                    多入口可执行文件
│   ├── api/main.go                         注册 api + gateway 两个 ghttp.Server
│   ├── watcher/main.go                     启动 wallet 链上监听
│   ├── recorder/main.go                    启动 usage 批量写入
│   └── settlement/main.go                  启动周期结算
│
├── internal/
│   ├── boot/
│   │   └── boot.go                         注册 router、middleware（api 入口共用）
│   ├── consts/
│   │   └── consts.go
│   ├── middleware/
│   │   ├── jwt.go                          JWT 校验
│   │   ├── apikey.go                       API Key 校验
│   │   └── common.go                       CORS / Recover / RequestID
│   │
│   ├── service/                            ← 所有领域对外接口（统一出口）
│   │   ├── identity.go                     IIdentity + RegisterIdentity / Identity()
│   │   ├── catalog.go                      ICatalog
│   │   ├── market.go                       IMarket
│   │   ├── wallet.go                       IWallet
│   │   ├── billing.go                      IBilling
│   │   ├── pricing.go                      IPricing
│   │   ├── usage.go                        IUsage
│   │   ├── gateway_llm.go                  ILLMRuntime
│   │   ├── gateway_mcp.go                  IMCPRuntime
│   │   └── gateway_agent.go                IAgentRuntime
│   │
│   ├── logic/                              ← 业务实现
│   │   ├── logic.go                        空导入所有子包以触发 init()
│   │   ├── identity/
│   │   │   ├── identity.go                 sIdentity struct + init() + New()
│   │   │   ├── user.go
│   │   │   ├── apikey.go
│   │   │   └── jwt.go
│   │   ├── catalog/
│   │   │   ├── catalog.go
│   │   │   ├── llm_item.go
│   │   │   ├── mcp_item.go
│   │   │   ├── agent_item.go
│   │   │   └── skill_item.go
│   │   ├── market/
│   │   │   ├── market.go
│   │   │   ├── order.go
│   │   │   └── review.go
│   │   ├── wallet/
│   │   │   ├── wallet.go                   IWallet 实现（GetDepositAddress / Withdraw）
│   │   │   ├── address.go                  HD 钱包地址派生
│   │   │   ├── rate.go                     汇率获取
│   │   │   └── chain/                      链适配器
│   │   │       ├── chain.go                IChainAdapter 接口
│   │   │       ├── bnb.go
│   │   │       ├── base.go
│   │   │       └── ton.go
│   │   ├── billing/
│   │   │   ├── billing.go                  IBilling 实现入口
│   │   │   ├── account.go                  账户管理（accounts 表 CRUD）
│   │   │   ├── transaction.go              复式记账写入
│   │   │   ├── credits.go                  充值入账（credits 增发）
│   │   │   ├── exchange.go                 credits → balance 兑换
│   │   │   ├── consume.go                  能力调用扣费 + 分账 + 发 points
│   │   │   └── settlement.go               供给方收入划账（credits → balance）
│   │   ├── pricing/
│   │   │   ├── pricing.go
│   │   │   ├── strategy_token.go
│   │   │   ├── strategy_call.go
│   │   │   └── strategy_subscription.go
│   │   ├── usage/
│   │   │   ├── usage.go                    查询 / 统计
│   │   │   └── recorder_client.go          IUsage.Record 内部：推到 recorder worker（队列）
│   │   └── gateway/
│   │       ├── gateway.go                  空导入 llm/mcp/agent
│   │       ├── channel/                    gateway 内部：上游路由
│   │       │   └── channel.go
│   │       ├── llm/
│   │       │   └── llm.go                  ILLMRuntime 实现
│   │       ├── mcp/
│   │       │   └── mcp.go                  IMCPRuntime 实现
│   │       └── agent/
│   │           └── agent.go                IAgentRuntime 实现
│   │
│   ├── worker/                             ← 后台 worker（每个对应一个 cmd 入口）
│   │   ├── walletwatcher/
│   │   │   ├── watcher.go                  Start(ctx) / Stop()
│   │   │   ├── bnb.go
│   │   │   ├── base.go
│   │   │   └── ton.go
│   │   ├── usagerecorder/
│   │   │   └── recorder.go                 Start(ctx) / Stop() + 批量写 worker
│   │   └── settlement/
│   │       └── settlement.go               Start(ctx) / Stop() + gcron 定时任务
│   │
│   ├── controller/                         ← HTTP handler
│   │   ├── api/                            挂 api server :8080
│   │   │   ├── identity/                   register/login/profile/keys
│   │   │   ├── catalog/                    上架/审核/浏览
│   │   │   ├── market/                     下单/购买/评价
│   │   │   ├── wallet/                     充值地址/链上记录/发起提现
│   │   │   ├── billing/                    账户/兑换/流水/points
│   │   │   └── usage/                      用量查询
│   │   └── gateway/                        挂 gateway server :8081
│   │       ├── llm/                        /v1/chat/completions 等 OpenAI 兼容
│   │       ├── mcp/                        /mcp/*
│   │       └── agent/                      /agent/*
│   │
│   ├── model/
│   │   ├── do/                             gf gen dao 产物
│   │   ├── entity/                         gf gen dao 产物
│   │   └── dto/                            service 接口入参出参
│   │       ├── identity.go
│   │       ├── catalog.go
│   │       ├── market.go
│   │       ├── wallet.go
│   │       ├── billing.go
│   │       ├── pricing.go
│   │       ├── usage.go
│   │       ├── gateway_llm.go
│   │       ├── gateway_mcp.go
│   │       └── gateway_agent.go
│   ├── dao/                                gf gen dao 产物
│   ├── packed/
│   └── utility/                            通用工具（hash / token gen / crypto / hd wallet）
│
├── resource/
└── README.md
```

### 3.1 删除的旧目录

`server/api/` `server/proto/` `server/api-gateway/` `server/ai-gateway/` `server/user-svc/` `server/asset-svc/` `server/market-svc/` `server/docker/` `server/scripts/` `server/ai-platform/`（孤立残留）

每个旧服务的内部 `grpcclient/`、`controller/admin/`、`controller/<svc>/`、`service/` 等全部迁入新结构。

## 4. 模块边界与调用规则

### 4.1 service 接口模式

每个领域在 `internal/service/<domain>.go` 定义一个 interface 和一对包级访问器：

```go
// service/identity.go
package service

import (
    "context"
    "ai-platform/internal/model/dto"
)

type IIdentity interface {
    Register(ctx context.Context, in dto.RegisterIn) (*dto.RegisterOut, error)
    Login(ctx context.Context, in dto.LoginIn) (*dto.LoginOut, error)
    GetUser(ctx context.Context, userID int64) (*dto.UserInfo, error)
    ValidateToken(ctx context.Context, token string) (*dto.TokenClaims, error)
    ValidateApiKey(ctx context.Context, key string) (*dto.ApiKeyInfo, error)
    CreateApiKey(ctx context.Context, in dto.CreateApiKeyIn) (*dto.ApiKeyOut, error)
    ListApiKeys(ctx context.Context, userID int64) ([]*dto.ApiKeyInfo, error)
    DeleteApiKey(ctx context.Context, userID, keyID int64) error
}

var localIdentity IIdentity

func RegisterIdentity(i IIdentity) { localIdentity = i }
func Identity() IIdentity {
    if localIdentity == nil {
        panic("service.Identity not registered")
    }
    return localIdentity
}
```

### 4.2 logic 实现 + 自注册

```go
// logic/identity/identity.go
package identity

import (
    "ai-platform/internal/service"
)

type sIdentity struct{}

func init() { service.RegisterIdentity(New()) }
func New() *sIdentity { return &sIdentity{} }

// ...各方法实现
```

### 4.3 启动时聚合导入

```go
// internal/logic/logic.go
package logic

import (
    _ "ai-platform/internal/logic/identity"
    _ "ai-platform/internal/logic/catalog"
    _ "ai-platform/internal/logic/market"
    _ "ai-platform/internal/logic/wallet"
    _ "ai-platform/internal/logic/billing"
    _ "ai-platform/internal/logic/pricing"
    _ "ai-platform/internal/logic/usage"
    _ "ai-platform/internal/logic/gateway"
)
```

```go
// cmd/api/main.go
package main

import (
    _ "ai-platform/internal/logic"   // 触发所有领域 init() 注册
    "ai-platform/internal/boot"
)

func main() { boot.RunAPI() }
```

### 4.4 跨模块调用规则

| 调用方 | 被调用 | 走什么 |
|---|---|---|
| `controller/*` | `logic/*` | `service.Xxx().Method(...)` |
| `logic/<A>` | `logic/<B>`（不同领域） | **必须经 service 接口**：`service.B().Method(...)` |
| `logic/<X>` 同包内 | 同包函数/方法 | 直接调用 |
| `logic/gateway/llm` | `logic/gateway/channel`（同 gateway 领域子包） | 允许直接 import |
| `worker/*` | `logic/*` | `service.Xxx().Method(...)` |

规则的目的：领域间依赖收敛到 service interface 这一层；同领域内的子模块（如 gateway 下 llm/mcp/agent/channel）允许直接 import，因为它们是同一领域的内部实现细节。

## 5. 数据库设计

单一 PostgreSQL database `ai_platform`。所有金额字段使用 BIGINT，单位为 micro-units（×10^6），避免浮点累计误差。

### 5.1 identity

```sql
users (
    id           BIGSERIAL PRIMARY KEY,
    username     TEXT UNIQUE NOT NULL,
    email        TEXT UNIQUE,
    password_hash TEXT NOT NULL,
    role         TEXT NOT NULL DEFAULT 'consumer',  -- consumer | provider | admin
    kyc_status   TEXT NOT NULL DEFAULT 'none',      -- none | pending | verified
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
)

api_keys (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id),
    key_hash      TEXT UNIQUE NOT NULL,
    name          TEXT,
    quota_credits BIGINT,                            -- 单 key 配额上限（NULL 不限）
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at    TIMESTAMPTZ
)
```

### 5.2 catalog

4 种能力类型（LLM / MCP / Agent / Skill）共享 `catalog_items`，type-specific 配置存 JSON：

```sql
catalog_items (
    id             BIGSERIAL PRIMARY KEY,
    owner_user_id  BIGINT NOT NULL REFERENCES users(id),
    type           TEXT NOT NULL,         -- llm | mcp | agent | skill
    name           TEXT NOT NULL,
    description    TEXT,
    status         TEXT NOT NULL,         -- draft | published | offline
    review_status  TEXT NOT NULL,         -- pending | approved | rejected
    version        INT NOT NULL DEFAULT 1,
    config_json    JSONB NOT NULL,        -- LLM: {base_url, upstream_model, ...}; MCP: {endpoint}; Agent: {entry}
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
)

catalog_categories (
    id        BIGSERIAL PRIMARY KEY,
    name      TEXT NOT NULL,
    parent_id BIGINT REFERENCES catalog_categories(id)
)

catalog_item_tags (
    item_id BIGINT NOT NULL REFERENCES catalog_items(id),
    tag     TEXT NOT NULL,
    PRIMARY KEY (item_id, tag)
)
```

### 5.3 market

```sql
orders (
    id             BIGSERIAL PRIMARY KEY,
    buyer_user_id  BIGINT NOT NULL REFERENCES users(id),
    item_id        BIGINT NOT NULL REFERENCES catalog_items(id),
    plan_type      TEXT NOT NULL,        -- one_off | subscription | package
    amount_credits BIGINT NOT NULL,
    status         TEXT NOT NULL,        -- pending | paid | refunded | cancelled
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
)

reviews (
    id            BIGSERIAL PRIMARY KEY,
    buyer_user_id BIGINT NOT NULL REFERENCES users(id),
    item_id       BIGINT NOT NULL REFERENCES catalog_items(id),
    rating        SMALLINT NOT NULL,
    body          TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
```

### 5.4 wallet

```sql
deposit_addresses (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id),
    chain      TEXT NOT NULL,            -- bnb | base | ton
    address    TEXT NOT NULL,
    hd_path    TEXT NOT NULL,            -- m/44'/60'/0'/0/<user_id> 等
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, chain),
    UNIQUE (chain, address)
)

chain_deposits (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT NOT NULL REFERENCES users(id),
    chain              TEXT NOT NULL,
    tx_hash            TEXT NOT NULL,
    from_addr          TEXT NOT NULL,
    to_addr            TEXT NOT NULL,
    amount_native      NUMERIC(38, 18) NOT NULL,    -- 原生币数量（高精度）
    amount_usd_at_time BIGINT NOT NULL,             -- 折算后 micro-USD
    rate_at_time       NUMERIC(38, 18) NOT NULL,
    confirmations      INT NOT NULL DEFAULT 0,
    status             TEXT NOT NULL,               -- observed | confirmed | credited
    observed_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (chain, tx_hash)
)

withdraw_requests (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id),
    chain           TEXT NOT NULL,
    to_address      TEXT NOT NULL,
    amount_balance  BIGINT NOT NULL,                -- micro-USD
    fee             BIGINT NOT NULL,
    status          TEXT NOT NULL,                  -- pending | broadcast | confirmed | failed
    tx_hash         TEXT,
    broadcast_at    TIMESTAMPTZ,
    confirmed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
```

### 5.5 billing（复式记账核心）

```sql
accounts (
    id            BIGSERIAL PRIMARY KEY,
    owner_type    TEXT NOT NULL,           -- user | platform
    owner_id      BIGINT NOT NULL,         -- user_id 或平台账户编号
    asset         TEXT NOT NULL,           -- credits | balance | points
    balance_micro BIGINT NOT NULL DEFAULT 0,
    version       BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (owner_type, owner_id, asset)
)

transactions (
    id         BIGSERIAL PRIMARY KEY,
    tx_type    TEXT NOT NULL,              -- deposit_credits | exchange | consume | settle | withdraw | points_grant
    ref_type   TEXT,                       -- chain_deposit | order | usage_record | withdraw_request
    ref_id     BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)

transaction_entries (
    id                  BIGSERIAL PRIMARY KEY,
    tx_id               BIGINT NOT NULL REFERENCES transactions(id),
    account_id          BIGINT NOT NULL REFERENCES accounts(id),
    delta_micro         BIGINT NOT NULL,      -- 正/负
    balance_after_micro BIGINT NOT NULL
)
```

**复式记账约束**：

- **守恒型 tx**（`exchange` / `consume` / `settle` / `withdraw`）：entries 按 asset 分组后 SUM(delta) 必须为 0。
- **发行型 tx**（`deposit_credits` / `points_grant`）：单边发行，不强制 SUM=0。由 `tx_type` 显式区分；写入逻辑里按类型走不同的校验。

测试不变量：所有 accounts 的 balance_micro 按 asset 求和 = SUM(发行型 tx delta) - SUM(销毁型 tx delta)（销毁型本设计暂无）。

**典型账户**：

- 用户：(user, user_id, credits) / (user, user_id, balance) / (user, user_id, points)
- 平台：(platform, 1, credits)（佣金池/手续费）/ (platform, 1, balance)（提现汇集）/ (platform, 1, points)（总池）

### 5.6 pricing

```sql
pricing_rules (
    id             BIGSERIAL PRIMARY KEY,
    item_id        BIGINT NOT NULL REFERENCES catalog_items(id),
    strategy       TEXT NOT NULL,           -- token | call | subscription
    params_json    JSONB NOT NULL,          -- {input_price_per_1k, output_price_per_1k, commission_rate, ...}
    effective_from TIMESTAMPTZ NOT NULL,
    effective_to   TIMESTAMPTZ
)

exchange_rates (
    id            BIGSERIAL PRIMARY KEY,
    asset_from    TEXT NOT NULL,
    asset_to      TEXT NOT NULL,
    rate_micro    BIGINT NOT NULL,          -- 兑换比率 ×10^6
    fee_rate_bps  INT NOT NULL DEFAULT 0,   -- 手续费 basis points (10000 = 100%)
    effective_at  TIMESTAMPTZ NOT NULL
)
```

### 5.7 usage

```sql
usage_records (
    id                       BIGSERIAL PRIMARY KEY,
    user_id                  BIGINT NOT NULL REFERENCES users(id),
    api_key_id               BIGINT REFERENCES api_keys(id),
    item_id                  BIGINT NOT NULL REFERENCES catalog_items(id),
    runtime                  TEXT NOT NULL,            -- llm | mcp | agent
    capability_kind          TEXT,                     -- chat | embedding | mcp_call | agent_exec | ...
    input_tokens             INT NOT NULL DEFAULT 0,
    output_tokens            INT NOT NULL DEFAULT 0,
    call_count               INT NOT NULL DEFAULT 1,
    latency_ms               INT NOT NULL DEFAULT 0,
    cost_credits             BIGINT NOT NULL,          -- 用户实际扣的 credits
    commission_credits       BIGINT NOT NULL,          -- 平台佣金
    provider_revenue_credits BIGINT NOT NULL,          -- 供给方收入
    points_to_consumer       BIGINT NOT NULL DEFAULT 0,
    points_to_provider       BIGINT NOT NULL DEFAULT 0,
    request_id               TEXT,
    channel_id               BIGINT,
    ip                       TEXT,
    is_stream                BOOLEAN NOT NULL DEFAULT FALSE,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
```

索引：`(user_id, created_at)` / `(item_id, created_at)` / `request_id`。

### 5.8 gateway 内部

```sql
upstream_channels (
    id            BIGSERIAL PRIMARY KEY,
    item_id       BIGINT NOT NULL REFERENCES catalog_items(id),
    provider      TEXT NOT NULL,            -- openai | anthropic | azure | ...
    base_url      TEXT NOT NULL,
    key_encrypted TEXT NOT NULL,
    priority      INT NOT NULL DEFAULT 0,
    status        TEXT NOT NULL DEFAULT 'active',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
```

一个 LLM `catalog_item` 可挂多个 `upstream_channels` 做容灾/路由。

## 6. 关键数据流

### 6.1 BNB 充值入账

```
1. 用户调 GET /api/v1/wallet/deposit-address?chain=bnb
   → controller/api/wallet/wallet.go
   → service.Wallet().GetDepositAddress(uid, "bnb")
   → logic/wallet/address.go
     - 查 deposit_addresses(user_id=uid, chain="bnb")
     - 没有则用 HD 派生路径 m/44'/60'/0'/0/<uid> 派生新地址，写入
   → 返回 "0xABC..."

2. 用户从外部钱包转 0.5 BNB → 0xABC...

3. cmd/watcher 进程的 worker/walletwatcher/bnb.go：
   - 轮询/订阅 BNB 节点
   - 发现 0xABC... 入账 0.5 BNB
   - 写 chain_deposits(status=observed, confirmations=1)
   - 等达到确认阈值 → status=confirmed
   - logic/wallet/rate.go 查当时汇率（如 1 BNB = 600 USD）
   - chain_deposits.amount_usd_at_time = 300_000_000 micro-USD
   - 调 service.Billing().DepositCredits({user_id, credits=300_000_000, ref_type=chain_deposit, ref_id=...})
   → logic/billing/credits.go 单事务内：
     - upsert accounts(user, uid, credits)
     - INSERT transactions(tx_type='deposit_credits', ref_type, ref_id)
     - INSERT transaction_entries(+300_000_000 to user.credits account)
     - UPDATE chain_deposits.status='credited'
```

### 6.2 LLM 能力调用（核心链路）

```
1. SDK: POST :8081/v1/chat/completions  Authorization: Bearer sk-xxx
2. middleware/apikey.go:
   - service.Identity().ValidateApiKey(ctx, "sk-xxx")
   - 注入 user_id / api_key_id / 关联的 item_id 到 ctx

3. controller/gateway/llm/llm.go:
   - service.LLMRuntime().Chat(ctx, in)

4. logic/gateway/llm/llm.go:
   a) service.Catalog().GetItem(ctx, item_id)
   b) service.Pricing().PreCalc(ctx, {item_id, est_tokens=4000})
      → est_cost_credits = 12_000_000
   c) service.Billing().Consume(ctx, {mode=reserve, user_id, item_id, credits=12_000_000})
      → 检查 user.credits 余额，预扣（事务+乐观锁），返回 reservation_id
   d) logic/gateway/channel/channel.go: Pick(item) 选 upstream_channels 中的一个
   e) 调上游 LLM API，拿到 response 和实际 tokens
   f) service.Pricing().FinalCalc(ctx, {item_id, actual_tokens})
      → actual_cost = 10_500_000
      → commission = actual_cost * commission_rate
      → provider_revenue = actual_cost - commission
      → points_to_consumer / points_to_provider 按规则计算
   g) 单事务内顺序写入（usage_record 先于 transactions 以便 ref 引用）：
      - INSERT usage_records(...) → 拿到 usage_record_id
      - service.Billing().Consume(ctx, {mode=settle, reservation_id, actual_cost,
                                         commission, provider_revenue,
                                         points_to_consumer, points_to_provider,
                                         ref_type='usage_record', ref_id=usage_record_id})
        → INSERT transactions(tx_type='consume', ref_type='usage_record', ref_id)
        → entries (credits，守恒型 SUM=0)：
          * -actual_cost from user.credits
          * +commission to platform.credits
          * +provider_revenue to provider.credits
        → 退还预扣差额 (12_000_000 - 10_500_000) 给 user.credits（同一 tx 或独立 tx，plan 阶段定）
        → INSERT transactions(tx_type='points_grant', ref_type='usage_record', ref_id)
        → entries (points，发行型)：
          * +points_to_consumer to user.points
          * +points_to_provider to provider.points
   h) （usage 聚合统计的批量异步处理由 cmd/recorder 进程消费 PG NOTIFY，不阻塞本调用）
   i) 返回 response
```

### 6.3 credits → balance 兑换

```
POST /api/v1/billing/exchange {credits: 50_000_000}
→ service.Billing().ExchangeCreditsToBalance(uid, 50_000_000)
→ logic/billing/exchange.go：
  - 查 exchange_rates(credits→balance) 最近 effective 记录（如 rate=0.95×10^6, fee=0）
  - 单事务：
    - INSERT transactions(tx_type='exchange')
    - entries:
      * -50_000_000 from user.credits
      * +47_500_000 to user.balance
      * +2_500_000 to platform.fee（手续费汇集账户）
```

### 6.4 供给方收入结算

```
cmd/settlement 进程，worker/settlement/settlement.go 注册 gcron 每日任务：

1. 遍历所有 owner_type=provider 的 provider.credits 账户
2. service.Billing().SettleProviderRevenue(ctx, provider_id)
→ logic/billing/settlement.go：
   - 单事务：
     - INSERT transactions(tx_type='settle')
     - -X from provider.credits
     - +X to provider.balance
```

### 6.5 用户提现

```
POST /api/v1/wallet/withdraw {chain: bnb, to: 0x..., amount: 100_000_000}
→ service.Wallet().Withdraw(ctx, in)
→ logic/wallet/withdraw.go：
  a) service.Billing().DebitBalanceForWithdraw(uid, 100_000_000, tx_ref)
     → -100_000_000 from user.balance
     → 写 withdraw_requests(status=pending)
  b) 用 HD 派生地址对应私钥签名交易
  c) chain/bnb.go 广播
  d) status=broadcast，记录 tx_hash
  e) worker 监听确认 → status=confirmed
```

## 7. 启动入口

### 7.1 cmd/api/main.go

```go
package main

import (
    _ "ai-platform/internal/logic"
    "ai-platform/internal/boot"
)

func main() { boot.RunAPI() }
```

### 7.2 internal/boot/boot.go（仅 api 入口用）

```go
func RunAPI() {
    ctx := gctx.New()

    apiSrv := g.Server("api")
    apiSrv.Group("/api/v1", func(group *ghttp.RouterGroup) {
        group.Middleware(middleware.CORS, middleware.Recover)

        group.POST("/auth/register", apiIdentity.Register)
        group.POST("/auth/login", apiIdentity.Login)
        group.GET("/catalog/items", apiCatalog.List)

        group.Group("/", func(g *ghttp.RouterGroup) {
            g.Middleware(middleware.JWTAuth)
            g.GET("/users/me", apiIdentity.Me)
            g.GET("/users/me/keys", apiIdentity.ListKeys)
            g.POST("/users/me/keys", apiIdentity.CreateKey)
            g.DELETE("/users/me/keys/{id}", apiIdentity.DeleteKey)

            g.POST("/catalog/items", apiCatalog.Create)
            g.PUT("/catalog/items/{id}", apiCatalog.Update)
            g.POST("/catalog/items/{id}/submit", apiCatalog.SubmitReview)

            g.POST("/market/orders", apiMarket.CreateOrder)
            g.GET("/market/orders", apiMarket.ListOrders)
            g.POST("/market/reviews", apiMarket.CreateReview)

            g.GET("/wallet/deposit-address", apiWallet.GetDepositAddress)
            g.GET("/wallet/deposits", apiWallet.ListDeposits)
            g.POST("/wallet/withdraw", apiWallet.Withdraw)

            g.GET("/billing/accounts", apiBilling.GetAccounts)
            g.POST("/billing/exchange", apiBilling.Exchange)
            g.GET("/billing/transactions", apiBilling.ListTx)

            g.GET("/usage/records", apiUsage.List)
            g.GET("/usage/aggregate", apiUsage.Aggregate)
        })
    })

    gwSrv := g.Server("gateway")
    gwSrv.Group("/v1", func(group *ghttp.RouterGroup) {
        group.Middleware(middleware.CORS, middleware.Recover, middleware.APIKeyAuth)
        group.POST("/chat/completions", llmCtl.Chat)
        group.POST("/embeddings", llmCtl.Embedding)
        group.GET("/models", llmCtl.ListModels)
    })
    gwSrv.Group("/mcp", func(group *ghttp.RouterGroup) {
        group.Middleware(middleware.CORS, middleware.Recover, middleware.APIKeyAuth)
        group.ALL("/*path", mcpCtl.Proxy)
    })
    gwSrv.Group("/agent", func(group *ghttp.RouterGroup) {
        group.Middleware(middleware.CORS, middleware.Recover, middleware.APIKeyAuth)
        group.POST("/execute", agentCtl.Execute)
    })

    g.Log().Infof(ctx, "ai-platform-api starting")
    g.Wait()
}
```

### 7.3 cmd/watcher/main.go

```go
package main

import (
    _ "ai-platform/internal/logic"
    "ai-platform/internal/worker/walletwatcher"
    "github.com/gogf/gf/v2/frame/g"
    "github.com/gogf/gf/v2/os/gctx"
)

func main() {
    ctx := gctx.New()
    walletwatcher.Start(ctx)
    g.Log().Infof(ctx, "ai-platform-watcher starting")
    g.Wait()
}
```

### 7.4 cmd/recorder/main.go

```go
package main

import (
    _ "ai-platform/internal/logic"
    "ai-platform/internal/worker/usagerecorder"
    "github.com/gogf/gf/v2/frame/g"
    "github.com/gogf/gf/v2/os/gctx"
)

func main() {
    ctx := gctx.New()
    usagerecorder.Start(ctx)
    g.Wait()
}
```

**关于 recorder 进程的职责定位（plan 阶段决策）**：

由于 API 和 recorder 跨进程，主路径上的 usage_records 写入必须由 API 进程在 §6.2 流程 g) 的同一事务内完成（与 transactions 一起），以确保扣费和用量记录原子一致 —— 这不能下放到异步队列。

cmd/recorder 进程的职责退化为：

- 聚合统计：按小时/天/能力维度预计算 usage_aggregates，加速 `service.Usage().AggregateByCapability` 查询
- 历史归档：将超过 N 天的 usage_records 转储到归档表
- 监听 NOTIFY 触发实时聚合（可选）

如果未来 API 端 INSERT usage_records 成为瓶颈，再引入 PG LISTEN/NOTIFY + 临时表的异步缓冲方案。第一版不做。

### 7.5 cmd/settlement/main.go

```go
package main

import (
    _ "ai-platform/internal/logic"
    "ai-platform/internal/worker/settlement"
    "github.com/gogf/gf/v2/frame/g"
    "github.com/gogf/gf/v2/os/gctx"
)

func main() {
    ctx := gctx.New()
    settlement.Start(ctx)
    g.Wait()
}
```

## 8. 配置

### 8.1 manifest/config/config.yaml

```yaml
server:
  api:
    address: ":8080"
  gateway:
    address: ":8081"

database:
  default:
    type: "pgsql"
    host: "${DB_HOST}"
    port: "5432"
    user: "aiplatform"
    pass: "aiplatform"
    name: "ai_platform"
    debug: false

logger:
  level: "info"
  stdout: true

jwt:
  secret: "${JWT_SECRET}"
  expireHours: 24

wallet:
  hd:
    mnemonic_env: "WALLET_MNEMONIC"     # 主助记词从环境变量读取，不进配置
  chains:
    bnb:
      rpc: "${BNB_RPC_URL}"
      confirmations: 12
      poll_interval_seconds: 5
    base:
      rpc: "${BASE_RPC_URL}"
      confirmations: 12
      poll_interval_seconds: 5
    ton:
      api: "${TON_API_URL}"
      confirmations: 1
      poll_interval_seconds: 5

usage:
  recorder:
    batch_size: 50
    flush_interval_ms: 100

settlement:
  cron: "0 0 3 * * *"     # 每天凌晨 3 点
```

环境变量：DB_HOST / JWT_SECRET / WALLET_MNEMONIC / BNB_RPC_URL / BASE_RPC_URL / TON_API_URL

## 9. 迁移策略

采用 **H1 一次性大改**，但 commit 拆细，每个 commit 内自洽可编译：

| 顺序 | Commit | 验证手段 |
|---|---|---|
| 1 | 建新 go.mod / cmd 骨架 / service 接口（空实现）/ 单库 migration | `go build ./...` |
| 2 | 迁入 identity 逻辑（旧 user-svc → logic/identity）| identity 单元测试 |
| 3 | 迁入 billing（含 accounts / transactions / entries 复式记账）、pricing、usage（旧 asset-svc → logic/billing+pricing+usage）| billing 单元测试，跑充值入账+消费的脚本 |
| 4 | 迁入 catalog、market（旧 market-svc + 部分 ai-gateway/admin → logic/catalog+market）| catalog/market 单元测试 |
| 5 | 迁入 gateway（旧 ai-gateway/relay → logic/gateway/{llm,channel}）| `:8081/v1/chat/completions` 端到端跑通 |
| 6 | 新建 wallet（含 HD 钱包、链适配器骨架；测试网先行）| 测试网上 BNB 充值能识别 |
| 7 | 删除 server/api / server/proto / server/{api,ai}-gateway / server/{user,asset,market}-svc / server/docker / server/scripts / server/ai-platform | `go build ./...`、`go run ./cmd/api` 启动 |
| 8 | docker-compose 重写为单镜像多 service 编排 | 容器化启动并跑通 |

整个分支完成后合并 master。前端 `app/web` 因为 HTTP 路径兼容，全程不动。

## 10. 测试策略

- **单元测试**：每个 logic 子包，mock 其他模块的 service 接口（这是 service 接口模式的天然好处）
- **集成测试**：`logic/billing/consume_test.go` 真实跑事务，验证复式记账 SUM=0 不变量
- **端到端**：
  - `:8080/api/v1/auth/register` → 登陆 → 创建 API key
  - `:8081/v1/chat/completions` 用真实/mock 上游跑一次，验证 `usage_records` 与 `transaction_entries` 一致
  - 充值流程：在 mock chain watcher 上注入 deposit event，验证 credits 入账
- **金额不变量测试**：随机化生成 N 笔混合 tx，最后断言每个 asset 的所有 accounts.balance_micro 之和 == 该 asset 净发行量（credits 发行总和 - 销毁；balance/points 类同）

## 11. 范围外（本次不做）

- 前端 `app/web` 改动
- Tauri 桌面端 `app/desktop` 改动
- 实际的链上签名/广播代码（chain 适配器接口先定，具体实现到 plan 阶段或后续 PR）
- 真正的供给方 KYC / 法律文档流
- 平台运营后台 UI（admin HTTP 已删，运营暂走 SQL / 配置）
- 实时 / WebSocket / 流式响应的具体实现（gateway/llm 的 `chat/completions` stream 模式 plan 阶段细化）
- MCP / Agent 的具体协议适配（先建接口和路由骨架，实际协议解析后续 PR）

## 12. 风险

- **复式记账实现复杂度**：需要严格事务 + 乐观锁，预扣/结算/退款边界容易出错。缓解：每个金额相关测试都验证 SUM=0 不变量。
- **HD 钱包安全**：主助记词泄漏 = 全用户充值地址私钥泄漏。缓解：助记词只在环境变量里，watcher/withdraw 进程隔离部署，未来引入 HSM/KMS。
- **链上 reorg**：观察到的 deposit 在确认数内可能被回滚。缓解：等足够 confirmations 后再写入 credits。
- **跨进程 usage 写入一致**：§7.4 已明确 API 进程直接同步写入 usage_records（与 billing transaction 同一事务），recorder 进程只做聚合/归档，不存在丢数据风险。
- **一次性大改风险**：中间 commit 可能某些功能短暂不可用。缓解：早期项目无生产流量，可接受；每个 commit 保持可编译。
