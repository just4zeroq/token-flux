# new-api-main → ai-platform 迁移报告

## 一、数据模型对比

### 1.1 new-api-main（单体，~20 表）

| 表 | 核心字段 | 功能 |
|---|---|---|
| `users` | quota(int), used_quota, group, setting(JSON), role, access_token, stripe_customer | 用户+余额+设置 |
| `tokens` | remain_quota(int), unlimited_quota, used_quota, model_limits, allow_ips, group, cross_group_retry | API 密钥+额度限制 |
| `channels` | type, key, weight, base_url, models, group, model_mapping, status_code_mapping, priority, auto_ban, balance, test_model, response_time | 渠道+上游状态 |
| `abilities` | group+model+channel_id (复合PK), enabled, priority, weight, tag | 模型路由 |
| `logs` | type(1-6), user_id, token_name, model_name, quota, prompt_tokens, completion_tokens, use_time, is_stream, channel_id, ip, request_id | 全量操作日志 |
| `options` | key(PK), value | 系统配置 KV |
| `pricings` | model_ratio, completion_ratio, cache_ratio, enable_groups, supported_endpoint_types | 模型定价 |
| `redemptions` | key, quota, count, used_user_id, expired_time | 兑换码 |
| `top_ups` | amount, money, trade_no, payment_method, payment_provider, status | 充值订单 |
| `checkins` | user_id+date, quota_awarded | 签到 |
| `quota_data` | user_id, model_name, created_at(hour), token_used, count, quota | 看板聚合 |
| `subscriptions` | plan, duration, reset_period, quota_per_period | 订阅计划 |

### 1.2 ai-platform（3 服务分库，10 表）

| 服务 | 表 | 核心字段 |
|---|---|---|
| user-svc | `users` | group_name(varchar), role, status |
| ] | `api_keys` | key, status, model_limits, group, expired_time |
| asset-svc | `balances` | balance(decimal), total_recharged, total_consumed, version(乐观锁) |
| ] | `transactions` | amount, balance_before, balance_after, reference_type |
| ] | `usage_records` | prompt_tokens, completion_tokens, quota |
| ] | `orders` | product_id, total_amount, status |
| ai-gateway | `channels` | type, key, weight, models, model_mapping, priority, base_url |
| ] | `abilities` | group_name, model, channel_id, enabled, priority, weight |
| ] | `model_pricing` | model_ratio(Decimal), completion_ratio(Decimal), is_free |
| ] | `group_ratios` | group_ratio(Decimal) |
| ] | `usage_records` | (同上结构，本地日志队列落库) |

---

## 二、功能差距矩阵

### P0 — 必须迁移（影响核心可用性）

| 功能 | new-api | ai-platform | 差距 |
|------|---------|-------------|------|
| 双层额度校验 | user.quota + token.remain_quota 两层校验 | 只有 asset-svc balance | Token 级别额度控制缺失。PreConsume 只查 user balance |
| 选项系统 KV | options 表管理所有设置（注册/OAuth/SMTP/支付/费率开关） | 硬编码 | 所有开关必须改代码部署 |
| 预扣+超时退还 | PreConsumedQuota=500，基于 token.remain_quota | ✅ 已实现 pre-consume 完整流程（Quota = ceil((prompt + completion × ratio) × model_ratio × group_ratio / 1000)） |
| 渠道自动禁言 | auto_ban+失败计数阈值 | ✅ 已实现（ChannelFailureTrack + RecordFailure + ShouldDisable） |
| 渠道自动启用 | AutomaticEnableChannelEnabled+定时恢复 | ❌ 缺失 |
| 用户分组 | group 字段支持 | ✅ group_ratios + abilities group_name |
| 模型定价匹配 | 精确→前缀→default | ✅ 完全一致 |
| 日志系统 | logs 表 + LogType(consume/topup/manage/error/refund) | ✅ LogQ 异步写入 usage_records，但缺少 type 分类和多维度索引 |
| 异步日志写入 | gopool 异步落库 | ✅ LogQueue batchInsert |
| 请求 IP 记录 | logs.ip | ❌ usage_records 无 ip 字段 |
| 流式/非流式区分 | logs.is_stream | ❌ 未记录 |
| 请求耗时记录 | logs.use_time | ❌ 未记录 |

