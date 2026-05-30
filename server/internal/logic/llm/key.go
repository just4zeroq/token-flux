package llm

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"

	"ai-platform/internal/model/dto"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func getEncryptionKey(ctx context.Context) (string, error) {
	key := g.Cfg().MustGet(ctx, "modelKey.encryptionKey").String()
	if key == "" {
		key = os.Getenv("MODEL_KEY_ENCRYPTION_KEY")
	}
	if key == "" {
		return "", gerror.New("MODEL_KEY_ENCRYPTION_KEY not set")
	}
	return key, nil
}

func encryptKey(ctx context.Context, plaintext string) (string, error) {
	hexKey, err := getEncryptionKey(ctx)
	if err != nil {
		return "", err
	}
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", gerror.Wrap(err, "invalid MODEL_KEY_ENCRYPTION_KEY hex")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", gerror.Wrap(err, "create AES cipher failed")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", gerror.Wrap(err, "create GCM failed")
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", gerror.Wrap(err, "generate nonce failed")
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

func decryptKey(ctx context.Context, ciphertextHex string) (string, error) {
	return DecryptKey(ctx, ciphertextHex)
}

// DecryptKey is the exported version for use by the relay handler.
func DecryptKey(ctx context.Context, ciphertextHex string) (string, error) {
	hexKey, err := getEncryptionKey(ctx)
	if err != nil {
		return "", err
	}
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", gerror.Wrap(err, "invalid MODEL_KEY_ENCRYPTION_KEY hex")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", gerror.Wrap(err, "create AES cipher failed")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", gerror.Wrap(err, "create GCM failed")
	}
	ciphertext, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", gerror.Wrap(err, "invalid ciphertext hex")
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", gerror.New("ciphertext too short")
	}
	nonce, sealed := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", gerror.Wrap(err, "decrypt failed")
	}
	return string(plaintext), nil
}

func maskKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return "****" + key[len(key)-4:]
}

// --- Model Key row and helpers ---

type modelKeyRow struct {
	ID                int64       `json:"id"`
	ProviderUserID    int64       `json:"provider_user_id"`
	ChannelID         int64       `json:"channel_id"`
	Name              string      `json:"name"`
	KeyEncrypted      string      `json:"key_encrypted"`
	KeyMasked         string      `json:"key_masked"`
	QuotaLimitCredits int64       `json:"quota_limit_credits"`
	QuotaUsedCredits  int64       `json:"quota_used_credits"`
	Status            string      `json:"status"`
	LastTestAt        *gtime.Time `json:"last_test_at"`
	LastTestStatus    string      `json:"last_test_status"`
	LastTestError     string      `json:"last_test_error"`
	TestAttempts      int         `json:"test_attempts"`
	CreatedAt         gtime.Time  `json:"created_at"`
	UpdatedAt         gtime.Time  `json:"updated_at"`
}

func (r *modelKeyRow) toDTO() *dto.LLMModelKeyInfo {
	info := &dto.LLMModelKeyInfo{
		ID:                r.ID,
		ProviderUserID:    r.ProviderUserID,
		ChannelID:         r.ChannelID,
		Name:              r.Name,
		KeyMasked:         r.KeyMasked,
		QuotaUsedCredits:  r.QuotaUsedCredits,
		Status:            r.Status,
		LastTestStatus:    r.LastTestStatus,
		LastTestError:     r.LastTestError,
		TestAttempts:      r.TestAttempts,
		CreatedAt:         r.CreatedAt.Time,
		UpdatedAt:         r.UpdatedAt.Time,
	}
	if r.LastTestAt != nil {
		info.LastTestAt = r.LastTestAt.Time
	}
	return info
}

