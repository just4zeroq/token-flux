# AI-Platform — 统一架构规划

---

## 一、现状

### 三段代码, 三种模式

```
server/internal/relay/channel/    ← 25 brand adaptor (各写各的)
ai-platform/pkg/provider/         ← 纯 HTTP body 发送 (太薄)
node/pkg/provider/                ← 包装 pkg, 自己拼 URL/Headers/Auth
```

**问题**:
- server 和 node 的 provider 调用逻辑重复
- pkg/provider 只做 `POST body → *http.Response`, 调用方重复做 URL 构建/Headers 组装/Auth
- node 数据库表名 `keys`/`bindings`/`usage_log` 和平台 `llm_model_keys`/`llm_model_key_models`/`llm_usage_records` 对不上

**9router 参考**: 按需借鉴 executor 模式, 不照搬 20+ 子类 (已讨论确认).

---

## 二、架构策略

### 1. Executor 层: 每个品牌各自适配, 统一接口

保留 server relay 现有的品牌级适配粒度, 但:
- 所有 adaptor **实现统一的 `Executor` 接口** (当前 server 的 Adaptor 设计正确, 只是位置不对)
- **搬到 `ai-platform/pkg/executor/`**, server 和 node 共同使用
- 新增 provider 只需新增一个 executor 文件 + registry 注册一行

```
pkg/executor/
├── executor.go           ← Executor interface
├── openai.go             ← OpenAI 适配器
├── claude.go             ← Claude/Anthropic
├── gemini.go             ← Google Gemini
├── deepseek.go           ← DeepSeek (FIM + thinking)
├── zhipu.go              ← Zhipu/GLM
├── ali.go                ← Ali/Qwen
├── baidu.go              ← Baidu/Ernie
├── ...
└── registry.go           ← provider_type → executor 映射
```

### 2. 数据库表: 尽量对齐平台命名

| 平台 (PGSQL) | Node 当前 (SQLite) | Node 对齐后 |
|---|---|---|
| `llm_channels` | — | `llm_channels` |
| `llm_model_specs` | — | `llm_model_specs` |
| `llm_model_prices` | — | `llm_model_prices` |
| `llm_model_keys` | `keys` | `llm_model_keys` |
| `llm_model_key_models` | `bindings` | `llm_model_key_models` |
| `llm_usage_records` | `usage_log` | `llm_usage_records` |
| — | `combos` | `node_combos` (node 特有) |
| — | `node_config` | `node_config` (node 特有) |

Node 特定功能 (combos, 本地配置) 用 `node_` 前缀, 其余复用平台命名.

---

## 三、Executor 接口设计

```go
// pkg/executor/executor.go

package executor

import (
    "context"
    "io"
    "net/http"
)

// Executor 是 AI provider 适配器接口。
// 每个 provider 品牌一个实现 (openai, claude, deepseek, zhipu, ...)。
type Executor interface {
    // Init 使用 channel 元数据初始化
    Init(channel *Channel)

    // GetRequestURL 构建上游请求 URL
    GetRequestURL(info *RequestInfo) (string, error)

    // SetupRequestHeader 设置上游请求头
    SetupRequestHeader(header http.Header, info *RequestInfo) error

    // TransformRequest 转换请求体 (格式转换 + 模型映射 + provider 特有注入)
    TransformRequest(ctx context.Context, info *RequestInfo, requestBody []byte) (io.Reader, error)

    // DoRequest 发送 HTTP 请求到上游
    DoRequest(ctx context.Context, info *RequestInfo, requestBody io.Reader) (*http.Response, error)

    // TransformResponse 处理上游响应并写回客户端
    TransformResponse(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error)

    // GetName 返回 provider 名称 (日志/监控)
    GetName() string
}
```

**与 server 现有 Adaptor 的关系**:

```
server/internal/relay/common/Adaptor    → 演变为 pkg/executor/Executor
server/internal/relay/common/RelayInfo   → 演变为 pkg/executor/RequestInfo
server/internal/relay/channel/*/adaptor  → 搬到 pkg/executor/*.go
server/internal/relay/channel/registry   → 改为调用 pkg/executor
```