### P1 — 重要（增强运营管理）

| 功能 | new-api | ai-platform | 差距 |
|------|---------|-------------|------|
| 兑换码系统 | redemptions 表，批量生成+用户兑换 | ❌ 缺失，需市场/营销功能 |
| 签到系统 | checkins 表，每日随机额度 | ❌ 缺失 |
| 分组跨组重试 | cross_group_retry + auto 分组 | ❌ 缺失 |
| 定价缓存比率 | cache_ratio + create_cache_ratio（prompt cache 折扣） | ❌ 缺失，新功能但重要 |
| 渠道上游余额 | channel.balance + balance_updated_time | ❌ 缺失 |
| 渠道连通性测试 | TestModel + TestTime + ResponseTime | ✅ admin API 有 /test 端点 |
| 模型白名单/黑名单 | token.model_limits + model_limits_enabled | ✅ 已实现 |
| IP 白名单 | token.allow_ips | ❌ 缺失 |
| 运营看板 | quota_data 表每小时聚合 | ❌ 缺失 |

### P2 — 扩展（更多 Provider/Endpoint）

| 功能 | new-api | ai-platform | 差距 |
|------|---------|-------------|------|
| Azure 适配器 | 支持 openai_organization + Azure 版本配置 | ✅ AzureAdaptor 已实现 |
| Claude 适配器 | ✅ | ✅ ClaudeAdaptor 已实现 |
| Gemini 适配器 | ✅ | ✅ GeminiAdaptor 已实现 |
| DeepSeek 适配器 | ✅ | ✅ DeepSeekAdaptor 已实现 |
| 文生图 | DALL-E / Stable Diffusion 转发 | ❌ 缺失 |
| 文生音频 | TTS / STT 转发 | ❌ 缺失 |
| Embedding | text-embedding-3-small 等 | ❌ 缺失 |
| Rerank | Cohere / Jina rerank | ❌ 缺失 |
| Realtime API | WebSocket 实时对话 | ❌ 缺失 |
| Midjourney | Discord 中转 | ❌ 缺失 |
| 模型能力分类 | supported_endpoint_types（chat/embedding/image/audio/rerank/realtime） | ❌ 缺失 |
| StatusCodeMapping | 渠道级状态码映射 | ❌ 缺失 |

### P3 — 非核心（用户增长/支付/前端）

| 功能 | new-api | ai-platform | 差距 |
|------|---------|-------------|------|
| 多渠道支付 | Epay / Stripe / Waffo / Creem | ❌ 缺失（示例用） |
| OAuth 登录 | GitHub / Linux DO / Telegram / WeChat / OIDC | ❌ 缺失 |
| Passkey 登录 | WebAuthn 无密码登录 | ❌ 缺失 |
| 邮件验证 | SMTP + 邮箱验证 | ❌ 缺失 |
| 订阅计划 | subscriptions 表定期重置额度 | ❌ 缺失 |
| 数据导出 | DataExportInterval + quota_data | ❌ 缺失 |
| 系统监控 | system_monitor + perf_metrics | ❌ 缺失 |
| 前端 Playground | web 调试台 | ❌ 缺失 |

---

## 三、架构差异

| 维度 | new-api-main | ai-platform |
|------|-------------|-------------|
| 架构 | 单体 Go (Gin+GORM) | 微服务 GoFrame (gRPC) |
| 数据库 | 单库 SQLite/MySQL/PostgreSQL | 3 分库 PostgreSQL（user_svc / asset_svc / ai_gateway） |
| 计费单位 | Quota = int，1 单位 ~ $0.002 | Quota = int64，1 单位 = $0.001 |
| 租户隔离 | 单租户+分组 | 多租户（tenant_id）+分组 |
| 配置管理 | options KV + init.go OprionMap | config.yaml + gRPC |
| 缓存策略 | Redis | 无 Redis，全内存 + DB 轮询 |
| 日志模式 | logs 表直接 INSERT | async batch insert via LogQueue |
| 权限分层 | user.role (admin/common) | user.role + tenant_id |
| 部署方式 | 单体 Docker | 微服务 Docker Compose |

---

## 四、已迁移功能

