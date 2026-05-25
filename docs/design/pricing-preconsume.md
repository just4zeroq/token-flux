# 定价系统 + Pre-Consume 设计

## 目标

将 ai-gateway relay 核心流程中的定价和预扣费从硬编码迁移为 DB 驱动的完整系统。

## 架构变更

```
Before:
  helpers.go (硬编码 map) → calcQuota → PreDeductQuota (只检查余额)
                                         → relay → ReportUsage (实扣)

After:
  model_pricing表 + group_ratios表 → PricingService (内存缓存)
                                     → calcQuota (model_ratio × completion_ratio × group_ratio)
                                     → PreConsume (asset-svc 预扣)
                                     → relay → PostConsume (asset-svc 实扣/退费)
```

## 新增数据模型

### ai-gateway 本地表

#### model_pricing

```sql
CREATE TABLE model_pricing (
    id SERIAL PRIMARY KEY,
    model VARCHAR(255) NOT NULL UNIQUE,      -- 模型名(精确匹配优先, 前缀匹配兜底)
    model_ratio DECIMAL(10,4) NOT NULL DEFAULT 1.0,  -- 模型倍率
    completion_ratio DECIMAL(10,4) NOT NULL DEFAULT 1.0, -- 输出倍率(相对输入)
    is_free BOOLEAN NOT NULL DEFAULT FALSE,   -- 免费模型(不计费)
    remark TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_model_pricing_model ON model_pricing(model);

INSERT INTO model_pricing (model, model_ratio, completion_ratio) VALUES
('gpt-4o-mini',    0.15,  1.0),
('gpt-4o',         5.0,   1.0),
('gpt-4-turbo',    10.0,  1.0),
('gpt-4',          30.0,  1.0),
('gpt-3.5-turbo',  1.0,   1.0),
('claude-3-opus',  15.0,  5.0),
('claude-3-sonnet',3.0,   5.0),
('claude-3-haiku', 0.25,  1.25),
('deepseek-chat',  0.5,   2.0),
('gemini',         0.5,   1.5),
('default',        10.0,  1.0);
```

**设计决策**: model_ratio 对应 new-api-main 中 $0.001/1K-token 为 1 单位的倍率。
- gpt-3.5-turbo: 1 倍 = $0.001/1K-input
- gpt-4: 30 倍 = $0.03/1K-input
- 实际费用 = (prompt_tokens × model_ratio + completion_tokens × model_ratio × completion_ratio) × group_ratio ÷ 1000

#### group_ratios

```sql
CREATE TABLE group_ratios (
    id SERIAL PRIMARY KEY,
    group_name VARCHAR(64) NOT NULL UNIQUE,
    group_ratio DECIMAL(10,4) NOT NULL DEFAULT 1.0,  -- 分组倍率(1.0=无折扣)
    remark TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO group_ratios (group_name, group_ratio) VALUES
('default', 1.0),
('vip',     0.8),
('internal', 0.5),
('free',    0.0);
```

### asset-svc 新增 RPC

#### RefundQuota

```protobuf
rpc RefundQuota(RefundQuotaReq) returns (RefundQuotaRes);

message RefundQuotaReq {
  int64 user_id = 1;
  double amount = 2;         // 退款金额(平台货币单位)
  string request_id = 3;     // 原始请求ID，用于幂等
  string remark = 4;
}
message RefundQuotaRes {
  double balance_after = 1;
}
```

asset-svc 实现:
- 原子增加余额 + 记录退款交易
- 按 request_id 幂等 (同 request_id 只退款一次)
- 无需预扣(ReportUsage 已在 relay 完成时实扣, RefundQuota 只在实扣额 > 实际消耗额时调用)

## 核心流程

### 1. 请求到达 ChatCompletions

```
Auth (TokenAuth)
  ↓
Model Rate Limit
  ↓
Parse body → ChatRequest
  ↓
Model Limits check (API Key whitelist)
  ↓
Estimate tokens (从 messages 粗略估算)
  ↓
calcQuota(model, promptEstimate, 256) → preQuota  (见公式)
  ↓
PreConsume (asset-svc ReportUsage: 预扣 preQuota)
  |  失败 → "insufficient balance"
  ↓
SelectChannel (abilities 权重随机)
  ↓
doRelay (Adaptor 模式转发)
  |  成功 → PostConsume
  |  失败 → RollbackPreConsume (asset-svc RefundQuota: 退还 preQuota)
  ↓
PostConsume:
  1. calcQuota(model, actualPrompt, actualCompletion) → actualQuota
  2. if actualQuota > preQuota:
       ReportUsage (差额补扣)
     else if actualQuota < preQuota:
       RefundQuota (退还差额, 按 request_id 幂等)
     else:
       已扣平, 无需操作
  3. 异步写 usage_records (LogQueue)
```

### 2. 定价查询 (PricingService)

