# Format Conversion & Request Flow

## Overview

Two runtime layers (Node, Platform) share one format conversion engine (`pkg/translator`) and one executor framework (`pkg/executor`). Both support multi-format inbound (OpenAI Chat, Claude Messages, OpenAI Responses, Gemini) and format negotiation via `X-Response-Format`.

---

## 1. Format Constants — Three Separate Definitions

These are semantically identical but independently defined. A unification target.

| Location | Type | Values |
|----------|------|--------|
| `pkg/translator` | `Format` | `"openai"`, `"claude"`, `"gemini"`, `"openai_responses"` |
| `server/internal/relay/constant` | `RelayFormat` | `"openai"`, `"claude"`, `"gemini"`, `"openai_responses"` |
| `pkg/executor/RequestInfo` | `string` | Same literal strings, no type distinction |

**Bridge layer** (`bridge.go`) converts between the two typed definitions:

```go
func relayFormatToString(f constant.RelayFormat) string {
    return strings.ToLower(string(f))
}
```

---

## 2. Core Translation Layer — `pkg/translator`

Zero dependencies. Pure JSON transformation. Used by both Node and Platform.

### Normalize — Any Format → OpenAI Chat

```
body + Format
  │
  ├─ FormatOpenAI           ──→ passthrough
  ├─ FormatClaude           ──→ claudeToOpenAI()
  ├─ FormatGemini           ──→ geminiToOpenAI()
  └─ FormatOpenAIResponses  ──→ responsesToOpenAI()
```

### Denormalize — OpenAI Chat → Any Format

Returns a `Denormalizer` interface with `ConvertBody(body)`, `ConvertChunk(chunk)`, `Finalize()`.

```
body + Format + isStream
  │
  ├─ FormatOpenAI           ──→ passthroughDenorm (no-op)
  ├─ FormatClaude           ──→ newClaudeDenorm(isStream)
  ├─ FormatGemini           ──→ newGeminiDenorm()
  └─ FormatOpenAIResponses  ──→ newResponsesDenorm()
```

### DetectFormat — Auto-Detect from Body

```go
func DetectFormat(body []byte) Format
```

Scans JSON keys:
| Key in body | Detected Format |
|-------------|----------------|
| `"contents"` | FormatGemini |
| `"input"` | FormatOpenAIResponses |
| `"anthropic_version"` or `"system"` (array) | FormatClaude |
| fallback | FormatOpenAI |

---

## 3. Executor Framework — `pkg/executor`

### Executor Interface

```go
type Executor interface {
    Init(channel *model.Channel)
    GetRequestURL(info *RequestInfo) (string, error)
    SetupRequestHeader(header http.Header, info *RequestInfo) error
    TransformRequest(ctx, info, body) (io.Reader, error)
    DoRequest(ctx, info, body) (*http.Response, error)
    TransformResponse(ctx, resp, info, writer) (*Usage, error)
    GetName() string
}
```

### RequestInfo — Format Carriers

```go
type RequestInfo struct {
    InboundFormat   string  // "openai" | "claude" | "gemini" | "openai_responses"
    ClientFormat    string  // original client format before conversion
    RelayMode       int     // from pkg/model
    Model           string
    ActualModelName string
    Channel         *model.Channel
    Protocol        *model.ProtocolEntry
    ApiKey          string
    IsStream        bool
    // ...
}
```

### FormatPlan — Optimal Format Selection Algorithm

```go
type EndpointCapability struct {
    Format    string          // "openai", "claude", "gemini"
    RelayMode model.RelayMode
}
type FormatPlan struct {
    UpstreamFormat    string
    UpstreamRelayMode model.RelayMode
    NeedRequestConv   bool
    NeedResponseConv  bool
}
type FormatCapable interface {
    NativeFormats() []EndpointCapability
}
func Plan(inputFormat, outputFormat string, caps []EndpointCapability) FormatPlan
```

Algorithm: `score(upstream) = (input != upstream) + (output != upstream)`.