### 核心中继 ✅
- 渠道管理 CRUD + 权重随机选择
- Abilities 模型路由 + 优先级
- Pre-consume → Relay → Post-consume 计费流程
- Retry 机制（最多 3 次）
- 错误分类 + 自动禁言
- 定价缓存（PricingService 60s 刷新）
- 模型权限控制（model_limits 黑白名单）

### Adaptor ✅
- OpenAI / Azure / Claude / DeepSeek / Gemini

### 管理 API ✅
- 渠道 CRUD + 连通性测试
- Abilities CRUD
- 用户管理 + 余额充值
- 使用记录查询
- API Key 管理

---

## 五、迁移优先级

### Phase 1 — 补全核心计费（2-3 天）

| 序号 | 任务 | 影响 | 文件 |
|------|------|------|------|
| 1.1 | 双层额度: token 级 remain_quota | 多租户额度隔离 | user-svc api_keys 加 remain_quota, unlimited_quota 字段；ai-gateway PreConsume 先 check token 额度再 check balance |
| 1.2 | 选项系统 KV | 运营配置热更新 | ai-gateway 加 options 表 + OptionsService + admin API |
| 1.3 | 日志增强: ip, is_stream, use_time, type | 审计 | usage_records 加列 + LogEntry 扩展 |
| 1.4 | IP 白名单 | 安全 | token_auth 加 IP check 中间件 |

### Phase 2 — 运营功能（3-5 天）

| 序号 | 任务 | 影响 | 文件 |
|------|------|------|------|
| 2.1 | 渠道自动启用 | 运营效率 | ChannelFailureTrack 加定时恢复 |
| 2.2 | 兑换码系统 | 市场推广 | asset-svc 加 redemptions 表 + redeem API |
| 2.3 | 签到系统 | 用户活跃 | user-svc 加 checkins 表 + checkin API |
| 2.4 | 看板聚合 | 数据驱动 | ai-gateway 加 quota_data 表 + 定时聚合 |
| 2.5 | 缓存定价比率 | 成本优化 | model_pricing 加 cache_ratio + create_cache_ratio |

### Phase 3 — 扩展 Provider（3-5 天）

| 序号 | 任务 | 影响 | 文件 |
|------|------|------|------|
| 3.1 | 模型能力分类 | 路由精准度 | 加 endpoint_types 枚举 + channel 能力标签 |
| 3.2 | Embedding + Rerank Adaptor | 功能覆盖 | 新 adaptor 文件 + DoResponse JSON 解析 |
| 3.3 | 文生图 Adaptor | 功能覆盖 | DALL-E / Stable Diffusion 适配 |
| 3.4 | StatusCodeMapping | 容错 | channel 配置预处理 |

### Phase 4 — 非核心（按需）

- 支付系统集成（Stripe/Waffo/Epay）
- OAuth 登录
- 前端 Playground
- 系统监控

---

## 六、关键数据模型对齐方案

### Token 级额度（Phase 1.1）

```sql
-- user-svc: api_keys 加字段
ALTER TABLE api_keys ADD COLUMN remain_quota BIGINT NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN used_quota BIGINT NOT NULL DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN unlimited_quota BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE api_keys ADD COLUMN allow_ips TEXT NOT NULL DEFAULT '';

-- ai-gateway: usage_records 加字段
ALTER TABLE usage_records ADD COLUMN ip VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE usage_records ADD COLUMN is_stream BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE usage_records ADD COLUMN use_time INT NOT NULL DEFAULT 0;  -- ms
ALTER TABLE usage_records ADD COLUMN log_type INT NOT NULL DEFAULT 2; -- 2=consume
```

### Options 系统（Phase 1.2）

```sql
-- ai-gateway 建表
CREATE TABLE options (
    key VARCHAR(255) PRIMARY KEY,
    value TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

---

## 七、总结

**当前迁移完成度：约 40%**（按核心功能覆盖计）

已完成的核心链路：中继转发 ✅ → 计费 ✅ → 日志 ✅ → 渠道管理 ✅ → 适配层 ✅

未完成的关键短板：
1. **双层额度** — Token 级 remain_quota，无此功能无法按 API Key 限流
2. **选项系统** — 所有开关硬编码，部署成本高
3. **IP 白名单** — 安全合规需求
4. **兑换码** — 缺少用户转化工具

建议 Phase 1 优先完成双层额度和 Options 系统（2-3 天），使计费系统与 new-api-main 功能对等。
