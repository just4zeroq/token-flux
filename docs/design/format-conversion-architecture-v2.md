# 格式转换架构 V2 — 模型级转换器

> 废弃旧版 hub 设计。每个模型定义自己的转换器，没有中间格式。

---

## 1. 三个协议标准格式

translator package 只定义**格式标识**和**数据结构类型**，不包含转换逻辑。

### 1.1 格式常量

```go
package translator

type Format string

const (
    FormatOpenAI         Format = "openai"
    FormatClaude         Format = "claude"
    FormatOpenAIResponses Format = "openai_responses"
)

type RelayMode string

const (
    RelayModeChatCompletions RelayMode = "chat-completions"
    RelayModeClaudeMessages  RelayMode = "claude-messages"
    RelayModeResponses       RelayMode = "responses"
)
```

### 1.2 格式 ↔ 端点映射表

| Format | 请求端点路径 | RelayMode | 响应体结构特征 |
|--------|-------------|-----------|--------------|
| `openai` | `/v1/chat/completions` | `ChatCompletions` | `{choices:[], usage:{}}` |
| `claude` | `/v1/messages` | `ClaudeMessages` | `{content:[], stop_reason}` |
| `openai_responses` | `/v1/responses` | `Responses` | `{output:[], status}` |

### 1.3 数据类型（仅结构体定义）

translator 定义 DataType 描述一个协议的结构：

```go
// ---- OpenAI Chat ----

type OpenAIChatRequest struct {
    Model       string            `json:"model"`
    Messages    []Message         `json:"messages"`
    MaxTokens   int               `json:"max_tokens,omitempty"`
    Temperature float64           `json:"temperature,omitempty"`
    TopP        float64           `json:"top_p,omitempty"`
    Stream      bool              `json:"stream,omitempty"`
    Tools       json.RawMessage   `json:"tools,omitempty"`
}

type OpenAIChatResponse struct {
    ID      string       `json:"id"`
    Model   string       `json:"model"`
    Choices []Choice     `json:"choices"`
    Usage   Usage        `json:"usage"`
}

// ---- Claude Messages ----

type ClaudeRequest struct {
    Model       string      `json:"model"`
    Messages    []Message   `json:"messages"`
    System      string      `json:"system,omitempty"`
    MaxTokens   int         `json:"max_tokens,omitempty"`
    Temperature float64     `json:"temperature,omitempty"`
    TopP        float64     `json:"top_p,omitempty"`
    Stream      bool        `json:"stream,omitempty"`
}

type ClaudeResponse struct {
    ID         string       `json:"id"`
    Model      string       `json:"model"`
    Content    []Block      `json:"content"`
    StopReason string       `json:"stop_reason"`
    Usage      Usage        `json:"usage"`
}

// ---- OpenAI Responses ----

type ResponsesRequest struct {
    Model           string      `json:"model"`
    Input           interface{} `json:"input"`         // string | []InputItem
    Instructions    string      `json:"instructions,omitempty"`
    MaxOutputTokens int         `json:"max_output_tokens,omitempty"`
    Temperature     float64     `json:"temperature,omitempty"`
    TopP            float64     `json:"top_p,omitempty"`
}

type ResponsesResponse struct {
    ID      string         `json:"id"`
    Model   string         `json:"model"`
    Status  string         `json:"status"`
    Output  []OutputItem   `json:"output"`
    Usage   Usage          `json:"usage"`
}
```

---

## 2. Executor 接口 — 模型级转换器

每个 executor 自己实现**请求格式转换**和**响应格式转换**。转换逻辑在 executor 里，因为：

- 转换依赖模型能力（DeepSeek 的 claude 端点和 Anthropic 的 claude 端点行为不一致）
- 每个模型可以选择实现自己需要的转换对，不需要实现全部
- 厂商定制（thinking injection、dashscope 兼容层）和格式转换在同一层，方便互动

### 2.1 Executor 接口变更

