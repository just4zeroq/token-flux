package pricing

import (
	"context"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

type sPricing struct{}

func init() { service.RegisterPricing(New()) }

func New() *sPricing { return &sPricing{} }

// ListPricingRules returns pricing rules, optionally filtered by itemID.
func (s *sPricing) ListPricingRules(ctx context.Context, itemID int64) ([]*dto.PricingRuleInfo, error) {
	m := g.DB().Model("pricing_rules").Ctx(ctx)
	if itemID > 0 {
		m = m.Where("item_id", itemID)
	}
	records, err := m.Order("effective_from DESC").All()
	if err != nil {
		return nil, gerror.Wrap(err, "query pricing rules failed")
	}
	rules := make([]*dto.PricingRuleInfo, 0, len(records))
	for _, r := range records {
		rule := &dto.PricingRuleInfo{
			ID:            r["id"].Int64(),
			ItemID:        r["item_id"].Int64(),
			Strategy:      r["strategy"].String(),
			Params:        r["params_json"].String(),
			EffectiveFrom: r["effective_from"].Time(),
			CreatedAt:     r["created_at"].Time(),
		}
		if !r["effective_to"].IsEmpty() {
			v := r["effective_to"].Time()
			rule.EffectiveTo = &v
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// GetPricingRule returns a single pricing rule by id.
func (s *sPricing) GetPricingRule(ctx context.Context, id int64) (*dto.PricingRuleInfo, error) {
	record, err := g.DB().Model("pricing_rules").Ctx(ctx).Where("id", id).One()
	if err != nil {
		return nil, gerror.Wrap(err, "query pricing rule failed")
	}
	if record.IsEmpty() {
		return nil, gerror.New("pricing rule not found")
	}
	rule := &dto.PricingRuleInfo{
		ID:            record["id"].Int64(),
		ItemID:        record["item_id"].Int64(),
		Strategy:      record["strategy"].String(),
		Params:        record["params_json"].String(),
		EffectiveFrom: record["effective_from"].Time(),
		CreatedAt:     record["created_at"].Time(),
	}
	if !record["effective_to"].IsEmpty() {
		v := record["effective_to"].Time()
		rule.EffectiveTo = &v
	}
	return rule, nil
}

// CreatePricingRule inserts a new pricing rule and returns the created record.
func (s *sPricing) CreatePricingRule(ctx context.Context, in dto.CreatePricingRuleIn) (*dto.PricingRuleInfo, error) {
	data := g.Map{
		"item_id":        in.ItemID,
		"strategy":       in.Strategy,
		"params_json":    in.Params,
		"effective_from": in.EffectiveFrom,
	}
	if in.EffectiveTo != nil {
		data["effective_to"] = *in.EffectiveTo
	}
	result, err := g.DB().Model("pricing_rules").Ctx(ctx).Insert(data)
	if err != nil {
		return nil, gerror.Wrap(err, "create pricing rule failed")
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, gerror.Wrap(err, "get last insert id failed")
	}
	return s.GetPricingRule(ctx, id)
}

// ListExchangeRates returns exchange rates, optionally filtered by asset pair.
func (s *sPricing) ListExchangeRates(ctx context.Context, from, to string) ([]*dto.ExchangeRateInfo, error) {
	m := g.DB().Model("exchange_rates").Ctx(ctx)
	if from != "" {
		m = m.Where("asset_from", from)
	}
	if to != "" {
		m = m.Where("asset_to", to)
	}
	records, err := m.Order("effective_at DESC").All()
	if err != nil {
		return nil, gerror.Wrap(err, "query exchange rates failed")
	}
	rates := make([]*dto.ExchangeRateInfo, 0, len(records))
	for _, r := range records {
		rate := &dto.ExchangeRateInfo{
			ID:          r["id"].Int64(),
			AssetFrom:   r["asset_from"].String(),
			AssetTo:     r["asset_to"].String(),
			RateMicro:   r["rate_micro"].Int64(),
			FeeRateBps:  r["fee_rate_bps"].Int(),
			EffectiveAt: r["effective_at"].Time(),
		}
		rates = append(rates, rate)
	}
	return rates, nil
}