For each endpoint capability:
- count request conversion needed (input → endpoint format)
- count response conversion needed (endpoint → client output)
- check conversion path feasibility (`canConvertRequest` / `canConvertResponse`)
- pick lowest score

Tiebreaker: when scores equal, prefer **request-side conversion** (avoids streaming denormalize complexity).

Conversion feasibility:
| Direction | Path | 
|-----------|------|
| any → openai | `translator.Normalize` |
| openai → claude | `openAIReqToClaudeReq` |
| openai → gemini | `convertOpenAIToGemini` |
| other direct | Not feasible (via openai intermediate) |
| any → any (response) | Executor native→openai → `Denormalize` |

Implemented in `pkg/executor/plan.go`.

### NativeFormats — Executor Capability Declaration

Each executor implements `FormatCapable` to declare supported formats:

| Executor | NativeFormats() | Notes |
|----------|----------------|-------|
| `OpenAIExecutor` | `[{openai, ChatCompletions}]` | All OpenAI-compatible providers |
| `ClaudeExecutor` | `[{claude, ClaudeMessages}]` | Anthropic Claude |
| `GeminiExecutor` | `[{gemini, GeminiChat}]` | Google Gemini |
| `DeepSeekExecutor` | `[{openai, ChatCompletions}, {claude, ClaudeMessages}]` | Dual-native |
| `AliExecutor` | `[{openai, ChatCompletions}, {claude, ClaudeMessages}]` | /compatible-mode/ + /anthropic/ |
| `ZhipuExecutor` | `[{openai, ChatCompletions}, {claude, ClaudeMessages}]` | /paas/v4/ + /anthropic/ |
| `MiniMaxExecutor` | `[{openai, ChatCompletions}, {claude, ClaudeMessages}]` | /chatcompletion_v2 + /anthropic/ |
| `BaiduExecutor` | `[{openai, ChatCompletions}]` | OpenAI-compatible |
| `TencentExecutor` | `[{openai, ChatCompletions}]` | OpenAI-compatible |

### TransformRequest — Per-Executor Logic

All executors use Plan() to decide conversion strategy:

```go
plan := Plan(info.InboundFormat, info.ClientFormat, e.NativeFormats())
if plan.NeedRequestConv {
    body = convertBody(body, info.InboundFormat, plan.UpstreamFormat)
}
info.InboundFormat = plan.UpstreamFormat   // update for downstream
info.RelayMode = plan.UpstreamRelayMode    // update URL routing
```

**OpenAIExecutor:**
- Plan → upstream openai (single cap)
- If NeedRequestConv → `translator.Normalize(body, inboundFormat)`
- Native path: inject stream_options, reasoning_effort

**ClaudeExecutor:**
- Plan → upstream claude (single cap)
- If NeedRequestConv → `translator.Denormalize(body, FormatClaude, isStream).ConvertBody(body)`
- Only non-claude→claude conversion needed

**GeminiExecutor:**
- Plan → upstream gemini (single cap)
- If NeedRequestConv → normalize to OpenAI → `convertOpenAIToGemini`
- Native path: strip stream field

**DeepSeekExecutor:**
- Plan selects optimal upstream (openai or claude) by score + tiebreak
- If NeedRequestConv:
  - openai → claude: `openAIReqToClaudeReq(body)`
  - claude → openai: `claudeReqToOpenAIReq(body)`  
  - gemini/responses → openai: `translator.Normalize(body, inboundFormat)`
- After conversion: `info.InboundFormat = plan.UpstreamFormat`, `info.RelayMode = plan.UpstreamRelayMode`
- Stream options injected only for openai upstream

### TransformResponse — ClientFormat Handling

**OpenAIExecutor:**
```go
if ClientFormat != "" && ClientFormat != "openai" && ClientFormat != InboundFormat
    → translator.Denormalize(nil, ClientFormat, isStream).ConvertBody(resp)
    → writeDenormalized(ctx, resp, info, writer, dn)
else → native passthrough (handleChatStream / handleChatNonStream)
```

