package llm

import (
	"ai-platform/internal/middleware"
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

// requireProvider checks the JWT-authenticated user has the provider role.
// Returns the userID, or 0 if access was denied (response already written).
func requireProvider(r *ghttp.Request) int64 {
	role := r.GetCtxVar(middleware.CtxKeyRole).Int()
	if role != dto.RoleProvider {
		r.Response.WriteStatusExit(403, map[string]any{"code": 403, "message": "provider access required"})
		return 0
	}
	return middleware.GetUserID(r)
}

func ProviderListChannels(r *ghttp.Request) {
	userID := requireProvider(r)
	if userID == 0 {
		return
	}
	var in dto.LLMListChannelsIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	// Show built-in channels + own channels.
	in.ProviderUserID = userID
	list, total, err := service.LLM().ListChannels(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"list": list, "total": total, "page": in.Page, "size": in.Size})
}

func ProviderCreateChannel(r *ghttp.Request) {
	userID := requireProvider(r)
	if userID == 0 {
		return
	}
	var in dto.LLMCreateChannelIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	in.SourceType = "provider"
	in.CreatedByUserID = userID
	out, err := service.LLM().CreateChannel(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

func ProviderCreateModel(r *ghttp.Request) {
	userID := requireProvider(r)
	if userID == 0 {
		return
	}
	var in dto.LLMCreateModelSpecIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	in.SourceType = "provider"
	in.CreatedByUserID = userID
	out, err := service.LLM().CreateModelSpec(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

func ProviderListModels(r *ghttp.Request) {
	userID := requireProvider(r)
	if userID == 0 {
		return
	}
	var in dto.LLMListModelSpecsIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	in.ProviderUserID = userID
	list, total, err := service.LLM().ListModelSpecs(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"list": list, "total": total, "page": in.Page, "size": in.Size})
}

func ProviderCreateModelKey(r *ghttp.Request) {
	userID := requireProvider(r)
	if userID == 0 {
		return
	}
	var in dto.LLMCreateModelKeyIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	out, err := service.LLM().CreateModelKey(r.Context(), userID, in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

func ProviderListModelKeys(r *ghttp.Request) {
	userID := requireProvider(r)
	if userID == 0 {
		return
	}
	var in dto.LLMListModelKeysIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	in.ProviderUserID = userID
	in.IsAdmin = false
	list, total, err := service.LLM().ListModelKeys(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"list": list, "total": total, "page": in.Page, "size": in.Size})
}

func ProviderDisableModelKey(r *ghttp.Request) {
	userID := requireProvider(r)
	if userID == 0 {
		return
	}
	keyID := r.Get("id").Int64()
	if keyID == 0 {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid key id"})
		return
	}
	if err := service.LLM().DisableModelKey(r.Context(), userID, keyID, false); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"ok": true})
}

func ProviderBindKeyModel(r *ghttp.Request) {
	userID := requireProvider(r)
	if userID == 0 {
		return
	}
	var in dto.LLMBindKeyModelIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	in.ModelKeyID = r.Get("id").Int64()
	out, err := service.LLM().BindKeyModel(r.Context(), userID, in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

func ProviderListKeyModels(r *ghttp.Request) {
	userID := requireProvider(r)
	if userID == 0 {
		return
	}
	var in dto.LLMListKeyModelsIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": "invalid params: " + err.Error()})
		return
	}
	in.ModelKeyID = r.Get("id").Int64()
	in.ProviderUserID = userID
	in.IsAdmin = false
	list, total, err := service.LLM().ListKeyModels(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]any{"list": list, "total": total, "page": in.Page, "size": in.Size})
}

func ProviderTriggerKeyModelTest(r *ghttp.Request) {
	if userID := requireProvider(r); userID == 0 {
		return
	}
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

