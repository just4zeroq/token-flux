package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
	wechat "github.com/go-pay/gopay/wechat/v3"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"ai-platform/internal/logic/systemconfig"
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"
)

type sPayment struct{}

func init() { service.RegisterPayment(New()) }

func New() *sPayment { return &sPayment{} }

// ========== DB Rows ==========

type channelRow struct {
	ID         int64     `json:"id"`
	Channel    string    `json:"channel"`
	Name       string    `json:"name"`
	AppID      string    `json:"app_id"`
	PrivateKey string    `json:"private_key"`
	PublicKey  string    `json:"public_key"`
	APIKey     string    `json:"api_key"`
	CertSN     string    `json:"cert_sn"`
	NotifyURL  string    `json:"notify_url"`
	IsProd     int       `json:"is_prod"`
	Status     int       `json:"status"`
	ConfigJSON string    `json:"config_json"`
	CreatedAt  time.Time `json:"created_at"`
}

type orderRow struct {
	ID            int64      `json:"id"`
	OrderNo       string     `json:"order_no"`
	UserID        int64      `json:"user_id"`
	Channel       string     `json:"channel"`
	AmountCredits int64      `json:"amount_credits"`
	AmountFiat    float64    `json:"amount_fiat"`
	Currency      string     `json:"currency"`
	TradeNo       string     `json:"trade_no"`
	Status        string     `json:"status"`
	PaidAt        *time.Time `json:"paid_at"`
	CreditedAt    *time.Time `json:"credited_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

func (r *orderRow) toDTO() *dto.PaymentOrderInfo {
	return &dto.PaymentOrderInfo{
		ID:            r.ID,
		OrderNo:       r.OrderNo,
		UserID:        r.UserID,
		Channel:       r.Channel,
		AmountCredits: r.AmountCredits,
		AmountFiat:    r.AmountFiat,
		Currency:      r.Currency,
		TradeNo:       r.TradeNo,
		Status:        r.Status,
		PaidAt:        r.PaidAt,
		CreditedAt:    r.CreditedAt,
		CreatedAt:     r.CreatedAt,
	}
}

// ========== Channel ==========

func (s *sPayment) getChannel(ctx context.Context, channel string) (*channelRow, error) {
	var row channelRow
	err := g.DB().Model("payment_channels").Ctx(ctx).
		Where("channel", channel).Where("status", 1).Scan(&row)
	if err != nil {
		return nil, gerror.Wrap(err, "query channel failed")
	}
	if row.ID == 0 {
		return nil, gerror.Newf("channel %s not found or disabled", channel)
	}
	return &row, nil
}

// ========== Order Helpers ==========

func (s *sPayment) genOrderNo(channel string) string {
	now := time.Now()
	return fmt.Sprintf("PAY%s%d%04d", channel[:2], now.Unix(), now.Nanosecond()%10000)
}

// ========== Create Recharge ==========

func (s *sPayment) CreateRecharge(ctx context.Context, userID int64, in dto.CreateRechargeIn) (*dto.CreateRechargeOut, error) {
	ch, err := s.getChannel(ctx, in.Channel)
	if err != nil {
		return nil, err
	}

	orderNo := s.genOrderNo(in.Channel)
	amountFiat := float64(in.AmountCredits) / float64(systemconfig.GetInt(ctx, "billing.credits_per_cny", 10))

	_, err = g.DB().Model("payment_orders").Ctx(ctx).Data(g.Map{
		"order_no":       orderNo,
		"user_id":        userID,
		"channel":        in.Channel,
		"amount_credits": in.AmountCredits,
		"amount_fiat":    amountFiat,
		"currency":       "CNY",
		"status":         "pending",
		"created_at":     gtime.Now(),
	}).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "create order failed")
	}

	var payURL string
	switch in.Channel {
	case "alipay":
		payURL, err = s.createAlipayOrder(ctx, ch, orderNo, in.AmountCredits, amountFiat)
	case "wechat":
		payURL, err = s.createWechatOrder(ctx, ch, orderNo, in.AmountCredits, amountFiat)
	default:
		return nil, gerror.Newf("unsupported channel: %s", in.Channel)
	}
	if err != nil {
		return nil, err
	}

	return &dto.CreateRechargeOut{OrderNo: orderNo, PayURL: payURL}, nil
}

// ========== Alipay ==========

func (s *sPayment) createAlipayOrder(ctx context.Context, ch *channelRow, orderNo string, credits int64, fiat float64) (string, error) {
	client, err := alipay.NewClient(ch.AppID, ch.PrivateKey, ch.IsProd == 1)
	if err != nil {
		return "", gerror.Wrap(err, "init alipay client failed")
	}
	client.SetCharset("utf-8").SetSignType(alipay.RSA2)
	client.SetNotifyUrl(ch.NotifyURL + "/api/v1/payment/notify/alipay")
	client.SetReturnUrl(ch.NotifyURL)

	subject := fmt.Sprintf("AI Platform - Recharge %d Credits", credits)
	totalAmount := fmt.Sprintf("%.2f", fiat)

	bm := make(gopay.BodyMap)
	bm.Set("out_trade_no", orderNo)
	bm.Set("total_amount", totalAmount)
	bm.Set("subject", subject)
	bm.Set("product_code", "FAST_INSTANT_TRADE_PAY")

	return client.TradePagePay(ctx, bm)
}

// ========== WeChat Pay ==========

func (s *sPayment) createWechatOrder(ctx context.Context, ch *channelRow, orderNo string, credits int64, fiat float64) (string, error) {
	client, err := wechat.NewClientV3(ch.AppID, ch.CertSN, ch.APIKey, ch.PrivateKey)
	if err != nil {
		return "", gerror.Wrap(err, "init wechat client failed")
	}

	amount := int64(fiat * 100) // convert yuan to fen
	description := fmt.Sprintf("AI Platform - Recharge %d Credits", credits)

	bm := make(gopay.BodyMap)
	bm.Set("appid", gjsonGet(ch.ConfigJSON, "appid"))
	bm.Set("mchid", ch.AppID)
	bm.Set("description", description)
	bm.Set("out_trade_no", orderNo)
	bm.Set("notify_url", ch.NotifyURL+"/api/v1/payment/notify/wechat")
	bm.SetBodyMap("amount", func(b gopay.BodyMap) {
		b.Set("total", amount)
		b.Set("currency", "CNY")
	})

	wxRsp, err := client.V3TransactionNative(ctx, bm)
	if err != nil {
		return "", gerror.Wrap(err, "wechat V3TransactionNative failed")
	}
	return wxRsp.Response.CodeUrl, nil
}

// ========== Admin: List All Orders ==========

func (s *sPayment) ListAllOrders(ctx context.Context, channel, status, keyword string, page, pageSize int) ([]*dto.PaymentOrderInfo, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	model := g.DB().Model("payment_orders").Ctx(ctx)
	if channel != "" {
		model = model.Where("channel", channel)
	}
	if status != "" {
		model = model.Where("status", status)
	}
	if keyword != "" {
		model = model.WhereLike("order_no", "%"+keyword+"%")
	}

	total, err := model.Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count orders failed")
	}

	var rows []*orderRow
	offset := (page - 1) * pageSize
	err = model.Order("id DESC").Limit(pageSize).Offset(offset).Scan(&rows)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query orders failed")
	}

	list := make([]*dto.PaymentOrderInfo, len(rows))
	for i, r := range rows {
		list[i] = r.toDTO()
	}
	return list, total, nil
}

// ========== Admin: Channel Management ==========

func (s *sPayment) ListChannels(ctx context.Context, channel string, page, pageSize int) ([]*dto.PaymentChannelInfo, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	model := g.DB().Model("payment_channels").Ctx(ctx)
	if channel != "" {
		model = model.Where("channel", channel)
	}

	total, err := model.Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count channels failed")
	}

	var rows []*channelRow
	offset := (page - 1) * pageSize
	err = model.Order("id DESC").Limit(pageSize).Offset(offset).Scan(&rows)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query channels failed")
	}

	list := make([]*dto.PaymentChannelInfo, len(rows))
	for i, r := range rows {
		list[i] = &dto.PaymentChannelInfo{
			ID:        r.ID,
			Channel:   r.Channel,
			Name:      r.Name,
			AppID:     r.AppID,
			IsProd:    r.IsProd,
			Status:    r.Status,
			CreatedAt: r.CreatedAt,
		}
	}
	return list, total, nil
}

func (s *sPayment) CreateChannel(ctx context.Context, in dto.CreatePaymentChannelIn) error {
	_, err := g.DB().Model("payment_channels").Ctx(ctx).Data(g.Map{
		"channel":     in.Channel,
		"name":        in.Name,
		"app_id":      in.AppID,
		"private_key": in.PrivateKey,
		"public_key":  in.PublicKey,
		"api_key":     in.APIKey,
		"cert_sn":     in.CertSN,
		"notify_url":  in.NotifyURL,
		"is_prod":     in.IsProd,
		"config_json": in.ConfigJSON,
		"status":      1,
	}).Insert()
	if err != nil {
		return gerror.Wrap(err, "create channel failed")
	}
	return nil
}

func (s *sPayment) UpdateChannel(ctx context.Context, id int64, in dto.UpdatePaymentChannelIn) error {
	data := g.Map{}
	if in.Status != nil {
		data["status"] = *in.Status
	}
	if in.Name != "" {
		data["name"] = in.Name
	}
	if in.AppID != "" {
		data["app_id"] = in.AppID
	}
	if in.PrivateKey != "" {
		data["private_key"] = in.PrivateKey
	}
	if in.PublicKey != "" {
		data["public_key"] = in.PublicKey
	}
	if in.APIKey != "" {
		data["api_key"] = in.APIKey
	}
	if in.CertSN != "" {
		data["cert_sn"] = in.CertSN
	}
	if in.NotifyURL != "" {
		data["notify_url"] = in.NotifyURL
	}
	if in.IsProd != nil {
		data["is_prod"] = *in.IsProd
	}
	if in.ConfigJSON != "" {
		data["config_json"] = in.ConfigJSON
	}
	if len(data) == 0 {
		return gerror.New("no fields to update")
	}

	_, err := g.DB().Model("payment_channels").Ctx(ctx).Where("id", id).Data(data).Update()
	if err != nil {
		return gerror.Wrap(err, "update channel failed")
	}
	return nil
}

// ========== List/Get Orders ==========

func (s *sPayment) ListOrders(ctx context.Context, userID int64, page, pageSize int) ([]*dto.PaymentOrderInfo, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	model := g.DB().Model("payment_orders").Ctx(ctx).Where("user_id", userID)
	total, err := model.Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count orders failed")
	}

	var rows []*orderRow
	offset := (page - 1) * pageSize
	err = model.Order("id DESC").Limit(pageSize).Offset(offset).Scan(&rows)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query orders failed")
	}

	list := make([]*dto.PaymentOrderInfo, len(rows))
	for i, r := range rows {
		list[i] = r.toDTO()
	}
	return list, total, nil
}

func (s *sPayment) GetOrder(ctx context.Context, userID int64, orderNo string) (*dto.PaymentOrderInfo, error) {
	var row orderRow
	err := g.DB().Model("payment_orders").Ctx(ctx).
		Where("order_no", orderNo).Where("user_id", userID).Scan(&row)
	if err != nil {
		return nil, gerror.Wrap(err, "query order failed")
	}
	if row.ID == 0 {
		return nil, gerror.New("order not found")
	}
	return row.toDTO(), nil
}

// ========== Notify Handler ==========

func (s *sPayment) HandleNotify(ctx context.Context, channel string, req *http.Request) error {
	switch channel {
	case "alipay":
		return s.handleAlipayNotify(ctx, req)
	case "wechat":
		return s.handleWechatNotify(ctx, req)
	default:
		return gerror.Newf("unsupported channel: %s", channel)
	}
}

func (s *sPayment) handleAlipayNotify(ctx context.Context, req *http.Request) error {
	notify, err := alipay.ParseNotifyToBodyMap(req)
	if err != nil {
		return gerror.Wrap(err, "parse alipay notify failed")
	}

	orderNo := notify.GetString("out_trade_no")
	tradeNo := notify.GetString("trade_no")
	tradeStatus := notify.GetString("trade_status")
	if tradeStatus != "TRADE_SUCCESS" {
		return nil
	}

	// Verify sign with Alipay public key
	ch, err := s.getChannel(ctx, "alipay")
	if err != nil {
		return err
	}
	client, _ := alipay.NewClient(ch.AppID, ch.PrivateKey, ch.IsProd == 1)
	client.AutoVerifySign([]byte(ch.PublicKey))

	return s.confirmOrder(ctx, orderNo, tradeNo, "alipay", notify.JsonBody())
}

func (s *sPayment) handleWechatNotify(ctx context.Context, req *http.Request) error {
	notifyReq, err := wechat.V3ParseNotify(req)
	if err != nil {
		return gerror.Wrap(err, "parse wechat notify failed")
	}

	ch, err := s.getChannel(ctx, "wechat")
	if err != nil {
		return err
	}

	result, err := notifyReq.DecryptPayCipherText(ch.APIKey)
	if err != nil {
		return gerror.Wrap(err, "decrypt wechat notify failed")
	}
	if result.TradeState != "SUCCESS" {
		return nil
	}

	client, _ := wechat.NewClientV3(ch.AppID, ch.CertSN, ch.APIKey, ch.PrivateKey)
	client.AutoVerifySign()
	if err := notifyReq.VerifySignByPKMap(client.WxPublicKeyMap()); err != nil {
		return gerror.Wrap(err, "wechat sign verify failed")
	}

	return s.confirmOrder(ctx, result.OutTradeNo, result.TransactionId, "wechat", toJSON(notifyReq))
}

// ========== Confirm Order & Credit Account ==========

func (s *sPayment) confirmOrder(ctx context.Context, orderNo, tradeNo, channel, notifyRaw string) error {
	var row orderRow
	err := g.DB().Model("payment_orders").Ctx(ctx).Where("order_no", orderNo).Scan(&row)
	if err != nil {
		return gerror.Wrap(err, "query order failed")
	}
	if row.ID == 0 {
		return gerror.Newf("order %s not found", orderNo)
	}
	if row.Status != "pending" {
		return nil // idempotent
	}

	_, err = g.DB().Model("payment_orders").Ctx(ctx).Where("id", row.ID).Data(g.Map{
		"status":     "paid",
		"trade_no":   tradeNo,
		"notify_raw": notifyRaw,
		"paid_at":    gtime.Now(),
	}).Update()
	if err != nil {
		return gerror.Wrap(err, "update order paid failed")
	}

	refType := fmt.Sprintf("payment:%s", channel)
	_, err = service.Billing().RechargeCredits(ctx, row.UserID, row.AmountCredits, refType, row.ID)
	if err != nil {
		return gerror.Wrap(err, "credit account failed")
	}

	_, err = g.DB().Model("payment_orders").Ctx(ctx).Where("id", row.ID).Data(g.Map{
		"status":      "credited",
		"credited_at": gtime.Now(),
	}).Update()
	if err != nil {
		return gerror.Wrap(err, "update order credited failed")
	}

	return nil
}

// ========== Helpers ==========

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func gjsonGet(raw, key string) string {
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}
