package usage

import (
	"context"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

type sUsage struct{}

func init() { service.RegisterUsage(New()) }

func New() *sUsage { return &sUsage{} }

// ReportUsage inserts a usage record and returns the newly created record.
func (s *sUsage) ReportUsage(ctx context.Context, in dto.ReportUsageIn) (*dto.UsageRecordInfo, error) {
	id, err := g.DB().Model("usage_records").Ctx(ctx).InsertAndGetId(g.Map{
		"user_id":         in.UserID,
		// api_key_id omitted when 0 to avoid FK violation
		"item_id":         in.ItemID,
		"runtime":         in.Runtime,
		"input_tokens":    in.InputTokens,
		"output_tokens":   in.OutputTokens,
		"call_count":      in.CallCount,
		"latency_ms":      in.LatencyMs,
		"cost_credits":    in.CostCredits,
		"request_id":      in.RequestID,
		"channel_id":      in.ChannelID,
		"is_stream":       in.IsStream,
	})
	if err != nil {
		return nil, gerror.Wrap(err, "insert usage record failed")
	}

	record, err := g.DB().Model("usage_records").Ctx(ctx).Where("id", id).One()
	if err != nil {
		return nil, gerror.Wrap(err, "query inserted usage record failed")
	}
	return mapRecord(record), nil
}

// ListUsage returns a paginated list of usage records for the given user,
// ordered by id descending. Returns the records and total count.
func (s *sUsage) ListUsage(ctx context.Context, userID int64, page, pageSize int) ([]*dto.UsageRecordInfo, int, error) {
	m := g.DB().Model("usage_records").Ctx(ctx).Where("user_id", userID)
	total, err := m.Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count usage records failed")
	}
	if total == 0 {
		return []*dto.UsageRecordInfo{}, 0, nil
	}

	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	records, err := m.Order("id DESC").Offset(offset).Limit(pageSize).All()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query usage records failed")
	}

	out := make([]*dto.UsageRecordInfo, 0, len(records))
	for _, r := range records {
		out = append(out, mapRecord(r))
	}
	return out, total, nil
}

// GetUsageStats returns aggregate usage statistics for the given user.
func (s *sUsage) GetUsageStats(ctx context.Context, userID int64) (*dto.UsageStats, error) {
	record, err := g.DB().Model("usage_records").Ctx(ctx).
		Where("user_id", userID).
		Fields(
			"COALESCE(SUM(call_count),0) AS total_calls",
			"COALESCE(SUM(input_tokens+output_tokens),0) AS total_tokens",
			"COALESCE(SUM(cost_credits),0) AS total_credits",
			"COALESCE(AVG(latency_ms),0) AS avg_latency_ms",
		).
		One()
	if err != nil {
		return nil, gerror.Wrap(err, "query usage stats failed")
	}

	return &dto.UsageStats{
		TotalCalls:   record["total_calls"].Int(),
		TotalTokens:  record["total_tokens"].Int(),
		TotalCredits: record["total_credits"].Int64(),
		AvgLatencyMs: record["avg_latency_ms"].Int(),
	}, nil
}

// mapRecord converts a raw DB record to UsageRecordInfo DTO.
func mapRecord(r gdb.Record) *dto.UsageRecordInfo {
	return &dto.UsageRecordInfo{
		ID:             r["id"].Int64(),
		UserID:         r["user_id"].Int64(),
		ApiKeyID:       r["api_key_id"].Int64(),
		ItemID:         r["item_id"].Int64(),
		Runtime:        r["runtime"].String(),
		CapabilityKind: r["capability_kind"].String(),
		InputTokens:    r["input_tokens"].Int(),
		OutputTokens:   r["output_tokens"].Int(),
		CallCount:      r["call_count"].Int(),
		LatencyMs:      r["latency_ms"].Int(),
		CostCredits:    r["cost_credits"].Int64(),
		RequestID:      r["request_id"].String(),
		IsStream:       r["is_stream"].Bool(),
		CreatedAt:      r["created_at"].Time(),
	}
}
