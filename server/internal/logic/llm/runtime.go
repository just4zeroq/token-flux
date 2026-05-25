package llm

import (
	"context"
	"encoding/json"
	"math/rand"
	"strings"
	"time"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

const protocolOpenAICompatible = "openai-compatible"

// scheduledKeyModel is the resolved candidate selected for a proxy call.
type scheduledKeyModel struct {
	ModelSpecID       int64
	ModelCode         string
	ModelKeyID        int64
	KeyModelID        int64
	ProviderUserID    int64
	ChannelID         int64
	UpstreamModelName string
	KeyEncrypted      string
	ProtocolsJson     string
	ProviderShareBps  int
}

// ListAvailableModels returns OpenAI-compatible model list visible to the given API key.
func (s *sLLM) ListAvailableModels(ctx context.Context, apiKeyID, userID int64) ([]*dto.OpenAIModelInfo, error) {
	// Load API key limits.
	var keyRow struct {
		ID                 int64  `json:"id"`
		ModelLimits        string `json:"model_limits"`
		ModelLimitsEnabled bool   `json:"model_limits_enabled"`
	}
	err := g.DB().Model("api_keys").Ctx(ctx).Where("id", apiKeyID).Scan(&keyRow)
	if err != nil {
		return nil, gerror.Wrap(err, "query api key failed")
	}

	var allowedCodes map[string]bool
	if keyRow.ID != 0 && keyRow.ModelLimitsEnabled && keyRow.ModelLimits != "" {
		allowedCodes = make(map[string]bool)
		for _, c := range strings.Split(keyRow.ModelLimits, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				allowedCodes[c] = true
			}
		}
	}

	// Fetch active model specs.
	var specRows []*modelSpecRow
	err = g.DB().Model("llm_model_specs").Ctx(ctx).Where("status", "active").Scan(&specRows)
	if err != nil {
		return nil, gerror.Wrap(err, "query model specs failed")
	}

	out := make([]*dto.OpenAIModelInfo, 0, len(specRows))
	for _, spec := range specRows {
		if allowedCodes != nil && !allowedCodes[spec.ModelCode] {
			continue
		}
		// Check at least one active candidate exists.
		candidates, err := loadCandidates(ctx, spec.ID)
		if err != nil {
			return nil, err
		}
		if len(candidates) == 0 {
			continue
		}
		out = append(out, &dto.OpenAIModelInfo{
			ID:      spec.ModelCode,
			Object:  "model",
			Created: spec.CreatedAt.Unix(),
			OwnedBy: spec.DeveloperName,
		})
	}
	return out, nil
}

// candidateRow is the joined view of an active key-model candidate.
type candidateRow struct {
	KeyModelID        int64  `json:"key_model_id"`
	ModelKeyID        int64  `json:"model_key_id"`
	ChannelID         int64  `json:"channel_id"`
	ProviderUserID    int64  `json:"provider_user_id"`
	UpstreamModelName string `json:"upstream_model_name"`
	KeyEncrypted      string `json:"key_encrypted"`
	ProtocolsJson     string `json:"protocols_json"`
	ProviderShareBps  int    `json:"provider_share_bps"`
	KMQuotaUsed       int64  `json:"km_quota_used"`
	KMQuotaLimit      int64  `json:"km_quota_limit"`
	MKQuotaUsed       int64  `json:"mk_quota_used"`
	MKQuotaLimit      int64  `json:"mk_quota_limit"`
}

