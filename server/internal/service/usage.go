package service

import (
	"context"

	"ai-platform/internal/model/dto"
)

type IUsage interface {
	ReportUsage(ctx context.Context, in dto.ReportUsageIn) (*dto.UsageRecordInfo, error)
	ListUsage(ctx context.Context, userID int64, page, pageSize int) ([]*dto.UsageRecordInfo, int, error)
	GetUsageStats(ctx context.Context, userID int64) (*dto.UsageStats, error)
}

var localUsage IUsage

func RegisterUsage(i IUsage) { localUsage = i }

func Usage() IUsage {
	if localUsage == nil {
		panic("service.Usage not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localUsage
}