```go
type Executor interface {
    Init(channel *model.Channel)
    GetName() string
    NativeFormats() []EndpointCapability
    GetRequestURL(info *RequestInfo) (string, error)
    SetupRequestHeader(header http.Header, info *RequestInfo) error

    // --- 格式转换（新增，替代 TransformRequest/TransformResponse） ---

    // ConvertRequest 将请求体从 from 格式转为 to 格式。
    // 每个 executor 自己实现支持的格式对。
    // 不支持的格式对返回错误，不会通过 hub 中转。
    ConvertRequest(body []byte, from, to translator.Format) ([]byte, error)

    // ConvertResponse 将响应体从 from 格式转为 to 格式。
    // 每个 executor 自己实现支持的格式对。
    ConvertResponse(body []byte, from, to translator.Format) ([]byte, error)

    // --- 厂商定制（在新的转换流程中调用） ---

    RequestCustomize(body []byte, info *RequestInfo) []byte
    ResponseCustomize(body []byte, info *RequestInfo) []byte

    // --- 旧接口保留过渡，Phase 3 迁移完成后删除 ---

    TransformRequest(ctx context.Context, info *RequestInfo, requestBody []byte) (io.Reader, error)
    TransformResponse(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error)
}
```

### 2.2 统一流程（所有 executor 使用同一模板）

```go
func HandleRequest(ctx context.Context, e Executor, info *RequestInfo, body []byte, w http.ResponseWriter) {
    // 1. FormatPlan: 选最优上游端点
    plan := Plan(info.InboundFormat, info.ClientFormat, e.NativeFormats())

    // 2. 请求格式转换（模型特有）
    if plan.NeedRequestConv {
        convertedBody, err := e.ConvertRequest(body, info.InboundFormat, plan.UpstreamFormat)
        if err != nil {
            // 降级：返回格式不支持错误
            http.Error(w, fmt.Sprintf("unsupported format conversion: %s → %s", info.InboundFormat, plan.UpstreamFormat), 400)
            return
        }
        body = convertedBody
    }

    info.InboundFormat = plan.UpstreamFormat  // 更新下游路由
    info.RelayMode = plan.UpstreamRelayMode

    // 3. 厂商定制（请求侧）
    body = e.RequestCustomize(body, info)

    // 4. HTTP 调用上游
    resp := e.DoRequest(ctx, info, body)

    // 5. 响应定制
    respBody := e.ResponseCustomize(resp.Body, info)

    // 6. 响应格式转换（如果客户端要求的输出格式 != 上游格式）
    if plan.NeedResponseConv {
        convertedBody, err := e.ConvertResponse(respBody, plan.UpstreamFormat, info.ClientFormat)
        if err != nil {
            // 降级：返回上游原始响应
            w.Write(respBody)
            return
        }
        respBody = convertedBody
    }

    // 7. 写回客户端
    w.Write(respBody)
}
```

### 2.3 FormatPlan 算法不变

```go
func Plan(input, output translator.Format, capabilities []EndpointCapability) FormatPlan {
    // score(upstream) = (input != upstream) + (output != upstream)
    // tiebreak: output 格式优先
}
```

---

## 3. 各模型的转换器表

### 3.1 OpenAIExecutor

```go
// 原生端点：openai
// 支持的请求转换：
//   claude → openai
//   openai_responses → openai
// 支持的响应转换：
//   openai → claude
//   openai → openai_responses
```

| ConvertRequest | 实现 |
|---------------|------|
| claude → openai | 解析 claude 消息体 → openai messages[] |
| openai_responses → openai | 解析 responses input → openai messages[] |

| ConvertResponse | 实现 |
|----------------|------|
| openai → claude | openai choices[] → claude content[] |
| openai → openai_responses | openai choices[] → responses output[] |

### 3.2 ClaudeExecutor

```go
// 原生端点：claude
// 支持的请求转换：
//   openai → claude
//   openai_responses → claude
// 支持的响应转换：
//   claude → openai
//   claude → openai_responses
```

