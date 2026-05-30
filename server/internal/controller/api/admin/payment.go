package admin

import (
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

// ========== Payment Channels ==========

// GET /api/admin/payment/channels
func ListPaymentChannels(r *ghttp.Request) {
	pageNum := r.Get("pageNum", 1).Int()
	pageSize := r.Get("pageSize", 20).Int()
	channel := r.Get("channel", "").String()

	list, total, err := service.Payment().ListChannels(r.Context(), channel, pageNum, pageSize)
	if err != nil {
		fail(r, err.Error())
		return
	}
	page(r, list, total, pageNum, pageSize)
}

// POST /api/admin/payment/channels
func CreatePaymentChannel(r *ghttp.Request) {
	var in dto.CreatePaymentChannelIn
	if err := r.Parse(&in); err != nil {
		fail(r, "invalid params: "+err.Error())
		return
	}

	if err := service.Payment().CreateChannel(r.Context(), in); err != nil {
		fail(r, err.Error())
		return
	}
	okMsg(r, "channel created")
}

// PUT /api/admin/payment/channels/:id
func UpdatePaymentChannel(r *ghttp.Request) {
	id := r.Get("id").Int64()
	if id == 0 {
		fail(r, "id required")
		return
	}

	var in dto.UpdatePaymentChannelIn
	if err := r.Parse(&in); err != nil {
		fail(r, "invalid params: "+err.Error())
		return
	}

	if err := service.Payment().UpdateChannel(r.Context(), id, in); err != nil {
		fail(r, err.Error())
		return
	}
	okMsg(r, "channel updated")
}

// ========== Payment Orders ==========

// GET /api/admin/payment/orders
func ListPaymentOrders(r *ghttp.Request) {
	pageNum := r.Get("pageNum", 1).Int()
	pageSize := r.Get("pageSize", 20).Int()
	channel := r.Get("channel", "").String()
	status := r.Get("status", "").String()
	keyword := r.Get("keyword", "").String()

	list, total, err := service.Payment().ListAllOrders(r.Context(), channel, status, keyword, pageNum, pageSize)
	if err != nil {
		fail(r, err.Error())
		return
	}
	page(r, list, total, pageNum, pageSize)
}
