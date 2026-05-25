# LLM 管理、计价与统一结算设计

## 1. 背景与目标

平台采用统一 credits 资产体系：用户充值后获得 credits，credits 可用于消费不同产品，例如 LLM、MCP、Agent 等。不同产品有各自的资源形态和计价体系，但消费完成后都应进入同一套账户与结算系统。

本设计聚焦 LLM 资源管理和 LLM 计价闭环。MCP、Agent 暂不强行抽象为统一产品表，只定义边界原则：各产品独立建模、独立计价、独立记录用量，最终共用 settlement/accounting 层。

核心目标：

- 第一阶段先跑通 LLM 完整闭环：注册、充值、Virtual Key、对接 LLM API、用量计费、统一结算、积分发放。
- LLM 管理链路清晰：Provider → Channel → Model Key → Key-Model → Model Spec → Virtual Key。
- 模型基础信息共用一份，不因不同 Provider/Channel 重复维护。
- Provider 可以在全局 Channel 下配置多个真实上游 Key，每个 Key 声明支持的模型列表和额度。
- Virtual Key 是用户调用平台所有服务的统一消费凭证。
- LLM 有独立计价体系，最终折算为 credits 后进入统一交易、结算、账本系统。
- 建立完整公共交易/结算/账本底座，统一支撑充值、消费、分账、积分、退款、提现等资金动作。
- 账务统一落到 `accounts`、`transactions`、`transaction_entries`，产品侧只提供结算输入。
- MCP/Agent 当前只定义原则和边界，不进入第一阶段实现范围。

## 2. 核心概念

### 2.1 Provider / 资源提供商

Provider 是平台用户的一种角色，表示提供上游 API Key 或资源的人。

例如：张三购买了火山方舟的 API Key，并把该 Key 接入平台，张三就是 Provider。

Provider 不单独建表，使用现有 `users.role` 表达：

```text
role = 1      普通消费者
role = 10     Provider
role >= 100   Admin
```

权限语义：

```text
role >= 10    可以管理自己的 LLM Model Keys
role >= 100   可以管理全平台 LLM 资源
```

平台自营资源使用一个系统用户作为 Provider，例如 `platform` 用户。所有 `llm_model_keys.provider_user_id` 都必须非空，不使用 NULL 表示平台自营。

### 2.2 Channel / 渠道

Channel 是全局渠道字典，表示火山、OpenRouter、DeepSeek 官方、OpenAI 官方、硅基流动等接入平台或线路。

Channel 不是 Provider 私有资源。Provider 提交新 Channel 并审核通过后，该 Channel 对所有 Provider 全局可用。

一个 Channel 可以支持多个协议，例如 OpenAI-compatible、Anthropic-compatible。协议和 base_url 属于 Channel 配置，不属于 Model Key。Model Key 不允许覆盖 base_url。

### 2.3 Developer / 模型开发者

Developer 只表示“模型是谁开发的”，例如 DeepSeek、OpenAI、Anthropic。Developer 不参与平台账号、收益、Key 管理，不单独建表，作为模型基础信息字段存储在 `llm_model_specs.developer_name`。

### 2.4 Model Spec / 模型基础信息

Model Spec 是模型主数据，共用一份。例如：

```text
Developer: DeepSeek
Model: deepseek-v4-flash
Context Window: 64K
Capabilities: chat, completion
Supported Params: temperature, top_p, max_tokens, stream
```

同一个模型即使由多个 Provider 通过多个 Channel 提供，也只维护一份 Model Spec。

唯一性：

```text
UNIQUE(developer_name, model_name)
UNIQUE(model_code)
```

对外请求只使用 `model_code`，不支持 alias。

### 2.5 Model Key / 上游真实 Key

Model Key 是 Provider 在某个 Channel 下配置的真实上游 API Key。

一位 Provider 可以在同一个 Channel 下配置多个 Key。每个 Key 可以声明自己支持哪些模型，以及每个模型的额度。

Model Key 必须应用层加密后存储，不能明文保存。

### 2.6 Key-Model / Key 支持模型关系

`llm_model_key_models` 表示“一把真实 Key 支持某个模型”的供应关系。

它记录：

- 真实 Key 支持哪个 Model Spec
- 上游实际模型名 `upstream_model_name`
- 该 Key 在该模型上的额度
- Provider 分成比例
- 测试状态和运行时健康状态

