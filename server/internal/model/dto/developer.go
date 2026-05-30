package dto

// DeveloperInfo is the public representation of a developer/provider.
type DeveloperInfo struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Website     string `json:"website"`
	LogoURL     string `json:"logo_url"`
	SortOrder   int    `json:"sort_order"`
	Status      int    `json:"status"`
}
