# 格式转换案例

## 1. Claude Messages → OpenAI Chat (Normalize)

**输入** — `/v1/messages` (Claude 格式):
```json
{
  "model": "claude-sonnet-4",
  "max_tokens": 1024,
  "system": "你是一个助手",
  "messages": [
    {"role": "user", "content": "你好"},
    {"role": "assistant", "content": "你好！有什么可以帮助？"},
    {"role": "user", "content": "写一段 Go 代码"}
  ],
  "temperature": 0.7
}
```

**输出** — 转成 OpenAI Chat Completions 格式:
```json
{
  "model": "claude-sonnet-4",
  "messages": [
    {"role": "system", "content": "你是一个助手"},
    {"role": "user", "content": "你好"},
    {"role": "assistant", "content": "你好！有什么可以帮助？"},
    {"role": "user", "content": "写一段 Go 代码"}
  ],
  "max_tokens": 1024,
  "temperature": 0.7
}
```

**关键转换：**
- `system` 字段（字符串）→ `messages` 数组第一条 `role: system`
- role 不变（user/assistant）
- max_tokens / temperature / top_p 直接搬运

---

## 2. Claude Image 多内容块 → OpenAI

**输入** (Claude multi-content block):
```json
{
  "model": "claude-sonnet-4",
  "messages": [
    {
      "role": "user",
      "content": [
        {"type": "text", "text": "描述这张图片"},
        {"type": "image", "source": {"media_type": "image/jpeg", "data": "/9j/4AAQ..."}}
      ]
    }
  ]
}
```

**输出** (OpenAI vision format):
```json
{
  "model": "claude-sonnet-4",
  "messages": [
    {
      "role": "user",
      "content": [
        {"type": "text", "text": "描述这张图片"},
        {"type": "image_url", "image_url": {"url": "data:image/jpeg;base64,/9j/4AAQ..."}}
      ]
    }
  ]
}
```

**关键转换：**
- Claude `type: image` / `source: {media_type, data}` → OpenAI `type: image_url` / `image_url: {url: data:...;base64,...}`
- text block 保持原样

---

## 3. Claude Tool Results → OpenAI

**输入** (Claude tool_use/tool_result):
```json
{
  "model": "claude-sonnet-4",
  "messages": [
    {"role": "user", "content": "天气怎么样"},
    {"role": "assistant", "content": [
      {"type": "text", "text": "让我查一下"},
      {"type": "tool_use", "id": "tu_123", "name": "get_weather", "input": {"city": "北京"}}
    ]},
    {"role": "user", "content": [
      {"type": "tool_result", "tool_use_id": "tu_123", "content": "25°C"}
    ]}
  ]
}
```

**输出**:
```json
{
  "model": "claude-sonnet-4",
  "messages": [
    {"role": "user", "content": "天气怎么样"},
    {"role": "assistant", "content": "让我查一下",
     "tool_calls": [{"id": "tu_123", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"北京\"}"}}]},
    {"role": "tool", "tool_call_id": "tu_123", "content": "25°C"}
  ]
}
```

**关键转换：**
- `tool_use` → 拆成独立 assistant message + `tool_calls` 数组
- `tool_result` → `role: tool` + `tool_call_id`
- 文本 content 保持在一个 message

---

## 4. Gemini → OpenAI (Normalize)

**输入** — Gemini API 格式:
```json
{
  "model": "gemini-2.0-flash",
  "systemInstruction": {"parts": [{"text": "你是一个助手"}]},
  "contents": [
    {"role": "user", "parts": [{"text": "你好"}]},
    {"role": "model", "parts": [{"text": "你好！你好"}]}
  ],
  "generationConfig": {
    "maxOutputTokens": 2048,
    "temperature": 0.8
  }
}
```

**输出** — OpenAI Chat:
```json
{
  "model": "gemini-2.0-flash",
  "messages": [
    {"role": "system", "content": "你是一个助手"},
    {"role": "user", "content": "你好"},
    {"role": "assistant", "content": "你好！你好"}
  ],
  "max_tokens": 2048,
  "temperature": 0.8
}
```

**关键转换：**
- `systemInstruction.parts[0].text` → `messages[0] role: system`
- `contents` 数组 → `messages` 数组
- `role: model` → `role: assistant`
- `parts[0].text` → `content` 字符串
- `generationConfig.maxOutputTokens` → `max_tokens`
- `generationConfig.temperature` → `temperature`

