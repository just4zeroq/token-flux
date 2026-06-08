package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"ai-platform-node/pkg/db"
)

// OAuth device flow state.
type deviceRequest struct {
	DeviceCode string
	UserCode   string
	Approved   bool
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

var (
	deviceMu     sync.RWMutex
	deviceByCode = map[string]*deviceRequest{}  // device_code → request
	deviceByUser = map[string]*deviceRequest{}  // user_code → request
)

const deviceCodeTTL = 10 * time.Minute

// handleOAuthDeviceCode handles POST /oauth/device/code.
// Returns device_code, user_code, verification_uri, interval for polling.
func (s *Server) handleOAuthDeviceCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceCode, _ := randomHex(32)
	userCode, _ := generateUserCode()

	req := &deviceRequest{
		DeviceCode: deviceCode,
		UserCode:   userCode,
		Approved:   false,
		ExpiresAt:  time.Now().Add(deviceCodeTTL),
		CreatedAt:  time.Now(),
	}

	deviceMu.Lock()
	deviceByCode[deviceCode] = req
	deviceByUser[userCode] = req
	deviceMu.Unlock()

	// Periodic cleanup of expired codes. Runs in background once.
	go cleanupExpiredDevices()

	json.NewEncoder(w).Encode(map[string]any{
		"device_code":      deviceCode,
		"user_code":        userCode,
		"verification_uri": "http://localhost:20128/oauth/verify",
		"interval":         5,
		"expires_in":       600,
	})
}

// handleOAuthDeviceToken handles POST /oauth/device/token.
// Exchanges device_code for an access token after user approves.
func (s *Server) handleOAuthDeviceToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceCode := r.FormValue("device_code")
	if deviceCode == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_request", "error_description": "device_code required"})
		return
	}

	deviceMu.RLock()
	req, exists := deviceByCode[deviceCode]
	deviceMu.RUnlock()

	if !exists || time.Now().After(req.ExpiresAt) {
		json.NewEncoder(w).Encode(map[string]string{"error": "expired_token", "error_description": "device code expired"})
		return
	}

	if !req.Approved {
		json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending", "error_description": "user hasn't approved yet"})
		return
	}

	// Generate a gateway API key as the access token.
	b := make([]byte, 32)
	rand.Read(b)
	secret := hex.EncodeToString(b)
	accessToken := "sk-" + secret[:48]
	prefix := accessToken[:12] + "..."

	h := sha256.Sum256([]byte(accessToken))
	hash := hex.EncodeToString(h[:])
	now := time.Now().Unix()
	label := "oauth:" + req.UserCode

	_, err := db.DB().Exec(
		"INSERT INTO node_api_keys(key_hash, key_prefix, key_value, label, status, last_used_at, created_at) VALUES (?, ?, ?, ?, 'active', 0, ?)",
		hash, prefix, accessToken, label, now)
	if err != nil {
		http.Error(w, `{"error":"server_error"}`, http.StatusInternalServerError)
		return
	}

	// Clean up used device code.
	deviceMu.Lock()
	delete(deviceByCode, deviceCode)
	delete(deviceByUser, req.UserCode)
	deviceMu.Unlock()

	json.NewEncoder(w).Encode(map[string]any{
		"access_token": accessToken,
		"token_type":   "bearer",
		"expires_in":   86400 * 30, // 30 days
	})
}

// handleOAuthVerify serves the user verification page.
// GET /oauth/verify?code=ABCD-1234
func (s *Server) handleOAuthVerify(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	action := r.URL.Query().Get("action")

	if action == "approve" && code != "" {
		deviceMu.Lock()
		req, exists := deviceByUser[code]
		if exists && !req.Approved && time.Now().Before(req.ExpiresAt) {
			req.Approved = true
		}
		deviceMu.Unlock()
		writeApprovedPage(w)
		return
	}

	if code != "" {
		writeVerifyPage(w, code)
		return
	}

	// List active pending codes (for OAuth tool testing).
	deviceMu.RLock()
	var pending []string
	for _, req := range deviceByUser {
		if !req.Approved && time.Now().Before(req.ExpiresAt) {
			pending = append(pending, req.UserCode)
		}
	}
	deviceMu.RUnlock()

	if len(pending) > 0 {
		writeCodeListPage(w, pending)
		return
	}
	writeNoPendingPage(w)
}

