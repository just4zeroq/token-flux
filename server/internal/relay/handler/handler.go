// Package handler orchestrates the relay pipeline: validate → pick → convert → call → settle.
//
// V2: Support dual-protocol (OpenAI + Anthropic + Gemini), dynamic format detection,
// protocol-based adaptor selection, AES-GCM key decryption, and usage-based cost computation.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"ai-platform/internal/logic/llm"
	"ai-platform/internal/relay/channel"
	"ai-platform/internal/relay/common"
	"ai-platform/internal/relay/constant"
	"ai-platform/internal/relay/helper"
	"ai-platform/internal/relay/scheduler"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// RequestContext carries the per-request state from the HTTP handler.
type RequestContext struct {
	UserID    int64
	ApiKeyID  int64
	RequestID string
}

// RelayRequest is the input to the relay handler.
type RelayRequest struct {
	Ctx        context.Context
	RC         *RequestContext
	Path       string // e.g. /v1/chat/completions
	RawBody    []byte
	IsStream   bool
	Capability string // "chat", "completion", "embedding"
}

// RelayResponse is the output from the relay handler.
type RelayResponse struct {
	StatusCode int
	Body       []byte
	Headers    map[string]string
}

// bodyBufferWriter implements http.ResponseWriter over a bytes.Buffer so adaptor's
// DoResponse can write converted output without a real network connection.
type bodyBufferWriter struct {
	header http.Header
	body   bytes.Buffer
	code   int
}

func (w *bodyBufferWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *bodyBufferWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.body.Write(b)
}

func (w *bodyBufferWriter) WriteHeader(code int) {
	w.code = code
}

// candidateRow is the DB join result for active key-model candidates.
type candidateRow struct {
	KeyModelID         int64  `json:"key_model_id"`
	ModelKeyID         int64  `json:"model_key_id"`
	ModelSpecID        int64  `json:"model_spec_id"`
	ChannelID          int64  `json:"channel_id"`
	ChannelName        string `json:"channel_name"`
	ProviderUserID     int64  `json:"provider_user_id"`
	Priority           int    `json:"priority"`
	Weight             int    `json:"weight"`
	UpstreamModelName  string `json:"upstream_model_name"`
	KeyEncrypted       string `json:"key_encrypted"`
	ProtocolsJson      string `json:"protocols_json"`
	ProviderShareBps   int    `json:"provider_share_bps"`
	CacheHitPricePer1K  int64  `json:"cache_hit_price_per_1k"`
	CacheMissPricePer1K int64  `json:"cache_miss_price_per_1k"`
	OutputPricePer1K    int64  `json:"output_price_per_1k"`
	KMQuotaUsed        int64  `json:"km_quota_used"`
	KMQuotaLimit       int64  `json:"km_quota_limit"`
	MKQuotaUsed        int64  `json:"mk_quota_used"`
	MKQuotaLimit       int64  `json:"mk_quota_limit"`
	ConsecutiveFails   int    `json:"consecutive_failures"`
}