| ConvertRequest | 实现 |
|---------------|------|
| openai → claude | openai messages[] → claude messages[] |
| openai_responses → claude | responses input → claude messages[] |

| ConvertResponse | 实现 |
|----------------|------|
| claude → openai | claude content[] → openai choices[] |
| claude → openai_responses | claude content[] → responses output[] |

### 3.3 GeminiExecutor

```go
// 原生端点：gemini
// 支持的请求转换：
//   openai → gemini
//   claude → gemini
//   openai_responses → gemini
// 支持的响应转换：
//   gemini → openai
//   gemini → claude
//   gemini → openai_responses
```

### 3.4 DeepSeekExecutor

```go
// 原生端点：openai, claude
// 支持的请求转换（6个方向）：
//   openai → claude             (请求claude端点)
//   openai → openai_responses   (请求responses端点)
//   claude → openai             (请求openai端点)
//   claude → openai_responses   (请求responses端点)
//   openai_responses → openai   (请求openai端点)
//   openai_responses → claude   (请求claude端点)
// 支持的响应转换（6个方向）：
//   openai → claude, openai → openai_responses
//   claude → openai, claude → openai_responses
//   openai_responses → openai, openai_responses → claude
```

DeepSeek 是典型的多端点模型，需要最多转换组合。

### 3.5 其他 executor

按需实现，原则：
- 只实现该 executor 原生端点支持的格式对
- 不支持的路径返回 `fmt.Errorf("unsupported conversion: %s → %s", from, to)`
- 不要 fallback、不要 hub

---

## 4. 新旧对比

| 方面 | 旧设计（V1，废弃） | 新设计（V2） |
|------|-------------------|-------------|
| 转换位置 | translator 通用转换（hub） | executor 模型级转换 |
| 未实现路径 | hub fallback（通过 openai） | 直接返回错误 |
| 中间格式 | OpenAI Chat（hub） | 无中间格式 |
| 转换函数 | Normalize/Denormalize | ConvertRequest/ConvertResponse |
| 响应转换 | Denormalizer 接口 | ConvertResponse 统一 |
| 检测统一 | translator.Detect | 不变（保持） |
| FormatPlan | 不变 | 不变 |

---

## 5. 迁移步骤

### Phase 1: 定义标准 + 调整 translator

- translator 只保留 Format 常量 + 数据类型结构体
- 删除所有 Normalize/Denormalize 代码
- 删除所有转换函数（OpenAIToClaude、OpenAIToGemini、claudeToOpenAI 等）
- 删除 Detect 检测逻辑？— 检测属于不同关注点，保持不动
- 校验：`go build ./pkg/translator/...`

### Phase 2: Executor 接口变更

- Executor 接口新增 ConvertRequest / ConvertResponse / RequestCustomize / ResponseCustomize
- 旧 TransformRequest / TransformResponse 标记 Deprecated
- 各 executor 实现自己的 ConvertRequest / ConvertResponse
- 共享工具函数放 executor/conv_shared.go（纯 JSON 解析 + 重组，没有 hub）

### Phase 3: 调用链切换

- HandleRequest 模板使用新流程（Plan → ConvertRequest → RequestCustomize → DoRequest → ResponseCustomize → ConvertResponse）
- 旧 TransformRequest / TransformResponse 删除
- FormatPlan NeedRequestConv / NeedResponseConv 使用新的转换接口

### Phase 4: 清理

- 删除 translator 里所有残留的转换代码
- 删除旧适配器（ali.Adaptor、zhipu.Adaptor 等）— 后续
- 文档更新
- `rg "Normalize|Denormalize|shouldPassthrough|convertOpenAI"` 零匹配

---

## 6. 关键原则

