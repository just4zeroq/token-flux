// Package usagetracker writes and queries llm_usage_records.
package usagetracker

import (
	"time"

	"ai-platform-node/pkg/db"
)

// Record holds data for a usage record insertion.
type Record struct {
	RequestID   string
	ModelKeyID  int64
	ChannelID   int64
	Capability  string
	IsStream    bool
	InputTokens int
	OutputTokens int
	TotalTokens int
	CostCredits int64
	LatencyMs   int64
	Status      string
	ErrorCode   string
	ErrorMessage string
	Source      string
}

// Insert writes a usage record to llm_usage_records.
func Insert(r *Record) error {
	stream := 0
	if r.IsStream {
		stream = 1
	}
	_, err := db.DB().Exec(
		`INSERT INTO llm_usage_records
		 (request_id, model_key_id, channel_id, capability, is_stream,
		  input_tokens, output_tokens, total_tokens, cost_credits, latency_ms,
		  status, error_code, error_message, source, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.RequestID, r.ModelKeyID, r.ChannelID, r.Capability, stream,
		r.InputTokens, r.OutputTokens, r.TotalTokens, r.CostCredits, r.LatencyMs,
		r.Status, r.ErrorCode, r.ErrorMessage, r.Source, time.Now().Unix())
	return err
}

// Aggregate holds usage summary.
type Aggregate struct {
	TotalCalls   int64 `json:"total_calls"`
	TotalTokens  int64 `json:"total_tokens"`
	TotalCost    int64 `json:"total_cost"`
	SuccessCalls int64 `json:"success_calls"`
	FailCalls    int64 `json:"fail_calls"`
}

// TokenTotals holds sum of input/output tokens.
type TokenTotals struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

// TokenSummary returns total input and output tokens for today.
func TokenSummary() (*TokenTotals, error) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	var t TokenTotals
	err := db.DB().QueryRow(
		`SELECT COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0)
		 FROM llm_usage_records WHERE created_at >= ?`, todayStart,
	).Scan(&t.InputTokens, &t.OutputTokens)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Summary returns aggregate usage stats.
func Summary() (*Aggregate, error) {
	var a Aggregate
	err := db.DB().QueryRow(
		`SELECT COALESCE(COUNT(*),0), COALESCE(SUM(total_tokens),0),
		        COALESCE(SUM(cost_credits),0),
		        COALESCE(SUM(CASE WHEN status='success' THEN 1 ELSE 0 END),0),
		        COALESCE(SUM(CASE WHEN status!='success' THEN 1 ELSE 0 END),0)
		 FROM llm_usage_records`,
	).Scan(&a.TotalCalls, &a.TotalTokens, &a.TotalCost, &a.SuccessCalls, &a.FailCalls)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ByModel returns usage grouped by model code.
type ByModel struct {
	ModelCode  string `json:"model_code"`
	Calls      int64  `json:"calls"`
	TotalTokens int64 `json:"total_tokens"`
	TotalCost  int64  `json:"total_cost"`
}

// BreakdownByModel returns usage grouped by model.
func BreakdownByModel() ([]ByModel, error) {
	rows, err := db.DB().Query(
		`SELECT COALESCE(m2.model_code, 'unknown'), COUNT(*), SUM(r.total_tokens), SUM(r.cost_credits)
		 FROM llm_usage_records r
		 LEFT JOIN llm_model_key_models m2 ON r.model_key_id = m2.model_key_id
		 GROUP BY m2.model_code
		 ORDER BY COUNT(*) DESC LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ByModel
	for rows.Next() {
		var b ByModel
		if err := rows.Scan(&b.ModelCode, &b.Calls, &b.TotalTokens, &b.TotalCost); err != nil {
			continue
		}
		out = append(out, b)
	}
	return out, nil
}

// RecentRequest holds one usage record with model info.
type RecentRequest struct {
	RequestID        string `json:"request_id"`
	ModelCode        string `json:"model_code"`
	ModelName        string `json:"model_name"`
	UpstreamModel    string `json:"upstream_model"`
	InputTokens      int    `json:"input_tokens"`
	OutputTokens     int    `json:"output_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	Status           string `json:"status"`
	Source           string `json:"source"`
	CreatedAt        int64  `json:"created_at"`
}

// RecentRequests returns the most recent tunnel usage records with model info.
func RecentRequests(limit int) ([]RecentRequest, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := db.DB().Query(
		`SELECT r.request_id, r.input_tokens, r.output_tokens, r.total_tokens,
		        r.status, r.source, r.created_at,
		        COALESCE(m2.model_code, ''),
		        COALESCE(ms.model_name, '')
		 FROM llm_usage_records r
		 LEFT JOIN llm_model_key_models m2 ON r.model_key_id = m2.model_key_id
		 LEFT JOIN llm_model_specs ms ON m2.model_code = ms.model_code
		 ORDER BY r.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RecentRequest
	for rows.Next() {
		var rr RecentRequest
		if err := rows.Scan(&rr.RequestID, &rr.InputTokens, &rr.OutputTokens, &rr.TotalTokens,
			&rr.Status, &rr.Source, &rr.CreatedAt,
			&rr.ModelCode, &rr.ModelName); err != nil {
			continue
		}
		out = append(out, rr)
	}
	return out, nil
}

// Recent returns the most recent usage records (legacy).
func Recent(limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := db.DB().Query(
		`SELECT request_id, model_key_id, channel_id, capability, is_stream,
		        input_tokens, output_tokens, total_tokens, cost_credits, latency_ms,
		        status, error_code, error_message, created_at
		 FROM llm_usage_records ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var rid string
		var mkid, chid int64
		var cap, status, errCode, errMsg string
		var stream, it, ot, tt int
		var cc, lat, ca int64
		rows.Scan(&rid, &mkid, &chid, &cap, &stream,
			&it, &ot, &tt, &cc, &lat, &status, &errCode, &errMsg, &ca)
		out = append(out, map[string]any{
			"request_id":   rid,
			"model_key_id": mkid,
			"channel_id":   chid,
			"capability":   cap,
			"stream":       stream == 1,
			"input_tokens": it,
			"output_tokens": ot,
			"total_tokens": tt,
			"cost_credits": cc,
			"latency_ms":   lat,
			"status":       status,
			"error":        errMsg,
			"created_at":   ca,
		})
	}
	return out, nil
}
