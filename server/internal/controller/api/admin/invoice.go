package admin

import (
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

func ListInvoices(r *ghttp.Request) {
	pageNum := r.Get("pageNum", 1).Int()
	pageSize := r.Get("pageSize", 20).Int()
	status := r.Get("status", "").String()

	list, total, err := service.Invoice().List(r.Context(), status, pageNum, pageSize)
	if err != nil {
		fail(r, err.Error())
		return
	}
	page(r, list, total, pageNum, pageSize)
}

func GetInvoice(r *ghttp.Request) {
	id := r.Get("id").Int64()
	info, err := service.Invoice().Get(r.Context(), id)
	if err != nil {
		fail(r, err.Error())
		return
	}
	ok(r, info)
}

func CreateInvoice(r *ghttp.Request) {
	var in dto.CreateInvoiceIn
	if err := r.Parse(&in); err != nil {
		fail(r, "invalid params: "+err.Error())
		return
	}
	out, err := service.Invoice().Create(r.Context(), in)
	if err != nil {
		fail(r, err.Error())
		return
	}
	ok(r, out)
}

func UpdateInvoiceStatus(r *ghttp.Request) {
	id := r.Get("id").Int64()
	var in dto.UpdateInvoiceStatusIn
	if err := r.Parse(&in); err != nil {
		fail(r, "invalid params: "+err.Error())
		return
	}
	if err := service.Invoice().UpdateStatus(r.Context(), id, in); err != nil {
		fail(r, err.Error())
		return
	}
	okMsg(r, "updated")
}
