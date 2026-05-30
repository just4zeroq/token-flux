package admin

import (
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

func ListSettlements(r *ghttp.Request) {
	pageNum := r.Get("pageNum", 1).Int()
	pageSize := r.Get("pageSize", 20).Int()
	status := r.Get("status", "").String()
	productType := r.Get("productType", "").String()

	list, total, err := service.Settlement().ListSettlements(r.Context(), status, productType, pageNum, pageSize)
	if err != nil {
		fail(r, err.Error())
		return
	}
	page(r, list, total, pageNum, pageSize)
}
