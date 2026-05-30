package catalog

import (
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// providerPricingRow represents a channel offering pricing for a model.
type providerPricingRow struct {
	ChannelID           int64  `json:"channel_id"`
	ChannelName         string `json:"channel_name"`
	ChannelCode         string `json:"channel_code"`
	UpstreamModelName   string `json:"upstream_model_name"`
	CacheHitPricePer1K  int64  `json:"cache_hit_price_per_1k"`
	CacheMissPricePer1K int64  `json:"cache_miss_price_per_1k"`
	OutputPricePer1K    int64  `json:"output_price_per_1k"`
	KeyStatus           string `json:"key_status"`
	KmStatus            string `json:"km_status"`
}

// modelSpecRow mirrors the llm_model_specs table row for public listing.
type modelSpecRow struct {
	ID               int64     `json:"id"`
	DeveloperName    string    `json:"developer_name"`
	ModelName        string    `json:"model_name"`
	ModelCode        string    `json:"model_code"`
	DisplayName      string    `json:"display_name"`
	ModelFamily      string    `json:"model_family"`
	Description      string    `json:"description"`
	CapabilitiesJson string    `json:"-"`
	ContextWindow    int       `json:"context_window"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

// toPublicMap converts a DB row to a map the frontend expects, transforming
// capabilities_json (JSON array) into a comma-separated capabilities string.
func (r *modelSpecRow) toPublicMap() map[string]any {
	capStr := ""
	raw := strings.TrimSpace(r.CapabilitiesJson)
	if len(raw) > 2 {
		// Strip JSON array brackets and quotes
		inner := raw[1 : len(raw)-1]
		parts := strings.Split(inner, ",")
		cleaned := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.Trim(p, ` "`)
			if p != "" {
				cleaned = append(cleaned, p)
			}
		}
		capStr = strings.Join(cleaned, ", ")
	}
	return map[string]any{
		"id":              r.ID,
		"developer_name":  r.DeveloperName,
		"model_name":      r.ModelName,
		"model_code":      r.ModelCode,
		"display_name":    r.DisplayName,
		"model_family":    r.ModelFamily,
		"description":     r.Description,
		"capabilities":    capStr,
		"context_window":  r.ContextWindow,
		"status":          r.Status,
		"created_at":      r.CreatedAt,
	}
}

// GetModelDetail returns model info plus provider pricing for a single model.
// GET /api/v1/catalog/models/:code
func GetModelDetail(r *ghttp.Request) {
	code := r.Get("code").String()
	if code == "" {
		r.Response.WriteJson(map[string]any{
			"code":    -1,
			"message": "model code required",
		})
		return
	}

	// Query model spec by code
	var row modelSpecRow
	err := g.DB().Model("llm_model_specs").Ctx(r.Context()).
		Where("model_code", code).
		Where("status", "active").
		Scan(&row)
	if err != nil {
		r.Response.WriteJson(map[string]any{
			"code":    -1,
			"message": err.Error(),
		})
		return
	}
	if row.ID == 0 {
		r.Response.WriteJson(map[string]any{
			"code":    -1,
			"message": "model not found",
		})
		return
	}

	// Query providers (channels) that offer this model with active bindings
	var providers []providerPricingRow
	err = g.DB().Model("llm_model_key_models", "km").
		Ctx(r.Context()).
		InnerJoin("llm_model_keys", "k", "km.model_key_id = k.id").
		InnerJoin("llm_channels", "c", "k.channel_id = c.id").
		Where("km.model_spec_id", row.ID).
		Where("km.status", "active").
		Where("k.status", "active").
		Where("c.status", "active").
		Fields(
			"c.id AS channel_id",
			"c.name AS channel_name",
			"c.code AS channel_code",
			"km.upstream_model_name",
			"km.cache_hit_price_per_1k",
			"km.cache_miss_price_per_1k",
			"km.output_price_per_1k",
			"k.status AS key_status",
			"km.status AS km_status",
		).
		Scan(&providers)
	if err != nil {
		r.Response.WriteJson(map[string]any{
			"code":    -1,
			"message": err.Error(),
		})
		return
	}

	r.Response.WriteJson(map[string]any{
		"code":    0,
		"message": "ok",
		"data": map[string]any{
			"model":     row.toPublicMap(),
			"providers": providers,
		},
	})
}

