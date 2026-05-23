package identity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"ai-platform/internal/model/dto"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

func (s *sIdentity) CreateApiKey(ctx context.Context, userID int64, in dto.CreateApiKeyIn) (*dto.ApiKeyOut, error) {
	key := "sk-" + generateAPIKeySuffix()

	data := g.Map{
		"user_id":    userID,
		"key":        key,
		"name":       in.Name,
		"status":     1,
		"created_at": gtime.Now(),
		"updated_at": gtime.Now(),
	}
	if in.QuotaCredits != nil {
		data["quota_credits"] = *in.QuotaCredits
		data["remain_quota"] = *in.QuotaCredits
	}
	if in.UnlimitedQuota {
		data["unlimited_quota"] = true
	}

	result, err := g.DB().Model("api_keys").Ctx(ctx).Data(data).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "create api key failed")
	}
	id, _ := result.LastInsertId()

	return &dto.ApiKeyOut{
		ID:        id,
		Key:       key,
		Name:      in.Name,
		Status:    1,
		CreatedAt: time.Now(),
	}, nil
}

func (s *sIdentity) ListApiKeys(ctx context.Context, userID int64) ([]*dto.ApiKeyInfo, error) {
	records, err := g.DB().Model("api_keys").Ctx(ctx).
		Where("user_id", userID).
		WhereNull("deleted_at").
		OrderDesc("id").
		All()
	if err != nil {
		return nil, gerror.Wrap(err, "list api keys failed")
	}

	out := make([]*dto.ApiKeyInfo, 0, len(records))
	for _, r := range records {
		info := &dto.ApiKeyInfo{
			ID:             r["id"].Int64(),
			UserID:         r["user_id"].Int64(),
			Name:           r["name"].String(),
			Status:         r["status"].Int(),
			RemainQuota:    r["remain_quota"].Int64(),
			UsedQuota:      r["used_quota"].Int64(),
			UnlimitedQuota: r["unlimited_quota"].Bool(),
			CreatedAt:      r["created_at"].Time(),
		}
		if v := r["quota_credits"]; !v.IsNil() {
			q := v.Int64()
			info.QuotaCredits = &q
		}
		if v := r["expire_time"]; !v.IsNil() {
			t := v.Time()
			info.ExpireAt = &t
		}
		out = append(out, info)
	}
	return out, nil
}

func (s *sIdentity) DeleteApiKey(ctx context.Context, userID, keyID int64) error {
	_, err := g.DB().Model("api_keys").Ctx(ctx).
		Where("id", keyID).
		Where("user_id", userID).
		WhereNull("deleted_at").
		Data(g.Map{
			"deleted_at": gtime.Now(),
			"updated_at": gtime.Now(),
		}).
		Update()
	if err != nil {
		return gerror.Wrap(err, "delete api key failed")
	}
	return nil
}

func (s *sIdentity) ValidateApiKey(ctx context.Context, key string) (*dto.ApiKeyInfo, error) {
	if len(key) < 4 || key[:3] != "sk-" {
		return nil, gerror.New("invalid api key format")
	}

	record, err := g.DB().Model("api_keys").Ctx(ctx).
		Where("key", key).
		Where("status", 1).
		WhereNull("deleted_at").
		One()
	if err != nil {
		return nil, gerror.Wrap(err, "query api key failed")
	}
	if record.IsEmpty() {
		return nil, gerror.New("api key not found or revoked")
	}

	if expireAt := record["expire_time"]; !expireAt.IsNil() {
		if expireAt.Time().Before(time.Now()) {
			return nil, gerror.New("api key expired")
		}
	}

	return &dto.ApiKeyInfo{
		ID:             record["id"].Int64(),
		UserID:         record["user_id"].Int64(),
		Name:           record["name"].String(),
		Status:         record["status"].Int(),
		RemainQuota:    record["remain_quota"].Int64(),
		UsedQuota:      record["used_quota"].Int64(),
		UnlimitedQuota: record["unlimited_quota"].Bool(),
	}, nil
}

func generateAPIKeySuffix() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
