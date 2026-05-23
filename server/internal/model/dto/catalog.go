package dto

import "time"

// CategoryInfo represents a catalog category.
type CategoryInfo struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ParentID *int64 `json:"parent_id,omitempty"`
}

// ItemInfo represents a catalog item (product/service).
type ItemInfo struct {
	ID           int64     `json:"id"`
	OwnerUserID  int64     `json:"owner_user_id"`
	Type         string    `json:"type"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	ReviewStatus string    `json:"review_status,omitempty"`
	Version      int       `json:"version"`
	ConfigJSON   string    `json:"config_json"`
	Tags         []string  `json:"tags"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