### 2.7 Virtual Key / 用户消费 Key

Virtual Key 使用现有 `api_keys` 表，是用户调用平台所有服务的统一消费凭证。

Virtual Key 默认可调用所有 active 服务和模型；如果启用 `model_limits_enabled`，则只允许调用 `model_limits` 中的模型。

## 3. 总体分层

```text
Identity / Virtual Key 层
  users
  api_keys

LLM 资源层
  llm_channels
  llm_model_specs
  llm_model_prices
  llm_model_keys
  llm_model_key_models

LLM 用量层
  llm_usage_records

公共交易、结算、账本层
  accounts
  transactions
  transaction_entries
  settlement_records
  ledger_snapshots
```

MCP、Agent 后续独立建模：

```text
mcp_* tables
agent_* tables
```

但最终都输出统一的结算输入：

```text
consumer_user_id
provider_user_id or developer_user_id
cost_credits
provider_revenue_credits
commission_credits
ref_type
ref_id
```

并写入同一套交易、结算、账本系统。

## 4. 表结构设计

### 4.1 `users`

复用现有用户表。

新增语义：

```text
role = 1      consumer
role = 10     provider
role >= 100   admin
```

Provider 只能管理自己的 Model Keys。Admin 可以管理全部。

### 4.2 `api_keys`

复用现有 API Key 表作为 Virtual Key。

建议新增字段：

```sql
disabled_reason VARCHAR(64) NOT NULL DEFAULT ''
```

用途：

- `disabled_reason = 'overdraft'` 表示因透支自动禁用。
- 用户充值并补正余额后，只自动恢复 `disabled_reason = 'overdraft'` 的 Key。
- 不自动恢复用户主动禁用、管理员封禁、风控禁用的 Key。

### 4.3 `llm_channels`

全局渠道字典。

