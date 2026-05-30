package llm

import (
	"context"
	"encoding/json"
	"strings"

	"ai-platform/internal/model/dto"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

type modelSpecRow struct {
	ID                 int64       `json:"id"`
	DeveloperName      string      `json:"developer_name"`
	ModelName          string      `json:"model_name"`
	ModelCode          string      `json:"model_code"`
	DisplayName        string      `json:"display_name"`
	ModelFamily        string      `json:"model_family"`
	Description        string      `json:"description"`
	CapabilitiesJson   string      `json:"capabilities_json"`
	ContextWindow      int         `json:"context_window"`
	MaxInputTokens     int         `json:"max_input_tokens"`
	MaxOutputTokens    int         `json:"max_output_tokens"`
	SupportsStream     bool        `json:"supports_stream"`
	SupportsTools      bool        `json:"supports_tools"`
	SupportsVision     bool        `json:"supports_vision"`
	SupportsJsonMode   bool        `json:"supports_json_mode"`
	SupportsReasoning  bool        `json:"supports_reasoning"`
	SupportsLogprobs   bool        `json:"supports_logprobs"`
	SupportedParamsJson string     `json:"supported_params_json"`
	DefaultParamsJson  string      `json:"default_params_json"`
	ParamLimitsJson    string      `json:"param_limits_json"`
	SourceType         string      `json:"source_type"`
	CreatedByUserID    int64       `json:"created_by_user_id"`
	Status             string      `json:"status"`
	ReviewedByUserID   int64       `json:"reviewed_by_user_id"`
	ReviewedAt         *gtime.Time `json:"reviewed_at"`
	ReviewNote         string      `json:"review_note"`
	CreatedAt          gtime.Time  `json:"created_at"`
	UpdatedAt          gtime.Time  `json:"updated_at"`
}

func (r *modelSpecRow) toDTO() *dto.LLMModelSpecInfo {
	return &dto.LLMModelSpecInfo{
		ID:                  r.ID,
		DeveloperName:       r.DeveloperName,
		ModelName:           r.ModelName,
		ModelCode:           r.ModelCode,
		DisplayName:         r.DisplayName,
		ModelFamily:         r.ModelFamily,
		Description:         r.Description,
		CapabilitiesJson:    r.CapabilitiesJson,
		ContextWindow:       r.ContextWindow,
		MaxInputTokens:      r.MaxInputTokens,
		MaxOutputTokens:     r.MaxOutputTokens,
		SupportsStream:      r.SupportsStream,
		SupportsTools:       r.SupportsTools,
		SupportsVision:      r.SupportsVision,
		SupportsJsonMode:    r.SupportsJsonMode,
		SupportsReasoning:   r.SupportsReasoning,
		SupportsLogprobs:    r.SupportsLogprobs,
		SupportedParamsJson: r.SupportedParamsJson,
		DefaultParamsJson:   r.DefaultParamsJson,
		ParamLimitsJson:     r.ParamLimitsJson,
		SourceType:          r.SourceType,
		CreatedByUserID:     r.CreatedByUserID,
		Status:              r.Status,
		CreatedAt:           r.CreatedAt.Time,
		UpdatedAt:           r.UpdatedAt.Time,
	}
}

func (s *sLLM) CreateModelSpec(ctx context.Context, in dto.LLMCreateModelSpecIn) (*dto.LLMModelSpecInfo, error) {
	// Validate capabilities_json is a valid JSON array.
	var caps []any
	if err := json.Unmarshal([]byte(in.CapabilitiesJson), &caps); err != nil {
		return nil, gerror.Wrap(err, "capabilities_json is not a valid JSON array")
	}

	// Generate model_code if empty.
	modelCode := in.ModelCode
	if modelCode == "" {
		modelCode = strings.ToLower(in.DeveloperName + "/" + in.ModelName)
	}

	data := g.Map{
		"developer_name":        in.DeveloperName,
		"model_name":            in.ModelName,
		"model_code":            modelCode,
		"display_name":          in.DisplayName,
		"model_family":          in.ModelFamily,
		"description":           in.Description,
		"capabilities_json":     in.CapabilitiesJson,
		"context_window":        in.ContextWindow,
		"max_input_tokens":      in.MaxInputTokens,
		"max_output_tokens":     in.MaxOutputTokens,
		"supports_stream":       in.SupportsStream,
		"supports_tools":        in.SupportsTools,
		"supports_vision":       in.SupportsVision,
		"supports_json_mode":    in.SupportsJsonMode,
		"supports_reasoning":    in.SupportsReasoning,
		"supports_logprobs":     in.SupportsLogprobs,
		"supported_params_json": in.SupportedParamsJson,
		"default_params_json":   in.DefaultParamsJson,
		"param_limits_json":     in.ParamLimitsJson,
		"source_type":           in.SourceType,
		"status":                "pending",
	}
	if in.CreatedByUserID != 0 {
		data["created_by_user_id"] = in.CreatedByUserID
	}
	result, err := g.DB().Model("llm_model_specs").Ctx(ctx).Data(data).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "insert model spec failed")
	}

	id, _ := result.LastInsertId()
	var row modelSpecRow
	err = g.DB().Model("llm_model_specs").Ctx(ctx).Where("id", id).Scan(&row)
	if err != nil {
		return nil, gerror.Wrap(err, "query created model spec failed")
	}
	return row.toDTO(), nil
}

