# 9Router vs Token Flux — 模型请求处理对比

> Branch: `dnode` · 2026-05-31

## 1. 请求入口与格式检测

| | 9Router | Token Flux |
|---|---|---|
| **入口** | 统一 `/v1/chat/completions` | 分散：`/v1/chat/completions`, `/v1/messages`, `/v1/responses`, `/v1/models:generateContent` |
| **格式检测** | 纯 body 检测（`contents`→Gemini, `anthropic_version`→Claude） | 路径优先，fallback body 检测（但路径始终命中→body 检测未启用） |
| **优势** | 一个端点兼容所有客户端，切换模型无需改 URL | URL 路径清晰，RESTful 规范 |
| **劣势** | 调试困难（同 URL 不同格式响应） | 客户端需知道不同路径，不够透明 |

**结论：9Router 的统一入口更符合"透明代理"初衷。** Token Flux 只需翻转优先级即可兼容。

---

## 2. 格式翻译架构

| | 9Router | Token Flux |
|---|---|---|
| **架构** | 两步翻译拼接 `source → OpenAI → target`（隐式 hub） | Hub-and-Spoke：所有格式 → OpenAI → channel adaptor 输出 |
| **中间格式** | OpenAI Chat Completions（未显声明） | OpenAI Chat Completions（显式 canon） |
| **支持格式** | 13 个：OpenAI, Claude, Gemini, Responses, Vertex, Kiro, Cursor, Codex, Ollama, Antigravity, Gemini CLI, CommandCode | 4 个：OpenAI, Claude, Gemini, Responses |
| **翻译文件数** | 12 request + 9 response = 21 个 | 4 request + 4 response = 8 个 |
| **优势** | 生态广，小众客户端也能接入 | 结构清晰，Go 静态类型安全 |
| **劣势** | JS 弱类型，翻译器维护成本高 | 格式覆盖不够广 |

**结论：9Router 格式多但维护成本高。** Token Flux 架构更干净，需要的是扩充格式覆盖。

---

## 3. 路由与供应商选择

| | 9Router | Token Flux |
|---|---|---|
| **模型识别** | `provider/model` 前缀（如 `deepseek/deepseek-v4-flash`） | 单 model_code 字段 |
| **多账号** | 按 provider 分组，round-robin + 冷却 | 轮询 + affinity 亲和 |
| **Fallback** | 三层：订阅→便宜→免费 | 单层 retry（max_retries） |
| **Combo** | ✅ 命名模型列表依次 fallback | ❌ 无 |
| **配额** | 实时 token 计数 + 成本估算 | 数据库 quota_used + 条件 UPDATE |
| **优势** | Combo 模式非常实用，一层内多个模型自动切换 | 数据库持久化配额 |
| **劣势** | 单机状态，重启丢失冷却 | 无 Combo 能力 |

**结论：9Router 的 Combo 模式是用户端最大的差异化功能。** Token Flux 应作为优先级功能加入。

---

## 4. 平台能力

| | 9Router | Token Flux |
|---|---|---|
| **计费/结算** | 本地 SQLite，无平台级结算 | ✅ PostgreSQL 双录入结算、对账 |
| **多租户** | 无，单用户工具 | ✅ 完整 user/provider 体系 |
| **Admin** | 本地 Web UI | ✅ 后台管理面板（:8082） |
| **API Key 管理** | 本地 JSON 存储 | ✅ 数据库 + AES-GCM 加密 |
| **OAuth** | ✅ 多平台 OAuth token 管理 | ❌ 仅 API Key |
| **国际化** | ❌ | ✅ 5 种语言 |
| **桌面客户端** | CLI + TUI | ✅ Tauri 桌面应用 |
| **前端** | 本地 dashboard | ✅ 全功能 SPA |
| **P2P** | ❌ 纯本地工具 | ✅ NetBird mesh 网络设计 |

**结论：Token Flux 是平台，9Router 是工具。** 平台级能力（结算、多租户、管理）是 Token Flux 的核心价值，9Router 不具备。

---

## 5. 性能与部署

| | 9Router | Token Flux |
|---|---|---|
| **语言** | Node.js (V8) | Go（编译型） |
| **启动速度** | 快 | 快 |
| **内存** | 较高（JS 运行时） | 低（Go 原生） |
| **部署** | 单机 `npx 9router` | 独立二进制 + 可选平台连接 |
| **数据库** | JSON + SQLite | PostgreSQL 16 |
| **依赖** | Node.js runtime | 无运行时依赖 |
| **扩展性** | 单机 | P2P 网络可线性扩展 |

**结论：Go 二进制无运行时依赖，对供应商更友好。** 但 JSON/SQLite 对单机更简便。

---

## 6. Token 优化

| | 9Router | Token Flux |
|---|---|---|
| **RTK Token Saver** | ✅ 压缩 diff/grep/ls 输出，省 20-40% | ❌ |
| **Caveman Mode** | ✅ 精简输出省 65% token | ❌ |
| **影响** | 对所有经过的请求生效 | 无 |

**结论：9Router 的 token 压缩是硬核差异化，** 直接降低用户成本，应该是 Token Flux 需要实现的功能。

---

## 7. 总结

| 维度 | 9Router 胜出 | Token Flux 胜出 |
|------|-------------|-----------------|
| **统一入口 + body 检测** | ✅ | 差一行代码 |
| **Combo / 多模型 fallback** | ✅ | 缺失 |
| **Token 压缩** | ✅ RTK + Caveman | 缺失 |
| **格式覆盖** | ✅ 13 种 | 4 种 |
| **OAuth** | ✅ | 缺失 |
| **平台级结算** | ❌ | ✅ 核心优势 |
| **多租户** | ❌ | ✅ |
| **Admin 管理** | ❌ | ✅ |
| **部署简单度** | ❌ 需 Node.js | ✅ Go 单二进制 |
| **P2P 网络** | ❌ | ✅ dnode 设计 |
| **国际化** | ❌ | ✅ 5 语言 |
| **架构清晰度** | 中等 | ✅ hub-and-spoke 清晰 |

### 9Router 应该搬到 Token Flux 的功能

| 功能 | 优先级 | 工作量 |
|------|--------|--------|
| **统一入口 + body 检测** | 🔴 高 | 一行改动 |
| **Combo 模型列表** | 🔴 高 | 中等 |
| **RTK Token Saver** | 🟡 中 | 高 |
| **Caveman Mode** | 🟢 低 | 低 |
| **OAuth token 管理** | 🟡 中 | 高 |
| **更多格式（Kiro, Cursor, Ollama）** | 🟢 低 | 中 |

### Token Flux 应该保留的核心优势

| 能力 | 价值 |
|------|------|
| 平台级结算/对账 | 商业模型的基础 |
| 多租户 + admin | 市场运营能力 |
| Go 单二进制部署 | 供应商零门槛接入 |
| P2P dnode 网络 | 差异化竞争 |
| AES-GCM 密钥加密 | 安全合规 |
| 国际化前端 | 全球市场 |
