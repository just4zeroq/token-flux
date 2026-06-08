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
		fail(r, "invalid params: "+err.Error())
		return
	}
	out, err := service.LLM().CreateChannel(r.Context(), in)
	if err != nil {
		fail(r, err.Error())
		return
	}
	ok(r, out)
}

func ListChannels(r *ghttp.Request) {
	var in dto.LLMListChannelsIn
	if err := r.Parse(&in); err != nil {
		fail(r, "invalid params: "+err.Error())
		return
	}
	if in.Page <= 0 {
		in.Page = 1
	}
	if in.Size <= 0 {
		in.Size = 20
	}
	list, total, err := service.LLM().ListChannels(r.Context(), in)
	if err != nil {
		fail(r, err.Error())
		return
	}
	page(r, list, total, in.Page, in.Size)
}

func ReviewChannel(r *ghttp.Request) {
	var in dto.LLMReviewChannelIn
	if err := r.Parse(&in); err != nil {
		fail(r, "invalid params: "+err.Error())
		return
	}
	in.ID = r.Get("id").Int64()
	in.ReviewerID = 0
	if err := service.LLM().ReviewChannel(r.Context(), in); err != nil {
		fail(r, err.Error())
		return
	}
	okMsg(r, "reviewed")
}

func DeleteChannel(r *ghttp.Request) {
	id := r.Get("id").Int64()
	if err := service.LLM().DeleteChannel(r.Context(), id); err != nil {
		fail(r, err.Error())
		return
	}
	okMsg(r, "deleted")
}

// ========== Model Spec ==========

func CreateModel(r *ghttp.Request) {
	var in dto.LLMCreateModelSpecIn
	if err := r.Parse(&in); err != nil {
		fail(r, "invalid params: "+err.Error())
		return
	}
	in.SourceType = "admin"
	out, err := service.LLM().CreateModelSpec(r.Context(), in)
	if err != nil {
		fail(r, err.Error())
		return
	}
	ok(r, out)
}

func ListModels(r *ghttp.Request) {
	var in dto.LLMListModelSpecsIn
	if err := r.Parse(&in); err != nil {
		fail(r, "invalid params: "+err.Error())
		return
	}
	if in.Page <= 0 {
		in.Page = 1
	}
	if in.Size <= 0 {
		in.Size = 20
	}
	list, total, err := service.LLM().ListModelSpecs(r.Context(), in)
	if err != nil {
		fail(r, err.Error())
		return
	}
	page(r, list, total, in.Page, in.Size)
}

func ReviewModel(r *ghttp.Request) {
	var in dto.LLMReviewModelSpecIn
	if err := r.Parse(&in); err != nil {
		fail(r, "invalid params: "+err.Error())
		return
	}
	in.ID = r.Get("id").Int64()
	in.ReviewerID = 0
	if err := service.LLM().ReviewModelSpec(r.Context(), in); err != nil {
		fail(r, err.Error())
		return
	}
	okMsg(r, "reviewed")
}

func DeleteModel(r *ghttp.Request) {
	id := r.Get("id").Int64()
	if err := service.LLM().DeleteModelSpec(r.Context(), id); err != nil {
		fail(r, err.Error())
		return
	}
	okMsg(r, "deleted")
}

// ========== Model Keys (admin-wide) ==========

func ListAllModelKeys(r *ghttp.Request) {
	var in dto.LLMListModelKeysIn
	if err := r.Parse(&in); err != nil {
		fail(r, "invalid params: "+err.Error())
		return
	}
	if in.Page <= 0 {
		in.Page = 1
	}
	if in.Size <= 0 {
		in.Size = 20
	}
	in.IsAdmin = true
	list, total, err := service.LLM().ListModelKeys(r.Context(), in)
	if err != nil {
		fail(r, err.Error())
		return
	}
	page(r, list, total, in.Page, in.Size)
}

func ListAllKeyModels(r *ghttp.Request) {
	var in dto.LLMListKeyModelsIn
	if err := r.Parse(&in); err != nil {
		fail(r, "invalid params: "+err.Error())
		return
	}
	if in.Page <= 0 {
		in.Page = 1
	}
	if in.Size <= 0 {
		in.Size = 20
	}
	in.IsAdmin = true
	list, total, err := service.LLM().ListKeyModels(r.Context(), in)
	if err != nil {
		fail(r, err.Error())
		return
	}
	page(r, list, total, in.Page, in.Size)
}

func TriggerKeyModelTest(r *ghttp.Request) {
	keyModelID := r.Get("id").Int64()
	if keyModelID == 0 {
		fail(r, "invalid key model id")
		return
	}
	if err := service.LLM().TriggerKeyModelTest(r.Context(), keyModelID); err != nil {
		fail(r, err.Error())
		return
	}
	okMsg(r, "test triggered")
}

// ── Provider Settings ──

func GetProviderSetting(r *ghttp.Request) {
	userID := r.Get("userId").Int64()
	if userID == 0 {
		fail(r, "invalid user id")
		return
	}
	shareBps, err := service.LLM().GetProviderShareBps(r.Context(), userID)
	if err != nil {
		fail(r, err.Error())
		return
	}
	ok(r, map[string]any{"user_id": userID, "share_bps": shareBps})
}

func SetProviderSetting(r *ghttp.Request) {
	userID := r.Get("userId").Int64()
	if userID == 0 {
		fail(r, "invalid user id")
		return
	}
	shareBps := r.Get("share_bps").Int()
	if shareBps <= 0 {
		fail(r, "invalid share_bps")
		return
	}
	if err := service.LLM().SetProviderShareBps(r.Context(), userID, shareBps); err != nil {
		fail(r, err.Error())
		return
	}
	okMsg(r, "ok")
}

// ---- Channel-Model Binding ----

func BindChannelModels(r *ghttp.Request) {
	id := r.Get("id").Int64()
	if id == 0 {
		fail(r, "invalid channel id")
		return
	}
	var in struct {
		ModelIDs []int64 `json:"model_ids"`
	}
	if err := r.Parse(&in); err != nil {
		fail(r, "invalid params: "+err.Error())
		return
	}
	if err := service.LLM().BindChannelModels(r.Context(), id, in.ModelIDs); err != nil {
		fail(r, err.Error())
		return
	}
	okMsg(r, "bound")
}

func ListChannelModels(r *ghttp.Request) {
	channelID := r.Get("id").Int64()
	if channelID == 0 {
		fail(r, "invalid channel id")
		return
	}
	list, err := service.LLM().ListChannelModels(r.Context(), dto.LLMListChannelModelsIn{ChannelID: channelID})
	if err != nil {
		fail(r, err.Error())
		return
	}
	ok(r, list)
}

func UnbindChannelModel(r *ghttp.Request) {
	channelID := r.Get("channelId").Int64()
	modelSpecID := r.Get("modelSpecId").Int64()
	if channelID == 0 || modelSpecID == 0 {
		fail(r, "invalid channel_id or model_spec_id")
		return
	}
	if err := service.LLM().UnbindChannelModel(r.Context(), channelID, modelSpecID); err != nil {
		fail(r, err.Error())
		return
	}
	okMsg(r, "unbound")
}
