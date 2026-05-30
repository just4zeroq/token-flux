package service

import (
	"context"

	"ai-platform/internal/model/dto"
)

// IDeveloper manages developer/provider records used by the frontend filter.
type IDeveloper interface {
	// ListActive returns all developers with status=1, ordered by sort_order then name.
	ListActive(ctx context.Context) ([]*dto.DeveloperInfo, error)
}

var localDeveloper IDeveloper

func RegisterDeveloper(i IDeveloper) { localDeveloper = i }

func Developer() IDeveloper {
	if localDeveloper == nil {
		panic("service.Developer not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localDeveloper
}
