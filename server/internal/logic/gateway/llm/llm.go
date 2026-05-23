package llm

import (
	"context"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

type sLLM struct{}

func init() { service.RegisterLLMRuntime(New()) }

func New() *sLLM { return &sLLM{} }

// ListChannels returns upstream channels, optionally filtered by itemID.
// Results are ordered by priority ascending.
func (s *sLLM) ListChannels(ctx context.Context, itemID int64) ([]*dto.ChannelInfo, error) {
	m := g.DB().Model("upstream_channels").Ctx(ctx)
	if itemID > 0 {
		m = m.Where("item_id", itemID)
	}
	records, err := m.Order("priority ASC").All()
	if err != nil {
		return nil, gerror.Wrap(err, "query channels failed")
	}
	out := make([]*dto.ChannelInfo, 0, len(records))
	for _, r := range records {
		out = append(out, mapChannel(r))
	}
	return out, nil
}

// GetChannel returns a single upstream channel by id.
func (s *sLLM) GetChannel(ctx context.Context, id int64) (*dto.ChannelInfo, error) {
	record, err := g.DB().Model("upstream_channels").Ctx(ctx).Where("id", id).One()
	if err != nil {
		return nil, gerror.Wrap(err, "query channel failed")
	}
	if record == nil {
		return nil, gerror.New("channel not found")
	}
	return mapChannel(record), nil
}

// CreateChannel inserts a new upstream channel and returns the created record.
func (s *sLLM) CreateChannel(ctx context.Context, in dto.CreateChannelIn) (*dto.ChannelInfo, error) {
	id, err := g.DB().Model("upstream_channels").Ctx(ctx).InsertAndGetId(g.Map{
		"item_id":       in.ItemID,
		"provider":      in.Provider,
		"base_url":      in.BaseURL,
		"key_encrypted": in.Key,
		"priority":      in.Priority,
	})
	if err != nil {
		return nil, gerror.Wrap(err, "insert channel failed")
	}

	record, err := g.DB().Model("upstream_channels").Ctx(ctx).Where("id", id).One()
	if err != nil {
		return nil, gerror.Wrap(err, "query inserted channel failed")
	}
	return mapChannel(record), nil
}

// UpdateChannelStatus updates the status of a channel by id.
func (s *sLLM) UpdateChannelStatus(ctx context.Context, id int64, status string) error {
	result, err := g.DB().Model("upstream_channels").Ctx(ctx).Where("id", id).Update(g.Map{
		"status": status,
	})
	if err != nil {
		return gerror.Wrap(err, "update channel status failed")
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return gerror.New("channel not found")
	}
	return nil
}

// mapChannel converts a raw DB record to ChannelInfo DTO.
func mapChannel(r gdb.Record) *dto.ChannelInfo {
	return &dto.ChannelInfo{
		ID:        r["id"].Int64(),
		ItemID:    r["item_id"].Int64(),
		Provider:  r["provider"].String(),
		BaseURL:   r["base_url"].String(),
		Priority:  r["priority"].Int(),
		Status:    r["status"].String(),
		CreatedAt: r["created_at"].Time(),
	}
}
