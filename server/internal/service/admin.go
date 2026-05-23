package service

import (
	"context"

	"ai-platform/internal/model/dto"
)

// IAdmin is the contract for the admin domain.
type IAdmin interface {
	ListUsers(ctx context.Context, page, pageSize int) ([]*dto.AdminUserInfo, int, error)
	GetUser(ctx context.Context, userID int64) (*dto.AdminUserInfo, error)
	UpdateUserStatus(ctx context.Context, in dto.UpdateUserStatusIn) error
	GetStats(ctx context.Context) (*dto.AdminStats, error)
}

var localAdmin IAdmin

// RegisterAdmin installs the admin implementation. Called once during
// init() from internal/logic/admin.
func RegisterAdmin(i IAdmin) { localAdmin = i }

// Admin returns the registered admin service. Panics if no implementation
// has been registered, which indicates a missing blank import of internal/logic.
func Admin() IAdmin {
	if localAdmin == nil {
		panic("service.Admin not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localAdmin
}