---

## 5. OpenAI Responses → OpenAI Chat (Normalize)

**输入** — `/v1/responses` 格式:
```json
{
  "model": "gpt-4o",
  "input": "你好",
  "instructions": "用中文回复",
  "max_output_tokens": 1000
}
```

**输出**:
```json
{
  "model": "gpt-4o",
  "messages": [
    {"role": "system", "content": "用中文回复"},
    {"role": "user", "content": "你好"}
  ],
  "max_tokens": 1000
}
```

**带历史的多消息输入:**
```json
{
  "model": "gpt-4o",
  "input": [
    {"role": "user", "content": "你好"},
    {"role": "assistant", "content": "你好！"},
    {"role": "user", "content": "Go 怎么用？"}
  ]
}
```

**输出**:
```json
{
  "model": "gpt-4o",
  "messages": [
    {"role": "user", "content": "你好"},
    {"role": "assistant", "content": "你好！"},
    {"role": "user", "content": "Go 怎么用？"}
  ]
}
```

**关键转换：**
- `instructions` → `messages[0] role: system`
- `input` 字符串 → `messages` 单条 user
- `input` 数组 → 逐条转换为 messages
- `input` 中的 `type: function_call_output` → `role: tool`

---

## 6. OpenAI Chat → Claude Messages (Denormalize — 输出转换)

**输入** — OpenAI 响应:
```json
{
  "id": "chatcmpl-abc123",
  "model": "gpt-4o",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "Go 语言是一种静态类型语言"
      }
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20
  }
}
```

**输出** — Claude Messages 格式:
```json
{
  "id": "msg_chatcmpl-abc123",
  "type": "message",
  "role": "assistant",
  "model": "gpt-4o",
  "stop_reason": "end_turn",
  "content": [
    {"type": "text", "text": "Go 语言是一种静态类型语言"}
  ],
  "usage": {
    "input_tokens": 10,
    "output_tokens": 20
  }
}
```

**Streaming** — OpenAI SSE chunk:
```
data: {"choices":[{"delta":{"content":"Go"}}]}
```

→ 输出 Claude SSE event:
```
event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Go"}}

```

结尾:
```
event: message_stop
data: {"type":"message_stop"}

```

**关键转换：**
- `id` → `msg_` 前缀
- `choices[0].message.content` → `content` 数组包 text block
- `prompt_tokens` → `input_tokens`
- `completion_tokens` → `output_tokens`
- Streaming: `data: {...}` → `event: content_block_delta`

---

## 7. OpenAI Chat → Gemini (Denormalize)

**输入** — OpenAI 响应:
```json
{
  "id": "chatcmpl-abc",
  "model": "gemini-2.0-flash",
  "choices": [
    {"message": {"content": "这是一个例子"}}
  ],
  "usage": {"prompt_tokens": 5, "completion_tokens": 3}
}
```

**输出** — Gemini 格式:
```json
{
  "candidates": [
    {
      "content": {
        "role": "model",
        "parts": [{"text": "这是一个例子"}]
      }
    }
  ]
}
```

**关键转换：**
- `choices[0].message.content` → `candidates[0].content.parts[0].text`
- role 固定为 `"model"`
- Gemini 没有 usage 字段（丢 token 统计）

---

## 8. OpenAI Chat → OpenAI Responses (Denormalize)

**输入** — OpenAI 响应:
```json
{
  "id": "chatcmpl-abc",
  "object": "chat.completion",
  "created": 1718000000,
  "model": "gpt-4o",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "Go 语言很棒"
      }
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 3,
    "total_tokens": 13
  }
}
```

**输出** — Responses API 格式:
```json
{
  "id": "resp_chatcmpl-abc",
  "object": "response",
  "status": "completed",
  "model": "gpt-4o",
  "output": [
    {
      "type": "message",
      "id": "msg_chatcmpl-abc",
      "status": "completed",
      "role": "assistant",
      "content": [{"type": "output_text", "text": "Go 语言很棒"}]
    }
  ],
  "usage": {
    "input_tokens": 10,
    "output_tokens": 3,
    "total_tokens": 13
  }
}
```

**关键转换：**
- `id` → `resp_` 前缀
- `choices[0].message` → `output` 数组包 message 对象
- content 转为 `type: output_text` block
- usage 字段重命名（input_tokens / output_tokens / total_tokens）

