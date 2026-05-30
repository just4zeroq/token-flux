package dto

import "time"

type InvoiceInfo struct {
	ID            int64      `json:"id"`
	UserID        int64      `json:"user_id"`
	AmountCredits int64      `json:"amount_credits"`
	RefType       string     `json:"ref_type"`
	RefID         int64      `json:"ref_id"`
	InvoiceNumber string     `json:"invoice_number"`
	Status        string     `json:"status"`
	Notes         string     `json:"notes"`
	IssuedAt      *time.Time `json:"issued_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type CreateInvoiceIn struct {
	UserID        int64  `json:"user_id" v:"required|min:1"`
	AmountCredits int64  `json:"amount_credits" v:"required|min:1"`
	RefType       string `json:"ref_type"`
	RefID         int64  `json:"ref_id"`
	Notes         string `json:"notes"`
}

type UpdateInvoiceStatusIn struct {
	Status string `json:"status" v:"required|in:pending,issued,cancelled"`
	Notes  string `json:"notes"`
}
