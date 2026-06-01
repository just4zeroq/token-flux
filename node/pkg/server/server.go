package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"ai-platform-node/pkg/keychain"
	"ai-platform-node/pkg/router"
	"ai-platform-node/pkg/types"
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

func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	// OpenAI-compatible endpoint (receives requests from local tools or platform)
	mux.HandleFunc("/v1/chat/completions", s.handleChat)
	mux.HandleFunc("/v1/models", s.handleModels)

	// Local admin API (for desktop client / CLI)
	mux.HandleFunc("/api/node/status", s.handleStatus)
	mux.HandleFunc("/api/node/keys", s.handleKeys)
	mux.HandleFunc("/api/node/models", s.handleModelsAdmin)
	mux.HandleFunc("/api/node/models/bind", s.handleBindModel)

	server := &http.Server{Addr: addr, Handler: mux}

	go func() {
		<-s.ctx.Done()
		server.Shutdown(context.Background())
	}()

	log.Printf("[server] listening on %s", addr)
	return server.ListenAndServe()
}

func (s *Server) Stop() { s.cancel() }

// ---- OpenAI-compatible /v1/chat/completions ----

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req types.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	env := &types.RequestEnvelope{Request: req, Model: req.Model}
	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks, errCh := s.router.RouteStream(r.Context(), env)
		for {
			select {
			case chunk, ok := <-chunks:
				if !ok {
					return
				}
				data, _ := json.Marshal(chunk)
				w.Write([]byte("data: " + string(data) + "\n\n"))
				flusher.Flush()
			case err := <-errCh:
				data, _ := json.Marshal(map[string]string{"error": err.Error()})
				w.Write([]byte("data: " + string(data) + "\n\n"))
				return
			case <-r.Context().Done():
				return
			}
		}
	} else {
		resp, err := s.router.Route(r.Context(), env)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	// Return a list of locally-bound models
	bindings, _ := keychain.SharedBindings()
	type modelEntry struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		OwnedBy string `json:"owned_by"`
	}
	var list []modelEntry
	for _, b := range bindings {
		list = append(list, modelEntry{ID: b.ModelCode, Object: "model", OwnedBy: "local"})
	}
	json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": list})
}

// ---- Local Admin API ----

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "running"})
}

func (s *Server) handleKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		keys, _ := keychain.ListKeys()
		json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	case http.MethodPost:
		var in struct {
			Label     string `json:"label"`
			Key       string `json:"key"`
			ChannelID string `json:"channel_id"`
			BaseURL   string `json:"base_url"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		e, err := keychain.AddKey(in.Label, in.Key, in.ChannelID, in.BaseURL)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(e)
	}
}

func (s *Server) handleModelsAdmin(w http.ResponseWriter, r *http.Request) {
	bindings, _ := keychain.SharedBindings()
	json.NewEncoder(w).Encode(map[string]any{"bindings": bindings})
}

func (s *Server) handleBindModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		KeyID     string `json:"key_id"`
		ModelCode string `json:"model_code"`
		ModelName string `json:"model_name"`
		Shared    bool   `json:"shared"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	b, err := keychain.AddBinding(in.KeyID, in.ModelCode, in.ModelName, in.Shared)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// If shared, register with local router
	if in.Shared {
		s.router.Register(in.ModelCode, b.KeyHash)
	}
	json.NewEncoder(w).Encode(b)
}
