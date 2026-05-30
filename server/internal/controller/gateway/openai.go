package gateway

import (
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