func (s *sLLM) CreateModelKey(ctx context.Context, providerUserID int64, in dto.LLMCreateModelKeyIn) (*dto.LLMModelKeyInfo, error) {
	// Verify channel exists and is active.
	var chRow channelRow
	err := g.DB().Model("llm_channels").Ctx(ctx).Where("id", in.ChannelID).Scan(&chRow)
	if err != nil {
		return nil, gerror.Wrap(err, "query channel failed")
	}
	if chRow.ID == 0 {
		return nil, gerror.New("channel not found")
	}
	if chRow.Status != "active" {
		return nil, gerror.New("channel is not active")
	}

	encrypted, err := encryptKey(ctx, in.Key)
	if err != nil {
		return nil, err
	}
	masked := maskKey(in.Key)

	var info *dto.LLMModelKeyInfo
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 1. Insert model key.
		result, err := tx.Model("llm_model_keys").Data(g.Map{
			"provider_user_id":    providerUserID,
			"channel_id":          in.ChannelID,
			"name":                in.Name,
			"key_encrypted":       encrypted,
			"key_masked":          masked,
			"status":              "pending",
		}).Insert()
		if err != nil {
			return gerror.Wrap(err, "insert model key failed")
		}
		keyID, _ := result.LastInsertId()

		// 2. Look up provider default share_bps.
		defaultShareBps, _ := getProviderShareBpsTx(ctx, tx, providerUserID)
		if defaultShareBps == 0 {
			defaultShareBps = 7000
		}

		// 3. Process each binding: upsert prices + bind model.
		for _, b := range in.ModelBindings {
			// Verify model spec exists.
			var msRow modelSpecRow
			if err := tx.Model("llm_model_specs").Where("id", b.ModelSpecID).Scan(&msRow); err != nil {
				return gerror.Wrap(err, "query model spec failed")
			}
			if msRow.ID == 0 {
				return gerror.Newf("model spec %d not found", b.ModelSpecID)
			}


			// Bind model to key.
			bindStatus := "pending_model_review"
			if msRow.Status == "active" {
				bindStatus = "pending_test"
			}
			_, err = tx.Model("llm_model_key_models").Data(g.Map{
				"model_key_id":        keyID,
				"model_spec_id":       b.ModelSpecID,
				"upstream_model_name": b.UpstreamModelName,
				"quota_limit_credits": 0,
				"provider_share_bps":  defaultShareBps,
				"status":              bindStatus,
				"cache_hit_price_per_1k":  b.CacheHitPricePer1K,
				"cache_miss_price_per_1k": b.CacheMissPricePer1K,
				"output_price_per_1k":    b.OutputPricePer1K,
			}).Insert()
			if err != nil {
				return gerror.Wrap(err, "insert key model binding failed")
			}
		}

		// 4. Return created key.
		var row modelKeyRow
		if err := tx.Model("llm_model_keys").Where("id", keyID).Scan(&row); err != nil {
			return gerror.Wrap(err, "query created model key failed")
		}
		info = row.toDTO()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return info, nil
}

// getProviderShareBpsTx looks up the provider's default share_bps within a transaction.
func getProviderShareBpsTx(ctx context.Context, tx gdb.TX, userID int64) (int, error) {
	var row struct {
		ShareBps int `json:"share_bps"`
	}
	err := tx.Model("provider_settings").Where("user_id", userID).Scan(&row)
	if err != nil {
		return 0, gerror.Wrap(err, "query provider settings failed")
	}
	return row.ShareBps, nil
}

func (s *sLLM) GetProviderShareBps(ctx context.Context, userID int64) (int, error) {
	var row struct {
		ShareBps int `json:"share_bps"`
	}
	err := g.DB().Model("provider_settings").Ctx(ctx).Where("user_id", userID).Scan(&row)
	if err != nil {
		return 0, gerror.Wrap(err, "query provider settings failed")
	}
	if row.ShareBps == 0 {
		return 7000, nil
	}
	return row.ShareBps, nil
}

func (s *sLLM) SetProviderShareBps(ctx context.Context, userID int64, shareBps int) error {
	_, err := g.DB().Model("provider_settings").Ctx(ctx).Data(g.Map{
		"user_id":   userID,
		"share_bps": shareBps,
	}).Insert()
	if err != nil {
		// Duplicate key — update instead.
		_, err = g.DB().Model("provider_settings").Ctx(ctx).
			Where("user_id", userID).
			Data(g.Map{"share_bps": shareBps}).Update()
	}
	return gerror.Wrap(err, "set provider share_bps failed")
}


func (s *sLLM) ListModelKeys(ctx context.Context, in dto.LLMListModelKeysIn) ([]*dto.LLMModelKeyInfo, int, error) {
	page := in.Page
	if page <= 0 {
		page = 1
	}
	size := in.Size
	if size <= 0 {
		size = 20
	}

	model := g.DB().Model("llm_model_keys").Ctx(ctx)

	// Admin sees all, provider sees only own.
	if !in.IsAdmin {
		model = model.Where("provider_user_id", in.ProviderUserID)
	}
	if in.Status != "" {
		model = model.Where("status", in.Status)
	}

	total, err := model.Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count model keys failed")
	}

	var rows []*modelKeyRow
	err = model.Page(page, size).Order("id DESC").Scan(&rows)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query model keys failed")
	}

	list := make([]*dto.LLMModelKeyInfo, len(rows))
	for i, r := range rows {
		list[i] = r.toDTO()
	}
	return list, total, nil
}

func (s *sLLM) DisableModelKey(ctx context.Context, providerUserID, keyID int64, isAdmin bool) error {
	var row modelKeyRow
	err := g.DB().Model("llm_model_keys").Ctx(ctx).Where("id", keyID).Scan(&row)
	if err != nil {
		return gerror.Wrap(err, "query model key failed")
	}
	if row.ID == 0 {
		return gerror.New("model key not found")
	}
	if !isAdmin && row.ProviderUserID != providerUserID {
		return gerror.New("not authorized to disable this key")
	}

	_, err = g.DB().Model("llm_model_keys").Ctx(ctx).
		Where("id", keyID).
		Data(g.Map{"status": "disabled"}).
		Update()
	if err != nil {
		return gerror.Wrap(err, "disable model key failed")
	}
	return nil
}