// Handle relays a request through the pipeline.
func Handle(req *RelayRequest) (resp *RelayResponse, err error) {
	ctx := req.Ctx
	g.Log().Infof(ctx, "[Relay] Handle ENTER: path=%s isStream=%v", req.Path, req.IsStream)
	defer func() {
		if r := recover(); r != nil {
			err = gerror.Newf("panic in relay handle: %v", r)
			g.Log().Errorf(ctx, "[Relay] panic recovered: %v", r)
		}
	}()

	// 1. Extract model from request body.
	modelCode, err := extractModel(req.RawBody)
	if err != nil {
		return nil, err
	}

	// 2. Detect inbound request format from body content + path.
	inboundFormat := detectInboundFormat(req.RawBody, req.Path)

	// 3. Balance check.
	if err := checkBalance(ctx, req.RC.UserID); err != nil {
		return nil, err
	}

	// 4. Resolve model spec.
	specID, resolvedCode, err := resolveModelSpec(ctx, modelCode)
	if err != nil {
		return nil, err
	}

	// 5. Load candidates (all protocols, not just openai-compatible).
	dbCandidates, err := loadActiveCandidates(ctx, specID)
	if err != nil {
		return nil, err
	}
	if len(dbCandidates) == 0 {
		return nil, gerror.Newf("no available key for model %s", resolvedCode)
	}

	chanCandidates := buildChannelCandidates(dbCandidates, inboundFormat)

	// 6. Check affinity — prefer same channel.
	affinityChannel, hasAffinity := scheduler.GlobalAffinity.Get(req.RC.UserID, resolvedCode)
	if hasAffinity {
		preferred := make([]common.ChannelCandidate, 0, len(chanCandidates))
		for _, c := range chanCandidates {
			if c.ChannelID == affinityChannel {
				preferred = append(preferred, c)
			}
		}
		if len(preferred) > 0 {
			chanCandidates = preferred
		}
	}

	// 7. Scheduler pick.
	picked := scheduler.Select(chanCandidates)
	if picked == nil {
		return nil, gerror.New("no channel available after scheduling")
	}

	// 8. Determine protocol adaptor from channel's protocols_json.
	protocolKey := picked.ProtocolKey
	if protocolKey == "" {
		return nil, gerror.Newf("channel %d (%s) has no supported protocol",
			picked.ChannelID, picked.ChannelName)
	}

	adp := channel.GetAdaptor(protocolKey)
	if adp == nil {
		return nil, gerror.Newf("no adaptor registered for protocol %q", protocolKey)
	}

	// 9. Decrypt upstream key (AES-GCM).
	apiKey, err := decryptUpstreamKey(ctx, picked.ApiKey)
	if err != nil {
		return nil, gerror.Wrapf(err, "decrypt key failed for model_key_id=%d", picked.ModelKeyID)
	}

	// 10. Build RelayInfo.
	info := &common.RelayInfo{
		Context:   ctx,
		UserID:    req.RC.UserID,
		ApiKeyID:  req.RC.ApiKeyID,
		RequestID: req.RC.RequestID,
		RelayMode: int(constant.RelayModeChatCompletions),
		IsStream:  req.IsStream,
		OriginModelName:  resolvedCode,
		RequestURLPath:   req.Path,
		StartTime:        time.Now(),
		StreamStatus:     common.NewStreamStatus(),
		InboundFormat:    inboundFormat,
		ClientFormat:     inboundFormat,
		ChannelMeta: &common.ChannelMeta{
			ChannelID:         picked.ChannelID,
			ChannelName:       picked.ChannelName,
			BaseURL:           picked.BaseURL,
			ApiKey:            apiKey,
			UpstreamModelName: picked.UpstreamModelName,
			IsModelMapped:     picked.UpstreamModelName != "",
		},
	}
	adp.Init(info)

	// 11. Convert request to upstream format.
	convertedBody, err := adp.ConvertRequest(ctx, info, req.RawBody)
	if err != nil {
		return nil, gerror.Wrap(err, "convert request failed")
	}

	// 12. Retry loop.
	maxRetries := scheduler.GetMaxRetries(ctx)
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		info.RetryIndex = attempt

		// Use a fresh reader for each attempt.
		var bodyReader io.Reader
		if convertedBytes, ok := convertedBody.(*bytes.Reader); ok {
			_, _ = convertedBytes.Seek(0, io.SeekStart)
			bodyReader = convertedBytes
		} else {
			bodyReader = bytes.NewReader(req.RawBody)
		}

		resp, err := adp.DoRequest(ctx, info, bodyReader)
		if err != nil {
			lastErr = err
			if scheduler.ShouldRetry(err, attempt, maxRetries) {
				scheduler.GlobalAffinity.Delete(req.RC.UserID, resolvedCode)
				continue
			}
			return nil, gerror.Wrap(err, "upstream request failed")
		}

		// Response handling.
		info.SetFirstResponseTime()

		if !req.IsStream {
			// Non-stream: use buffer writer to capture DoResponse output.
			bw := &bodyBufferWriter{header: make(http.Header)}
			usage, doRespErr := adp.DoResponse(ctx, resp, info, bw)

			// Use the buffer content regardless of doRespErr (error might be partial).
			statusCode := bw.code
			if statusCode == 0 {
				statusCode = http.StatusOK
			}
			respBody := bw.body.Bytes()
			respHeaders := make(map[string]string)
			for k := range bw.header {
				respHeaders[k] = bw.header.Get(k)
			}

			// Enqueue async settlement + sync quota update (atomic).
			cost := int64(0)
			if usage != nil && usage.TotalTokens > 0 && statusCode == http.StatusOK {
				cost = enqueueSettlement(req, picked, usage, req.RawBody, respBody, "")
			}
			if cost > 0 {
				updateQuotaSync(ctx, picked.ModelKeyID, picked.KeyModelID, cost)
			}

			scheduler.GlobalAffinity.Set(req.RC.UserID, resolvedCode, picked.ChannelID)

			if doRespErr != nil {
				// DoResponse reported an error (e.g. upstream error in body).
				// Return the converted body so the client gets the error message.
				if len(respBody) == 0 {
					return nil, doRespErr
				}
			}

			return &RelayResponse{
				StatusCode: statusCode,
				Body:       respBody,
				Headers:    respHeaders,
			}, nil
		}

		return nil, lastErr
	}

	return nil, lastErr
}