**ClaudeExecutor:**
```go
// Priority: streaming first, then ClientFormat, then native.
if IsStream && ClientFormat != "" && ClientFormat != "claude" && ClientFormat != InboundFormat
    → handleStreamToClientFormat(ctx, resp, info, writer)
      Parse Claude SSE events: message_start → delta{role}, content_block_delta → delta{content},
      message_delta → finish_reason + usage, message_stop → [DONE]
      Each OpenAI chunk optionally passed through Denormalizer.ConvertChunk()
else if IsStream → handleStream (native Claude SSE passthrough)
else if ClientFormat != "" && ClientFormat != "claude" && ClientFormat != InboundFormat
    → denormalizeToClientFormat(ctx, resp, info, writer)
      Step 1: claudeResponseToOpenAI(body)     // Claude native → OpenAI Chat
      Step 2: translator.Denormalize(oaiBody, ClientFormat, isStream).ConvertBody()
else → handleNonStream
```

**GeminiExecutor:**
```go
// Streaming takes priority over ClientFormat check.
if IsStream && ClientFormat != "" && ClientFormat != "gemini" && ClientFormat != InboundFormat
    → handleStreamToClientFormat(ctx, resp, info, writer)
      Parse Gemini SSE data lines → extract text + finishReason + usageMetadata
      → build OpenAI delta chunks → optionally Denormalizer.ConvertChunk()
else if IsStream → handleStream (native Gemini SSE passthrough)
else if ClientFormat != "" && ClientFormat != "gemini" && ClientFormat != InboundFormat
    → denormalizeToClientFormat(ctx, resp, info, writer)
      Step 1: geminiResponseToOpenAI(body)     // Gemini native → OpenAI Chat
      Step 2: translator.Denormalize(oaiBody, ClientFormat, isStream).ConvertBody()
else → native non-streaming passthrough
```

**DeepSeekExecutor:**
```go
if info.InboundFormat == "claude"
    → delegate to ClaudeExecutor.TransformResponse
else → delegate to OpenAIExecutor.TransformResponse
// ClientFormat pass-through relies on OpenAIExecutor/ClaudeExecutor
```

---

## 4. Node Request Flow

### Route → Handler Mapping

| HTTP Endpoint | Handler | InboundFormat | RelayMode |
|--------------|---------|---------------|-----------|
| `POST /v1/chat/completions` | `handleChat` | `"openai"` | `RelayModeChatCompletions` |
| `POST /v1/messages` | `handleMessages` | `"claude"` | `RelayModeClaudeMessages` |
| `POST /v1/responses` | `handleResponses` | `"openai_responses"` | `RelayModeChatCompletions` |
| `POST /v1/completions` | `handleCompletions` | `"openai"` | `RelayModeCompletions` |
| `POST /v1/embeddings` | `handleEmbeddings` | `"openai"` | `RelayModeEmbeddings` |
| `GET /v1/models` | `handleModels` | — | — |

### Handler → Pipeline (same for all handlers)