// providerSummaryRow for the provider list aggregation.
type providerSummaryRow struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Website         string `json:"website"`
	LogoURL         string `json:"logo_url"`
	SortOrder       int    `json:"sort_order"`
	ReputationScore int    `json:"reputation_score"`
	ModelCount      int    `json:"model_count"`
	TotalCalls      int64  `json:"total_calls"`
	TotalTokens     int64  `json:"total_tokens"`
	TotalCredits    int64  `json:"total_credits"`
}

// ListProviders returns all active developers with aggregated stats.
// GET /api/v1/catalog/providers
func ListProviders(r *ghttp.Request) {
	sort := r.Get("sort", "model_count").String()
	search := r.Get("search", "").String()

	q := g.DB().Model("developers", "d").Ctx(r.Context()).
		LeftJoin("llm_model_specs", "ms", "ms.developer_name = d.name AND ms.status = 'active'").
		LeftJoin("llm_usage_records", "ur", "ur.model_spec_id = ms.id").
		Where("d.status", 1).
		Group("d.id, d.name, d.description, d.website, d.logo_url, d.sort_order, d.reputation_score").
		Fields(
			"d.id, d.name, d.description, d.website, d.logo_url, d.sort_order, d.reputation_score",
			"COUNT(DISTINCT ms.id) AS model_count",
			"COALESCE(COUNT(ur.id), 0) AS total_calls",
			"COALESCE(SUM(ur.total_tokens), 0) AS total_tokens",
			"COALESCE(SUM(ur.cost_credits), 0) AS total_credits",
		)

	if search != "" {
		q = q.Where("d.name ILIKE ?", "%"+search+"%")
	}

	switch sort {
	case "calls":
		q = q.Order("total_calls DESC, d.name ASC")
	case "reputation":
		q = q.Order("d.reputation_score DESC, d.name ASC")
	default:
		q = q.Order("model_count DESC, d.name ASC")
	}

	var rows []providerSummaryRow
	if err := q.Scan(&rows); err != nil {
		r.Response.WriteJson(map[string]any{
			"code":    -1,
			"message": err.Error(),
		})
		return
	}
	if rows == nil {
		rows = []providerSummaryRow{}
	}

	r.Response.WriteJson(map[string]any{
		"code": 0,
		"data": map[string]any{
			"list": rows,
		},
	})
}

// ---- Supplier (actual platform users with provider role) ----

