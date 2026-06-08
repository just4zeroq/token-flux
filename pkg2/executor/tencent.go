package executor

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-platform/pkg/model"
	"ai-platform/pkg/translator"
)

const (
	tencentDefaultHost    = "hunyuan.tencentcloudapi.com"
	tencentDefaultBaseURL = "https://hunyuan.tencentcloudapi.com/"
	tencentAPIVersion     = "2023-09-01"
	tencentServiceName    = "hunyuan"
)

func init() {
	Register(model.ProviderTencent, &TencentExecutor{})
	// Tencent has no ProtocolTencent — it's OpenAI-compatible via protocol resolution.
}

// TencentExecutor handles Tencent Hunyuan API.
// Uses TC3-HMAC-SHA256 signing for authentication.
// API Key format: "secretId|secretKey" (pipe-delimited).
type TencentExecutor struct {
	channel *model.Channel
}

func (e *TencentExecutor) Init(channel *model.Channel) {
	e.channel = channel
}

func (e *TencentExecutor) NativeFormats() []EndpointCapability {
	return NativeFormat("openai", model.RelayModeChatCompletions)
}

func (e *TencentExecutor) GetName() string {
	if e.channel != nil && e.channel.Name != "" {
		return e.channel.Name
	}
	return "Tencent"
}

func (e *TencentExecutor) GetRequestURL(info *RequestInfo) (string, error) {
	if info.Protocol != nil && info.Protocol.BaseURL != "" {
		return strings.TrimSuffix(info.Protocol.BaseURL, "/") + "/", nil
	}
	return tencentDefaultBaseURL, nil
}

func (e *TencentExecutor) SetupRequestHeader(header http.Header, info *RequestInfo) error {
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")

	// Determine Host.
	host := tencentDefaultHost
	if info.Protocol != nil && info.Protocol.BaseURL != "" {
		trimmed := strings.TrimPrefix(info.Protocol.BaseURL, "https://")
		trimmed = strings.TrimPrefix(trimmed, "http://")
		trimmed = strings.TrimSuffix(trimmed, "/")
		if trimmed != "" {
			host = trimmed
		}
	}
	header.Set("Host", host)

	// Determine Action.
	action := "ChatCompletions"
	switch info.RelayMode {
	case model.RelayModeEmbeddings:
		action = "GetEmbedding"
	}
	header.Set("X-TC-Action", action)
	header.Set("X-TC-Version", tencentAPIVersion)

	timestamp := time.Now().Unix()
	header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))

	// Signature is calculated in DoRequest (needs full body).
	return nil
}

func (e *TencentExecutor) TransformRequest(ctx context.Context, info *RequestInfo, requestBody []byte) (io.Reader, error) {
	body := bodyFromBytes(requestBody)

	// Non-OpenAI format → normalize to OpenAI first.
	if !shouldPassthrough(info, "openai") {
		var tf translator.Format
		switch info.InboundFormat {
		case "gemini":
			tf = translator.FormatGemini
		case "claude":
			tf = translator.FormatClaude
		case "openai_responses":
			tf = translator.FormatOpenAIResponses
		default:
			tf = translator.FormatOpenAI
		}
		converted, err := translator.Normalize(body, tf)
		if err != nil {
			// keep original on error
		} else {
			body = converted
		}
	}

	// Model mapping.
	if info.Protocol != nil && info.Protocol.IsModelMapped && info.Protocol.UpstreamModelName != "" {
		var rawMap map[string]json.RawMessage
		if err := json.Unmarshal(body, &rawMap); err != nil {
			return bytes.NewReader(body), nil
		}
		rawMap["model"] = json.RawMessage(`"` + info.Protocol.UpstreamModelName + `"`)
		mapped, err := json.Marshal(rawMap)
		if err != nil {
			return nil, fmt.Errorf("marshal request failed: %w", err)
		}
		return bytes.NewReader(mapped), nil
	}

	return bytes.NewReader(body), nil
}

func (e *TencentExecutor) DoRequest(ctx context.Context, info *RequestInfo, requestBody io.Reader) (*http.Response, error) {
	reqURL, err := e.GetRequestURL(info)
	if err != nil {
		return nil, err
	}

	// Read the full body for TC3 signing.
	var bodyBytes []byte
	if requestBody != nil {
		bodyBytes, err = io.ReadAll(requestBody)
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if err := e.SetupRequestHeader(httpReq.Header, info); err != nil {
		return nil, fmt.Errorf("setup header: %w", err)
	}

	// Parse API key: "secretId|secretKey"
	secretID, secretKey := parseTencentAPIKey(info.ApiKey)

	// Calculate TC3-HMAC-SHA256 signature.
	if secretKey != "" {
		host := httpReq.Header.Get("Host")
		contentType := httpReq.Header.Get("Content-Type")
		timestamp := time.Now().Unix()

		httpReq.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))

		authorization := signTC3(secretID, secretKey, tencentServiceName, host, contentType, bodyBytes, timestamp)
		httpReq.Header.Set("Authorization", authorization)
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

func (e *TencentExecutor) TransformResponse(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
	// Tencent Hunyuan returns OpenAI-compatible format — delegate to OpenAIExecutor.
	oe := &OpenAIExecutor{}
	oe.Init(info.Channel)
	return oe.TransformResponse(ctx, resp, info, writer)
}

// ---- Tencent-specific helpers ----

// parseTencentAPIKey parses "secretId|secretKey" format API key.
// Falls back to using entire key as secretID if no separator.
func parseTencentAPIKey(apiKey string) (secretID, secretKey string) {
	parts := strings.SplitN(apiKey, "|", 2)
	secretID = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		secretKey = strings.TrimSpace(parts[1])
	}
	return
}

// signTC3 computes TC3-HMAC-SHA256 signature and returns the full Authorization header value.
//
// Signature flow:
//  1. Build canonical request
//  2. Build string to sign
//  3. Derive signing key (SecretDate → SecretService → SecretSigning)
//  4. Compute signature and assemble Authorization header
func signTC3(secretID, secretKey, service, host, contentType string, payload []byte, timestamp int64) string {
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")

	// Step 1: Canonical Request.
	httpRequestMethod := "POST"
	canonicalURI := "/"
	canonicalQueryString := ""
	signedHeaders := "content-type;host"
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\n", contentType, host)
	hashedPayload := sha256Hex(payload)

	canonicalRequest := strings.Join([]string{
		httpRequestMethod,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedPayload,
	}, "\n")

	// Step 2: String to Sign.
	algorithm := "TC3-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)

	stringToSign := strings.Join([]string{
		algorithm,
		strconv.FormatInt(timestamp, 10),
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	// Step 3: Derive Signing Key.
	secretDate := hmacSHA256([]byte("TC3"+secretKey), []byte(date))
	secretService := hmacSHA256(secretDate, []byte(service))
	secretSigning := hmacSHA256(secretService, []byte("tc3_request"))

	// Step 4: Compute Signature.
	signature := hex.EncodeToString(hmacSHA256(secretSigning, []byte(stringToSign)))

	return fmt.Sprintf(
		"%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, secretID, credentialScope, signedHeaders, signature,
	)
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
