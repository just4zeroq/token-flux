package dto

import "time"

// ========== Payment Channel ==========

type PaymentChannelInfo struct {
	ID         int64     `json:"id"`
	Channel    string    `json:"channel"`
	Name       string    `json:"name"`
	AppID      string    `json:"app_id"`
	IsProd     int       `json:"is_prod"`
	Status     int       `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// ========== Payment Order ==========

type PaymentOrderInfo struct {
	ID             int64      `json:"id"`
	OrderNo        string     `json:"order_no"`
	UserID         int64      `json:"user_id"`
	Channel        string     `json:"channel"`
	AmountCredits  int64      `json:"amount_credits"`
	AmountFiat     float64    `json:"amount_fiat"`
	Currency       string     `json:"currency"`
	TradeNo        string     `json:"trade_no"`
	Status         string     `json:"status"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	CreditedAt     *time.Time `json:"credited_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// ========== Request/Response DTOs ==========

type CreateRechargeIn struct {
	Channel       string `json:"channel" v:"required|in:alipay,wechat"`
	AmountCredits int64  `json:"amount_credits" v:"required|min:1"`
}

type CreateRechargeOut struct {
	OrderNo string `json:"order_no"`
	PayURL  string `json:"pay_url"`
	QRCode  string `json:"qrcode,omitempty"`
}

type ListPaymentOrdersIn struct {
	Page     int `json:"page" v:"min:1"`
	PageSize int `json:"pageSize" v:"min:1|max:100"`
}

// ========== Provider Application ==========

type ProviderApplicationIn struct {
	Company      string `json:"company" v:"required|max-length:256"`
	Contact      string `json:"contact" v:"required|max-length:128"`
	Email        string `json:"email" v:"required|email|max-length:256"`
	Website      string `json:"website" v:"max-length:512"`
	Bio          string `json:"bio"`
	ModelName    string `json:"model_name" v:"max-length:256"`
	ModelFamily  string `json:"model_family" v:"max-length:128"`
	APIEndpoint  string `json:"api_endpoint" v:"max-length:512"`
	Documentation string `json:"documentation" v:"max-length:512"`
}

// ========== Provider Application ==========

type ProviderApplicationInfo struct {
	ID           int64      `json:"id"`
	UserID       int64      `json:"user_id"`
	Company      string     `json:"company"`
	Contact      string     `json:"contact"`
	Email        string     `json:"email"`
	Website      string     `json:"website"`
	Bio          string     `json:"bio"`
	ModelName    string     `json:"model_name"`
	ModelFamily  string     `json:"model_family"`
	APIEndpoint  string     `json:"api_endpoint"`
	Documentation string    `json:"documentation"`
	Status       string     `json:"status"`
	ReviewNote   string     `json:"review_note"`
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type ReviewProviderApplicationIn struct {
	Status     string `json:"status" v:"required|in:approved,rejected"`
	ShareBps   int    `json:"share_bps"`   // default share (basis points), required when approved
	ReviewNote string `json:"review_note"`
}

// ========== Admin DTOs ==========

type CreatePaymentChannelIn struct {
	Channel    string `json:"channel" v:"required|in:alipay,wechat"`
	Name       string `json:"name" v:"required|max:128"`
	AppID      string `json:"app_id" v:"required|max:64"`
	PrivateKey string `json:"private_key" v:"required"`
	PublicKey  string `json:"public_key"`
	APIKey     string `json:"api_key"`
	CertSN     string `json:"cert_sn"`
	NotifyURL  string `json:"notify_url" v:"max:512"`
	IsProd     int    `json:"is_prod"`
	ConfigJSON string `json:"config_json"`
}

type UpdatePaymentChannelIn struct {
	Status     *int   `json:"status"`
	Name       string `json:"name"`
	AppID      string `json:"app_id"`
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	APIKey     string `json:"api_key"`
	CertSN     string `json:"cert_sn"`
	NotifyURL  string `json:"notify_url"`
	IsProd     *int   `json:"is_prod"`
	ConfigJSON string `json:"config_json"`
}
