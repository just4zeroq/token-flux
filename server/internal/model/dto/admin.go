package dto

import "time"

// AdminUserInfo is the admin-facing user detail, including internal fields.
type AdminUserInfo struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Avatar      string    `json:"avatar"`
	Role        int       `json:"role"`
	KYCStatus   string    `json:"kyc_status"`
	Status      int       `json:"status"`
	Source      string    `json:"source"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UpdateUserStatusIn is the request to change a user's status.
type UpdateUserStatusIn struct {
	UserID int64 `json:"user_id" v:"required"`
	Status int   `json:"status" v:"required"`
}

// AdminStats holds aggregate counts across entities.
type AdminStats struct {
	TotalUsers        int `json:"total_users"`
	ActiveUsers       int `json:"active_users"`
	TotalAPIKeys      int `json:"total_api_keys"`
	TotalOrders       int `json:"total_orders"`
	TotalUsageRecords int `json:"total_usage_records"`
}