1. **没有 hub** — 没有任何格式是中间格式
2. **每个模型只实现自己需要的转换** — 不要求全量
3. **不支持的路径直接报错** — 不降级、不 fallback、不偷偷中转
4. **转换和定制在同一层** — executor 同时负责格式转换和厂商定制，因为两者互相依赖（例如 thinking injection 需要知道上游格式是 claude 还是 openai）
5. **检测和 FormatPlan 不变** — 检测输入格式、选最优上游端点的逻辑保持不变

---

## 7. 流式响应处理

流式响应不能像非流式一样 buffer 后整体转换。每个协议有自己的 SSE 结构：

| 格式 | SSE 结构 | Stream 数据节点 | 结束标志 |
|------|---------|---------------|---------|
| OpenAI Chat | `data: {"choices":[{"delta":{"content":"..."}}]}` | delta.content | `data: [DONE]` |
| Claude | `event: content_block_delta\ndata: {"delta":{"text":"..."}}` | delta.text | `event: message_stop` |
| OpenAI Responses | `event: response.output_item.delta\ndata: {"delta":"..."}` | delta | `event: response.completed` |

### 7.1 ResponseStream 接口

executor.Eexecutor 新增 streaming 方法，返回逐 chunk 转换器：

```go
type Executor interface {
    // 非流式响应转换（原有）
    ConvertResponse(body []byte, from, to translator.Format) ([]byte, error)

    // 流式响应转换
    // 返回 nil, nil 表示该格式对不支持 streaming（降级为非流式或 error）
    NewResponseStream(from, to translator.Format) (ResponseStream, error)

    // 其他方法...
}

// ResponseStream 逐 chunk 转换上游 SSE → 客户端 SSE。
// 创建时绑定一次 from→to 方向，后续 Feed 调用不需要再传格式。
type ResponseStream interface {
    // Feed 输入一个上游 SSE 数据 chunk。
    // 返回需要发给客户端的 SSE 数据（可能为空）。
    Feed(chunk []byte) ([]byte, error)

    // End 上游 SSE 结束（[DONE] / message_stop / response.completed）。
    // 返回可能还需要的尾部数据。
    End() ([]byte, error)

    // Usage 从已累积的 stream 数据中提取 token 用量。
    // 非流式模式从响应头提取；流式模式从特定事件提取。
    Usage() *Usage
}
```

### 7.2 统一流程（含 streaming）

```go
func HandleRequest(ctx context.Context, e Executor, info *RequestInfo, body []byte, w http.ResponseWriter) {
    // 1. FormatPlan
    plan := Plan(info.InboundFormat, info.ClientFormat, e.NativeFormats())

    // 2. 请求格式转换
    if plan.NeedRequestConv {
        converted, err := e.ConvertRequest(body, info.InboundFormat, plan.UpstreamFormat)
        if err != nil {
            http.Error(w, "unsupported request conversion", 400)
            return
        }
        body = converted
    }
    info.InboundFormat = plan.UpstreamFormat
    info.RelayMode = plan.UpstreamRelayMode

    // 3. 厂商请求定制
    body = e.RequestCustomize(body, info)

    // 4. HTTP 调用
    resp, err := e.DoRequest(ctx, info, body)
    if err != nil { ... }

    // 5. 判断是否需要响应转换
    needResponseConv := plan.NeedResponseConv

    // 6. 响应分支
    if info.IsStream {
        // --- 流式路径 ---
        var streamConv ResponseStream
        if needResponseConv {
            streamConv, err = e.NewResponseStream(plan.UpstreamFormat, info.ClientFormat)
            if err != nil || streamConv == nil {
                // 不支持 streaming 转换 → 按原始上游格式返回
                needResponseConv = false
            }
        }

        writer.Header().Set("Content-Type", "text/event-stream")
        // ... flush headers

        scanner := NewSSEScanner(resp.Body)
        for scanner.Scan() {
            chunk := scanner.Bytes()
            outputChunk := chunk

            if needResponseConv && streamConv != nil {
                converted, err := streamConv.Feed(chunk)
                if err != nil { continue /* 跳过坏 chunk */ }
                if converted != nil {
                    outputChunk = converted
                } else {
                    continue // 还在缓冲中，不输出
                }
            }

            writer.Write(outputChunk)
            flusher.Flush()
        }

        if needResponseConv && streamConv != nil {
            tail, _ := streamConv.End()
            if tail != nil {
                writer.Write(tail)
                flusher.Flush()
            }
            extractUsage = streamConv.Usage
        }

    } else {
        // --- 非流式路径 ---
        respBody, _ := io.ReadAll(resp.Body)

        if needResponseConv {
            converted, err := e.ConvertResponse(respBody, plan.UpstreamFormat, info.ClientFormat)
            if err == nil {
                respBody = converted
            }
        }

        writer.Header().Set("Content-Type", "application/json")
        writer.Write(respBody)
    }
}
```