type supplierRow struct {
	ID         int64  `json:"id"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	ModelCount int    `json:"model_count"`
	TotalCalls int64  `json:"total_calls"`
	TotalTokens int64 `json:"total_tokens"`
	TotalCredits int64 `json:"total_credits"`
}

// ListSuppliers returns platform users with role=provider aggregated stats.
// GET /api/v1/catalog/suppliers
func ListSuppliers(r *ghttp.Request) {
	sort := r.Get("sort", "model_count").String()
	search := r.Get("search", "").String()

	q := g.DB().Model("users", "u").Ctx(r.Context()).
		LeftJoin("llm_model_keys", "k", "k.provider_user_id = u.id AND k.status = 'active'").
		LeftJoin("llm_model_key_models", "km", "km.model_key_id = k.id AND km.status = 'active'").
		LeftJoin("llm_usage_records", "ur", "ur.provider_user_id = u.id").
		Where("u.role", 1).
		Group("u.id, u.username, u.email").
		Fields(
			"u.id, u.username, u.email",
			"COUNT(DISTINCT km.id) AS model_count",
			"COALESCE(COUNT(ur.id), 0) AS total_calls",
			"COALESCE(SUM(ur.total_tokens), 0) AS total_tokens",
			"COALESCE(SUM(ur.cost_credits), 0) AS total_credits",
		)

	if search != "" {
		q = q.Where("u.username ILIKE ?", "%"+search+"%")
	}

	switch sort {
	case "calls":
		q = q.Order("total_calls DESC, u.username ASC")
	case "credits":
		q = q.Order("total_credits DESC, u.username ASC")
	default:
		q = q.Order("model_count DESC, u.username ASC")
	}

	var rows []supplierRow
	if err := q.Scan(&rows); err != nil {
		r.Response.WriteJson(map[string]any{
			"code":    -1,
			"message": err.Error(),
		})
		return
	}
	if rows == nil {
		rows = []supplierRow{}
	}
	r.Response.WriteJson(map[string]any{
		"code": 0,
		"data": map[string]any{
			"list": rows,
		},
	})
}

// GetSupplierModels returns models and pricing for a specific supplier user.
// GET /api/v1/catalog/suppliers/:username
func GetSupplierModels(r *ghttp.Request) {
	username := r.Get("username").String()
	if username == "" {
		r.Response.WriteJson(map[string]any{"code": -1, "message": "username required"})
		return
	}

	// Find user
	type userRow struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	var u userRow
	err := g.DB().Model("users").Ctx(r.Context()).
		Where("username", username).Where("role", 1).
		Scan(&u)
	if err != nil || u.ID == 0 {
		r.Response.WriteJson(map[string]any{"code": -1, "message": "supplier not found"})
		return
	}

	// Supplier aggregate stats
	type aggRow struct {
		ModelCount  int   `json:"model_count"`
		TotalCalls  int64 `json:"total_calls"`
		TotalTokens int64 `json:"total_tokens"`
		TotalCredits int64 `json:"total_credits"`
	}
	var agg aggRow
	g.DB().Model("llm_model_key_models", "km").Ctx(r.Context()).
		InnerJoin("llm_model_keys", "k", "k.id = km.model_key_id AND k.status = 'active'").
		LeftJoin("llm_usage_records", "ur", "ur.key_model_id = km.id").
		Where("k.provider_user_id", u.ID).
		Fields(
			"COUNT(DISTINCT km.id) AS model_count",
			"COALESCE(COUNT(ur.id), 0) AS total_calls",
			"COALESCE(SUM(ur.total_tokens), 0) AS total_tokens",
			"COALESCE(SUM(ur.cost_credits), 0) AS total_credits",
		).
		Scan(&agg)

	// Models with pricing
	type modelInfo struct {
		ModelCode       string `json:"model_code"`
		ModelName       string `json:"model_name"`
		ChannelName     string `json:"channel_name"`
		CacheMissPrice  int64  `json:"input_price_per_1k"`
		OutputPrice     int64  `json:"output_price_per_1k"`
		CacheHitPrice   int64  `json:"cache_hit_price_per_1k"`
	}
	var models []modelInfo
	err = g.DB().Model("llm_model_key_models", "km").Ctx(r.Context()).
		InnerJoin("llm_model_keys", "k", "k.id = km.model_key_id AND k.status = 'active'").
		InnerJoin("llm_channels", "c", "c.id = k.channel_id AND c.status = 'active'").
		InnerJoin("llm_model_specs", "ms", "ms.id = km.model_spec_id").
		Where("k.provider_user_id", u.ID).
		Where("km.status", "active").
		Fields(
			"ms.model_code, ms.model_name, c.name AS channel_name",
			"km.cache_miss_price_per_1k, km.output_price_per_1k, km.cache_hit_price_per_1k",
		).
		Scan(&models)
	if err != nil {
		r.Response.WriteJson(map[string]any{"code": -1, "message": err.Error()})
		return
	}
	if models == nil {
		models = []modelInfo{}
	}

	r.Response.WriteJson(map[string]any{
		"code": 0,
		"data": map[string]any{
			"supplier": u,
			"stats":    agg,
			"models":   models,
		},
	})
}

// ---- Developer providers (legacy) ----

// providerModelRow for the provider detail models list.
type providerModelRow struct {
	ID               int64  `json:"id"`
	ModelName        string `json:"model_name"`
	ModelCode        string `json:"model_code"`
	DisplayName      string `json:"display_name"`
	ModelFamily      string `json:"model_family"`
	ContextWindow    int    `json:"context_window"`
	CapabilitiesJson string `json:"-"`
	Capabilities     string `json:"capabilities"`
	CacheHitPrice    int64  `json:"cache_hit_price_per_1k"`
	CacheMissPrice   int64  `json:"cache_miss_price_per_1k"`
	OutputPrice      int64  `json:"output_price_per_1k"`
	ChannelName      string `json:"channel_name"`
	KmStatus         string `json:"km_status"`
}

// GetProviderModels returns models + pricing for a specific developer/provider.
// GET /api/v1/catalog/providers/:name/models
func GetProviderModels(r *ghttp.Request) {
	name := r.Get("name").String()
	if name == "" {
		r.Response.WriteJson(map[string]any{
			"code":    -1,
			"message": "provider name required",
		})
		return
	}

	// Developer info
	type devRow struct {
		ID              int64  `json:"id"`
		Name            string `json:"name"`
		Description     string `json:"description"`
		Website         string `json:"website"`
		LogoURL         string `json:"logo_url"`
		ReputationScore int    `json:"reputation_score"`
	}
	var dev devRow
	err := g.DB().Model("developers").Ctx(r.Context()).
		Where("name", name).
		Where("status", 1).
		Scan(&dev)
	if err != nil || dev.ID == 0 {
		r.Response.WriteJson(map[string]any{
			"code":    -1,
			"message": "provider not found",
		})
		return
	}

	// Provider aggregate stats
	type aggRow struct {
		ModelCount  int   `json:"model_count"`
		TotalCalls  int64 `json:"total_calls"`
		TotalTokens int64 `json:"total_tokens"`
	}
	var agg aggRow
	g.DB().Model("llm_model_specs", "ms").Ctx(r.Context()).
		LeftJoin("llm_usage_records", "ur", "ur.model_spec_id = ms.id").
		Where("ms.developer_name", name).
		Where("ms.status", "active").
		Fields(
			"COUNT(DISTINCT ms.id) AS model_count",
			"COALESCE(COUNT(ur.id), 0) AS total_calls",
			"COALESCE(SUM(ur.total_tokens), 0) AS total_tokens",
		).
		Scan(&agg)

	// Models with pricing from key-model bindings
	var models []providerModelRow
	err = g.DB().Model("llm_model_specs", "ms").Ctx(r.Context()).
		InnerJoin("llm_model_key_models", "km", "km.model_spec_id = ms.id AND km.status = 'active'").
		InnerJoin("llm_model_keys", "k", "k.id = km.model_key_id AND k.status = 'active'").
		InnerJoin("llm_channels", "c", "c.id = k.channel_id AND c.status = 'active'").
		Where("ms.developer_name", name).
		Where("ms.status", "active").
		Fields(
			"ms.id, ms.model_name, ms.model_code, ms.display_name, ms.model_family, ms.context_window, ms.capabilities_json",
			"km.cache_hit_price_per_1k, km.cache_miss_price_per_1k, km.output_price_per_1k",
			"c.name AS channel_name, km.status AS km_status",
		).
		Order("ms.id DESC").
		Scan(&models)
	if err != nil {
		r.Response.WriteJson(map[string]any{
			"code":    -1,
			"message": err.Error(),
		})
		return
	}

	// Map capabilities_json to comma-separated string
	type modelOut struct {
		ID            int64  `json:"id"`
		ModelName     string `json:"model_name"`
		ModelCode     string `json:"model_code"`
		DisplayName   string `json:"display_name"`
		ModelFamily   string `json:"model_family"`
		ContextWindow int    `json:"context_window"`
		Capabilities  string `json:"capabilities"`
		InputPrice    int64  `json:"input_price_per_1k"`
		OutputPrice   int64  `json:"output_price_per_1k"`
		CachePrice    int64  `json:"cache_miss_price_per_1k"`
		ChannelName   string `json:"channel_name"`
	}
	out := make([]modelOut, 0, len(models))
	for _, m := range models {
		capStr := ""
		raw := strings.TrimSpace(m.CapabilitiesJson)
		if len(raw) > 2 {
			inner := raw[1 : len(raw)-1]
			parts := strings.Split(inner, ",")
			cleaned := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.Trim(p, ` "`)
				if p != "" {
					cleaned = append(cleaned, p)
				}
			}
			capStr = strings.Join(cleaned, ", ")
		}
		out = append(out, modelOut{
			ID:            m.ID,
			ModelName:     m.ModelName,
			ModelCode:     m.ModelCode,
			DisplayName:   m.DisplayName,
			ModelFamily:   m.ModelFamily,
			ContextWindow: m.ContextWindow,
			Capabilities:  capStr,
			InputPrice:    m.CacheMissPrice,
			OutputPrice:   m.OutputPrice,
			CachePrice:    m.CacheHitPrice,
			ChannelName:   m.ChannelName,
		})
	}

	r.Response.WriteJson(map[string]any{
		"code": 0,
		"data": map[string]any{
			"developer": dev,
			"stats":     agg,
			"models":    out,
		},
	})
}

// ListModels returns all active model specs for the public catalog.
// GET /api/v1/catalog/models
func ListModels(r *ghttp.Request) {
	var rows []modelSpecRow
	err := g.DB().Model("llm_model_specs").Ctx(r.Context()).
		Where("status", "active").
		Order("id DESC").
		Scan(&rows)
	if err != nil {
		r.Response.WriteJson(map[string]any{
			"code":    -1,
			"message": err.Error(),
		})
		return
	}
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toPublicMap())
	}
	r.Response.WriteJson(map[string]any{
		"code":    0,
		"message": "ok",
		"data": map[string]any{
			"list": out,
		},
	})
}
