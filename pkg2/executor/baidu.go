package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ai-platform/pkg/model"
)

func init() {
	Register(model.ProviderBaiduV2, &BaiduExecutor{})
	RegisterProtocol(model.ProtocolBaidu, &BaiduExecutor{})
}

// BaiduExecutor handles Baidu ERNIE API (V2).
// OpenAI-compatible format with "token|appid" API key split, -search suffix for web search.
type BaiduExecutor struct {
	channel *model.Channel
}

func (e *BaiduExecutor) Init(channel *model.Channel) {
	e.channel = channel
}

func (e *BaiduExecutor) NativeFormats() []EndpointCapability {
	return NativeFormat("openai", model.RelayModeChatCompletions)
}

func (e *BaiduExecutor) GetName() string {
	if e.channel != nil && e.channel.Name != "" {
		return e.channel.Name
	}
	return "Baidu"
}

func (e *BaiduExecutor) GetRequestURL(info *RequestInfo) (string, error) {
	baseURL := ""
	if info.Protocol != nil {
		baseURL = strings.TrimSuffix(info.Protocol.BaseURL, "/")
	}
	if baseURL == "" {
		baseURL = "https://aip.baidubce.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	switch info.RelayMode {
	case model.RelayModeChatCompletions, model.RelayModeClaudeMessages:
		return baseURL + "/v2/chat/completions", nil
	case model.RelayModeEmbeddings:
		return baseURL + "/v2/embeddings", nil
	case model.RelayModeImagesGenerations:
		return baseURL + "/v2/images/generations", nil
	default:
		return "", fmt.Errorf("baidu: unsupported relay mode: %d", info.RelayMode)
	}
}

func (e *BaiduExecutor) SetupRequestHeader(header http.Header, info *RequestInfo) error {
	// API Key format: "token|appid"
	token, appID := parseBaiduAPIKey(info.ApiKey)

	header.Set("Authorization", "Bearer "+token)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	if appID != "" {
		header.Set("appid", appID)
	}

	return nil
}

func (e *BaiduExecutor) TransformRequest(ctx context.Context, info *RequestInfo, requestBody []byte) (io.Reader, error) {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawMap); err != nil {
		return bytes.NewReader(bodyFromBytes(requestBody)), nil
	}

	// Determine upstream model name.
	modelName := info.Model
	if info.Protocol != nil && info.Protocol.IsModelMapped && info.Protocol.UpstreamModelName != "" {
		modelName = info.Protocol.UpstreamModelName
	}

	// Detect "-search" suffix → enable web search mode.
	if strings.HasSuffix(modelName, "-search") {
		modelName = strings.TrimSuffix(modelName, "-search")
		webSearch := map[string]interface{}{
			"enable":          true,
			"enable_citation": true,
			"enable_trace":    true,
		}
		wsJSON, err := json.Marshal(webSearch)
		if err != nil {
			return nil, fmt.Errorf("marshal web_search failed: %w", err)
		}
		rawMap["web_search"] = json.RawMessage(wsJSON)
	}

	// Set model name.
	rawMap["model"] = json.RawMessage(`"` + modelName + `"`)

	result, err := json.Marshal(rawMap)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	return bytes.NewReader(result), nil
}

func (e *BaiduExecutor) DoRequest(ctx context.Context, info *RequestInfo, requestBody io.Reader) (*http.Response, error) {
	reqURL, err := e.GetRequestURL(info)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if err := e.SetupRequestHeader(httpReq.Header, info); err != nil {
		return nil, fmt.Errorf("setup header: %w", err)
	}

	timeout := 60
	if info.Channel != nil && info.Channel.Settings.TimeoutSeconds > 0 {
		timeout = info.Channel.Settings.TimeoutSeconds
	}

	client := &http.Client{Timeout: secondsAsDuration(timeout)}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	return resp, nil
}

func (e *BaiduExecutor) TransformResponse(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	// Baidu V2 returns OpenAI-compatible format — delegate to OpenAIExecutor.
	oe := &OpenAIExecutor{}
	oe.Init(info.Channel)
	return oe.TransformResponse(ctx, resp, info, writer)
}

// ---- Baidu-specific helpers ----

// parseBaiduAPIKey parses "token|appid" format API key.
// If no "|" separator, entire key is used as token.
func parseBaiduAPIKey(apiKey string) (token, appID string) {
	parts := strings.SplitN(apiKey, "|", 2)
	token = parts[0]
	if len(parts) > 1 {
		appID = parts[1]
	}
	return
}