### 7.3 各 executor 的 streaming 策略

**OpenAIExecutor** — 上游返回 openai SSE → 客户端可能期望 claude/responses：
- OpenAI SSE delta.content → Claude SSE content_block_delta
- OpenAI SSE delta.content → Responses SSE response.output_text.delta
- stream state: 不需要累积（每 chunk 独立转换）
- usage: OpenAI 在最后一个 chunk 的 stream_options 带 usage

**ClaudeExecutor** — 上游返回 claude SSE → 客户端可能期望 openai/responses：
- Claude SSE content_block_delta → OpenAI SSE delta.content
- 需要累积 state：content_block_start + delta 内容块
- usage: Claude message_delta 事件带 usage

**DeepSeekExecutor** — 同时支持 openai + claude 端点：
- 如果上游是 claude 端点 → 同 ClaudeExecutor 转换规则
- 如果上游是 openai 端点 → 同 OpenAIExecutor 转换规则
- DeepSeek 本身在 claude 端点返回的是 claude 格式 SSE

### 7.4 stream state 生命周期

每个 ResponseStream 实现内部维护状态，使用时即创即用：

```go
// claudeToOpenAIStream 示例
type claudeToOpenAIStream struct {
    buffer bytes.Buffer  // 累积多个 delta
    usage  *Usage
}

func (s *claudeToOpenAIStream) Feed(chunk []byte) ([]byte, error) {
    event, data := parseSSEEvent(chunk)
    switch event {
    case "content_block_delta":
        text := extractClaudeDeltaText(data)
        s.buffer.WriteString(text)
        return formatOpenAIChunk(text), nil
    case "message_delta":
        s.usage = extractClaudeUsage(data)
        return nil, nil
    case "message_stop":
        return nil, nil  // End() 处理
    }
    return nil, nil
}

func (s *claudeToOpenAIStream) End() ([]byte, error) {
    return []byte("data: [DONE]\n\n"), nil
}

func (s *claudeToOpenAIStream) Usage() *Usage {
    return s.usage
}
```

---

## 8. 共享工具函数

Converter 逻辑放在 executor 包内的 `conv_shared.go`，纯 JSON 解析 + 重组，没有 hub，不导出：

```go
// conv_shared.go — executor 包内私有，各模型按需调用

// openAIRequestToClaudeMessages 解析 openai 请求 → claude 格式的 messages
// 供 OpenAIExecutor.ConvertRequest(claude→openai) 和
//  DeepSeekExecutor.ConvertRequest(claude→openai) 共用
func openAIRequestToClaudeMessages(body []byte) ([]byte, error)

// claudeResponseToOpenAIChoices 解析 claude 响应 → openai 格式的 choices
// 供需要 claude→openai 响应转换的 executor 使用
func claudeResponseToOpenAIChoices(body []byte) ([]byte, error)

// 等等...
```

原则：
- 共享工具是纯函数操作 JSON，不依赖任何 executor 或 model 类型
- 各 executor 可以自由选择用共享工具还是自己实现
- 共享工具不导出让 Plan 之类的逻辑跨包依赖
