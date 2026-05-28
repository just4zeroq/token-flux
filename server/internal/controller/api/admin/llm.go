package admin

import (
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

// ========== Channel ==========

func CreateChannel(r *ghttp.Request) {
	var in dto.LLMCreateChannelIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	out, err := service.LLM().CreateChannel(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

func ListChannels(r *ghttp.Request) {
	var in dto.LLMListChannelsIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	list, total, err := service.LLM().ListChannels(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"list": list, "total": total, "page": in.Page, "size": in.Size})
}

func ReviewChannel(r *ghttp.Request) {
	var in dto.LLMReviewChannelIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	in.ID = r.Get("id").Int64()
	in.ReviewerID = 0
	if err := service.LLM().ReviewChannel(r.Context(), in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"ok": true})
}

// ========== Model Spec ==========

func CreateModel(r *ghttp.Request) {
	var in dto.LLMCreateModelSpecIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	in.SourceType = "admin"
	out, err := service.LLM().CreateModelSpec(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

func ListModels(r *ghttp.Request) {
	var in dto.LLMListModelSpecsIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	list, total, err := service.LLM().ListModelSpecs(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"list": list, "total": total, "page": in.Page, "size": in.Size})
}

func ReviewModel(r *ghttp.Request) {
	var in dto.LLMReviewModelSpecIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	in.ID = r.Get("id").Int64()
	in.ReviewerID = 0
	if err := service.LLM().ReviewModelSpec(r.Context(), in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"ok": true})
}

// ========== Model Price ==========

func UpsertModelPrice(r *ghttp.Request) {
	var in dto.LLMUpsertModelPriceIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	in.ModelSpecID = r.Get("id").Int64()
	out, err := service.LLM().UpsertModelPrice(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

func ListModelPrices(r *ghttp.Request) {
	modelSpecID := r.Get("id").Int64()
	list, err := service.LLM().ListModelPrices(r.Context(), modelSpecID)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"list": list})
}

// ========== Model Keys (admin-wide) ==========

func ListAllModelKeys(r *ghttp.Request) {
	var in dto.LLMListModelKeysIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	in.IsAdmin = true
	list, total, err := service.LLM().ListModelKeys(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"list": list, "total": total, "page": in.Page, "size": in.Size})
}

func ListAllKeyModels(r *ghttp.Request) {
	var in dto.LLMListKeyModelsIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	in.IsAdmin = true
	list, total, err := service.LLM().ListKeyModels(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"list": list, "total": total, "page": in.Page, "size": in.Size})
}

func TriggerKeyModelTest(r *ghttp.Request) {
	keyModelID := r.Get("id").Int64()
	if keyModelID == 0 {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid key model id"})
		return
	}
	if err := service.LLM().TriggerKeyModelTest(r.Context(), keyModelID); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"ok": true})
}
