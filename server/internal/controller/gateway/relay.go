package gateway

import (
	"encoding/json"
	"io"

	"ai-platform/internal/relay/handler"

	"github.com/gogf/gf/v2/net/ghttp"
)

// RelayChatCompletions handles /v1/chat/completions via the relay pipeline.
func RelayChatCompletions(r *ghttp.Request) {
	relayProxy(r, "chat")
}

// RelayCompletions handles /v1/completions via the relay pipeline.
func RelayCompletions(r *ghttp.Request) {
	relayProxy(r, "completion")
}

// RelayEmbeddings handles /v1/embeddings via the relay pipeline.
func RelayEmbeddings(r *ghttp.Request) {
	relayProxy(r, "embedding")
}

// RelayResponses handles /v1/responses via the relay pipeline.
func RelayResponses(r *ghttp.Request) {
	relayProxy(r, "chat") // Responses API requests are converted to Chat Completions internally
}

func relayProxy(r *ghttp.Request, capability string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"error": "invalid request body"})
		return
	}

	isStream := detectStream(body)

	req := &handler.RelayRequest{
		Ctx:        r.Context(),
		RC: &handler.RequestContext{
			UserID:    r.GetCtxVar("user_id").Int64(),
			ApiKeyID:  r.GetCtxVar("api_key_id").Int64(),
			RequestID: r.GetCtxVar("request_id").String(),
		},
		Path:       r.Request.URL.Path,
		RawBody:    body,
		IsStream:   isStream,
		Capability: capability,
	}

	if isStream {
		// Streaming: handler writes SSE directly to the client ResponseWriter.
		if err := handler.HandleStream(r.Response.Writer, req); err != nil {
			// Error before first byte — safe to write error response.
			r.Response.WriteStatusExit(500, map[string]any{"error": err.Error()})
		}
		return
	}

	// Non-streaming: handler returns buffered response.
	res, err := handler.Handle(req)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{"error": err.Error()})
		return
	}

	for k, v := range res.Headers {
		r.Response.Header().Set(k, v)
	}
	r.Response.WriteStatus(res.StatusCode)
	r.Response.Write(res.Body)
}

// detectStream checks if the request body has "stream": true.
func detectStream(body []byte) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return false
	}
	if v, ok := raw["stream"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err == nil && b {
			return true
		}
	}
	return false
}