```go
type PricingService struct {
    mu     sync.RWMutex
    prices map[string]*ModelPricing  // model → pricing, 含 "default"
    groups map[string]float64        // group_name → group_ratio, 含 "default"
    lastLoad time.Time
}

func (s *PricingService) GetPricing(modelName string) *ModelPricing
func (s *PricingService) GetGroupRatio(groupName string) float64
func (s *PricingService) Refresh()  // 每60秒重载

// 匹配逻辑:
// 1. 精确匹配 model_pricing.model
// 2. 前缀匹配 (model LIKE '%' || modelPricing.model || '%')
// 3. 兜底 "default"
```

### 3. 扣费公式

```
quota = ceil(
    (prompt_tokens + completion_tokens * completion_ratio)
    * model_ratio
    * group_ratio
    / 1000
)
```

单位: 1 quota ≈ $0.001 (匹配 new-api-main common.QuotaPerUnit)。

asset-svc balance 使用 DECIMAL(20,4) 平台货币单位, quota/1000 = 平台货币额。

例:
- gpt-4 (model_ratio=30), 100 prompt + 50 completion, default group (ratio=1.0)
  → (100 + 50×1) × 30 × 1.0 ÷ 1000 = 4.5 → ceil = 5 quota = $0.005

- gpt-3.5-turbo (model_ratio=1), 500 prompt + 200 completion, vip group (ratio=0.8)
  → (500 + 200×1) × 1 × 0.8 ÷ 1000 = 0.56 → ceil = 1 quota = $0.001

- claude-3-opus (model_ratio=15, completion_ratio=5), 1000 prompt + 300 completion
  → (1000 + 300×5) × 15 × 1.0 ÷ 1000 = 37.5 → ceil = 38 quota = $0.038

### 4. 预扣金额估算

```
preQuota = calcQuota(model, promptEstimate, 256)
```
promptEstimate 从 messages 字符数粗略估算(1 token ≈ 4 chars × 1.2 安全系数)。
completion 用 256 tokens 作为最低估算，确保足够覆盖短输出。

安全: preQuota 应略大于实际消耗，避免 PostConsume 二次补扣(需要额外 gRPC 调用)。

## 文件变更

### 新增

| 文件 | 内容 |
|------|------|
| `server/ai-gateway/internal/service/pricing.go` | PricingService (DB读取 + 内存缓存 + 匹配逻辑 + calcQuota) |
| `server/ai-gateway/internal/controller/admin/pricing.go` | 定价 CRUD + group_ratio CRUD |
| `server/ai-gateway/migrations/003_create_model_pricing.sql` | 建表 |
| `server/ai-gateway/migrations/004_create_group_ratios.sql` | 建表 |
| `server/proto/asset/v1/asset.proto` | RefundQuota RPC 定义 (追加到现有 proto) |

### 修改

| 文件 | 改动 |
|------|------|
| `server/ai-gateway/internal/service/relay.go` | ChatCompletions → PreConsume/PostConsume/Rollback; 移除硬编码定价; 替换 calcQuota 为 PricingService |
| `server/ai-gateway/internal/service/helpers.go` | 移除 modelPricing map, getPricing, calcQuota → 移到 pricing.go |
| `server/ai-gateway/internal/router/relay.go` | admin路由: POST/GET/PUT/DELETE /pricing + /group-ratios |
| `server/ai-gateway/internal/cmd/cmd.go` | 初始化 PricingService + 启动定时刷新 |
| `server/asset-svc/internal/service/asset.go` | 实现 RefundQuota |
| `server/asset-svc/internal/controller/asset/asset.go` | 注册 RefundQuota handler |
| `server/api/asset/v1/asset.pb.go` | 重新编译 proto |

## 实施步骤

### Step 1: Proto + asset-svc RefundQuota

1. 添加 RefundQuota RPC 到 asset.proto
2. protoc 重新生成
3. asset-svc 实现 RefundQuota (幂等, 按 request_id)

### Step 2: DB migration + PricingService

1. 创建 model_pricing 表 + 种子数据
2. 创建 group_ratios 表 + 种子数据
3. 实现 PricingService (带60s缓存刷新, 匹配逻辑)
4. PricingService.Refresh 每60秒从 DB 重载

### Step 3: 重构 relay.go

1. ChatCompletions: 用 PricingService 替换 helpers.go 的硬编码定价
2. PreConsume: 估算 tokens → calcQuota → asset-svc ReportUsage(预扣)
3. PostConsume: 实际tokens → calcQuota → asset-svc ReportUsage/RefundQuota
4. Rollback: on failure → asset-svc RefundQuota
5. 移除 helpers.go 的 modelPricing map

### Step 4: Admin CRUD

1. pricing CRUD (GET/POST/PUT/DELETE)
2. group_ratios CRUD (GET/POST/PUT/DELETE)
3. 注册到 admin 路由组

### Step 5: 验证

```
1. 查询定价:   GET  /api/v1/admin/pricing
2. 创建定价:   POST /api/v1/admin/pricing  {"model":"gpt-4o","model_ratio":5,"completion_ratio":1}
3. 无余额调用: 余额不足 → 返回 403 insufficient balance
4. 正常调用:   扣费正确, usage_records 有记录
5. 免费模型:   is_free=true → 不扣费
6. VIP group:  group_ratio=0.8 → 费用 8 折
7. 失败退款:   渠道返回500 → 预扣金额退还
```
