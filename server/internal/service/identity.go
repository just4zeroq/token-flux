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
