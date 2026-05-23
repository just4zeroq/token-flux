package dto

import "time"

type DepositAddressInfo struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Chain     string    `json:"chain"`
	Address   string    `json:"address"`
	HdPath    string    `json:"hd_path"`
	CreatedAt time.Time `json:"created_at"`
}

type ChainDepositInfo struct {
	ID              int64     `json:"id"`
	UserID          int64     `json:"user_id"`
	Chain           string    `json:"chain"`
	TxHash          string    `json:"tx_hash"`
	FromAddr        string    `json:"from_addr"`
	ToAddr          string    `json:"to_addr"`
	AmountNative    string    `json:"amount_native"`
	AmountUsdAtTime int64     `json:"amount_usd_at_time"`
	Status          string    `json:"status"`
	ObservedAt      time.Time `json:"observed_at"`
}

type WithdrawRequestInfo struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	Chain         string    `json:"chain"`
	ToAddress     string    `json:"to_address"`
	AmountBalance int64     `json:"amount_balance"`
	Fee           int64     `json:"fee"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

type CreateDepositAddressIn struct {
	Chain string `json:"chain" v:"required"`
}

type CreateWithdrawIn struct {
	Chain         string `json:"chain" v:"required"`
	ToAddress     string `json:"to_address" v:"required"`
	AmountBalance int64  `json:"amount_balance" v:"required|min:1"`
}