```sql
CREATE TABLE llm_channels (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    protocols_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_by_user_id BIGINT REFERENCES users(id),
    reviewed_by_user_id BIGINT REFERENCES users(id),
    reviewed_at TIMESTAMPTZ NULL,
    review_note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

`protocols_json` 示例：

```json
{
  "openai-compatible": {
    "base_url": "https://ark.cn-beijing.volces.com/api/v3",
    "enabled": true
  },
  "anthropic-compatible": {
    "base_url": "https://ark.cn-beijing.volces.com/api/anthropic",
    "enabled": true
  }
}
```

约束：

- 协议只允许平台内置枚举。
- MVP 只实现 `openai-compatible` adapter。
- `base_url` 按协议存储在 `protocols_json`。
- Model Key 不允许覆盖 base_url。

状态：

```text
pending
active
disabled
rejected
```

### 4.4 `llm_model_specs`

LLM 模型基础信息，共用一份。

```sql
CREATE TABLE llm_model_specs (
    id BIGSERIAL PRIMARY KEY,
    developer_name VARCHAR(255) NOT NULL,
    model_name VARCHAR(255) NOT NULL,
    model_code VARCHAR(255) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    model_family VARCHAR(128) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    capabilities_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    context_window INT NOT NULL DEFAULT 0,
    max_input_tokens INT NOT NULL DEFAULT 0,
    max_output_tokens INT NOT NULL DEFAULT 0,
    supports_stream BOOLEAN NOT NULL DEFAULT FALSE,
    supports_tools BOOLEAN NOT NULL DEFAULT FALSE,
    supports_vision BOOLEAN NOT NULL DEFAULT FALSE,
    supports_json_mode BOOLEAN NOT NULL DEFAULT FALSE,
    supports_reasoning BOOLEAN NOT NULL DEFAULT FALSE,
    supports_logprobs BOOLEAN NOT NULL DEFAULT FALSE,
    supported_params_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    default_params_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    param_limits_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_type VARCHAR(32) NOT NULL DEFAULT 'admin',
    created_by_user_id BIGINT REFERENCES users(id),
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    reviewed_by_user_id BIGINT REFERENCES users(id),
    reviewed_at TIMESTAMPTZ NULL,
    review_note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(developer_name, model_name)
);
```

`model_code`：

- 默认自动生成：`normalized(developer_name) + '/' + normalized(model_name)`。
- Admin 可修改。
- 对外调用只接受 `model_code`。
- 不支持 alias。

`capabilities_json` 示例：

```json
["chat", "completion", "embedding"]
```

模型参数能力使用“常用能力列 + JSON 扩展”：

- 常用能力列用于筛选和展示。
- `supported_params_json`、`default_params_json`、`param_limits_json` 用于灵活描述参数能力。

MVP 请求参数校验策略：

- 强校验：model 必填且存在、Virtual Key 有权限、请求体基本结构、`max_tokens` 不超过已知上限。
- 复杂参数先透传给上游。

### 4.5 `llm_model_prices`

LLM 专属价格表，不复用当前 `pricing_rules`。

LLM 输入侧按缓存命中/非命中拆分计费：

```text
input_tokens = cache_hit_tokens + cache_miss_tokens
```

不再对 `input_tokens` 单独计费。

```sql
CREATE TABLE llm_model_prices (
    id BIGSERIAL PRIMARY KEY,
    model_spec_id BIGINT NOT NULL REFERENCES llm_model_specs(id),
    capability VARCHAR(64) NOT NULL,
    currency_asset VARCHAR(32) NOT NULL DEFAULT 'credits',
    cache_hit_price_per_1k BIGINT NOT NULL DEFAULT 0,
    cache_miss_price_per_1k BIGINT NOT NULL DEFAULT 0,
    output_price_per_1k BIGINT NOT NULL DEFAULT 0,
    effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to TIMESTAMPTZ NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_llm_model_prices_model ON llm_model_prices(model_spec_id, capability, status);
```

计费公式：

```text
cost_credits =
  cache_hit_tokens  * cache_hit_price_per_1k  / 1000
+ cache_miss_tokens * cache_miss_price_per_1k / 1000
+ output_tokens     * output_price_per_1k     / 1000
```

如果上游不返回缓存明细：

```text
cache_hit_tokens = 0
cache_miss_tokens = input_tokens
```

Embedding 可复用同一价格结构：

```text
cache_hit_price_per_1k = 0
cache_miss_price_per_1k = embedding input price
output_price_per_1k = 0
```

### 4.6 `llm_model_keys`

Provider 在某个 Channel 下配置的真实上游 API Key。

```sql
CREATE TABLE llm_model_keys (
    id BIGSERIAL PRIMARY KEY,
    provider_user_id BIGINT NOT NULL REFERENCES users(id),
    channel_id BIGINT NOT NULL REFERENCES llm_channels(id),
    name VARCHAR(255) NOT NULL DEFAULT '',
    key_encrypted TEXT NOT NULL,
    key_masked VARCHAR(64) NOT NULL DEFAULT '',
    quota_limit_credits BIGINT NOT NULL DEFAULT 0,
    quota_used_credits BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    last_test_at TIMESTAMPTZ NULL,
    last_test_status VARCHAR(32) NOT NULL DEFAULT '',
    last_test_error TEXT NOT NULL DEFAULT '',
    test_attempts INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_llm_model_keys_provider ON llm_model_keys(provider_user_id);
CREATE INDEX idx_llm_model_keys_channel ON llm_model_keys(channel_id);
CREATE INDEX idx_llm_model_keys_status ON llm_model_keys(status);
```

约束：

- 不存 protocol。
- 不存 base_url_override。
- protocol/base_url 来自 Channel。
- quota 只做资源消耗统计，不扣 Provider 账户余额。

Key 加密：

- AES-GCM。
- 密钥来自 `MODEL_KEY_ENCRYPTION_KEY` 或配置项。
- 建议 32-byte key，base64 编码。
- 存储格式：`v1:<base64(nonce)>:<base64(ciphertext)>`。
- API 返回时只返回 `key_masked`。
- 不在日志打印 raw key。

Key 轮换：

- 新建一条 Model Key。
- 旧 Key 置为 disabled。
- 新 Key 额度重新填写，不迁移旧 Key 剩余额度。
- 新 Key 需要重新自动测试后才能 active。

状态：

```text
pending
testing
active
test_failed
disabled
rejected
```

### 4.7 `llm_model_key_models`

一把真实 Key 支持的模型列表和单模型额度。

```sql
CREATE TABLE llm_model_key_models (
    id BIGSERIAL PRIMARY KEY,
    model_key_id BIGINT NOT NULL REFERENCES llm_model_keys(id),
    model_spec_id BIGINT NOT NULL REFERENCES llm_model_specs(id),
    upstream_model_name VARCHAR(255) NOT NULL,
    quota_limit_credits BIGINT NOT NULL DEFAULT 0,
    quota_used_credits BIGINT NOT NULL DEFAULT 0,
    provider_share_bps INT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'pending_test',
    last_test_at TIMESTAMPTZ NULL,
    last_test_status VARCHAR(32) NOT NULL DEFAULT '',
    last_test_error TEXT NOT NULL DEFAULT '',
    test_attempts INT NOT NULL DEFAULT 0,
    consecutive_failures INT NOT NULL DEFAULT 0,
    last_error_code VARCHAR(64) NOT NULL DEFAULT '',
    last_error_message TEXT NOT NULL DEFAULT '',
    last_error_at TIMESTAMPTZ NULL,
    last_success_at TIMESTAMPTZ NULL,
    next_test_at TIMESTAMPTZ NULL,
    priority INT NOT NULL DEFAULT 0,
    weight INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(model_key_id, model_spec_id, upstream_model_name)
);

CREATE INDEX idx_llm_key_models_model ON llm_model_key_models(model_spec_id, status);
CREATE INDEX idx_llm_key_models_key ON llm_model_key_models(model_key_id);
CREATE INDEX idx_llm_key_models_next_test ON llm_model_key_models(status, next_test_at);
```

状态：

```text
pending_model_review
pending_test
testing
active
test_failed
disabled
rejected
```

`upstream_model_name` 是实际请求渠道时传给上游的模型名。它可能不同于平台 `model_code` 或 `model_name`，例如火山 endpoint、Azure deployment name。

配额：

- `llm_model_keys` 记录 Key 总额度。
- `llm_model_key_models` 记录该 Key 在某个模型上的单模型额度。
- 两者都使用 credits。
- Provider 自填额度，平台只做可用性检测，不审核额度真实性。

Provider 分成：

```text
provider_revenue_credits = cost_credits * provider_share_bps / 10000
commission_credits = cost_credits - provider_revenue_credits
```

收益进入 Provider 的 credits 账户。

### 4.8 `llm_usage_records`

LLM 专属用量记录表。

不再把 LLM/MCP/Agent 强塞进一个通用 usage 表。LLM 用量独立记录，结算时通过 `ref_type/ref_id` 接入统一账务。

```sql
CREATE TABLE llm_usage_records (
    id BIGSERIAL PRIMARY KEY,
    consumer_user_id BIGINT NOT NULL REFERENCES users(id),
    virtual_key_id BIGINT REFERENCES api_keys(id),
    model_spec_id BIGINT NOT NULL REFERENCES llm_model_specs(id),
    model_key_id BIGINT NOT NULL REFERENCES llm_model_keys(id),
    key_model_id BIGINT NOT NULL REFERENCES llm_model_key_models(id),
    provider_user_id BIGINT NOT NULL REFERENCES users(id),
    channel_id BIGINT NOT NULL REFERENCES llm_channels(id),
    request_id VARCHAR(128) NOT NULL DEFAULT '',
    external_request_id VARCHAR(128) NOT NULL DEFAULT '',
    capability VARCHAR(64) NOT NULL,
    is_stream BOOLEAN NOT NULL DEFAULT FALSE,
    input_tokens INT NOT NULL DEFAULT 0,
    cache_hit_tokens INT NOT NULL DEFAULT 0,
    cache_miss_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    total_tokens INT NOT NULL DEFAULT 0,
    cost_credits BIGINT NOT NULL DEFAULT 0,
    provider_revenue_credits BIGINT NOT NULL DEFAULT 0,
    commission_credits BIGINT NOT NULL DEFAULT 0,
    latency_ms INT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'success',
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_llm_usage_consumer_created ON llm_usage_records(consumer_user_id, created_at);
CREATE INDEX idx_llm_usage_provider_created ON llm_usage_records(provider_user_id, created_at);
CREATE INDEX idx_llm_usage_model_created ON llm_usage_records(model_spec_id, created_at);
CREATE INDEX idx_llm_usage_key_model_created ON llm_usage_records(key_model_id, created_at);
CREATE INDEX idx_llm_usage_request_id ON llm_usage_records(request_id);
```

## 5. 自动检测与运行时健康

Provider 创建 Model Key 和 Key-Model 后，接口立即返回 pending/testing 状态，系统异步检测。

检测粒度：每条 `llm_model_key_models` 单独检测。

MVP 检测只实现 OpenAI-compatible adapter：

```text
POST {channel.protocols_json["openai-compatible"].base_url}/chat/completions
Authorization: Bearer {decrypted_model_key}
model: key_model.upstream_model_name
messages: [{role: "user", content: "ping"}]
max_tokens: 1
```

如果 Model Spec 仍是 pending：

```text
key_model.status = pending_model_review
```

模型审核 active 后，自动触发相关 key_model 测试。

测试成功：

```text
key_model.status = active
```

测试失败：

```text
key_model.status = test_failed
next_test_at = now + backoff_interval
```

运行时失败处理：

- 调用成功：`consecutive_failures = 0`，更新 `last_success_at`。
- 调用失败：`consecutive_failures += 1`，记录 `last_error_*`。
- 连续失败达到阈值，例如 3 次：`status = test_failed`。

恢复方式：

- Provider/Admin 手动重新测试。
- 系统定时扫描 `status = test_failed AND next_test_at <= now` 自动重试。

## 6. 调度策略

MVP 使用随机调度。

候选条件：

```text
model_spec.status = active
channel.status = active
channel.protocols_json 支持当前接口所需协议
model_key.status = active
key_model.status = active
model_key.quota_used_credits < model_key.quota_limit_credits 或 quota_limit_credits = 0
key_model.quota_used_credits < key_model.quota_limit_credits 或 quota_limit_credits = 0
```

`priority` 和 `weight` 先作为预留字段，MVP 不使用。后续可升级为：

- weighted random
- priority failover
- cost-aware routing
- health-aware routing
- dynamic score routing

## 7. 对外 LLM 接口

MVP 支持 OpenAI-compatible 风格接口：

```text
GET  /v1/models
POST /v1/chat/completions
POST /v1/completions
POST /v1/embeddings
```

`GET /v1/models` 对普通调用方只返回当前 Virtual Key 实际可调用模型：

```text
model_spec.status = active
至少一个 active key_model 可用
channel 支持对应协议
key/model 配额未耗尽
Virtual Key 未启用模型限制，或 model_code 命中 model_limits
```

管理端可查看全部模型基础信息和 pending/testing/failed 状态。

## 8. 余额检查与透支处理

不做预扣。

请求前只检查：

```text
consumer credits balance > 0
api_key.status = active
```

响应后按实际 usage 实扣。

如果扣费后用户 credits 余额小于 0：

```sql
UPDATE api_keys
SET status = disabled,
    disabled_reason = 'overdraft'
WHERE user_id = ?;
```

用户充值后，如果余额恢复为正：

```sql
UPDATE api_keys
SET status = active,
    disabled_reason = ''
WHERE user_id = ?
  AND disabled_reason = 'overdraft';
```

## 9. 公共交易、结算、账本系统

平台需要一套完整的公共交易、结算、账本系统。LLM、MCP、Agent 等产品只负责各自的计价和用量归集，最终都向公共账本提交标准化结算输入。

### 9.1 职责边界

产品侧负责：

```text
用量采集
产品内计价
计算 cost_credits
计算 provider_revenue_credits / commission_credits
计算 points_to_consumer / points_to_provider
生成 ref_type / ref_id
```

账本侧负责：

```text
账户余额维护
交易原子性
复式分录
资金守恒校验
幂等控制
结算状态追踪
退款/冲正
提现出账
对账快照
审计追踪
```

### 9.2 账户体系

继续复用 `accounts`，但需要把它明确为多资产账本账户。

```text
owner_type: user / platform / system
owner_id: 用户ID或系统ID
asset: credits / balance / points
balance_micro: 余额，micro 单位
version: 乐观锁版本
```

核心资产：

```text
credits   用户充值后获得的统一消费资产
points    积分资产，用于运营激励
balance   后续提现或法币余额资产，第一阶段可暂不启用
```

第一阶段 LLM 闭环主要使用：

```text
consumer credits
provider credits
platform credits
consumer points
provider points
```

### 9.3 交易主表

继续复用 `transactions`，但语义上作为所有资金动作的交易主表。

建议交易类型：

```text
recharge              充值换取 credits
llm_usage_settle      LLM 消费结算
mcp_usage_settle      MCP 消费结算，预留
agent_usage_settle    Agent 消费结算，预留
points_grant          积分发放
refund                退款
reversal              冲正
withdraw              提现
adjustment            管理调整
```

每笔交易必须有：

```text
tx_type
ref_type
ref_id
created_at
```

`ref_type/ref_id` 指向产品用量、充值订单、提现单、退款单等业务来源。

### 9.4 复式分录

继续复用 `transaction_entries`，每笔交易至少包含一条或多条分录。

每条分录记录：

```text
account_id
delta_micro
balance_after_micro
```

账务规则：

- 同一笔交易内，业务要求守恒的资产必须借贷平衡。
- `credits` 消费结算中，消费者扣减额应等于 Provider 收益加平台佣金。
- 积分发放可以由 system/platform points 账户支出，或作为运营发行交易入账，具体规则可在积分体系中继续细化。

LLM credits 结算分录：

```text
consumer credits    -cost_credits
provider credits    +provider_revenue_credits
platform credits    +commission_credits
```

约束：

```text
cost_credits = provider_revenue_credits + commission_credits
```

LLM 积分发放分录：

```text
consumer points    +points_to_consumer
provider points    +points_to_provider
```

如果采用平台积分池，则对应增加：

```text
platform/system points    -(points_to_consumer + points_to_provider)
```

### 9.5 结算记录

建议新增 `settlement_records`，用于记录产品用量到交易的结算状态，避免产品表和账本表强耦合。

```sql
CREATE TABLE settlement_records (
    id BIGSERIAL PRIMARY KEY,
    product_type VARCHAR(32) NOT NULL,
    ref_type VARCHAR(64) NOT NULL,
    ref_id BIGINT NOT NULL,
    consumer_user_id BIGINT NOT NULL REFERENCES users(id),
    provider_user_id BIGINT REFERENCES users(id),
    cost_credits BIGINT NOT NULL DEFAULT 0,
    provider_revenue_credits BIGINT NOT NULL DEFAULT 0,
    commission_credits BIGINT NOT NULL DEFAULT 0,
    points_to_consumer BIGINT NOT NULL DEFAULT 0,
    points_to_provider BIGINT NOT NULL DEFAULT 0,
    transaction_id BIGINT REFERENCES transactions(id),
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at TIMESTAMPTZ NULL,
    UNIQUE(ref_type, ref_id)
);

CREATE INDEX idx_settlement_records_consumer ON settlement_records(consumer_user_id, created_at);
CREATE INDEX idx_settlement_records_provider ON settlement_records(provider_user_id, created_at);
CREATE INDEX idx_settlement_records_status ON settlement_records(status, created_at);
```

状态：

```text
pending
settled
failed
reversed
refunded
```

LLM 调用完成后先写 `llm_usage_records`，再创建或提交 `settlement_records`，最终由账本服务生成 `transactions` 和 `transaction_entries`。

### 9.6 幂等与原子性

所有结算必须幂等。

幂等键：

```text
UNIQUE(ref_type, ref_id)
```

同一条 LLM 用量记录只能结算一次。重试结算时，如果 `settlement_records` 已经是 `settled`，直接返回已结算结果。

账本写入必须在数据库事务中完成：

```text
创建 transactions
更新 accounts 余额
写 transaction_entries
更新 settlement_records.status = settled
```

账户余额更新使用 `version` 乐观锁或行级锁，避免并发扣款造成余额错误。

### 9.7 充值交易

用户充值后获得 credits。

充值业务可以先由现有钱包或订单系统产生充值确认，最终进入账本：

```text
tx_type = recharge
ref_type = recharge_order / chain_deposit
ref_id = 对应业务ID
```

分录示例：

```text
user credits       +recharge_credits
platform credits   -recharge_credits
```

如果暂不维护平台 credits 发行池，也可以先只给用户入账，但长期应保留平台/system 发行账户，便于审计 credits 发行量。

### 9.8 消费结算

LLM、MCP、Agent 各自算出 `cost_credits` 后，统一走消费结算。

LLM：

```text
tx_type = llm_usage_settle
ref_type = llm_usage_record
ref_id = llm_usage_records.id
```

MCP/Agent 后续类似：

```text
tx_type = mcp_usage_settle
ref_type = mcp_usage_record

tx_type = agent_usage_settle
ref_type = agent_usage_record
```

### 9.9 退款与冲正

退款和冲正不能直接修改历史交易或分录，只能新增反向交易。

退款：

```text
tx_type = refund
ref_type = 原业务来源或 refund_order
```

冲正：

```text
tx_type = reversal
ref_type = 原 transaction
```

反向分录应清晰对应原始交易，便于审计。

### 9.10 提现

第一阶段 Provider 收益进入 credits。提现规则可以后续单独设计，但账本需要预留提现交易类型。

长期可支持：

```text
credits → balance → withdraw
```

提现交易：

```text
tx_type = withdraw
ref_type = withdraw_request
ref_id = withdraw_requests.id
```

提现只影响账本，不应直接绕过 `transactions`。

### 9.11 账本快照与对账

建议新增 `ledger_snapshots` 用于定期对账和快速查询。

```sql
CREATE TABLE ledger_snapshots (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id),
    asset VARCHAR(32) NOT NULL,
    balance_micro BIGINT NOT NULL,
    snapshot_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ledger_snapshots_account_time ON ledger_snapshots(account_id, snapshot_at);
```

快照不替代交易分录，交易分录仍是账本事实来源。快照只用于：

- 日终对账
- 快速报表
- 异常排查
- 重算校验

### 9.12 透支处理

不做预扣。

请求前只检查：

```text
consumer credits balance > 0
```

响应后实扣。如果扣费后余额小于 0：

```text
禁用该用户所有 Virtual Keys
api_keys.disabled_reason = 'overdraft'
```

用户充值并余额恢复为正后，只恢复 `disabled_reason = 'overdraft'` 的 Virtual Keys。

## 10. 与现有表的关系

### 10.1 `catalog_items`、`pricing_rules`

LLM 暂不复用 `catalog_items` 和 `pricing_rules`。LLM 有独立的模型、价格、Key、用量表。

MCP、Agent 后续也各自设计计价体系。

### 10.2 `usage_records`

现有 `usage_records` 可以保留给旧接口或通用统计。新 LLM 链路使用 `llm_usage_records`。

### 10.3 `upstream_channels`

当前 `upstream_channels` 混合了 channel、provider、base_url、key、catalog item 绑定等职责，不适合新的 LLM 模型。

后续应迁移为：

```text
llm_channels
llm_model_keys
llm_model_key_models
```

## 11. LLM 调用链路

```text
用户请求 /v1/chat/completions
  model = deepseek/deepseek-v4-flash
  Authorization = Bearer virtual_key

1. 校验 Virtual Key。
2. 检查用户 credits balance > 0。
3. 通过 model_code 查询 llm_model_specs。
4. 检查 Virtual Key 模型限制。
5. 查询当前有效 llm_model_prices。
6. 找 active key_model 候选。
7. 随机选择一个 key_model。
8. 解密 model_key。
9. 从 channel.protocols_json 取当前协议 base_url。
10. 使用 key_model.upstream_model_name 请求上游。
11. 归一化 usage：input/cache_hit/cache_miss/output tokens。
12. 按 llm_model_prices 计算 cost_credits。
13. 按 provider_share_bps 计算 Provider 收益和平台佣金。
14. 写 llm_usage_records。
15. 创建 settlement_records。
16. 账本服务在事务内写 transactions / transaction_entries 并更新 settlement_records。
17. 更新 model_key 和 key_model 的 quota_used_credits。
18. 如果用户余额为负，禁用该用户所有 Virtual Keys。
19. 返回 OpenAI-compatible 响应。
```

## 12. MCP / Agent 后续设计原则

MCP 和 Agent 不与 LLM 强行共用产品表。

原则：

```text
统一充值资产：credits
产品计价分开：llm_model_prices / mcp_prices / agent_prices
产品用量分开：llm_usage_records / mcp_usage_records / agent_usage_records
统一结算分录：accounts / transactions / transaction_entries
```

MCP 可能按以下维度计价：

- tool call count
- tool execution duration
- data transfer size
- external API cost

Agent 可能按以下维度计价：

- run count
- step count
- duration
- token usage
- tool usage

这些不在本设计中展开，避免过早抽象。

## 13. 未纳入本阶段的内容

以下能力暂不作为 MVP 必须项：

- 多协议 adapter 全量实现。
- 动态成本最优调度。
- Provider 额度真实性校验。
- Key version 表。
- LLM/MCP/Agent 统一商品市场。
- Provider credits 提现规则。
- 严格参数级校验。
- 完整风控和反欺诈。

这些都可以在当前表结构基础上迭代。