```
Client → HTTP POST /v1/chat/completions
  │
  ├─ apiKeyAuth middleware → validate sk-xxx key
  │
  ├─ handler parses body, sets:
  │   ├─ InboundFormat: from route (e.g., "openai")
  │   ├─ ClientFormat:  from X-Response-Format header, fallback = InboundFormat
  │   └─ RelayMode:     from route
  │
  ├─ router.Pick(model) → find key hash (random selection among matching keys)
  │
  └─ provider.ExecuteWithWriter(ctx, body, hash, info, w)
       │
       ├─ keychain.FindKeyByHash(hash) → entry + binding
       │
       ├─ build model.Channel:
       │   ├─ ProviderType, Status, ApiKeyEncrypted
       │   └─ Protocol = channel.ResolveProtocol(info.InboundFormat)
       │         └─ matches inbound format → native protocol entry
       │         └─ no match → default protocol entry (usually openai-compatible)
       │
       ├─ executor.GetByProvider(channel.ProviderType) → e.g., OpenAIExecutor
       │
       ├─ info.Channel = channel
       ├─ info.Protocol = channel.ResolveProtocol(info.InboundFormat)
       ├─ info.ApiKey = entry.Key
       ├─ info.ActualModelName = binding.UpstreamModelName
       │
       ├─ e.TransformRequest(ctx, info, body) → io.Reader
       │   ├─ shouldPassthrough? → body as-is
       │   └─ else → translator.Normalize(body, inboundFormat)
       │
       ├─ e.DoRequest(ctx, info, reader) → *http.Response
       │   ├─ e.GetRequestURL(info) → upstream URL
       │   ├─ e.SetupRequestHeader(header, info) → auth + content-type
       │   └─ httpClient.Do(req) → upstream HTTP call
       │
       ├─ e.TransformResponse(ctx, resp, info, w) → *Usage
       │   ├─ ClientFormat ≠ InboundFormat? → translator.Denormalize(resp, ClientFormat)
       │   └─ else → native format response handling
       │
       └─ usagetracker.RecordUsage(...) → persist to DB
```

### ClientFormat Header

`X-Response-Format` request header controls output format:

```go
func clientFormatFromHeader(r *http.Request, inboundFormat string) string {
    f := r.Header.Get("X-Response-Format")
    if f == "openai" || f == "claude" || f == "openai_responses" || f == "gemini" {
        return f
    }
    return inboundFormat // default = match input format
}
```

### Full Example — Claude Input, OpenAI Output via DeepSeek

```
1. Client → POST /v1/messages (anthropic_version + system + messages array)
2. handleMessages: InboundFormat="claude", ClientFormat="openai"
3. router.Pick → DeepSeek key
4. provider.ExecuteWithWriter
5. DeepSeekExecutor.TransformRequest:
   shouldPassthrough(info, "openai", "claude") → true → body as-is
6. DeepSeekExecutor.DoRequest → POST https://api.deepseek.com/chat/completions
   (DeepSeek serves Claude-format at its OpenAI endpoint)
7. DeepSeekExecutor.TransformResponse:
   InboundFormat="claude" → delegate to ClaudeExecutor.TransformResponse
     → ClientFormat="openai" → denormalize Claude→OpenAI response
8. Client receives OpenAI Chat Completions format
```

### Tunnel Request Flow (Different Path)

Tunnel requests enter via WebSocket, bypass the HTTP server handlers.

```
Remote Client → WebSocket → tunnel.go → types.RequestEnvelope
  └─ Execute(ctx, envelope, keyHash) → direct provider call

  RequestEnvelope carries:
    ├─ InboundFormat: from tunnel client (e.g. "openai", "claude")
    ├─ ClientFormat:  from tunnel client (for response denormalize)
    └─ Request:       ChatRequest body
```

Tunnel flow uses `req.InboundFormat` (fallback `"openai"`) and passes `req.ClientFormat` through `RequestInfo`. Same executor pipeline as HTTP handlers — format conversion supported.

---

## 5. Platform Relay Flow

### Three Servers

| Server | Port | Format Handling |
|--------|------|----------------|
| `:8080` API (JWT) | Provider LLM proxy | Direct relay (not format-aware) |
| `:8081` Gateway (API Key) | **Primary format-conversion entry** | Full relay pipeline |
| `:8082` Admin (Token) | Management | No LLM relay |

### Gateway Relay Pipeline