// --- Key-Model binding row and helpers ---

type keyModelRow struct {
	ID                  int64       `json:"id"`
	ModelKeyID          int64       `json:"model_key_id"`
	ModelSpecID         int64       `json:"model_spec_id"`
	UpstreamModelName   string      `json:"upstream_model_name"`
	QuotaLimitCredits   int64       `json:"quota_limit_credits"`
	QuotaUsedCredits    int64       `json:"quota_used_credits"`
	ProviderShareBps    int         `json:"provider_share_bps"`
	CacheHitPricePer1K  int64       `json:"cache_hit_price_per_1k"`
		CacheMissPricePer1K int64       `json:"cache_miss_price_per_1k"`
		OutputPricePer1K    int64       `json:"output_price_per_1k"`
	Status              string      `json:"status"`
	TestAttempts        int         `json:"test_attempts"`
	LastTestAt          *gtime.Time `json:"last_test_at"`
	LastTestStatus      string      `json:"last_test_status"`
	LastTestError       string      `json:"last_test_error"`
	NextTestAt          *gtime.Time `json:"next_test_at"`
	ConsecutiveFailures int         `json:"consecutive_failures"`
	LastErrorCode       string      `json:"last_error_code"`
	LastErrorMessage    string      `json:"last_error_message"`
	CreatedAt           gtime.Time  `json:"created_at"`
	UpdatedAt           gtime.Time  `json:"updated_at"`
}

func (r *keyModelRow) toDTO() *dto.LLMKeyModelInfo {
	return &dto.LLMKeyModelInfo{
		ID:                  r.ID,
		ModelKeyID:          r.ModelKeyID,
		ModelSpecID:         r.ModelSpecID,
		UpstreamModelName:   r.UpstreamModelName,
		QuotaUsedCredits:    r.QuotaUsedCredits,
		Status:              r.Status,
		CacheHitPricePer1K:  r.CacheHitPricePer1K,
			CacheMissPricePer1K: r.CacheMissPricePer1K,
			OutputPricePer1K:    r.OutputPricePer1K,
		ConsecutiveFailures: r.ConsecutiveFailures,
		LastErrorCode:       r.LastErrorCode,
		LastErrorMessage:    r.LastErrorMessage,
		CreatedAt:           r.CreatedAt.Time,
		UpdatedAt:           r.UpdatedAt.Time,
	}
}

func (s *sLLM) BindKeyModel(ctx context.Context, providerUserID int64, in dto.LLMBindKeyModelIn) (*dto.LLMKeyModelInfo, error) {
	// Verify model key exists and is active.
	var mkRow modelKeyRow
	err := g.DB().Model("llm_model_keys").Ctx(ctx).Where("id", in.ModelKeyID).Scan(&mkRow)
	if err != nil {
		return nil, gerror.Wrap(err, "query model key failed")
	}
	if mkRow.ID == 0 {
		return nil, gerror.New("model key not found")
	}
	if mkRow.Status != "active" && mkRow.Status != "pending" {
		return nil, gerror.New("model key is not active")
	}
	if mkRow.ProviderUserID != providerUserID {
		return nil, gerror.New("not authorized to bind models to this key")
	}

	// Verify model spec exists.
	var msRow modelSpecRow
	err = g.DB().Model("llm_model_specs").Ctx(ctx).Where("id", in.ModelSpecID).Scan(&msRow)
	if err != nil {
		return nil, gerror.Wrap(err, "query model spec failed")
	}
	if msRow.ID == 0 {
		return nil, gerror.New("model spec not found")
	}

	// Look up provider default share_bps.
	defaultShareBps, _ := s.GetProviderShareBps(ctx, providerUserID)

	// Determine status: if model spec is active, set pending_test; otherwise pending_model_review.
	bindStatus := "pending_model_review"
	if msRow.Status == "active" {
		bindStatus = "pending_test"
	}

	result, err := g.DB().Model("llm_model_key_models").Ctx(ctx).Data(g.Map{
		"model_key_id":        in.ModelKeyID,
		"model_spec_id":       in.ModelSpecID,
		"upstream_model_name": in.UpstreamModelName,
		"quota_limit_credits": 0,
		"provider_share_bps":  defaultShareBps,
		"status":              bindStatus,
				"cache_hit_price_per_1k":  in.CacheHitPricePer1K,
				"cache_miss_price_per_1k": in.CacheMissPricePer1K,
				"output_price_per_1k":    in.OutputPricePer1K,
	}).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "insert key model binding failed")
	}

	id, _ := result.LastInsertId()
	var kmRow keyModelRow
	err = g.DB().Model("llm_model_key_models").Ctx(ctx).Where("id", id).Scan(&kmRow)
	if err != nil {
		return nil, gerror.Wrap(err, "query created key model binding failed")
	}
	return kmRow.toDTO(), nil
}

