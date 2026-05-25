package gateway

import (
	"context"
	"io"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

func Models(r *ghttp.Request) {
	userID := r.GetCtxVar("user_id").Int64()
	apiKeyID := r.GetCtxVar("api_key_id").Int64()
	models, err := service.LLM().ListAvailableModels(r.Context(), apiKeyID, userID)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{"error": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"object": "list", "data": models})
}

func ChatCompletions(r *ghttp.Request) { proxy(r, "chat", service.LLM().ProxyChatCompletions) }
func Completions(r *ghttp.Request)     { proxy(r, "completion", service.LLM().ProxyCompletions) }
func Embeddings(r *ghttp.Request)      { proxy(r, "embedding", service.LLM().ProxyEmbeddings) }

func proxy(r *ghttp.Request, capability string, fn func(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error)) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"error": "invalid request body"})
		return
	}
	res, err := fn(r.Context(), dto.OpenAIProxyRequest{
		UserID:     r.GetCtxVar("user_id").Int64(),
		ApiKeyID:   r.GetCtxVar("api_key_id").Int64(),
		RequestID:  r.GetCtxVar("request_id").String(),
		Capability: capability,
		RawBody:    body,
		IsStream:   false,
	})
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