func (s *sLLM) ListModelSpecs(ctx context.Context, in dto.LLMListModelSpecsIn) ([]*dto.LLMModelSpecInfo, int, error) {
	page := in.Page
	if page <= 0 {
		page = 1
	}
	size := in.Size
	if size <= 0 {
		size = 20
	}

	model := g.DB().Model("llm_model_specs").Ctx(ctx)
	if in.Status != "" {
		model = model.Where("status", in.Status)
	}
	if in.SourceType != "" {
		model = model.Where("source_type", in.SourceType)
	}
	if in.CreatedByUserID != 0 {
		model = model.Where("created_by_user_id", in.CreatedByUserID)
	}
	if in.ProviderUserID != 0 {
		model = model.Where("(source_type = 'admin' OR created_by_user_id = ?)", in.ProviderUserID)
	}

	total, err := model.Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count model specs failed")
	}

	var rows []*modelSpecRow
	err = model.Page(page, size).Order("id DESC").Scan(&rows)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query model specs failed")
	}

	list := make([]*dto.LLMModelSpecInfo, len(rows))
	for i, r := range rows {
		list[i] = r.toDTO()
	}
	return list, total, nil
}

func (s *sLLM) ReviewModelSpec(ctx context.Context, in dto.LLMReviewModelSpecIn) error {
	var row modelSpecRow
	err := g.DB().Model("llm_model_specs").Ctx(ctx).Where("id", in.ID).Scan(&row)
	if err != nil {
		return gerror.Wrap(err, "query model spec failed")
	}
	if row.ID == 0 {
		return gerror.New("model spec not found")
	}
	if row.Status != "pending" {
		return gerror.Newf("model spec status is %s, only pending specs can be reviewed", row.Status)
	}

	_, err = g.DB().Model("llm_model_specs").Ctx(ctx).
		Where("id", in.ID).
		Data(g.Map{
			"status":              in.Status,
			"reviewed_by_user_id": in.ReviewerID,
			"reviewed_at":         gtime.Now(),
			"review_note":         in.ReviewNote,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "update model spec review failed")
	}

	return nil
}

func (s *sLLM) DeleteModelSpec(ctx context.Context, id int64) error {
	rows, err := g.DB().Model("llm_model_specs").Ctx(ctx).Where("id", id).Delete()
	if err != nil {
		return gerror.Wrap(err, "delete model spec failed")
	}
	affected, _ := rows.RowsAffected()
	if affected == 0 {
		return gerror.New("model spec not found")
	}
	return nil
}