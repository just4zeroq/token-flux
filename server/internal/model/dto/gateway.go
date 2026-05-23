package dto

import "time"

type ChannelInfo struct {
	ID        int64     `json:"id"`
	ItemID    int64     `json:"item_id"`
	Provider  string    `json:"provider"`
	BaseURL   string    `json:"base_url"`
	Priority  int       `json:"priority"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateChannelIn struct {
	ItemID   int64  `json:"item_id" v:"required"`
	Provider string `json:"provider" v:"required"`
	BaseURL  string `json:"base_url" v:"required"`
	Key      string `json:"key" v:"required"`
	Priority int    `json:"priority"`
}

type UpdateChannelStatusIn struct {
	Status string `json:"status" v:"required|in:active,inactive"`
}
