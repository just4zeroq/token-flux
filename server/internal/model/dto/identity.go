package dto

import "time"

// ========== Registration ==========

type RegisterIn struct {
	Email    string `json:"email" v:"required|email|length:1,255"`
	Password string `json:"password" v:"required|length:8,128"`
}

type RegisterOut struct {
	Message string `json:"message"`
}

type VerifyEmailIn struct {
	Email string `json:"email" v:"required|email"`
	Code  string `json:"code" v:"required|length:6,6"`
}

type VerifyEmailOut struct {
	Message string `json:"message"`
}

// ========== Login ==========

type LoginIn struct {
	Email    string `json:"email" v:"required|email"`
	Password string `json:"password" v:"required"`
}

type LoginOut struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}

// ========== User ==========

type UserInfo struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar"`
	Role        int    `json:"role"`
	KYCStatus   string `json:"kyc_status"`
}

// ========== JWT / Auth ==========

type TokenClaims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     int    `json:"role"`
}

// ========== API Key ==========

type CreateApiKeyIn struct {
	Name           string `json:"name" v:"required|length:1,64"`
	QuotaCredits   *int64 `json:"quota_credits"`
	UnlimitedQuota bool   `json:"unlimited_quota"`
}

type ApiKeyOut struct {
	ID        int64      `json:"id"`
	Key       string     `json:"key"`
	Name      string     `json:"name"`
	Status    int        `json:"status"`
	ExpireAt  *time.Time `json:"expire_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type ApiKeyInfo struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	Name           string     `json:"name"`
	Status         int        `json:"status"`
	QuotaCredits   *int64     `json:"quota_credits,omitempty"`
	RemainQuota    int64      `json:"remain_quota"`
	UsedQuota      int64      `json:"used_quota"`
	UnlimitedQuota bool       `json:"unlimited_quota"`
	ModelLimits    string     `json:"model_limits"`
	AllowIPs       string     `json:"allow_ips"`
	ExpireAt       *time.Time `json:"expire_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type DeleteApiKeyIn struct {
	KeyID int64 `json:"key_id" v:"required"`
}