// ---- helpers ----

func randomHex(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generateUserCode() (string, error) {
	part1, _ := rand.Int(rand.Reader, big.NewInt(10000))
	part2, _ := rand.Int(rand.Reader, big.NewInt(10000))
	return fmt.Sprintf("%04d-%04d", part1, part2), nil
}

func cleanupExpiredDevices() {
	deviceMu.Lock()
	defer deviceMu.Unlock()
	now := time.Now()
	for k, v := range deviceByCode {
		if now.After(v.ExpiresAt) {
			delete(deviceByCode, k)
			delete(deviceByUser, v.UserCode)
		}
	}
}

// ---- HTML pages ----

func writeVerifyPage(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Authorize Device</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
body{font-family:-apple-system,sans-serif;display:flex;justify-content:center;align-items:center;min-height:100vh;margin:0;background:#f8f9fb;color:#1e293b}
.card{background:#fff;border-radius:12px;padding:40px;box-shadow:0 1px 3px rgba(0,0,0,.06);text-align:center;max-width:400px}
.code{font-size:32px;font-weight:700;letter-spacing:4px;color:#4f46e5;margin:20px 0;font-family:monospace}
.btn{background:#4f46e5;color:#fff;border:none;padding:12px 32px;border-radius:8px;font-size:16px;cursor:pointer;text-decoration:none;display:inline-block}
.btn:hover{background:#4338ca}
.hint{color:#64748b;font-size:14px;margin:16px 0}
</style></head><body>
<div class="card">
<div style="font-size:48px;margin-bottom:12px">🔐</div>
<h1>Authorize Device</h1>
<p class="hint">A device requested access. Enter the code below to approve.</p>
<div class="code">%s</div>
<a class="btn" href="/oauth/verify?code=%s&amp;action=approve">Approve</a>
</div></body></html>`, html.EscapeString(code), html.EscapeString(code))
}

func writeApprovedPage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Authorized</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
body{font-family:-apple-system,sans-serif;display:flex;justify-content:center;align-items:center;min-height:100vh;margin:0;background:#f8f9fb;color:#1e293b}
.card{background:#fff;border-radius:12px;padding:40px;box-shadow:0 1px 3px rgba(0,0,0,.06);text-align:center;max-width:400px}
.check{font-size:48px;margin-bottom:12px}
</style></head><body>
<div class="card">
<div class="check">✅</div>
<h1>Authorized!</h1>
<p style="color:#64748b">You can close this window and return to the application.</p>
</div></body></html>`)
}

func writeCodeListPage(w http.ResponseWriter, codes []string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	items := ""
	for _, c := range codes {
		items += fmt.Sprintf(`<a href="/oauth/verify?code=%s" style="display:block;padding:12px 16px;border:1px solid #e9edf2;border-radius:8px;margin:8px 0;text-decoration:none;color:inherit;font-family:monospace;font-size:18px;font-weight:600">%s</a>`, c, c)
	}
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Pending Authorizations</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
body{font-family:-apple-system,sans-serif;max-width:480px;margin:auto;padding:24px;background:#f8f9fb;color:#1e293b}
h1{font-size:20px}
</style></head><body>
<h1>Pending Authorizations</h1>
<p style="color:#64748b">Click a code to approve the device.</p>
%s
</body></html>`, items)
}

func writeNoPendingPage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>No Pending</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
body{font-family:-apple-system,sans-serif;display:flex;justify-content:center;align-items:center;min-height:100vh;margin:0;background:#f8f9fb;color:#1e293b}
.card{background:#fff;border-radius:12px;padding:40px;box-shadow:0 1px 3px rgba(0,0,0,.06);text-align:center;max-width:400px}
</style></head><body>
<div class="card">
<h1>No Pending Requests</h1>
<p style="color:#64748b">There are no devices waiting for authorization.</p>
</div></body></html>`)
}

// apiKeyFromRequest extracts the Bearer token from Authorization header.
// Returns empty string if not present.
func apiKeyFromRequest(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