```
Client → POST :8081/v1/chat/completions (sk-xxx key)
  │
  ├─ APIKeyAuth middleware → user_id + api_key_id
  │
  ├─ Extract model from body
  │
  ├─ detectInboundFormat(body, path):
  │   ├─ DetectFormatFromPath(path): /v1/chat/completions → RelayFormatOpenAI
  │   │                               /v1/messages        → RelayFormatClaude
  │   │                               /v1/responses       → RelayFormatOpenAIResponses
  │   └─ DetectInboundFormat(body):   key-based detection (same as translator.DetectFormat)
  │
  ├─ extractOutputFormat(body):
  │   ├─ X-Response-Format header (if present)
  │   ├─ body.output_format field (if present)
  │   └─ unset → matches inbound
  │
  ├─ Build RelayInfo:
  │   ├─ InboundFormat:  constant.RelayFormat
  │   ├─ ClientFormat:   GetOriginalClientFormat() → outputFormat.Or(inboundFormat)
  │   └─ RelayMode:      int(constant.RelayModeChatCompletions)
  │
  ├─ Check balance + status
  │
  ├─ Resolve model spec → channel → adaptor
  │   ├─ For migrated providers: channel.GetAdaptor(type) → bridge.New(executor)
  │   └─ For legacy providers:   channel.GetAdaptor(type) → native adaptor (ali, baidu, etc.)
  │
  ├─ adaptor.Init(info) → store channel config
  │
  ├─ adaptor.ConvertRequest(ctx, info, body) → io.Reader
  │   [bridge path] → buildRequestInfo(info) → executor.TransformRequest(ctx, ri, body)
  │   [legacy path] → adaptor's own ConvertRequest
  │
  ├─ Pick upstream key-model (random + quota-aware)
  │
  ├─ Decrypt API key (AES-GCM)
  │
  ├─ DoRequest(ctx, info, body) → upstream HTTP call
  │
  ├─ DoResponse(ctx, resp, info, writer) → *common.Usage
  │   [bridge path] → buildRequestInfo(info) → executor.TransformResponse(ctx, ri, resp, writer)
  │   [legacy path] → adaptor's own DoResponse
  │
  ├─ Record usage (llm_usage_records)
  │
  ├─ Settle: SubmitAndSettle → postLedger → accounts UPDATE
  │
  └─ Quota tracking + overdraft check
```

### Bridge Adaptor — The Sharing Mechanism

`server/internal/relay/channel/bridge/bridge.go` wraps `pkg/executor.Executor` to implement `common.Adaptor`.

```go
type Adaptor struct {
    exec pkgexecutor.Executor
    info *common.RelayInfo
}

func buildRequestInfo(info *common.RelayInfo) *pkgexecutor.RequestInfo {
    return &pkgexecutor.RequestInfo{
        InboundFormat:   relayFormatToString(info.InboundFormat),
        ClientFormat:    string(info.GetOriginalClientFormat()),
        RelayMode:       mapRelayMode(info.RelayMode),
        Model:           info.OriginModelName,
        ActualModelName: info.ChannelMeta.UpstreamModelName,
        ApiKey:          info.ApiKey,
        IsStream:        info.IsStream,
        // ... channel, protocol mapped from info.ChannelMeta
    }
}
```

Each adaptor method is a thin wrapper:
- `ConvertRequest` → `executor.TransformRequest`
- `DoRequest` → `executor.DoRequest`
- `DoResponse` → `executor.TransformResponse`

### Platform-Specific Format Detection (Separate from translator)

`server/internal/relay/helper/detect_format.go` has its own detection logic that mirrors `translator.DetectFormat` but returns `constant.RelayFormat`:

```go
func DetectFormatFromPath(path string) (RelayFormat, bool)
func DetectInboundFormat(body []byte) RelayFormat
```

This is a duplication — could be unified via `translator.DetectFormat` + type conversion.

### Migrated vs Legacy Adaptors

**Migrated (via bridge → pkg/executor):**
OpenAI, Azure, AI360, Lingyi, OpenRouter, XInference, Claude, Gemini, DeepSeek

**Legacy (own adaptor, inline conversion):**
Ali, Zhipu, Moonshot, Baidu, Volcengine, Tencent, MiniMax, Xunfei, Coze, Dify, Ollama, Jimeng, Codex, AWS, Node, SiliconFlow, SubModel, Cloudflare, Mistral

---

## 6. Sharing Architecture

