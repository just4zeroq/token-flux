package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"ai-platform-node/pkg/channel"
	"ai-platform-node/pkg/db"
	"ai-platform-node/pkg/keychain"
	"ai-platform-node/pkg/modelspec"
	"ai-platform-node/pkg/provider"
	"ai-platform-node/pkg/router"
	"ai-platform-node/pkg/types"
	"ai-platform-node/pkg/usagetracker"

	"ai-platform/pkg/executor"
	"ai-platform/pkg/model"
	"ai-platform/pkg/translator"
)

type Server struct {
	router *router.Router
	ctx    context.Context
	cancel context.CancelFunc
}

func New(r *router.Router) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{router: r, ctx: ctx, cancel: cancel}
}

// Not used — tunnel lifecycle managed by main.go NodeService.
// Kept for future use.

func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", apiKeyAuth(s.handleChat))
	mux.HandleFunc("/v1/messages", apiKeyAuth(s.handleMessages))
	mux.HandleFunc("/v1/completions", apiKeyAuth(s.handleCompletions))
	mux.HandleFunc("/v1/embeddings", apiKeyAuth(s.handleEmbeddings))
		mux.HandleFunc("/v1/responses", apiKeyAuth(s.handleResponses))
	mux.HandleFunc("/v1/models", apiKeyAuth(s.handleModels))

	mux.HandleFunc("/oauth/device/code", s.handleOAuthDeviceCode)
	mux.HandleFunc("/oauth/device/token", s.handleOAuthDeviceToken)
	mux.HandleFunc("/oauth/verify", s.handleOAuthVerify)

	mux.HandleFunc("/api/node/status", adminOnly(s.handleStatus))
	mux.HandleFunc("/api/node/keys", adminOnly(s.handleKeys))
	mux.HandleFunc("/api/node/models", adminOnly(s.handleModelsAdmin))
	mux.HandleFunc("/api/node/models/bind", adminOnly(s.handleBindModel))
	mux.HandleFunc("/api/node/api-keys", adminOnly(s.handleAPIKeys))
	mux.HandleFunc("/api/node/channels", adminOnly(s.handleChannels))
	mux.HandleFunc("/api/node/channel-models", adminOnly(s.handleChannelModels))
	mux.HandleFunc("/api/node/model-specs", adminOnly(s.handleModelSpecs))
	mux.HandleFunc("/api/node/usage", adminOnly(s.handleUsage))
	mux.HandleFunc("/api/node/tunnel", adminOnly(s.handleTunnel))
	mux.HandleFunc("/api/node/wallet", adminOnly(s.handleWallet))
	mux.HandleFunc("/api/node/network", adminOnly(s.handleNetwork))
	mux.HandleFunc("/api/node/config", adminOnly(s.handleConfig))

	server := &http.Server{Addr: addr, Handler: corsHandler(mux)}
	go func() {
		<-s.ctx.Done()
		server.Shutdown(context.Background())
	}()
	log.Printf("[server] listening on %s", addr)
	return server.ListenAndServe()
}

func (s *Server) Stop() { s.cancel() }

