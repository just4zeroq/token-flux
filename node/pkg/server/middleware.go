package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"ai-platform-node/pkg/db"
)

// apiKeyAuth middleware validates sk-xxx API key from Authorization header.
// On success, injects key info into context (via header for simplicity).
// On failure, returns 401.
func apiKeyAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			writeAuthError(w, "missing or invalid authorization header")
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if len(token) < 8 {
			writeAuthError(w, "invalid api key format")
			return
		}

		// Hash the token to look up in node_api_keys.
		h := sha256.Sum256([]byte(token))
		hash := hex.EncodeToString(h[:])

		var id int64
		var status string
		err := db.DB().QueryRow(
			"SELECT id, status FROM node_api_keys WHERE key_hash = ?", hash,
		).Scan(&id, &status)
		if err != nil {
			writeAuthError(w, "invalid api key")
			return
		}
		if status != "active" {
			writeAuthError(w, "api key is disabled")
			return
		}

		// Update last_used_at.
		_, _ = db.DB().Exec("UPDATE node_api_keys SET last_used_at = ? WHERE id = ?", time.Now().Unix(), id)

		// Pass key info to handler via headers (simpler than context wrapping).
		r.Header.Set("X-Node-ApiKey-Id", strings.Split(token, "-")[0])

		next(w, r)
	}
}

// adminOnly middleware restricts to localhost for admin API.
func adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Allow localhost only
		if !isLocalRequest(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// corsHandler wraps an http.Handler with CORS headers.
// Needed because the Wails frontend runs on wails.localhost:* while the API runs on localhost:20128.
func corsHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isLocalRequest(r *http.Request) bool {
	// Check RemoteAddr first (production mode).
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host == "127.0.0.1" || host == "::1" || host == "localhost" {
		return true
	}

	// Fallback: check Origin/Referer for Wails dev mode.
	origin := r.Header.Get("Origin")
	if strings.Contains(origin, "wails.localhost") || strings.Contains(origin, "localhost") {
		return true
	}
	referer := r.Header.Get("Referer")
	if strings.Contains(referer, "wails.localhost") || strings.Contains(referer, "localhost") {
		return true
	}

	return false
}

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
