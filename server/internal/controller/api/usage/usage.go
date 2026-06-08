package usage

import (
	"ai-platform/internal/middleware"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// ---------- Response types (returned directly, not wrapped) ----------

// statsRow is returned as the JSON body for GET /api/v1/usage/stats.
type statsRow struct {
	TotalCalls   int     `json:"total_calls"`
	TotalTokens  int64   `json:"total_tokens"`
	TotalCredits int64   `json:"total_credits"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// recordRow is one item in the array returned by GET /api/v1/usage/records.
type recordRow struct {
	ID        int64  `json:"id"`
	Model     string `json:"model"`
	Tokens    int64  `json:"tokens"`
	Credits   int64  `json:"credits"`
	LatencyMs int    `json:"latency_ms"`
	CreatedAt string `json:"created_at"`
}

// modelStatRow is one item returned by GET /api/v1/usage/stats-by-model.
type modelStatRow struct {
	ModelCode   string `json:"model_code"`
	ModelName   string `json:"model_name"`
	CallCount   int    `json:"call_count"`
	TotalTokens int64  `json:"total_tokens"`
	TotalCredits int64 `json:"total_credits"`
}

// Stats returns aggregate usage stats for the current user.
// GET /api/v1/usage/stats
func Stats(r *ghttp.Request) {
	userID := middleware.GetUserID(r)

	var agg statsRow
	err := g.DB().Model("llm_usage_records").Ctx(r.Context()).
		Where("consumer_user_id", userID).
		Where("status", "success").
		Fields(
			"COUNT(*) AS total_calls",
			"COALESCE(SUM(total_tokens),0) AS total_tokens",
			"COALESCE(SUM(cost_credits),0) AS total_credits",
			"COALESCE(AVG(latency_ms),0) AS avg_latency_ms",
		).
		Scan(&agg)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": err.Error(),
		})
		return
	}

	r.Response.WriteJson(agg)
}

// Records returns recent usage records for the current user.
// GET /api/v1/usage/records
func Records(r *ghttp.Request) {
	userID := middleware.GetUserID(r)
	page := r.Get("page", 1).Int()
	pageSize := r.Get("page_size", 20).Int()
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	var rows []recordRow
	err := g.DB().Model("llm_usage_records", "ur").
		Ctx(r.Context()).
		LeftJoin("llm_model_specs", "ms", "ur.model_spec_id = ms.id").
		Where("ur.consumer_user_id", userID).
		Order("ur.id DESC").
		Limit(pageSize).
		Offset(offset).
		Fields(
			"ur.id",
			"COALESCE(ms.model_code, ms.model_name, 'unknown') AS model",
			"COALESCE(ur.total_tokens,0) AS tokens",
			"COALESCE(ur.cost_credits,0) AS credits",
			"COALESCE(ur.latency_ms,0) AS latency_ms",
			"ur.created_at",
		).
		Scan(&rows)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": err.Error(),
		})
		return
	}

	// Return array directly to match frontend UsageRecord[] expectation.
	r.Response.WriteJson(rows)
}

// StatsByModel returns usage stats grouped by model for the current user.
// GET /api/v1/usage/stats-by-model
func StatsByModel(r *ghttp.Request) {
	userID := middleware.GetUserID(r)

	var rows []modelStatRow
	err := g.DB().Model("llm_usage_records", "ur").
		Ctx(r.Context()).
		InnerJoin("llm_model_specs", "ms", "ur.model_spec_id = ms.id").
		Where("ur.consumer_user_id", userID).
		Where("ur.status", "success").
		Group("ur.model_spec_id, ms.model_code, ms.model_name").
		Fields(
			"ms.model_code",
			"ms.model_name",
			"COUNT(*) AS call_count",
			"COALESCE(SUM(ur.total_tokens),0) AS total_tokens",
			"COALESCE(SUM(ur.cost_credits),0) AS total_credits",
		).
		Order("call_count DESC").
		Scan(&rows)
	if err != nil {
		r.Response.WriteStatusExit(400, map[string]any{
			"code": 400, "message": err.Error(),
		})
		return
	}

	r.Response.WriteJson(map[string]any{
		"list": rows,
	})
}