// clientFormatFromHeader reads X-Response-Format header, falls back to inboundFormat.
func clientFormatFromHeader(r *http.Request, inboundFormat translator.Format) translator.Format {
	f := r.Header.Get("X-Response-Format")
	if f == "openai" || f == "claude" || f == "openai_responses" || f == "gemini" {
		return translator.Format(f)
	}
	return inboundFormat
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req types.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	info := &executor.RequestInfo{
		RequestID:     r.Header.Get("X-Request-Id"),
		RelayMode:     translator.RelayModeChatCompletions,
		IsStream:      req.Stream,
		Model:         req.Model,
		InboundFormat: translator.FormatOpenAI,
		ClientFormat:  clientFormatFromHeader(r, translator.FormatOpenAI),
	}

	body, _ := json.Marshal(req)

	if req.Stream {
		hash, err := s.router.Pick(req.Model)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
			return
		}
		provider.ExecuteWithWriter(r.Context(), body, hash, info, w)
		return
	}

	hash, err := s.router.Pick(req.Model)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	pp, err := provider.PrepareRequest(r.Context(), hash, info)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	resp, err := pp.DoRaw(r.Context(), body)
	if err == nil && resp.StatusCode == http.StatusOK {
		pp.WriteResponse(r.Context(), resp, w)
		return
	}
	if resp != nil {
		resp.Body.Close()
	}

	hashes, _ := s.router.PickAll(req.Model)
	for _, h := range hashes {
		if h == hash {
			continue
		}
		pp, err := provider.PrepareRequest(r.Context(), h, info)
		if err != nil {
			continue
		}
		resp, err := pp.DoRaw(r.Context(), body)
		if err == nil && resp.StatusCode == http.StatusOK {
			s.router.MarkFailed(req.Model, h)
			pp.WriteResponse(r.Context(), resp, w)
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		s.router.MarkFailed(req.Model, h)
	}
	http.Error(w, `{"error":"all upstream keys failed"}`, http.StatusServiceUnavailable)
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var claudeReq struct {
		Model     string          `json:"model"`
		Stream    bool            `json:"stream"`
		MaxTokens int             `json:"max_tokens"`
		Messages  json.RawMessage `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&claudeReq); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	hash, err := s.router.Pick(claudeReq.Model)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	info := &executor.RequestInfo{
		RequestID:     r.Header.Get("X-Request-Id"),
		RelayMode:     translator.RelayModeClaudeMessages,
		IsStream:      claudeReq.Stream,
		Model:         claudeReq.Model,
		InboundFormat: translator.FormatClaude,
			ClientFormat:  clientFormatFromHeader(r, translator.FormatClaude),
	}

	body, _ := json.Marshal(claudeReq)
	provider.ExecuteWithWriter(r.Context(), body, hash, info, w)
}

func (s *Server) handleCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		Stream bool   `json:"stream"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	hash, err := s.router.Pick(req.Model)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	info := &executor.RequestInfo{
		RequestID:     r.Header.Get("X-Request-Id"),
		RelayMode:     translator.RelayModeChatCompletions,
		IsStream:      req.Stream,
		Model:         req.Model,
		InboundFormat: translator.FormatOpenAI,
			ClientFormat:  clientFormatFromHeader(r, translator.FormatOpenAI),
	}

	body, _ := json.Marshal(req)
	provider.ExecuteWithWriter(r.Context(), body, hash, info, w)
}

func (s *Server) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Model string `json:"model"`
		Input any    `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	hash, err := s.router.Pick(req.Model)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	info := &executor.RequestInfo{
		RequestID:     r.Header.Get("X-Request-Id"),
		RelayMode:     translator.RelayModeChatCompletions,
		IsStream:      false,
		Model:         req.Model,
		InboundFormat: translator.FormatOpenAI,
			ClientFormat:  clientFormatFromHeader(r, translator.FormatOpenAI),
	}

	body, _ := json.Marshal(req)
	provider.ExecuteWithWriter(r.Context(), body, hash, info, w)
}

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Model           string          `json:"model"`
		Input           json.RawMessage `json:"input"`
		Instructions    json.RawMessage `json:"instructions"`
		MaxOutputTokens int             `json:"max_output_tokens,omitempty"`
		Temperature     float64         `json:"temperature,omitempty"`
		TopP            float64         `json:"top_p,omitempty"`
		Stream          bool            `json:"stream,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	hash, err := s.router.Pick(req.Model)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusNotFound)
		return
	}

	info := &executor.RequestInfo{
		RequestID:     r.Header.Get("X-Request-Id"),
		RelayMode:     translator.RelayModeChatCompletions,
		IsStream:      req.Stream,
		Model:         req.Model,
		InboundFormat: translator.FormatOpenAIResponses,
		ClientFormat:  clientFormatFromHeader(r, translator.FormatOpenAIResponses),
	}

	body, _ := json.Marshal(req)
	provider.ExecuteWithWriter(r.Context(), body, hash, info, w)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	bindings, _ := keychain.SharedBindings()
	type modelEntry struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		OwnedBy string `json:"owned_by"`
	}
	var list []modelEntry
	seen := map[string]bool{}
	for _, b := range bindings {
		if seen[b.ModelCode] {
			continue
		}
		seen[b.ModelCode] = true
		list = append(list, modelEntry{ID: b.ModelCode, Object: "model", OwnedBy: "local"})
	}
	json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": list})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	usageSummary, _ := usagetracker.Summary()
	json.NewEncoder(w).Encode(map[string]any{
		"running":      true,
		"models":       len(s.router.Models()),
		"total_calls":  usageSummary.TotalCalls,
		"total_tokens": usageSummary.TotalTokens,
		"tunnel":       false, // tunnel status managed by main.go NodeService
	})
}

