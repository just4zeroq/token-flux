package llm

import (
	"context"
	"encoding/json"

	"ai-platform/internal/model/dto"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

var allowedProtocolKeys = map[string]bool{
	"openai-compatible":     true,
	"anthropic-compatible":  true,
	"gemini-compatible":     true,
	"azure-openai":          true,
}

type channelRow struct {
	ID               int64       `json:"id"`
	Code             string      `json:"code"`
	Name             string      `json:"name"`
	Description      string      `json:"description"`
	ProtocolsJson    string      `json:"protocols_json"`
	Status           string      `json:"status"`
	CreatedByUserID  int64       `json:"created_by_user_id"`
	ReviewedByUserID int64       `json:"reviewed_by_user_id"`
	ReviewedAt       *gtime.Time `json:"reviewed_at"`
	ReviewNote       string      `json:"review_note"`
	CreatedAt        gtime.Time  `json:"created_at"`
	UpdatedAt        gtime.Time  `json:"updated_at"`
}

func (r *channelRow) toDTO() *dto.LLMChannelInfo {
	info := &dto.LLMChannelInfo{
		ID:              r.ID,
		Code:            r.Code,
		Name:            r.Name,
		Description:     r.Description,
		ProtocolsJson:   r.ProtocolsJson,
		Status:          r.Status,
		CreatedByUserID: r.CreatedByUserID,
		ReviewedByUserID: r.ReviewedByUserID,
		ReviewNote:      r.ReviewNote,
		CreatedAt:       r.CreatedAt.Time,
		UpdatedAt:       r.UpdatedAt.Time,
	}
	if r.ReviewedAt != nil {
		info.ReviewedAt = r.ReviewedAt.Time
	}
	return info
}

func (s *sLLM) CreateChannel(ctx context.Context, in dto.LLMCreateChannelIn) (*dto.LLMChannelInfo, error) {
	// Validate protocols_json is valid JSON.
	var protocols map[string]any
	if err := json.Unmarshal([]byte(in.ProtocolsJson), &protocols); err != nil {
		return nil, gerror.Wrap(err, "protocols_json is not valid JSON")
	}
	// Validate protocol keys are in allowed set.
	for k := range protocols {
		if !allowedProtocolKeys[k] {
			return nil, gerror.Newf("unknown protocol key: %s", k)
		}
	}

	result, err := g.DB().Model("llm_channels").Ctx(ctx).Data(g.Map{
		"code":              in.Code,
		"name":              in.Name,
		"description":       in.Description,
		"protocols_json":    in.ProtocolsJson,
		"status":            "pending",
		"created_by_user_id": in.CreatedByUserID,
	}).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "insert channel failed")
	}

	id, _ := result.LastInsertId()
	var row channelRow
	err = g.DB().Model("llm_channels").Ctx(ctx).Where("id", id).Scan(&row)
	if err != nil {
		return nil, gerror.Wrap(err, "query created channel failed")
	}
	return row.toDTO(), nil
}

func (s *sLLM) ListChannels(ctx context.Context, in dto.LLMListChannelsIn) ([]*dto.LLMChannelInfo, int, error) {
	page := in.Page
	if page <= 0 {
		page = 1
	}
	size := in.Size
	if size <= 0 {
		size = 20
	}

	model := g.DB().Model("llm_channels").Ctx(ctx)
	if in.Status != "" {
		model = model.Where("status", in.Status)
	}

	total, err := model.Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count channels failed")
	}

	var rows []*channelRow
	err = model.Page(page, size).Order("id DESC").Scan(&rows)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query channels failed")
	}

	list := make([]*dto.LLMChannelInfo, len(rows))
	for i, r := range rows {
		list[i] = r.toDTO()
	}
	return list, total, nil
}

func (s *sLLM) ReviewChannel(ctx context.Context, in dto.LLMReviewChannelIn) error {
	// Verify channel exists and status is pending.
	var row channelRow
	err := g.DB().Model("llm_channels").Ctx(ctx).Where("id", in.ID).Scan(&row)
	if err != nil {
		return gerror.Wrap(err, "query channel failed")
	}
	if row.ID == 0 {
		return gerror.New("channel not found")
	}
	if row.Status != "pending" {
		return gerror.Newf("channel status is %s, only pending channels can be reviewed", row.Status)
	}

	_, err = g.DB().Model("llm_channels").Ctx(ctx).
		Where("id", in.ID).
		Data(g.Map{
			"status":              in.Status,
			"reviewed_by_user_id": in.ReviewerID,
			"reviewed_at":         gtime.Now(),
			"review_note":         in.ReviewNote,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "update channel review failed")
	}
	return nil
}