package channel

import (
	"encoding/json"
	"fmt"
	"time"

	"ai-platform-node/pkg/db"
	"ai-platform/pkg/model"
)

// Channel represents an upstream provider connection.
type Channel struct {
	ID           int64               `json:"id"`
	Code         string              `json:"code"`
	Name         string              `json:"name"`
	Description  string              `json:"description"`
	ProviderType model.ProviderType   `json:"provider_type"`
	Protocols    []model.ProtocolEntry `json:"protocols"`
	Status       string              `json:"status"`
	Source       string              `json:"source"`
	CreatedAt    int64               `json:"created_at"`
	UpdatedAt    int64               `json:"updated_at"`
}

// List returns all channels.
func List() ([]Channel, error) {
	rows, err := db.DB().Query(
		`SELECT id, code, name, description, provider_type, protocols_json, status, source, created_at, updated_at
		 FROM llm_channels ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		var c Channel
		var pj string
		if err := rows.Scan(&c.ID, &c.Code, &c.Name, &c.Description, &c.ProviderType,
			&pj, &c.Status, &c.Source, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(pj), &c.Protocols)
		if c.Protocols == nil {
			c.Protocols = []model.ProtocolEntry{}
		}
		out = append(out, c)
	}
	return out, nil
}

// GetByID returns a single channel.
func GetByID(id int64) (*Channel, error) {
	var c Channel
	var pj string
	err := db.DB().QueryRow(
		`SELECT id, code, name, description, provider_type, protocols_json, status, source, created_at, updated_at
		 FROM llm_channels WHERE id = ?`, id,
	).Scan(&c.ID, &c.Code, &c.Name, &c.Description, &c.ProviderType,
		&pj, &c.Status, &c.Source, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("channel %d: %w", id, err)
	}
	json.Unmarshal([]byte(pj), &c.Protocols)
	if c.Protocols == nil {
		c.Protocols = []model.ProtocolEntry{}
	}
	return &c, nil
}

// Create inserts a new channel with protocols.
func Create(code, name, description string, providerType model.ProviderType, protocols []model.ProtocolEntry) (*Channel, error) {
	now := time.Now().Unix()

	if len(protocols) == 0 {
		defaultURL := defaultBaseURL(providerType)
		protocols = []model.ProtocolEntry{{
			Protocol: resolveProtocol(providerType),
			BaseURL:  defaultURL,
		}}
	}

	pj, _ := json.Marshal(protocols)

	_, err := db.DB().Exec(
		`INSERT INTO llm_channels(code, name, description, provider_type, protocols_json, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 'active', ?, ?)`,
		code, name, description, providerType, string(pj), now, now)
	if err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}

	var id int64
	db.DB().QueryRow("SELECT last_insert_rowid()").Scan(&id)
	return GetByID(id)
}

// Update modifies channel name, protocols, or status.
func Update(id int64, name string, protocols []model.ProtocolEntry, status string) (*Channel, error) {
	now := time.Now().Unix()
	pj, _ := json.Marshal(protocols)
	_, err := db.DB().Exec(
		`UPDATE llm_channels SET name = ?, protocols_json = ?, status = ?, updated_at = ? WHERE id = ?`,
		name, string(pj), status, now, id)
	if err != nil {
		return nil, fmt.Errorf("update channel: %w", err)
	}
	return GetByID(id)
}

// Delete removes a channel.
func Delete(id int64) error {
	_, err := db.DB().Exec("DELETE FROM llm_channels WHERE id = ?", id)
	return err
}


// UnbindChannelModel removes a channel-model binding.
func UnbindChannelModel(bindingID int64) error {
	_, err := db.DB().Exec("DELETE FROM llm_channel_models WHERE id = ?", bindingID)
	return err
}

// BindChannelModels creates model bindings for a channel.
func BindChannelModels(channelID int64, modelIDs []int64) error {
	now := time.Now().Unix()
	for _, msid := range modelIDs {
		_, err := db.DB().Exec(
			`INSERT OR IGNORE INTO llm_channel_models(channel_id, model_spec_id, upstream_model_name, source, created_at, updated_at)
			 VALUES (?, ?, '', 'local', ?, ?)`,
			channelID, msid, now, now)
		if err != nil {
			return fmt.Errorf("bind model %d to channel %d: %w", msid, channelID, err)
		}
	}
	return nil
}

// ---- helpers ----


func resolveProtocol(pt model.ProviderType) model.ProtocolType {
	switch pt {
	case model.ProviderClaude:
		return model.ProtocolClaude
	case model.ProviderGemini, model.ProviderVertex:
		return model.ProtocolGemini
	case model.ProviderAli:
		return model.ProtocolAli
	case model.ProviderBaiduV2:
		return model.ProtocolBaidu
	case model.ProviderZhipu:
		return model.ProtocolZhipu
	case model.ProviderVolcengine:
		return model.ProtocolVolcengine
	case model.ProviderAWS:
		return model.ProtocolAWS
	default:
		return model.ProtocolOpenAI
	}
}

func defaultBaseURL(pt model.ProviderType) string {
	switch pt {
	case model.ProviderOpenAI:
		return "https://api.openai.com/v1"
	case model.ProviderClaude:
		return "https://api.anthropic.com/v1"
	case model.ProviderGemini:
		return "https://generativelanguage.googleapis.com"
	case model.ProviderDeepSeek:
		return "https://api.deepseek.com"
	case model.ProviderAli:
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	case model.ProviderZhipu:
		return "https://open.bigmodel.cn/api/paas/v4"
	case model.ProviderBaiduV2:
		return "https://aip.baidubce.com"
	case model.ProviderTencent:
		return "https://api.hunyuan.cloud.tencent.com"
	case model.ProviderMoonshot:
		return "https://api.moonshot.cn/v1"
	case model.ProviderVolcengine:
		return "https://ark.cn-beijing.volces.com/api/v3"
	case model.ProviderSiliconFlow:
		return "https://api.siliconflow.cn/v1"
	case model.ProviderXAI:
		return "https://api.x.ai/v1"
	case model.ProviderMistral:
		return "https://api.mistral.ai/v1"
	default:
		return ""
	}
}
// ChannelModelBinding represents a channel-model binding from the local DB.
type ChannelModelBinding struct {
	ID                int64  `json:"id"`
	ChannelID         int64  `json:"channel_id"`
	ModelSpecID       int64  `json:"model_spec_id"`
	UpstreamModelName string `json:"upstream_model_name"`
	ModelName         string `json:"model_name"`
	ModelCode         string `json:"model_code"`
	DeveloperName     string `json:"developer_name"`
	Source            string `json:"source"`
	CreatedAt         int64  `json:"created_at"`
}

// ListChannelModels returns all model bindings for a given channel.
func ListChannelModels(channelID int64) ([]ChannelModelBinding, error) {
	rows, err := db.DB().Query(
		`SELECT cm.id, cm.channel_id, cm.model_spec_id, cm.upstream_model_name,
			 ms.model_name, ms.model_code, ms.developer_name, cm.source, cm.created_at
			 FROM llm_channel_models cm
			 LEFT JOIN llm_model_specs ms ON ms.id = cm.model_spec_id
			 WHERE cm.channel_id = ? ORDER BY ms.model_name`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChannelModelBinding
	for rows.Next() {
		var b ChannelModelBinding
		if err := rows.Scan(&b.ID, &b.ChannelID, &b.ModelSpecID, &b.UpstreamModelName,
			&b.ModelName, &b.ModelCode, &b.DeveloperName, &b.Source, &b.CreatedAt); err != nil {
			continue
		}
		out = append(out, b)
	}
	return out, nil
}