// HandleStream relays a streaming request, writing SSE events directly to the client's
// http.ResponseWriter. Settlement is enqueued asynchronously after the stream completes.
// Returns an error only when the pipeline fails before the first byte is written;
// once streaming starts the status is already sent to the client.

// captureResponseWriter wraps http.ResponseWriter, capturing all Write() data to buf.
type captureResponseWriter struct {
	http.ResponseWriter
	buf bytes.Buffer
}

func (w *captureResponseWriter) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

func HandleStream(w http.ResponseWriter, req *RelayRequest) (err error) {
	ctx := req.Ctx
	defer func() {
		if r := recover(); r != nil {
			err = gerror.Newf("panic in relay stream: %v", r)
			g.Log().Errorf(ctx, "[Relay] stream panic recovered: %v", r)
			// Try to write an error to the client if nothing sent yet.
			if !req.IsStream {
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}
	}()

	// 1. Extract model from request body.
	modelCode, e := extractModel(req.RawBody)
	if e != nil {
		return e
	}

	// 2. Detect inbound request format.
	inboundFormat := detectInboundFormat(req.RawBody, req.Path)

	// 3. Balance check.
	if e := checkBalance(ctx, req.RC.UserID); e != nil {
		return e
	}

	// 4. Resolve model spec.
	specID, resolvedCode, e := resolveModelSpec(ctx, modelCode)
	if e != nil {
		return e
	}

	// 5. Load candidates.
	dbCandidates, e := loadActiveCandidates(ctx, specID)
	if e != nil {
		return e
	}
	if len(dbCandidates) == 0 {
		return gerror.Newf("no available key for model %s", resolvedCode)
	}

	chanCandidates := buildChannelCandidates(dbCandidates, inboundFormat)

	// 6. Affinity check.
	affinityChannel, hasAffinity := scheduler.GlobalAffinity.Get(req.RC.UserID, resolvedCode)
	if hasAffinity {
		preferred := make([]common.ChannelCandidate, 0, len(chanCandidates))
		for _, c := range chanCandidates {
			if c.ChannelID == affinityChannel {
				preferred = append(preferred, c)
			}
		}
		if len(preferred) > 0 {
			chanCandidates = preferred
		}
	}

	// 7. Scheduler pick.
	picked := scheduler.Select(chanCandidates)
	if picked == nil {
		return gerror.New("no channel available after scheduling")
	}

	// 8. Determine protocol adaptor.
	protocolKey := picked.ProtocolKey
	if protocolKey == "" {
		return gerror.Newf("channel %d (%s) has no supported protocol",
			picked.ChannelID, picked.ChannelName)
	}

	adp := channel.GetAdaptor(protocolKey)
	if adp == nil {
		return gerror.Newf("no adaptor registered for protocol %q", protocolKey)
	}

	// 9. Decrypt upstream key.
	apiKey, e := decryptUpstreamKey(ctx, picked.ApiKey)
	if e != nil {
		return gerror.Wrapf(e, "decrypt key failed for model_key_id=%d", picked.ModelKeyID)
	}

	// 10. Build RelayInfo.
	info := &common.RelayInfo{
		Context:   ctx,
		UserID:    req.RC.UserID,
		ApiKeyID:  req.RC.ApiKeyID,
		RequestID: req.RC.RequestID,
		RelayMode: int(constant.RelayModeChatCompletions),
		IsStream:  true,
		OriginModelName:  resolvedCode,
		RequestURLPath:   req.Path,
		StartTime:        time.Now(),
		StreamStatus:     common.NewStreamStatus(),
		InboundFormat:    inboundFormat,
		ClientFormat:     inboundFormat,
		ChannelMeta: &common.ChannelMeta{
			ChannelID:         picked.ChannelID,
			ChannelName:       picked.ChannelName,
			BaseURL:           picked.BaseURL,
			ApiKey:            apiKey,
			UpstreamModelName: picked.UpstreamModelName,
			IsModelMapped:     picked.UpstreamModelName != "",
		},
	}
	adp.Init(info)

	// 11. Convert request.
	convertedBody, e := adp.ConvertRequest(ctx, info, req.RawBody)
	if e != nil {
		return gerror.Wrap(e, "convert request failed")
	}

	// 12. Retry loop — streaming writes directly to the client ResponseWriter.
	maxRetries := scheduler.GetMaxRetries(ctx)
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		info.RetryIndex = attempt

		var bodyReader io.Reader
		if convertedBytes, ok := convertedBody.(*bytes.Reader); ok {
			_, _ = convertedBytes.Seek(0, io.SeekStart)
			bodyReader = convertedBytes
		} else {
			bodyReader = bytes.NewReader(req.RawBody)
		}

		resp, e := adp.DoRequest(ctx, info, bodyReader)
		if e != nil {
			lastErr = e
			if scheduler.ShouldRetry(e, attempt, maxRetries) {
				scheduler.GlobalAffinity.Delete(req.RC.UserID, resolvedCode)
				continue
			}
			return gerror.Wrap(e, "upstream request failed")
		}

		info.SetFirstResponseTime()

		// Write SSE directly to the client (real http.ResponseWriter).
		cw := &captureResponseWriter{ResponseWriter: w}
		usage, streamErr := adp.DoResponse(ctx, resp, info, cw)

		scheduler.GlobalAffinity.Set(req.RC.UserID, resolvedCode, picked.ChannelID)

		// Build chat log response body from accumulated response text.
		// Streaming body is SSE (not JSON), store extracted text content.
		var respBody json.RawMessage
		if info.ResponseText != "" {
			respBody, _ = json.Marshal(map[string]interface{}{
				"content":   info.ResponseText,
				"is_stream": true,
			})
		}

		// Always enqueue — worker handles both paths:
		//   1. upstream returned usage (normal path, known cost)
		//   2. no usage from upstream (lazy tokenize + chat log)
		cost := enqueueSettlement(req, picked, usage, req.RawBody, respBody, info.ResponseText)
		if cost > 0 {
			updateQuotaSync(ctx, picked.ModelKeyID, picked.KeyModelID, cost)
		}

		if streamErr != nil {
			return gerror.Wrap(streamErr, "stream response failed")
		}
		return nil
	}

	return lastErr
}

