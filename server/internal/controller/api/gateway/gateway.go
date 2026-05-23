package gateway

import (
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

// ListChannels handles GET /gateway/channels
func ListChannels(r *ghttp.Request) {
	itemID := r.Get("item_id").Int64()
	channels, err := service.LLMRuntime().ListChannels(r.Context(), itemID)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{
			"code": 500, "message": err.Error(),
		})
		return
	}
	r.Response.WriteJson(channels)
}

// GetChannel handles GET /gateway/channels/:id
func GetChannel(r *ghttp.Request) {
	id := r.Get("id").Int64()
	if id == 0 {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid channel id",
		})
		return
	}
	channel, err := service.LLMRuntime().GetChannel(r.Context(), id)
	if err != nil {
		r.Response.WriteStatusExit(404, map[string]any{
			"code": 404, "message": err.Error(),
		})
		return
	}
	r.Response.WriteJson(channel)
}

// CreateChannel handles POST /gateway/channels
func CreateChannel(r *ghttp.Request) {
	var in dto.CreateChannelIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": err.Error(),
		})
		return
	}
	channel, err := service.LLMRuntime().CreateChannel(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{
			"code": 500, "message": err.Error(),
		})
		return
	}
	r.Response.WriteJson(channel)
}

// UpdateChannelStatus handles PATCH /gateway/channels/:id/status
func UpdateChannelStatus(r *ghttp.Request) {
	id := r.Get("id").Int64()
	if id == 0 {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid channel id",
		})
		return
	}
	var in dto.UpdateChannelStatusIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": err.Error(),
		})
		return
	}
	if err := service.LLMRuntime().UpdateChannelStatus(r.Context(), id, in.Status); err != nil {
		r.Response.WriteStatusExit(500, map[string]any{
			"code": 500, "message": err.Error(),
		})
		return
	}
	r.Response.WriteJson(map[string]any{
		"code": 0, "message": "ok",
	})
}