func (s *Server) handleKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		keys, _ := keychain.ListKeys()
		json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	case http.MethodPost:
		var in struct {
			Name      string `json:"name"`
			Key       string `json:"key"`
			ChannelID int64  `json:"channel_id"`
			BaseURL   string `json:"base_url"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		e, err := keychain.AddKey(in.Name, in.Key, in.BaseURL, in.ChannelID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(e)
	case http.MethodDelete:
		var in struct{ ID int64 `json:"id"` }
		json.NewDecoder(r.Body).Decode(&in)
		_, _ = db.DB().Exec("DELETE FROM llm_model_keys WHERE id = ?", in.ID)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleModelsAdmin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		bindings, _ := keychain.SharedBindings()
		json.NewEncoder(w).Encode(map[string]any{"bindings": bindings})
	case http.MethodDelete:
		var in struct{ ID int64 `json:"id"` }
		json.NewDecoder(r.Body).Decode(&in)
		_, _ = db.DB().Exec("DELETE FROM llm_model_key_models WHERE id = ?", in.ID)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleBindModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		KeyID             int64  `json:"key_id"`
		ModelCode         string `json:"model_code"`
		UpstreamModelName string `json:"upstream_model_name"`
		Shared            bool   `json:"shared"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	b, err := keychain.AddBinding(in.KeyID, in.ModelCode, in.UpstreamModelName, 0, in.Shared, 0, 1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if in.Shared {
		s.router.Register(in.ModelCode, b.KeyHash)
	}
	json.NewEncoder(w).Encode(b)
}

func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB().Query("SELECT id, key_prefix, key_value, label, status, last_used_at, created_at FROM node_api_keys ORDER BY created_at DESC")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var keys []map[string]any
		for rows.Next() {
			var id int64
			var prefix, keyVal, label, status string
			var lastUsed, createdAt int64
			rows.Scan(&id, &prefix, &keyVal, &label, &status, &lastUsed, &createdAt)
			keys = append(keys, map[string]any{"id": id, "key_prefix": prefix, "key": keyVal, "label": label, "status": status, "last_used_at": lastUsed, "created_at": createdAt})
		}
		if keys == nil {
			keys = []map[string]any{}
		}
		json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	case http.MethodPost:
		var in struct {
			Label string `json:"label"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		key, err := keychain.GenerateAPIKey(in.Label)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(key)
	case http.MethodDelete:
		var in struct{ ID int64 `json:"id"` }
		json.NewDecoder(r.Body).Decode(&in)
		_, _ = db.DB().Exec("DELETE FROM node_api_keys WHERE id = ?", in.ID)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleChannels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := channel.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"channels": list})
	case http.MethodPost:
		var in struct {
			Code         string                `json:"code"`
			Name         string                `json:"name"`
			Description  string                `json:"description"`
			ProviderType model.ProviderType     `json:"provider_type"`
			Protocols    []model.ProtocolEntry  `json:"protocols"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		c, err := channel.Create(in.Code, in.Name, in.Description, in.ProviderType, in.Protocols)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(c)
	case http.MethodPut:
		var in struct {
			ID        int64                 `json:"id"`
			Name      string                `json:"name"`
			Protocols []model.ProtocolEntry `json:"protocols"`
			Status    string                `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		c, err := channel.Update(in.ID, in.Name, in.Protocols, in.Status)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(c)
	case http.MethodDelete:
		var in struct{ ID int64 `json:"id"` }
		json.NewDecoder(r.Body).Decode(&in)
		if err := channel.Delete(in.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleChannelModels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		channelID := int64(0)
		if idStr := r.URL.Query().Get("channel_id"); idStr != "" {
			fmt.Sscanf(idStr, "%d", &channelID)
		}
		list, err := channel.ListChannelModels(channelID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"models": list})
	case http.MethodPost:
		var in struct {
			ChannelID int64   `json:"channel_id"`
			ModelIDs  []int64 `json:"model_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := channel.BindChannelModels(in.ChannelID, in.ModelIDs); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		var in struct{ ID int64 `json:"id"` }
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := channel.UnbindChannelModel(in.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleModelSpecs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := modelspec.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"model_specs": list})
	case http.MethodPost:
		var in struct {
			DeveloperName   string   `json:"developer_name"`
			ModelName       string   `json:"model_name"`
			ModelCode       string   `json:"model_code"`
			DisplayName     string   `json:"display_name"`
			ModelFamily     string   `json:"model_family"`
			Description       string `json:"description"`
				CapabilitiesJSON  string `json:"capabilities_json"`
			ContextWindow   int      `json:"context_window"`
			MaxInputTokens  int      `json:"max_input_tokens"`
			MaxOutputTokens int      `json:"max_output_tokens"`
			SupportsStream  bool     `json:"supports_stream"`
			SupportsTools   bool     `json:"supports_tools"`
			SupportsVision  bool     `json:"supports_vision"`
				SupportsJsonMode bool     `json:"supports_json_mode"`
				SupportsReasoning bool    `json:"supports_reasoning"`
				SupportsLogprobs bool     `json:"supports_logprobs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		spec, err := modelspec.Create(in.DeveloperName, in.ModelName, in.ModelCode,
			in.DisplayName, in.ModelFamily, in.Description, in.CapabilitiesJSON, in.ContextWindow,
			in.MaxInputTokens, in.MaxOutputTokens, in.SupportsStream, in.SupportsTools, in.SupportsVision, in.SupportsJsonMode, in.SupportsReasoning, in.SupportsLogprobs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(spec)
	case http.MethodDelete:
		var in struct{ ID int64 `json:"id"` }
		json.NewDecoder(r.Body).Decode(&in)
		if err := modelspec.Delete(in.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		source := r.URL.Query().Get("source")
		if source == "tunnel" {
			// Return tunnel-specific usage summary.
			var totalReqs, totalTokens int64
			db.DB().QueryRow(
				"SELECT COALESCE(COUNT(*),0), COALESCE(SUM(total_tokens),0) FROM llm_usage_records WHERE source = 'tunnel'",
			).Scan(&totalReqs, &totalTokens)
			rows, _ := db.DB().Query(
				`SELECT COALESCE(ms.model_name,''), COALESCE(ms.model_code,''), COUNT(*), SUM(r.total_tokens)
				 FROM llm_usage_records r
				 LEFT JOIN llm_model_key_models mk ON r.model_key_id = mk.model_key_id
				 LEFT JOIN llm_model_specs ms ON mk.model_spec_id = ms.id
				 WHERE r.source = 'tunnel'
				 GROUP BY ms.model_name ORDER BY COUNT(*) DESC LIMIT 20`)
			var breakdown []map[string]any
			if rows != nil {
				defer rows.Close()
				for rows.Next() {
					var name, code string
					var reqs, toks int64
					if rows.Scan(&name, &code, &reqs, &toks) == nil {
						breakdown = append(breakdown, map[string]any{"model_name": name, "model_code": code, "requests": reqs, "total_tokens": toks})
					}
				}
			}
			if breakdown == nil {
				breakdown = []map[string]any{}
			}
			json.NewEncoder(w).Encode(map[string]any{"total_requests": totalReqs, "total_tokens": totalTokens, "model_breakdown": breakdown})
			return
		}
		summary, _ := usagetracker.Summary()
		byModel, _ := usagetracker.BreakdownByModel()
		recent, _ := usagetracker.Recent(20)
		json.NewEncoder(w).Encode(map[string]any{"summary": summary, "by_model": byModel, "recent": recent})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTunnel(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{
		"note":     "tunnel management moved to NodeService API",
		"endpoint": "/api/node/network",
	})
}

func (s *Server) handleWallet(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{
		"note": "use Wails runtime binding for wallet operations",
	})
}

func (s *Server) handleNetwork(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{
		"note": "use Wails runtime binding for network operations",
	})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	case http.MethodPost:
		var cfg map[string]any
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}
		// Dev-only fallback: write to config.yaml in CWD
		data, _ := json.MarshalIndent(cfg, "", "  ")
		os.WriteFile("config.yaml", data, 0644)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