**保留 server 特有的逻辑** (不进 pkg):
- 多租户计费/结算
- 渠道审核流
- gfast admin API 响应格式
- JWT/APIKey auth
- Request ID 追踪

---

## 四、数据模型 (pkg/model/)

```go
// pkg/model/channel.go — 从 server/internal/model/dto 迁移
// 对齐 llm_channels 表结构

type Channel struct {
    ID                  int64           `json:"id"`
    Code                string          `json:"code"`
    Name                string          `json:"name"`
    Description         string          `json:"description"`
    ProtocolsJSON       json.RawMessage `json:"protocols_json"`
    ProviderType        int             `json:"provider_type"`       // 1=OpenAI, 2=Claude ...
    ProtocolType        string          `json:"protocol_type"`       // "openai-compatible", "anthropic-compatible"
    BaseURL             string          `json:"base_url"`
    APiKeyEncrypted     string          `json:"-"`
    IsModelMapped       bool            `json:"is_model_mapped"`
    UpstreamModelName   string          `json:"upstream_model_name"`
    Status              string          `json:"status"`
    CreatedAt           int64           `json:"created_at"`
    UpdatedAt           int64           `json:"updated_at"`
}

// pkg/model/modelspec.go
// 对齐 llm_model_specs 表

type ModelSpec struct {
    ID              int64    `json:"id"`
    DeveloperName   string   `json:"developer_name"`
    ModelName       string   `json:"model_name"`
    ModelCode       string   `json:"model_code"`
    DisplayName     string   `json:"display_name"`
    ModelFamily     string   `json:"model_family"`
    Description     string   `json:"description"`
    Capabilities    []string `json:"capabilities"`
    ContextWindow   int      `json:"context_window"`
    MaxInputTokens  int      `json:"max_input_tokens"`
    MaxOutputTokens int      `json:"max_output_tokens"`
    SupportsStream  bool     `json:"supports_stream"`
    SupportsTools   bool     `json:"supports_tools"`
    SupportsVision  bool     `json:"supports_vision"`
    Status          string   `json:"status"`
}

// pkg/model/modelkey.go
// 对齐 llm_model_keys + llm_model_key_models

type ModelKey struct {
    ID              int64  `json:"id"`
    ProviderUserID  int64  `json:"provider_user_id"`
    ChannelID       int64  `json:"channel_id"`
    Name            string `json:"name"`
    KeyEncrypted    string `json:"-"`
    KeyMasked       string `json:"key_masked"`
    QuotaLimit      int64  `json:"quota_limit_credits"`
    QuotaUsed       int64  `json:"quota_used_credits"`
    Status          string `json:"status"`
}

type KeyModelBinding struct {
    ID                int64  `json:"id"`
    ModelKeyID        int64  `json:"model_key_id"`
    ModelSpecID       int64  `json:"model_spec_id"`
    UpstreamModelName string `json:"upstream_model_name"`
    Priority          int    `json:"priority"`
    Weight            int    `json:"weight"`
    Status            string `json:"status"`
}

// pkg/model/usage.go
// 对齐 llm_usage_records

type UsageRecord struct {
    ID                  int64  `json:"id"`
    RequestID           string `json:"request_id"`
    ModelSpecID         int64  `json:"model_spec_id"`
    ModelKeyID          int64  `json:"model_key_id"`
    ChannelID           int64  `json:"channel_id"`
    InputTokens         int    `json:"input_tokens"`
    OutputTokens        int    `json:"output_tokens"`
    TotalTokens         int    `json:"total_tokens"`
    CostCredits         int64  `json:"cost_credits"`
    LatencyMs           int    `json:"latency_ms"`
    Status              string `json:"status"`
    CreatedAt           int64  `json:"created_at"`
}
```

---

## 五、Node 数据库 Schema (对齐后)

