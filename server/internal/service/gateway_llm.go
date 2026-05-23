package service

import (
	"context"
	"ai-platform/internal/model/dto"
)

type ILLMRuntime interface {
	ListChannels(ctx context.Context, itemID int64) ([]*dto.ChannelInfo, error)
	GetChannel(ctx context.Context, id int64) (*dto.ChannelInfo, error)
	CreateChannel(ctx context.Context, in dto.CreateChannelIn) (*dto.ChannelInfo, error)
	UpdateChannelStatus(ctx context.Context, id int64, status string) error
}

var localLLMRuntime ILLMRuntime

func RegisterLLMRuntime(i ILLMRuntime) { localLLMRuntime = i }

func LLMRuntime() ILLMRuntime {
	if localLLMRuntime == nil {
		panic("service.LLMRuntime not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localLLMRuntime
}
