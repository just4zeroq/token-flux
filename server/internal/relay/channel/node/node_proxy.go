// Package node provides a relay Adaptor that forwards requests to connected nodes
// via WebSocket data channel.
package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	nodeapi "ai-platform/internal/controller/api/node"
	"ai-platform/internal/relay/common"

	"github.com/gogf/gf/v2/frame/g"
)

// Adaptor implements common.Adaptor for node-routed requests.
type Adaptor struct {
	info *common.RelayInfo
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	return "", nil // not used — WS transport
}

func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	return nil // not used — WS transport
}

// ConvertRequest passes through the raw body unchanged.
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	return bytes.NewReader(requestBody), nil
}

// DoRequest forwards the request to the connected node via WebSocket and returns
// the node's response wrapped as an *http.Response.
func (a *Adaptor) DoRequest(ctx context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	body, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// Extract wallet_addr and key_hash from ChannelMeta.
	walletAddr := info.ChannelMeta.ChannelName
	keyHash := info.ChannelMeta.UpstreamModelName
	if walletAddr == "" {
		return nil, fmt.Errorf("node adaptor: empty wallet address in ChannelName")
	}

	// Get user_id from the node_models entry.
	var userID int64
	g.DB().Model("node_models").Ctx(ctx).
		Where("wallet_address", walletAddr).
		Where("model_code", info.OriginModelName).
		Fields("user_id").Scan(&userID)

	// Build RequestEnvelope.
	var chatReq map[string]any
	json.Unmarshal(body, &chatReq)
	env := g.Map{
		"request_id": info.RequestID,
		"model":      info.OriginModelName,
		"key_hash":   keyHash,
		"request":    chatReq,
	}
	envJSON, _ := json.Marshal(env)

	// Send via WebSocket and wait for response.
	resp, err := nodeapi.SendRequestToNode(walletAddr, info.RequestID, envJSON)
	if err != nil {
		return nil, fmt.Errorf("node send: %w", err)
	}

	switch resp.Type {
	case "done":
		// Successful response — data contains the ChatResponse JSON.
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(resp.Data)),
			Header:     make(http.Header),
		}, nil
	case "error":
		return nil, fmt.Errorf("node error: %s", resp.Error)
	default:
		return nil, fmt.Errorf("unexpected node response type: %s", resp.Type)
	}
}

// DoResponse writes the upstream response body directly to the client.
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, w http.ResponseWriter) (*common.Usage, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read node response: %w", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)

	// Parse usage from the response for settlement.
	var chatResp struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &chatResp); err == nil && chatResp.Usage.TotalTokens > 0 {
		return &common.Usage{
			TotalTokens: chatResp.Usage.TotalTokens,
		}, nil
	}

	return nil, nil
}

func (a *Adaptor) GetChannelName() string {
	return "node"
}
