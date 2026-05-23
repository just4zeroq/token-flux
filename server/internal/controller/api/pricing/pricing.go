package pricing

import (
	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

// ListPricingRules handles GET /pricing/rules
func ListPricingRules(r *ghttp.Request) {
	itemID := r.Get("item_id").Int64()
	rules, err := service.Pricing().ListPricingRules(r.Context(), itemID)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{
			"code": 500, "message": err.Error(),
		})
		return
	}
	r.Response.WriteJson(rules)
}

// GetPricingRule handles GET /pricing/rules/:id
func GetPricingRule(r *ghttp.Request) {
	id := r.Get("id").Int64()
	if id == 0 {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid pricing rule id",
		})
		return
	}
	rule, err := service.Pricing().GetPricingRule(r.Context(), id)
	if err != nil {
		r.Response.WriteStatusExit(404, map[string]any{
			"code": 404, "message": err.Error(),
		})
		return
	}
	r.Response.WriteJson(rule)
}

// CreatePricingRule handles POST /pricing/rules
func CreatePricingRule(r *ghttp.Request) {
	var in dto.CreatePricingRuleIn
	if err := r.Parse(&in); err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": "invalid params: " + err.Error(),
		})
		return
	}
	rule, err := service.Pricing().CreatePricingRule(r.Context(), in)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": err.Error(),
		})
		return
	}
	r.Response.WriteJson(rule)
}

// ListExchangeRates handles GET /pricing/exchange-rates
func ListExchangeRates(r *ghttp.Request) {
	from := r.Get("from").String()
	to := r.Get("to").String()
	rates, err := service.Pricing().ListExchangeRates(r.Context(), from, to)
	if err != nil {
		r.Response.WriteStatusExit(500, map[string]any{
			"code": 500, "message": err.Error(),
		})
		return
	}
	r.Response.WriteJson(rates)
}