```sql
-- 对齐平台 llm_channels
CREATE TABLE llm_channels (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    code           TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    protocols_json TEXT NOT NULL DEFAULT '{}',
    provider_type  INTEGER NOT NULL DEFAULT 1,
    base_url       TEXT NOT NULL DEFAULT '',
    api_key_encrypted TEXT NOT NULL DEFAULT '',
    is_model_mapped INTEGER NOT NULL DEFAULT 0,
    upstream_model_name TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'active',
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
);

-- 对齐平台 llm_model_specs
CREATE TABLE llm_model_specs (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    developer_name    TEXT NOT NULL,
    model_name        TEXT NOT NULL,
    model_code        TEXT NOT NULL UNIQUE,
    display_name      TEXT NOT NULL DEFAULT '',
    model_family      TEXT NOT NULL DEFAULT '',
    description       TEXT NOT NULL DEFAULT '',
    capabilities_json TEXT NOT NULL DEFAULT '[]',
    context_window    INTEGER NOT NULL DEFAULT 0,
    max_input_tokens  INTEGER NOT NULL DEFAULT 0,
    max_output_tokens INTEGER NOT NULL DEFAULT 0,
    supports_stream   INTEGER NOT NULL DEFAULT 0,
    supports_tools    INTEGER NOT NULL DEFAULT 0,
    supports_vision   INTEGER NOT NULL DEFAULT 0,
    status            TEXT NOT NULL DEFAULT 'active',
    created_at        INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL
);

-- 对齐平台 llm_model_keys
CREATE TABLE llm_model_keys (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id        INTEGER NOT NULL,
    name              TEXT NOT NULL DEFAULT '',
    key_encrypted     TEXT NOT NULL,
    key_masked        TEXT NOT NULL DEFAULT '',
    quota_limit_credits INTEGER NOT NULL DEFAULT 0,
    quota_used_credits  INTEGER NOT NULL DEFAULT 0,
    status            TEXT NOT NULL DEFAULT 'active',
    created_at        INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL
);

-- 对齐平台 llm_model_key_models
CREATE TABLE llm_model_key_models (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    model_key_id        INTEGER NOT NULL,
    model_spec_id       INTEGER NOT NULL,
    upstream_model_name TEXT NOT NULL,
    priority            INTEGER NOT NULL DEFAULT 0,
    weight              INTEGER NOT NULL DEFAULT 1,
    status              TEXT NOT NULL DEFAULT 'active',
    last_error          TEXT NOT NULL DEFAULT '',
    created_at          INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL,
    UNIQUE(model_key_id, model_spec_id, upstream_model_name)
);

-- 对齐平台 llm_usage_records
CREATE TABLE llm_usage_records (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id        TEXT NOT NULL DEFAULT '',
    model_spec_id     INTEGER NOT NULL DEFAULT 0,
    model_key_id      INTEGER NOT NULL DEFAULT 0,
    channel_id        INTEGER NOT NULL DEFAULT 0,
    capability        TEXT NOT NULL DEFAULT 'chat',
    is_stream         INTEGER NOT NULL DEFAULT 0,
    input_tokens      INTEGER NOT NULL DEFAULT 0,
    output_tokens     INTEGER NOT NULL DEFAULT 0,
    total_tokens      INTEGER NOT NULL DEFAULT 0,
    cost_credits      INTEGER NOT NULL DEFAULT 0,
    latency_ms        INTEGER NOT NULL DEFAULT 0,
    status            TEXT NOT NULL DEFAULT 'success',
    error_code        TEXT NOT NULL DEFAULT '',
    error_message     TEXT NOT NULL DEFAULT '',
    created_at        INTEGER NOT NULL
);

-- Node 特有: 本地 API Key (用于 sk-xxx 网关认证)
CREATE TABLE node_api_keys (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    key_hash    TEXT NOT NULL UNIQUE,
    key_prefix  TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active',
    last_used_at INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL
);

-- Node 特有: 组合路由策略
CREATE TABLE node_combos (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    models      TEXT NOT NULL DEFAULT '[]',
    strategy    TEXT NOT NULL DEFAULT 'fallback',
    sticky      INTEGER NOT NULL DEFAULT 1,
    created_at  INTEGER NOT NULL
);

-- Node 特有: 本地配置
CREATE TABLE node_config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

---

## 六、功能规划

### Phase 1 — 共享核心 (ai-platform/pkg)

目标: 创建 pkg/executor + pkg/model, 为 server 和 node 共用

| 任务 | 内容 |
|------|------|
| 1.1 pkg/model/ | Channel, ModelSpec, ModelKey, KeyModelBinding, UsageRecord |
| 1.2 pkg/executor/interface | Executor + RequestInfo + Usage |
| 1.3 pkg/executor/openai | OpenAI-compatible (覆盖 60% provider) |
| 1.4 pkg/executor/claude | Anthropic Messages API |
| 1.5 pkg/executor/gemini | Google Gemini |
| 1.6 pkg/executor/deepseek | DeepSeek (FIM + thinking 注入) |
| 1.7 pkg/executor/zhipu | Zhipu/GLM |
| 1.8 pkg/executor/ali | Ali/Qwen |
| 1.9 pkg/executor/baidu | Baidu/Ernie |
| 1.10 pkg/executor/others | volcengine, moonshot, minimax, xunfei... |
| 1.11 pkg/executor/registry | provider_type/protocol → executor 映射 |
| 1.12 Server relay 迁移 | registry 改为调 pkg/executor, 删 channel/*/ 中重复 adaptor |

### Phase 2 — Node 完整功能

目标: 可独立运行的本地 AI 网关

| 任务 | 内容 |
|------|------|
| 2.1 Node DB 迁移 | 从当前 schema 迁移到对齐后 schema |
| 2.2 node/provider 重构 | 使用 pkg/executor 替代现有包装 |
| 2.3 HTTP server 增强 | /v1/chat/completions, /v1/messages, /v1/models |
| 2.4 API Key 网关认证 | sk-xxx 生成 + 验证 middleware |
| 2.5 Channel CRUD | 桌面端管理 provider channels |
| 2.6 Model Spec CRUD | 模型定义管理 |
| 2.7 Model Key 管理 | Key 加密存储 + 绑定模型 |
| 2.8 Combo 路由 | fallback/round-robin (node_combos) |
| 2.9 Usage 追踪 | 写入 llm_usage_records |
| 2.10 Frontend 全功能 | Dashboard, Channels, Models, Keys, Combos, Usage, Logs, Settings |

### Phase 3 — Node 增强

| 任务 | 内容 |
|------|------|
| 3.1 Account Fallback | 多 key 绑定同一 model, 失败自动切换 |
| 3.2 CLI 增强 | Cobra subcommands |
| 3.3 System Tray | 后台运行 + 托盘菜单 |
| 3.4 OAuth Provider | Claude Code / Codex CLI OAuth 集成 |
| 3.5 Tunnel | WebSocket 连接到 platform, 配置同步 |

---

## 七、Phase 1 详细步骤

### Week 1: pkg/model + executor interface

```
Day 1-2: pkg/model/ — Channel, ModelSpec, ModelKey, KeyModelBinding, UsageRecord
Day 3-4: pkg/executor/ — Executor interface + RequestInfo + Usage
Day 5:   Registry + 适配器骨架
```

### Week 2-3: executor 逐个迁移 (从 server relay)

按使用频率排序, 每个适配器:
1. 从 `server/internal/relay/channel/<name>/adaptor.go` 提取
2. 适配到统一的 `Executor` 接口
3. 注册到 registry

```
Day 1-2:  openai (最高频)
Day 3:    claude
Day 4:    deepseek
Day 5:    gemini
Day 6:    zhipu
Day 7:    ali
Day 8-9:  baidu, volcengine, moonshot, minimax, mistral, xai ...
Day 10:   server relay 迁移, 集成测试
```

---

## 八、数据流对比

### 当前 (重复)

```
server:  Controller → Router → Adaptor{GetURL,SetupHeader,ConvertReq,DoReq,DoResp}
                                                  │
node:    Server → Router → provider{Call+ExecuteStream} → pkg/provider (thin)
                                  ↑
                         自己拼 URL/Headers
```

### 目标 (统一)

```
server:  Controller → Router → Executor{...} → pkg/provider (thin)
                                        │
node:    Server → Router → Executor{...} → pkg/provider (thin)
                                        │
                          同一套代码, 只依赖 pkg/executor + pkg/model
```
