package payment

import (
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

func fail(r *ghttp.Request, msg string) {
	r.Response.WriteJson(map[string]any{"code": -1, "message": msg})
}

func ok(r *ghttp.Request, data any) {
	r.Response.WriteJson(map[string]any{"code": 0, "message": "ok", "data": data})
}

// POST /api/v1/payment/recharge
func Recharge(r *ghttp.Request) {
	userID := r.GetCtxVar("user_id").Int64()

	var in dto.CreateRechargeIn
	if err := r.Parse(&in); err != nil {
		fail(r, "invalid params: "+err.Error())
		return
	}

	out, err := service.Payment().CreateRecharge(r.Context(), userID, in)
	if err != nil {
		fail(r, err.Error())
		return
	}
	ok(r, out)
}

// GET /api/v1/payment/orders
func ListOrders(r *ghttp.Request) {
	userID := r.GetCtxVar("user_id").Int64()
	page := r.Get("pageNum", 1).Int()
	pageSize := r.Get("pageSize", 20).Int()

	list, total, err := service.Payment().ListOrders(r.Context(), userID, page, pageSize)
	if err != nil {
		fail(r, err.Error())
		return
	}
	r.Response.WriteJson(map[string]any{
		"code":    0,
		"message": "ok",
		"data": map[string]any{
			"list":     list,
			"total":    total,
			"pageNum":  page,
			"pageSize": pageSize,
		},
	})
}

// GET /api/v1/payment/orders/:orderNo
func GetOrder(r *ghttp.Request) {
	userID := r.GetCtxVar("user_id").Int64()
	orderNo := r.Get("orderNo").String()

	info, err := service.Payment().GetOrder(r.Context(), userID, orderNo)
	if err != nil {
		fail(r, err.Error())
		return
	}
	ok(r, info)
}

// POST /api/v1/payment/notify/alipay
func AlipayNotify(r *ghttp.Request) {
	if err := service.Payment().HandleNotify(r.Context(), "alipay", r.Request); err != nil {
		fail(r, err.Error())
		return
	}
	r.Response.Write("success")
}

// POST /api/v1/payment/notify/wechat
func WechatNotify(r *ghttp.Request) {
	if err := service.Payment().HandleNotify(r.Context(), "wechat", r.Request); err != nil {
		r.Response.WriteJson(wechatFailRsp())
		return
	}
	r.Response.WriteJson(wechatSuccessRsp())
}

func wechatSuccessRsp() map[string]string {
	return map[string]string{"code": "SUCCESS", "message": "OK"}
}

func wechatFailRsp() map[string]string {
	return map[string]string{"code": "FAIL", "message": "sign error"}
}