// detectInboundFormat determines the client's request format from body + path.
func detectInboundFormat(body []byte, path string) constant.RelayFormat {
	// Path takes precedence for unambiguous routes.
	if fmt, ok := helper.DetectFormatFromPath(path); ok {
		return fmt
	}
	return helper.DetectInboundFormat(body)
}

// headersFromResp copies HTTP response headers.
func headersFromResp(resp *http.Response) map[string]string {
	h := make(map[string]string, len(resp.Header))
	for k := range resp.Header {
		h[k] = resp.Header.Get(k)
	}
	return h
}

func extractModel(body []byte) (string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", gerror.New("invalid request body")
	}
	if v, ok := raw["model"]; ok {
		var m string
		if err := json.Unmarshal(v, &m); err == nil && m != "" {
			return m, nil
		}
	}
	// Claude uses "model" too, but also check "anthropic_version"
	if v, ok := raw["anthropic_version"]; ok {
		var m string
		if err := json.Unmarshal(v, &m); err == nil && m != "" {
			// Try to get model from a different key or default
			if modelRaw, ok := raw["model"]; ok {
				var model string
				if err := json.Unmarshal(modelRaw, &model); err == nil && model != "" {
					return model, nil
				}
			}
		}
	}
	return "", gerror.New("model is required in request body")
}