```
                    ┌─────────────────────────┐
                    │    pkg/translator        │
                    │  Pure JSON conversion    │
                    │  Normalize / Denormalize │
                    │  DetectFormat            │
                    └──────────┬──────────────┘
                               │
              ┌────────────────┼────────────────────┐
              │                │                     │
    ┌─────────▼──────┐  ┌─────▼──────┐  ┌──────────▼──────────┐
    │  pkg/executor   │  │  Node      │  │  Platform Relay     │
    │                 │  │            │  │                     │
    │  TransformReq   │  │  server.go │  │  constant.RelayFmt  │
    │  TransformResp  │  │  handler   │  │  helper.DetectFmt   │
    │  shouldPassthru │  │  direct    │  │  handler.Handle()   │
    │  RequestInfo    │  │  call      │  └──────────┬──────────┘
    └────────┬───────┘  │            │             │
             │          └────────────┘    ┌────────▼────────┐
             │                            │   bridge.Adaptor │
             └────────────────────────────┤                  │
                                          │ buildRequestInfo │
                                          │ RelayInfo → Req  │
                                          └──────────────────┘
```

### Format Detection — Two Paths

| Scope | Node | Platform |
|-------|------|----------|
| Inbound detection | Fixed per route handler | `helper.DetectFormatFromPath` + `helper.DetectInboundFormat` |
| Output format | `X-Response-Format` header | `X-Response-Format` header + `body.output_format` |
| Unification target | — | `translator.DetectFormat` + bridge type-cast |

### Conversion Engine — Shared

Both Node and Platform (via bridge) route through `pkg/executor.TransformRequest` → `translator.Normalize`, then `TransformResponse` → `translator.Denormalize`.

### Executor Instances — Shared

Same `pkg/executor` instances serve both:
- Node: `executor.GetByProvider(type)` called from `provider.go`
- Platform: `executor.GetByProvider(type)` called from `bridge.Init()`

### Format Constants — Not Shared (Tech Debt)

`pkg/translator.Format` and `constant.RelayFormat` have identical values but no import relationship. Bridge does string conversion.

---

## 7. Format Decision Matrix

Given executor native formats, inbound format, and client format:

### Request to Upstream (via FormatPlan)

| Executor Natively Supports | Inbound → ClientOutput | Upstream (Plan picks) | Conversions |
|---|---|---|---|
| OpenAIChat | OpenAIChat → unset | OpenAIChat | score 0, passthrough |
| OpenAIChat | Claude → openai | OpenAIChat | req conv (claude→openai), response passthrough |
| OpenAIChat | OpenAIChat → claude | OpenAIChat | req passthrough, resp conv (denormalize) |
| ClaudeMessages | Claude → unset | ClaudeMessages | score 0, passthrough |
| ClaudeMessages | OpenAIChat → claude | ClaudeMessages | req conv (denormalize), response passthrough |
| DeepSeek (dual) | OpenAIChat → unset | OpenAIChat | score 0, passthrough |
| DeepSeek (dual) | Claude → unset | ClaudeMessages | score 0, passthrough |
| DeepSeek (dual) | OpenAIChat → claude | **ClaudeMessages** | req conv (openai→claude), response passthrough |
| DeepSeek (dual) | Claude → openai | **OpenAIChat** | req conv (claude→openai), response passthrough |
| DeepSeek (dual) | Gemini → openai | OpenAIChat | req conv (gemini→openai), response passthrough |
| Gemini | Gemini → unset | Gemini | score 0, passthrough |
| Gemini | OpenAIChat → gemini | Gemini | req conv (openai→gemini), response passthrough |
| Gemini | OpenAIChat → claude | Gemini | req conv (openai→gemini), resp conv (gemini→claude via denorm) |

### Response to Client