// loadCandidates returns all active candidates joined across key-model x model-key x channel
// after filtering by quota and openai-compatible protocol.
func loadCandidates(ctx context.Context, modelSpecID int64) ([]candidateRow, error) {
	var rows []candidateRow
	err := g.DB().Model("llm_model_key_models AS km").Ctx(ctx).
		Fields(
			"km.id AS key_model_id",
			"km.model_key_id AS model_key_id",
			"mk.channel_id AS channel_id",
			"mk.provider_user_id AS provider_user_id",
			"km.upstream_model_name AS upstream_model_name",
			"mk.key_encrypted AS key_encrypted",
			"ch.protocols_json AS protocols_json",
			"km.provider_share_bps AS provider_share_bps",
			"km.quota_used_credits AS km_quota_used",
			"km.quota_limit_credits AS km_quota_limit",
			"mk.quota_used_credits AS mk_quota_used",
			"mk.quota_limit_credits AS mk_quota_limit",
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
		// Skip exhausted quotas.
		if r.MKQuotaLimit > 0 && r.MKQuotaUsed >= r.MKQuotaLimit {
			continue
		}
		if r.KMQuotaLimit > 0 && r.KMQuotaUsed >= r.KMQuotaLimit {
			continue
		}
		// Channel must support openai-compatible.
		if !channelSupportsOpenAI(r.ProtocolsJson) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// channelSupportsOpenAI returns true when the channel's protocols_json contains
// the openai-compatible key (object form) or includes it in an array form.
func channelSupportsOpenAI(protocolsJson string) bool {
	if protocolsJson == "" {
		return false
	}
	// Try object form: {"openai-compatible": {...}}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(protocolsJson), &obj); err == nil {
		if _, ok := obj[protocolOpenAICompatible]; ok {
			return true
		}
		return false
	}
	// Fall back to array form: ["openai-compatible", ...]
	var arr []string
	if err := json.Unmarshal([]byte(protocolsJson), &arr); err == nil {
		for _, p := range arr {
			if p == protocolOpenAICompatible {
				return true
			}
		}
	}
	return false
}

// pickRandomKeyModel resolves the model spec, gathers active candidates, and returns one at random.
// `capability` is reserved for future filtering (e.g. per-capability key restrictions).
func pickRandomKeyModel(ctx context.Context, modelCode string, capability string) (scheduledKeyModel, error) {
	_ = capability

	var spec modelSpecRow
	err := g.DB().Model("llm_model_specs").Ctx(ctx).
		Where("model_code", modelCode).
		Where("status", "active").
		Scan(&spec)
	if err != nil {
		return scheduledKeyModel{}, gerror.Wrap(err, "query model spec failed")
	}
	if spec.ID == 0 {
		return scheduledKeyModel{}, gerror.Newf("model %s not found or inactive", modelCode)
	}

	candidates, err := loadCandidates(ctx, spec.ID)
	if err != nil {
		return scheduledKeyModel{}, err
	}
	if len(candidates) == 0 {
		return scheduledKeyModel{}, gerror.Newf("no available key for model %s", modelCode)
	}

	pick := candidates[rand.Intn(len(candidates))]
	return scheduledKeyModel{
		ModelSpecID:       spec.ID,
		ModelCode:         spec.ModelCode,
		ModelKeyID:        pick.ModelKeyID,
		KeyModelID:        pick.KeyModelID,
		ProviderUserID:    pick.ProviderUserID,
		ChannelID:         pick.ChannelID,
		UpstreamModelName: pick.UpstreamModelName,
		KeyEncrypted:      pick.KeyEncrypted,
		ProtocolsJson:     pick.ProtocolsJson,
		ProviderShareBps:  pick.ProviderShareBps,
	}, nil
}

// extractBaseURL pulls the upstream base URL from the channel's protocols_json.
// Convention: {"openai-compatible": {"base_url": "https://..."}}.
// Falls back to a top-level "base_url" string field.
func extractBaseURL(protocolsJson string) (string, error) {
	if protocolsJson == "" {
		return "", gerror.New("channel protocols_json is empty")
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(protocolsJson), &obj); err != nil {
		return "", gerror.Wrap(err, "channel protocols_json is not a JSON object")
	}
	if raw, ok := obj[protocolOpenAICompatible]; ok {
		var inner struct {
			BaseURL string `json:"base_url"`
		}
		if err := json.Unmarshal(raw, &inner); err == nil && inner.BaseURL != "" {
			return inner.BaseURL, nil
		}
	}
	if raw, ok := obj["base_url"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && s != "" {
			return s, nil
		}
	}
	return "", gerror.New("channel base_url not configured")
}

func (s *sLLM) ProxyChatCompletions(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error) {
	return s.proxyOpenAI(ctx, in, "/v1/chat/completions", "chat")
}

func (s *sLLM) ProxyCompletions(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error) {
	return s.proxyOpenAI(ctx, in, "/v1/completions", "completion")
}

func (s *sLLM) ProxyEmbeddings(ctx context.Context, in dto.OpenAIProxyRequest) (*dto.OpenAIProxyResponse, error) {
	return s.proxyOpenAI(ctx, in, "/v1/embeddings", "embedding")
}

// proxyOpenAI is the shared OpenAI-compatible proxy flow.
func (s *sLLM) proxyOpenAI(ctx context.Context, in dto.OpenAIProxyRequest, path, capability string) (*dto.OpenAIProxyResponse, error) {
	// 1. Credits check.
	var balance struct {
		BalanceMicro int64 `json:"balance_micro"`
	}
	err := g.DB().Model("accounts").Ctx(ctx).
		Where("owner_type", "user").
		Where("owner_id", in.UserID).
		Where("asset", "credits").
		Fields("balance_micro").
		Scan(&balance)
	if err != nil {
		return nil, gerror.Wrap(err, "query consumer balance failed")
	}
	if balance.BalanceMicro <= 0 {
		return nil, gerror.New("insufficient credits")
	}

	// 2. Extract model from request body.
	modelCode, err := extractRequestedModel(in.RawBody)
	if err != nil {
		return nil, err
	}

	// 3. Schedule a candidate.
	scheduled, err := pickRandomKeyModel(ctx, modelCode, capability)
	if err != nil {
		return nil, err
	}

	// 4. Decrypt upstream key.
	apiKey, err := decryptKey(scheduled.KeyEncrypted)
	if err != nil {
		return nil, gerror.Wrapf(err, "decrypt key failed for model_key_id=%d", scheduled.ModelKeyID)
	}

	// 5. Determine upstream base_url.
	baseURL, err := extractBaseURL(scheduled.ProtocolsJson)
	if err != nil {
		return nil, gerror.Wrapf(err, "channel_id=%d base_url lookup", scheduled.ChannelID)
	}

	// 6. Replace request model.
	newBody, _, err := replaceRequestModel(in.RawBody, scheduled.UpstreamModelName)
	if err != nil {
		return nil, err
	}

	// 7. Call upstream and time it.
	start := time.Now()
	status, respBody, headers, callErr := proxyOpenAICompatible(ctx, baseURL, apiKey, path, newBody)
	latencyMs := int(time.Since(start).Milliseconds())

	// 8. Failure path: log failure record and return without settlement.
	if callErr != nil || status >= 400 {
		errCode := ""
		errMessage := ""
		if callErr != nil {
			errCode = "upstream_error"
			errMessage = callErr.Error()
		} else {
			errCode = http2errCode(status)
			errMessage = truncate(string(respBody), 1024)
		}
		_, insertErr := g.DB().Model("llm_usage_records").Ctx(ctx).Data(g.Map{
			"consumer_user_id": in.UserID,
			"virtual_key_id":   in.ApiKeyID,
			"model_spec_id":    scheduled.ModelSpecID,
			"model_key_id":     scheduled.ModelKeyID,
			"key_model_id":     scheduled.KeyModelID,
			"provider_user_id": scheduled.ProviderUserID,
			"channel_id":       scheduled.ChannelID,
			"request_id":       in.RequestID,
			"capability":       capability,
			"is_stream":        in.IsStream,
			"latency_ms":       latencyMs,
			"status":           "failed",
			"error_code":       errCode,
			"error_message":    errMessage,
		}).Insert()
		if insertErr != nil {
			g.Log().Warningf(ctx, "insert failed usage record failed: %v", insertErr)
		}
		if callErr != nil {
			return nil, callErr
		}
		return &dto.OpenAIProxyResponse{StatusCode: status, Body: respBody, Headers: headers}, nil
	}

	// 9. Success: normalize usage and compute cost.
	usage := normalizeOpenAIUsage(respBody)
	var price modelPriceRow
	err = g.DB().Model("llm_model_prices").Ctx(ctx).
		Where("model_spec_id", scheduled.ModelSpecID).
		Where("capability", capability).
		Where("status", "active").
		Scan(&price)
	if err != nil {
		return nil, gerror.Wrap(err, "query model price failed")
	}

	var cost, revenue, commission int64
	if price.ID == 0 {
		g.Log().Warningf(ctx, "no active price for model_spec_id=%d capability=%s; settling with zero cost",
			scheduled.ModelSpecID, capability)
	} else {
		cost = price.CacheHitPricePer1K*int64(usage.CacheHitTokens)/1000 +
			price.CacheMissPricePer1K*int64(usage.CacheMissTokens)/1000 +
			price.OutputPricePer1K*int64(usage.OutputTokens)/1000
		revenue = cost * int64(scheduled.ProviderShareBps) / 10000
		commission = cost - revenue
	}

	// 10. Insert success usage record.
	insertRes, err := g.DB().Model("llm_usage_records").Ctx(ctx).Data(g.Map{
		"consumer_user_id":         in.UserID,
		"virtual_key_id":           in.ApiKeyID,
		"model_spec_id":            scheduled.ModelSpecID,
		"model_key_id":             scheduled.ModelKeyID,
		"key_model_id":             scheduled.KeyModelID,
		"provider_user_id":         scheduled.ProviderUserID,
		"channel_id":               scheduled.ChannelID,
		"request_id":               in.RequestID,
		"capability":               capability,
		"is_stream":                in.IsStream,
		"input_tokens":             usage.InputTokens,
		"cache_hit_tokens":         usage.CacheHitTokens,
		"cache_miss_tokens":        usage.CacheMissTokens,
		"output_tokens":            usage.OutputTokens,
		"total_tokens":             usage.TotalTokens,
		"cost_credits":             cost,
		"provider_revenue_credits": revenue,
		"commission_credits":       commission,
		"latency_ms":               latencyMs,
		"status":                   "success",
	}).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "insert usage record failed")
	}
	usageID, _ := insertRes.LastInsertId()

	// 11. Settle (idempotent on ref_type+ref_id).
	_, err = service.Settlement().SubmitAndSettle(ctx, dto.SettlementSubmitIn{
		ProductType:            "llm",
		RefType:                "llm_usage_record",
		RefID:                  usageID,
		ConsumerUserID:         in.UserID,
		ProviderUserID:         scheduled.ProviderUserID,
		CostCredits:            cost,
		ProviderRevenueCredits: revenue,
		CommissionCredits:      commission,
	})
	if err != nil {
		g.Log().Errorf(ctx, "settle usage_record=%d failed: %v", usageID, err)
	}

	// 12. Quota tracking on key + key-model.
	if cost > 0 {
		_, qerr := g.DB().Model("llm_model_keys").Ctx(ctx).
			Where("id", scheduled.ModelKeyID).
			Data(g.Map{"quota_used_credits": gdb.Raw("quota_used_credits + " + i64s(cost))}).
			Update()
		if qerr != nil {
			g.Log().Warningf(ctx, "update model_key quota failed key_id=%d: %v", scheduled.ModelKeyID, qerr)
		}
		_, qerr = g.DB().Model("llm_model_key_models").Ctx(ctx).
			Where("id", scheduled.KeyModelID).
			Data(g.Map{"quota_used_credits": gdb.Raw("quota_used_credits + " + i64s(cost))}).
			Update()
		if qerr != nil {
			g.Log().Warningf(ctx, "update key_model quota failed key_model_id=%d: %v", scheduled.KeyModelID, qerr)
		}
	}

	// 13. Overdraft check: if consumer balance went negative, disable their api keys.
	var post struct {
		BalanceMicro int64 `json:"balance_micro"`
	}
	err = g.DB().Model("accounts").Ctx(ctx).
		Where("owner_type", "user").
		Where("owner_id", in.UserID).
		Where("asset", "credits").
		Fields("balance_micro").
		Scan(&post)
	if err == nil && post.BalanceMicro < 0 {
		_, derr := g.DB().Model("api_keys").Ctx(ctx).
			Where("user_id", in.UserID).
			Where("status", 1).
			Where("deleted_at IS NULL").
			Data(g.Map{
				"status":           0,
				"disabled_reason":  "overdraft",
				"updated_at":       gdb.Raw("NOW()"),
			}).Update()
		if derr != nil {
			g.Log().Warningf(ctx, "disable overdraft api_keys failed user=%d: %v", in.UserID, derr)
		}
	}

	// 14. Return upstream response.
	return &dto.OpenAIProxyResponse{StatusCode: status, Body: respBody, Headers: headers}, nil
}

func i64s(n int64) string {
	// Minimal integer-to-string for gdb.Raw composition. strconv is safer but adds an import;
	// fmt-style would also work. Keep it explicit and dependency-free.
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func http2errCode(status int) string {
	switch {
	case status == 401 || status == 403:
		return "upstream_auth"
	case status == 404:
		return "upstream_not_found"
	case status == 429:
		return "upstream_rate_limit"
	case status >= 500:
		return "upstream_5xx"
	case status >= 400:
		return "upstream_4xx"
	default:
		return "upstream_error"
	}
}