---

## 9. DeepSeek 双格式透传场景

DeepSeek 同时原生支持 OpenAI Chat 和 Claude Messages，`shouldPassthrough` 返回 `true`，不经转换直接转发。

### 场景 A：客户端发 Claude，要 Claude 输出

```
客户端 → POST /v1/messages (Claude 格式)
  → InboundFormat = "claude", ClientFormat = "claude"
  → DeepSeekExecutor.TransformRequest:
    → shouldPassthrough(info, "openai", "claude") → true → 不转换
  → POST https://api.deepseek.com/chat/completions (body 保持 Claude 格式)
  → DeepSeekExecutor.TransformResponse:
    → InboundFormat = "claude" → 委托 ClaudeExecutor
      → ClientFormat = "claude" → 保持 Claude 格式 → passthrough
  → 客户端收到 Claude 格式响应
```

**请求体不变**（透传）:
```json
{
  "model": "deepseek-chat",
  "messages": [{"role": "user", "content": "hello"}],
  "stream": true
}
```

### 场景 B：客户端发 OpenAI，要 Claude 输出

```
客户端 → POST /v1/chat/completions (OpenAI 格式)
  → InboundFormat = "openai", ClientFormat = "claude"
  → DeepSeekExecutor.TransformRequest:
    → shouldPassthrough(info, "openai", "claude") → true → 透传 OpenAI 格式
    → 但 ClientFormat="claude" != InboundFormat="openai"
    → openAIReqToClaudeReq(body) → 转换为 Claude Messages 格式
    → info.InboundFormat = "claude"
    → info.RelayMode = RelayModeClaudeMessages
  → POST https://api.deepseek.com/anthropic/v1/messages (Claude 格式)
    (DeepSeek 原生接受 Claude 格式，直接返回 Claude 格式响应)
  → DeepSeekExecutor.TransformResponse:
    → InboundFormat = "claude" → 委托 ClaudeExecutor
      → ClientFormat = "claude" → 透传原生 Claude 格式
  → 客户端收到 Claude Messages 格式响应 (零转换开销)
```

### 场景 C：客户端发 Gemini，要 OpenAI 输出

```
客户端 → POST /v1/chat/completions... 但 body 是 Gemini 格式
  → InboundFormat = "gemini", ClientFormat = "openai"
  → DeepSeekExecutor.TransformRequest:
    → shouldPassthrough(info, "openai", "claude") → false
    → translator.Normalize(body, FormatGemini) → 转 OpenAI
  → POST https://api.deepseek.com/chat/completions (OpenAI 格式)
  → DeepSeekExecutor.TransformResponse:
    → InboundFormat = "openai" → 委托 OpenAIExecutor
      → ClientFormat = "openai" → passthrough (等于 InboundFormat)
  → 客户端收到 OpenAI Chat 格式响应
```

---

## 10. Platform Relay Bridge 完整流程

以平台 Gateway (:8081) 处理 `/v1/chat/completions` 请求为例：

```
客户端 → POST :8081/v1/chat/completions
  Header: Authorization: sk-xxx
  Body: {"model": "gpt-4o", "messages": [{"role": "user", "content": "你好"}]}

1. handler.detectInboundFormat(body, "/v1/chat/completions")
   → RelayFormatOpenAI (由路径决定)

2. handler.extractOutputFormat(body)
   → 检查 X-Response-Format 头 → 没设置 → outputFormat = unset

3. Build RelayInfo:
   - InboundFormat: constant.RelayFormat("openai")
   - ClientFormat: string(info.GetOriginalClientFormat()) → "openai" (fallback)

4. channel.GetAdaptor(1) → ProviderOpenAI → bridge.New(OpenAIExecutor)

5. bridge.ConvertRequest → buildRequestInfo(info) → RequestInfo{InboundFormat:"openai"}
   → OpenAIExecutor.TransformRequest → shouldPassthrough("openai") → true → 透传

6. bridge.DoRequest → HTTP POST https://api.openai.com/v1/chat/completions

7. bridge.DoResponse → buildRequestInfo → RequestInfo{ClientFormat:"openai"}
   → OpenAIExecutor.TransformResponse → ClientFormat="openai" → passthrough
   → 写入 http.ResponseWriter

8. Settle: 记录 usage, 结算, 扣配额
```