func checkBalance(ctx context.Context, userID int64) error {
	var row struct {
		BalanceMicro int64 `json:"balance_micro"`
	}
	err := g.DB().Model("accounts").Ctx(ctx).
		Where("owner_type", "user").
		Where("owner_id", userID).
		Where("asset", "credits").
		Fields("balance_micro").
		Scan(&row)
	if err != nil {
		return gerror.Wrap(err, "query balance failed")
	}
	if row.BalanceMicro <= 0 {
		return gerror.New("insufficient credits")
	}
	return nil
}

func resolveModelSpec(ctx context.Context, modelCode string) (int64, string, error) {
	var row struct {
		ID        int64  `json:"id"`
		ModelCode string `json:"model_code"`
	}
	err := g.DB().Model("llm_model_specs").Ctx(ctx).
		Where("model_code", modelCode).
		Where("status", "active").
		Fields("id, model_code").
		Scan(&row)
	if err != nil {
		return 0, "", gerror.Wrap(err, "query model spec failed")
	}
	if row.ID == 0 {
		return 0, "", gerror.Newf("model %s not found or inactive", modelCode)
	}
	return row.ID, row.ModelCode, nil
}

func loadActiveCandidates(ctx context.Context, modelSpecID int64) ([]candidateRow, error) {
	var rows []candidateRow
	err := g.DB().Model("llm_model_key_models AS km").Ctx(ctx).
		Fields(
			"km.id AS key_model_id",
			"km.model_key_id AS model_key_id",
			"km.model_spec_id AS model_spec_id",
			"mk.channel_id AS channel_id",
			"ch.name AS channel_name",
			"mk.provider_user_id AS provider_user_id",
			"ch.priority AS priority",
			"ch.weight AS weight",
			"km.upstream_model_name AS upstream_model_name",
			"mk.key_encrypted AS key_encrypted",
			"ch.protocols_json AS protocols_json",
			"km.provider_share_bps AS provider_share_bps",
			"km.cache_hit_price_per_1k AS cache_hit_price_per_1k",
			"km.cache_miss_price_per_1k AS cache_miss_price_per_1k",
			"km.output_price_per_1k AS output_price_per_1k",
			"km.quota_used_credits AS km_quota_used",
			"km.quota_limit_credits AS km_quota_limit",
			"mk.quota_used_credits AS mk_quota_used",
			"mk.quota_limit_credits AS mk_quota_limit",
			"km.consecutive_failures AS consecutive_failures",
		).
		InnerJoin("llm_model_keys AS mk", "km.model_key_id = mk.id").
		InnerJoin("llm_channels AS ch", "mk.channel_id = ch.id").
		Where("km.model_spec_id", modelSpecID).
		Where("km.status", "active").
		Where("mk.status", "active").
		Where("ch.status", "active").
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "query candidates failed")
	}

	out := make([]candidateRow, 0, len(rows))
	for _, r := range rows {
		if r.MKQuotaLimit > 0 && r.MKQuotaUsed >= r.MKQuotaLimit {
			continue
		}
		if r.KMQuotaLimit > 0 && r.KMQuotaUsed >= r.KMQuotaLimit {
			continue
		}
		// Channel must support at least one protocol.
		if r.ProtocolsJson == "" {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func buildChannelCandidates(rows []candidateRow, inboundFormat constant.RelayFormat) []common.ChannelCandidate {
	out := make([]common.ChannelCandidate, 0, len(rows))
	for _, r := range rows {
		score := healthScore(r.ConsecutiveFails)

		protocols := helper.ParseProtocolsJSON(r.ProtocolsJson)
		if len(protocols) == 0 {
			continue
		}
		protocolKey, baseURL := helper.BestProtocolMatch(protocols, inboundFormat)
		if protocolKey == "" {
			continue
		}

		out = append(out, common.ChannelCandidate{
			ChannelID:           r.ChannelID,
			ChannelName:         r.ChannelName,
			BaseURL:             baseURL,
			Priority:            r.Priority,
			Weight:              r.Weight,
			HealthScore:         score,
			ModelKeyID:          r.ModelKeyID,
			KeyModelID:          r.KeyModelID,
			UpstreamModelName:   r.UpstreamModelName,
			IsModelMapped:       r.UpstreamModelName != "",
			ApiKey:              r.KeyEncrypted,
			ProviderUserID:      r.ProviderUserID,
			ModelSpecID:         r.ModelSpecID,
			ProtocolKey:         protocolKey,
			ProtocolsJSON:       r.ProtocolsJson,
			ProviderShareBps:    r.ProviderShareBps,
			CacheHitPricePer1K:  r.CacheHitPricePer1K,
			CacheMissPricePer1K: r.CacheMissPricePer1K,
			OutputPricePer1K:    r.OutputPricePer1K,
		})
	}
	return out
}

func healthScore(consecutiveFails int) float64 {
	switch {
	case consecutiveFails == 0:
		return 100
	case consecutiveFails == 1:
		return 80
	case consecutiveFails == 2:
		return 60
	case consecutiveFails == 3:
		return 40
	case consecutiveFails == 4:
		return 20
	default:
		return 10
	}
}

// decryptUpstreamKey performs AES-GCM decryption of the stored upstream key.
func decryptUpstreamKey(ctx context.Context, encrypted string) (string, error) {
	return llm.DecryptKey(ctx, encrypted)
}

// enqueueSettlement builds a SettlementTask, sends it to the async worker pool,
// and returns the computed cost. Returns 0 if nothing to settle.
//
// When upstream returns usage (TotalTokens > 0), cost is computed immediately
// and returned for sync quota update. When upstream doesn\'t return usage,
// the task carries ResponseText for worker-side lazy tokenize; returns 0
// (quota update deferred to worker).
func enqueueSettlement(req *RelayRequest, picked *common.ChannelCandidate, usage *common.Usage, rawBody, respBody []byte, responseText string) int64 {
	hasUsage := usage != nil && usage.TotalTokens > 0
	hasText := responseText != ""
	if !hasUsage && !hasText {
		return 0
	}

	task := SettlementTask{
		UserID:           req.RC.UserID,
		ApiKeyID:         req.RC.ApiKeyID,
		ModelKeyID:       picked.ModelKeyID,
		KeyModelID:       picked.KeyModelID,
		ProviderUserID:   picked.ProviderUserID,
		ChannelID:        picked.ChannelID,
		ModelSpecID:      picked.ModelSpecID,
		RequestID:        req.RC.RequestID,
		Capability:       req.Capability,
		IsStream:         req.IsStream,
		Usage:            usage,
		ProviderShareBps: picked.ProviderShareBps,
		RequestBody:      rawBody,
		ResponseBody:     respBody,
	}

	if hasUsage {
		task.Cost = computeCost(usage, picked)
	} else {
		// Lazy tokenize: worker counts tokens from response text, then computes cost.
		task.ResponseText = responseText
		task.OutputPricePer1K = picked.OutputPricePer1K
		task.CacheHitPricePer1K = picked.CacheHitPricePer1K
		task.CacheMissPricePer1K = picked.CacheMissPricePer1K
		// Extract model name from request body for tiktoken.
		var body struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(rawBody, &body); err == nil && body.Model != "" {
			task.ModelName = body.Model
		}
	}

	EnqueueSettlement(task)
	return task.Cost
}

// computeCost calculates the cost in micro-credits from usage × binding prices.
// Prices from llm_model_key_models (cache_hit_price_per_1k, cache_miss_price_per_1k,
// output_price_per_1k) are stored per-1000-tokens in micro-credits.
func computeCost(usage *common.Usage, picked *common.ChannelCandidate) int64 {
	if usage == nil || picked == nil {
		return 0
	}
	// Price fields are already on the picked candidate (from DB join).
	cacheHitPrice := picked.CacheHitPricePer1K
	cacheMissPrice := picked.CacheMissPricePer1K
	outputPrice := picked.OutputPricePer1K

	if cacheHitPrice == 0 && cacheMissPrice == 0 && outputPrice == 0 {
		g.Log().Debugf(context.Background(), "[Cost] ALL ZERO — returning 0 for spec=%d ch=%d", picked.ModelSpecID, picked.ChannelID)
		return 0
	}

	// Split prompt tokens into cache hit vs miss.
	promptTokens := int64(usage.PromptTokens)
	cacheHitTokens := int64(0)
	if usage.PromptTokensDetails != nil && usage.PromptTokensDetails.CachedTokens > 0 {
		cacheHitTokens = int64(usage.PromptTokensDetails.CachedTokens)
	}
	cacheMissTokens := promptTokens - cacheHitTokens
	if cacheMissTokens < 0 {
		cacheMissTokens = promptTokens
		cacheHitTokens = 0
	}
	outputTokens := int64(usage.CompletionTokens)

	cost := int64(0)
	if cacheHitPrice > 0 && cacheHitTokens > 0 {
		cost += cacheHitPrice * cacheHitTokens / 1000
	}
	if cacheMissPrice > 0 && cacheMissTokens > 0 {
		cost += cacheMissPrice * cacheMissTokens / 1000
	}
	if outputPrice > 0 && outputTokens > 0 {
		cost += outputPrice * outputTokens / 1000
	}
	return cost
}

// updateQuotaSync atomically increments quota counters on the request path.
// Uses conditional UPDATE to prevent exceeding limits (race-condition-free).
// Errors are logged but not returned — the response has already been sent to the client.
func updateQuotaSync(ctx context.Context, modelKeyID, keyModelID, cost int64) {
	if cost <= 0 {
		return
	}

	// llm_model_keys level: quota_used += cost (if limit not exceeded).
	res, err := g.DB().Exec(ctx,
		"UPDATE llm_model_keys SET quota_used_credits = quota_used_credits + ? WHERE id = ? AND (quota_limit_credits = 0 OR quota_used_credits + ? <= quota_limit_credits)",
		cost, modelKeyID, cost)
	if err != nil {
		g.Log().Errorf(ctx, "[Relay] model_key quota update failed: id=%d cost=%d err=%v", modelKeyID, cost, err)
	} else if n, _ := res.RowsAffected(); n == 0 {
		g.Log().Warningf(ctx, "[Relay] model_key=%d quota exceeded (cost=%d)", modelKeyID, cost)
	}

	// llm_model_key_models level: same pattern.
	res, err = g.DB().Exec(ctx,
		"UPDATE llm_model_key_models SET quota_used_credits = quota_used_credits + ? WHERE id = ? AND (quota_limit_credits = 0 OR quota_used_credits + ? <= quota_limit_credits)",
		cost, keyModelID, cost)
	if err != nil {
		g.Log().Errorf(ctx, "[Relay] key_model quota update failed: id=%d cost=%d err=%v", keyModelID, cost, err)
	} else if n, _ := res.RowsAffected(); n == 0 {
		g.Log().Warningf(ctx, "[Relay] key_model=%d quota exceeded (cost=%d)", keyModelID, cost)
	}
}

// StripModelSuffix removes thinking/effort suffixes from model names.
// Scans known suffixes so "claude-3.5-sonnet-thinking" strips only "-thinking",
// not everything after the first dash.
func StripModelSuffix(model string) string {
	knownSuffixes := []string{
		"-thinking", "-nothinking",
		"-high", "-medium", "-low", "-xhigh", "-minimal", "-max",
	}
	for _, suffix := range knownSuffixes {
		if strings.HasSuffix(model, suffix) {
			return model[:len(model)-len(suffix)]
		}
	}
	return model
}