func (s *sLLM) ListKeyModels(ctx context.Context, in dto.LLMListKeyModelsIn) ([]*dto.LLMKeyModelInfo, int, error) {
	page := in.Page
	if page <= 0 {
		page = 1
	}
	size := in.Size
	if size <= 0 {
		size = 20
	}

	model := g.DB().Model("llm_model_key_models").Ctx(ctx)

	if in.ModelKeyID != 0 {
		model = model.Where("model_key_id", in.ModelKeyID)
	}
	if in.ModelSpecID != 0 {
		model = model.Where("model_spec_id", in.ModelSpecID)
	}
	if in.Status != "" {
		model = model.Where("status", in.Status)
	}

	if !in.IsAdmin {
		// Provider sees only bindings on their own keys.
		model = model.
			InnerJoin("llm_model_keys", "llm_model_key_models.model_key_id = llm_model_keys.id").
			Where("llm_model_keys.provider_user_id", in.ProviderUserID)
		model = model.Fields("llm_model_key_models.*")
	}

	total, err := model.Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count key model bindings failed")
	}

	var rows []*keyModelRow
	err = model.Page(page, size).Order("llm_model_key_models.id DESC").Scan(&rows)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query key model bindings failed")
	}

	list := make([]*dto.LLMKeyModelInfo, len(rows))
	for i, r := range rows {
		list[i] = r.toDTO()
	}
	return list, total, nil
}

func (s *sLLM) TriggerKeyModelTest(ctx context.Context, keyModelID int64) error {
	// Look up key model binding.
	var kmRow keyModelRow
	if err := g.DB().Model("llm_model_key_models").Ctx(ctx).Where("id", keyModelID).Scan(&kmRow); err != nil {
		return gerror.Wrap(err, "query key model binding failed")
	}
	if kmRow.ID == 0 {
		return gerror.New("key model binding not found")
	}

	// Look up model key for upstream key.
	var mkRow modelKeyRow
	if err := g.DB().Model("llm_model_keys").Ctx(ctx).Where("id", kmRow.ModelKeyID).Scan(&mkRow); err != nil {
		return gerror.Wrap(err, "query model key failed")
	}
	if mkRow.ID == 0 {
		return gerror.New("model key not found")
	}

	// Decrypt upstream API key.
	upstreamKey, err := decryptKey(ctx, mkRow.KeyEncrypted)
	if err != nil {
		return gerror.Wrap(err, "decrypt upstream key failed")
	}

	// Look up channel for base URL.
	var chRow channelRow
	if err := g.DB().Model("llm_channels").Ctx(ctx).Where("id", mkRow.ChannelID).Scan(&chRow); err != nil {
		return gerror.Wrap(err, "query channel failed")
	}
	if chRow.ID == 0 {
		return gerror.New("channel not found")
	}

	// Extract base URL from protocols_json.
	baseURL, err := extractBaseURL(chRow.ProtocolsJson)
	if err != nil {
		return gerror.Wrap(err, "extract base URL failed")
	}

	// Make test call to upstream /v1/models.
	statusCode, body, _, err := proxyOpenAICompatible(ctx, baseURL, upstreamKey, "/v1/models", nil)
	if err == nil && statusCode >= 200 && statusCode < 300 {
		// Success: update test status.
		_, _ = g.DB().Model("llm_model_key_models").Ctx(ctx).
			Where("id", keyModelID).
			Data(g.Map{
				"last_test_status":    "success",
				"consecutive_failures": 0,
				"test_attempts":       gdb.Raw("test_attempts + 1"),
				"last_test_at":        gtime.Now(),
			}).Update()
		return nil
	}

	// Failure: capture error.
	errMsg := "unknown"
	if err != nil {
		errMsg = err.Error()
	} else {
		errMsg = string(body)
		if len(errMsg) > 500 {
			errMsg = errMsg[:500]
		}
	}
	_, _ = g.DB().Model("llm_model_key_models").Ctx(ctx).
		Where("id", keyModelID).
		Data(g.Map{
			"last_test_status":     "failed",
			"last_test_error":      errMsg,
			"consecutive_failures": gdb.Raw("consecutive_failures + 1"),
			"test_attempts":        gdb.Raw("test_attempts + 1"),
			"last_test_at":         gtime.Now(),
		}).Update()
	return gerror.Newf("test failed: %s", errMsg)
}
