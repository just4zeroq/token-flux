package llm

import (
	"context"
	"encoding/json"

	"ai-platform/internal/model/dto"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// buildAllowedProtocolMap builds a lookup map from the DTO list.
func buildAllowedProtocolMap() map[string]bool {
	m := make(map[string]bool, len(dto.AllowedProtocolKeys))
	for _, k := range dto.AllowedProtocolKeys {
		m[k] = true
	}
	return m
}

type channelRow struct {
	ID               int64       `json:"id"`
	Code             string      `json:"code"`
	Name             string      `json:"name"`
	Description      string      `json:"description"`
	ProtocolsJson    string      `json:"protocols_json"`
	Status           string      `json:"status"`
	SourceType       string      `json:"source_type"`
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
		SourceType:      r.SourceType,
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

// protocolsToJSON converts a slice of ProtocolEntry to the internal JSON object format.
// Input:  [{protocol:"openai-compatible", base_url:"https://..."}]
// Output: {"openai-compatible": {"base_url": "https://..."}}
func protocolsToJSON(entries []dto.ProtocolEntry) (string, error) {
	if len(entries) == 0 {
		return "", gerror.New("at least one protocol entry is required")
	}
	allowed := buildAllowedProtocolMap()
	obj := make(map[string]map[string]string, len(entries))
	for _, e := range entries {
		if !allowed[e.Protocol] {
			return "", gerror.Newf("unknown protocol: %s (allowed: %v)", e.Protocol, dto.AllowedProtocolKeys)
		}
		if e.BaseURL == "" {
			return "", gerror.Newf("base_url is required for protocol: %s", e.Protocol)
		}
		obj[e.Protocol] = map[string]string{"base_url": e.BaseURL}
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return "", gerror.Wrap(err, "marshal protocols to JSON failed")
	}
	return string(b), nil
}

func (s *sLLM) CreateChannel(ctx context.Context, in dto.LLMCreateChannelIn) (*dto.LLMChannelInfo, error) {
	// Convert structured protocol entries to internal JSON object format.
	protocolsJSON, err := protocolsToJSON(in.Protocols)
	if err != nil {
		return nil, err
	}

	sourceType := in.SourceType
	if sourceType == "" {
		sourceType = "admin"
	}

	// Provider-added channels start as pending (need admin review).
	// Admin-created channels start active directly.
	status := "active"
	if sourceType == "provider" {
		status = "pending"
	}

	data := g.Map{
		"code":           in.Code,
		"name":           in.Name,
		"description":    in.Description,
		"protocols_json": protocolsJSON,
		"source_type":    sourceType,
		"status":         status,
	}
	if in.CreatedByUserID != 0 {
		data["created_by_user_id"] = in.CreatedByUserID
	}
	result, err := g.DB().Model("llm_channels").Ctx(ctx).Data(data).Insert()
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
	if in.SourceType != "" {
		model = model.Where("source_type", in.SourceType)
	}
	if in.CreatedByUserID != 0 {
		model = model.Where("created_by_user_id", in.CreatedByUserID)
	}
	// ProviderUserID: show admin channels OR channels owned by this user.
	if in.ProviderUserID != 0 {
		model = model.Where("(source_type = 'admin' OR created_by_user_id = ?)", in.ProviderUserID)
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

func (s *sLLM) DeleteChannel(ctx context.Context, id int64) error {
	rows, err := g.DB().Model("llm_channels").Ctx(ctx).Where("id", id).Delete()
	if err != nil {
		return gerror.Wrap(err, "delete channel failed")
	}
	affected, _ := rows.RowsAffected()
	if affected == 0 {
		return gerror.New("channel not found")
	}
	return nil
}