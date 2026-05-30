package service

import (
	"context"

	"ai-platform/internal/model/dto"
)

// IIdentity is the contract for the identity domain (users, JWT, API keys).
type IIdentity interface {
	Register(ctx context.Context, in dto.RegisterIn) (*dto.RegisterOut, error)
	VerifyEmail(ctx context.Context, in dto.VerifyEmailIn) (*dto.VerifyEmailOut, error)
	Login(ctx context.Context, in dto.LoginIn) (*dto.LoginOut, error)
	GetUser(ctx context.Context, userID int64) (*dto.UserInfo, error)
	ValidateToken(ctx context.Context, token string) (*dto.TokenClaims, error)
	ValidateApiKey(ctx context.Context, key string) (*dto.ApiKeyInfo, error)
	CreateApiKey(ctx context.Context, userID int64, in dto.CreateApiKeyIn) (*dto.ApiKeyOut, error)
	ListApiKeys(ctx context.Context, userID int64) ([]*dto.ApiKeyInfo, error)
	DeleteApiKey(ctx context.Context, userID, keyID int64) error

	// Admin
	ListUsers(ctx context.Context, page, pageSize int, status, role *int, keyword string) ([]*dto.UserInfo, int, error)
	CreateUser(ctx context.Context, in dto.CreateUserIn) (*dto.UserInfo, error)
	UpdateUserStatus(ctx context.Context, userID int64, status int) error
	UpdateUserRole(ctx context.Context, userID int64, role int) error

	// Provider application
	ApplyProvider(ctx context.Context, userID int64, in dto.ProviderApplicationIn) error
	ListProviderApplications(ctx context.Context, page, pageSize int, status string) ([]*dto.ProviderApplicationInfo, int, error)
	ReviewProviderApplication(ctx context.Context, appID int64, in dto.ReviewProviderApplicationIn) error
}

var localIdentity IIdentity

// RegisterIdentity installs the identity implementation. Called once during
// init() from internal/logic/identity.
func RegisterIdentity(i IIdentity) { localIdentity = i }

// Identity returns the registered identity service. Panics if no implementation
// has been registered, which indicates a missing blank import of internal/logic.
func Identity() IIdentity {
	if localIdentity == nil {
		panic("service.Identity not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localIdentity
}
