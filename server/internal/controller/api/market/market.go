package market

import (
	"ai-platform/internal/middleware"
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

func CreateOrder(r *ghttp.Request) {
	var in dto.CreateOrderIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
		return
	}
	userID := middleware.GetUserID(r)
	out, err := service.Market().CreateOrder(r.Context(), userID, in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

func ListOrders(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	orders, err := service.Market().ListOrders(r.Context(), userID)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(orders)
}

func CreateReview(r *ghttp.Request) {
	var in dto.CreateReviewIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
		return
	}
	userID := middleware.GetUserID(r)
	out, err := service.Market().CreateReview(r.Context(), userID, in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(out)
}

func ListReviews(r *ghttp.Request) {
	itemID := r.Get("item_id").Int64()
	reviews, err := service.Market().ListReviews(r.Context(), itemID)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(reviews)
}