| ClientFormat | Upstream Response | Response to Client |
|---|---|---|
| unset (=Inbound) | passthrough | Upstream native |
| openai | OpenAIChat → passthrough | OpenAIChat |
| openai | Claude → denormalize | OpenAIChat |
| claude | Claude → passthrough | ClaudeMessages |
| claude | OpenAIChat → denormalize | ClaudeMessages |
| gemini | Gemini → passthrough | Gemini |
| gemini | OpenAIChat → denormalize | Gemini |
| openai_responses | OpenAIChat → denormalize | OpenAIResponses |

---

## 8. Key Files Reference

### Node Side

| File | Role |
|------|------|
| `node/pkg/server/server.go` | HTTP handlers, route→InboundFormat mapping, ClientFormat header |
| `node/pkg/provider/provider.go` | ExecuteWithWriter, Execute, ExecuteStream — builds channel, calls executor pipeline |
| `node/pkg/router/router.go` | Pick, Route, RouteStream — key selection |
| `node/pkg/types/types.go` | RequestEnvelope (InboundFormat, ClientFormat, Request) |

### Shared Packages

| File | Role |
|------|------|
| `pkg/translator/translator.go` | Normalize, Denormalize, DetectFormat, Format types |
| `pkg/executor/executor.go` | Executor interface, RequestInfo, Usage |
| `pkg/executor/plan.go` | FormatPlan, Plan() algorithm, FormatCapable interface, NativeFormat helper |
| `pkg/executor/helper.go` | shouldPassthrough (legacy), resolveFormat, secondsAsDuration |
| `pkg/executor/openai.go` | OpenAIExecutor — base format handling, NativeFormats() |
| `pkg/executor/claude.go` | ClaudeExecutor, NativeFormats() |
| `pkg/executor/deepseek.go` | DeepSeekExecutor — dual-native, Plan()-based TransformRequest |
| `pkg/executor/gemini.go` | GeminiExecutor, NativeFormats() |
| `pkg/executor/ali.go` | AliExecutor, NativeFormats() (openai + claude) |
| `pkg/executor/zhipu.go` | ZhipuExecutor, NativeFormats() (openai + claude) |
| `pkg/executor/minimax.go` | MiniMaxExecutor, NativeFormats() (openai + claude) |
| `pkg/executor/baidu.go` | BaiduExecutor, NativeFormats() |
| `pkg/executor/tencent.go` | TencentExecutor, NativeFormats() |
| `pkg/model/model.go` | ProviderType, RelayMode, Channel, ProtocolEntry |

### Platform Side

| File | Role |
|------|------|
| `server/internal/relay/handler/handler.go` | Handle(), detectInboundFormat, extractOutputFormat, RelayInfo building |
| `server/internal/relay/channel/bridge/bridge.go` | Adaptor wrapper, buildRequestInfo — RelayInfo→RequestInfo mapping |
| `server/internal/relay/channel/registry.go` | GetAdaptor — provider type → adaptor selection |
| `server/internal/relay/constant/relay_format.go` | RelayFormat type + constants |
| `server/internal/relay/helper/detect_format.go` | DetectFormatFromPath, DetectInboundFormat |
| `server/internal/relay/common/relay_info.go` | RelayInfo struct |

---

## 9. Pending Gaps

| Gap | Impact | Location |
|-----|--------|----------|
| Platform `DetectFormat` duplicates `translator.DetectFormat` | Two implementations with same logic | `server/internal/relay/helper/` |
| Three independent format constant sets | Unnecessary indirection, potential drift | All locations |
| Streaming tool_use conversion in Claude→OpenAI | Tool calls in streaming are dropped in SSE conversion | `pkg/executor/claude.go` handleStreamToClientFormat |
| ClaudeExecutor streaming passthrough does not check ClientFormat | Native Claude SSE always sent as-is, no conversion | `pkg/executor/claude.go` handleStream |
| `canConvertRequest` only supports direct paths (any→openai, openai→claude, openai→gemini) | Score-2 request conversions via openai intermediate not auto-selected | `pkg/executor/plan.go` |
| `canConvertResponse` always returns true for non-openai→any | May route to 2-step conversion when direct path exists | `pkg/executor/plan.go` |
