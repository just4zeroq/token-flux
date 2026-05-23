package dto

import "time"

type AccountInfo struct {
	ID           int64     `json:"id"`
	OwnerType    string    `json:"owner_type"`
	OwnerID      int64     `json:"owner_id"`
	Asset        string    `json:"asset"`
	BalanceMicro int64     `json:"balance_micro"`
	CreatedAt    time.Time `json:"created_at"`
}

type TransactionInfo struct {
	ID        int64                  `json:"id"`
	TxType    string                 `json:"tx_type"`
	RefType   string                 `json:"ref_type,omitempty"`
	RefID     int64                  `json:"ref_id,omitempty"`
	Entries   []TransactionEntryInfo `json:"entries"`
	CreatedAt time.Time              `json:"created_at"`
}

type TransactionEntryInfo struct {
	ID                int64 `json:"id"`
	AccountID         int64 `json:"account_id"`
	DeltaMicro        int64 `json:"delta_micro"`
	BalanceAfterMicro int64 `json:"balance_after_micro"`
}

type CreditAccountIn struct {
	OwnerType   string `json:"owner_type" v:"required"`
	OwnerID     int64  `json:"owner_id" v:"required"`
	Asset       string `json:"asset" v:"required"`
	AmountMicro int64  `json:"amount_micro" v:"required|min:1"`
	RefType     string `json:"ref_type"`
	RefID       int64  `json:"ref_id"`
}

type DebitAccountIn struct {
	OwnerType   string `json:"owner_type" v:"required"`
	OwnerID     int64  `json:"owner_id" v:"required"`
	Asset       string `json:"asset" v:"required"`
	AmountMicro int64  `json:"amount_micro" v:"required|min:1"`
	RefType     string `json:"ref_type"`
	RefID       int64  `json:"ref_id"`
}
