// Package node handles node registration and WebSocket tunnel.
package node

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// RegisterRequest is the JSON body for POST /api/v1/nodes/register.
type RegisterRequest struct {
	WalletAddress string `json:"wallet_address"`
	Signature     string `json:"signature"`
	Name          string `json:"name"`
}

// Register handles node registration.
// POST /api/v1/nodes/register
func Register(r *ghttp.Request) {
	var req RegisterRequest
	if err := r.Parse(&req); err != nil {
		r.Response.WriteJson(g.Map{"code": -1, "message": "bad request: " + err.Error()})
		return
	}
	if req.WalletAddress == "" || req.Signature == "" {
		r.Response.WriteJson(g.Map{"code": -1, "message": "wallet_address and signature required"})
		return
	}

	// Verify signature.
	pubBytes, err := hex.DecodeString(req.WalletAddress)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		r.Response.WriteJson(g.Map{"code": -1, "message": "invalid wallet_address"})
		return
	}
	msg := []byte("flux-node-register-" + req.WalletAddress)
	sigBytes, err := hex.DecodeString(req.Signature)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		r.Response.WriteJson(g.Map{"code": -1, "message": "invalid signature"})
		return
	}
	if !ed25519.Verify(pubBytes, msg, sigBytes) {
		r.Response.WriteJson(g.Map{"code": -1, "message": "signature verification failed"})
		return
	}

	ctx := r.Context()
	walletAddr := req.WalletAddress

	// Auto-create provider user if not exists.
	userID := ensureProviderUser(ctx, walletAddr, req.Name)

	// Upsert node with user_id.
	_, err = g.DB().Model("nodes").Ctx(ctx).Data(g.Map{
		"wallet_address": walletAddr,
		"user_id":        userID,
		"name":           req.Name,
		"signature":      req.Signature,
		"last_seen_at":   time.Now(),
		"status":         "active",
	}).Insert()
	if err != nil {
		_, err = g.DB().Model("nodes").Ctx(ctx).Where("wallet_address", walletAddr).
			Data(g.Map{"name": req.Name, "signature": req.Signature, "user_id": userID, "last_seen_at": time.Now(), "status": "active"}).
			Update()
		if err != nil {
			r.Response.WriteJson(g.Map{"code": -1, "message": "db error: " + err.Error()})
			return
		}
	}

	log.Printf("[node] registered wallet=%s name=%s user_id=%d", walletAddr[:16], req.Name, userID)
	r.Response.WriteJson(g.Map{"code": 0, "message": "ok", "data": g.Map{
		"wallet_address": walletAddr,
		"user_id":        userID,
	}})
}

// ensureProviderUser finds or creates a provider user for the given wallet address.
func ensureProviderUser(ctx context.Context, walletAddr, name string) int64 {
	var existingUserID int64
	err := g.DB().Model("nodes").Ctx(ctx).Where("wallet_address", walletAddr).
		Fields("user_id").Scan(&existingUserID)
	if err == nil && existingUserID > 0 {
		c, _ := g.DB().Model("users").Ctx(ctx).Where("id", existingUserID).Count()
		if c > 0 {
			return existingUserID
		}
	}

	nodeName := name
	if nodeName == "" {
		nodeName = "node-" + walletAddr[:16]
	}
	username := fmt.Sprintf("wallet_%s", walletAddr[:16])
	pwHash := fmt.Sprintf("%x", sha256.Sum256([]byte(walletAddr+time.Now().String())))[:32]

	result, err := g.DB().Model("users").Ctx(ctx).Data(g.Map{
		"username":     username,
		"password":     "$2a$10$" + pwHash,
		"email":        "",
		"display_name": nodeName,
		"source":       "wallet",
		"role":         1, // provider
		"status":       1,
		"group_name":   "default",
		"remark":       walletAddr,
	}).Insert()
	if err != nil {
		// Duplicate username — find existing user by username.
		v, _ := g.DB().GetValue(ctx, "SELECT id FROM users WHERE username = ? LIMIT 1", username)
		if !v.IsEmpty() {
			existingID := v.Int64()
			log.Printf("[node] reused existing provider user id=%d wallet=%s", existingID, walletAddr[:16])
			return existingID
		}
		log.Printf("[node] create provider user failed: %v", err)
		return 0
	}
	id, _ := result.LastInsertId()
	log.Printf("[node] created provider user id=%d wallet=%s", id, walletAddr[:16])
	return id
}

// Count returns the number of active nodes.
// GET /api/v1/nodes/count
func Count(r *ghttp.Request) {
	ctx := r.Context()
	count, err := g.DB().Model("nodes").Ctx(ctx).WhereIn("status", g.Slice{"active", "online"}).Count()
	if err != nil {
		count = 0
	}
	if count < 0 {
		count = 0
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "ok", "data": g.Map{"count": count}})
}

// ---- WebSocket ----

// wsConn wraps *ghttp.WebSocket.
type wsConn struct {
	*ghttp.WebSocket
}

// connectedNode holds an active WebSocket connection.
type connectedNode struct {
	WalletAddr  string
	UserID      int64
	Token       string      // session token for data WS auth
	Signalin    *wsConn     // signaling connection
	DataConns   []*wsConn   // data connection pool
	ConnectedAt time.Time
}

// WsResponse is the internal type for pending response dispatch.
type WsResponse struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Error     string          `json:"error,omitempty"`
}

var (
	nodesMu   sync.RWMutex
	connected = make(map[string]*connectedNode) // wallet_addr → node

	sessionMu    sync.RWMutex
	sessionTokens = make(map[string]string) // token → wallet_addr

	pendMu    sync.Mutex
	pendingResps = make(map[string]chan *WsResponse) // request_id → response channel
)

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ---- Signaling WebSocket ----

// WS handles the signaling WebSocket for nodes.
// GET /api/v1/nodes/ws
func WS(r *ghttp.Request) {
	raw, err := r.WebSocket()
	if err != nil {
		r.Response.WriteJson(g.Map{"code": -1, "message": "websocket upgrade failed"})
		return
	}
	ws := &wsConn{raw}
	defer ws.Close()

	// Step 1: auth — receive wallet address.
	var authMsg struct {
		Type   string `json:"type"`
		NodeID string `json:"node_id"`
	}
	if err := ws.ReadJSON(&authMsg); err != nil || authMsg.Type != "auth" || authMsg.NodeID == "" {
		ws.WriteJSON(g.Map{"type": "error", "error": "expected auth"})
		return
	}
	walletAddr := authMsg.NodeID

	// Step 2: send challenge nonce.
	nonce := make([]byte, 32)
	rand.Read(nonce)
	nonceHex := hex.EncodeToString(nonce)
	if err := ws.WriteJSON(g.Map{"type": "challenge", "nonce": nonceHex}); err != nil {
		return
	}

	// Step 3: receive signed nonce.
	var authResp struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := ws.ReadJSON(&authResp); err != nil || authResp.Type != "auth_response" {
		ws.WriteJSON(g.Map{"type": "error", "error": "expected auth_response"})
		return
	}
	var sigHex string
	json.Unmarshal(authResp.Data, &sigHex)
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		ws.WriteJSON(g.Map{"type": "error", "error": "bad signature"})
		return
	}
	pubBytes, err := hex.DecodeString(walletAddr)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		ws.WriteJSON(g.Map{"type": "error", "error": "bad wallet address"})
		return
	}
	if !ed25519.Verify(pubBytes, nonce, sigBytes) {
		ws.WriteJSON(g.Map{"type": "error", "error": "signature mismatch"})
		return
	}

	// Step 4: check binary attestation (if enabled).
	if IsAttestEnabled() {
		attestedMu.RLock()
		attested := attestedWallets[walletAddr]
		attestedMu.RUnlock()
		if !attested {
			ws.WriteJSON(g.Map{"type": "error", "error": "binary attestation required — complete /api/v1/nodes/attest/verify first"})
			log.Printf("[node] WS rejected: wallet=%s not attested", walletAddr[:16])
			return
		}
	}

	// Step 5: check/find user_id — auto-register if wallet not yet in nodes table.
	ctx := r.Context()
	userID := lookupUserIDByWallet(walletAddr)
	if userID <= 0 {
		// Auto-register: create provider user + nodes row.
		userID = ensureProviderUser(ctx, walletAddr, "")
		if userID <= 0 {
			ws.WriteJSON(g.Map{"type": "error", "error": "failed to create provider user"})
			log.Printf("[node] WS rejected: wallet=%s cannot create user", walletAddr[:16])
			return
		}
		_, err = g.DB().Exec(ctx,
			`INSERT INTO nodes (wallet_address, user_id, status, last_seen_at)
			 VALUES ($1, $2, 'online', $3)
			 ON CONFLICT (wallet_address) DO UPDATE SET status = 'online', last_seen_at = $3, user_id = $2`,
			walletAddr, userID, time.Now())
		if err != nil {
			ws.WriteJSON(g.Map{"type": "error", "error": "failed to register node"})
			log.Printf("[node] WS rejected: wallet=%s insert node failed: %v", walletAddr[:16], err)
			return
		}
		log.Printf("[node] auto-registered wallet=%s user_id=%d", walletAddr[:16], userID)
	}

	// Step 6: authenticated — generate session token.
	token := generateToken()

	sessionMu.Lock()
	sessionTokens[token] = walletAddr
	sessionMu.Unlock()

	ws.WriteJSON(g.Map{"type": "auth_ok", "session_token": token, "user_id": userID})
	log.Printf("[node] signaling auth wallet=%s user_id=%d", walletAddr[:16], userID)

	// Set up heartbeat detection: 40s ReadDeadline, node pings every 30s.
	ws.SetReadDeadline(time.Now().Add(40 * time.Second))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(40 * time.Second))
		return nil
	})

	// Register/update in connected map.
	cn := &connectedNode{
		WalletAddr:  walletAddr,
		UserID:      userID,
		Token:       token,
		Signalin:    ws,
		DataConns:   []*wsConn{},
		ConnectedAt: time.Now(),
	}
	nodesMu.Lock()
	if existing, ok := connected[walletAddr]; ok {
		cn.DataConns = existing.DataConns // preserve existing data connections
	}
	connected[walletAddr] = cn
	nodesMu.Unlock()

	// Mark node online.
	g.DB().Model("nodes").Where("wallet_address", walletAddr).Data(g.Map{"status": "online", "last_seen_at": time.Now()}).Update()
	broadcastNodeCount()

	defer func() {
		nodesMu.Lock()
		if cn, ok := connected[walletAddr]; ok {
			for _, dc := range cn.DataConns {
				dc.Close()
			}
		}
		delete(connected, walletAddr)
		nodesMu.Unlock()
		sessionMu.Lock()
		delete(sessionTokens, token)
		sessionMu.Unlock()
		attestedMu.Lock()
		delete(attestedWallets, walletAddr)
		attestedMu.Unlock()
		// Mark node + models offline.
		g.DB().Model("nodes").Where("wallet_address", walletAddr).Data(g.Map{"status": "offline"}).Update()
		g.DB().Model("node_models").Where("wallet_address", walletAddr).Data(g.Map{"status": "offline"}).Update()
		log.Printf("[node] signaling disconnected wallet=%s", walletAddr[:16])
	}()

	// Read loop for signaling messages.
	for {
		var msg struct {
			Type      string          `json:"type"`
			RequestID string          `json:"request_id,omitempty"`
			Data      json.RawMessage `json:"data,omitempty"`
			Error     string          `json:"error,omitempty"`
		}
		if err := ws.ReadJSON(&msg); err != nil {
			break
		}

		switch msg.Type {
		case "ping":
			ws.WriteJSON(g.Map{"type": "pong"})
		case "register":
			handleRegisterModels(walletAddr, userID, msg.Data)
		default:
			log.Printf("[node] unknown signaling msg type=%s from=%s", msg.Type, walletAddr[:16])
		}
	}
}

// handleRegisterModels parses a register payload and upserts node_models.
// Extended payload includes node_name and version.
func handleRegisterModels(walletAddr string, userID int64, data json.RawMessage) {
	var payload struct {
		NodeName string `json:"node_name"`
		Version  string `json:"version"`
		Bindings []struct {
			ModelCode   string `json:"model_code"`
			KeyHash     string `json:"key_hash"`
			InputPrice  int64  `json:"input_price_per_1k"`
			OutputPrice int64  `json:"output_price_per_1k"`
		} `json:"bindings"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		log.Printf("[node] bad register payload from %s: %v", walletAddr[:16], err)
		return
	}

	ctx := context.Background()

	// Update node info: name (if provided) and version.
	nodeUpdate := g.Map{"last_seen_at": time.Now()}
	if payload.NodeName != "" {
		nodeUpdate["name"] = payload.NodeName
	}
	if payload.Version != "" {
		nodeUpdate["version"] = payload.Version
	}
	g.DB().Model("nodes").Ctx(ctx).Where("wallet_address", walletAddr).Data(nodeUpdate).Update()

	// Clear existing models for this node.
	g.DB().Model("node_models").Ctx(ctx).Where("wallet_address", walletAddr).Delete()

	for _, b := range payload.Bindings {
		_, err := g.DB().Model("node_models").Ctx(ctx).Insert(g.Map{
			"wallet_address": walletAddr,
			"user_id":        userID,
			"model_code":     b.ModelCode,
			"key_hash":       b.KeyHash,
			"status":         "active",
			"input_price":    b.InputPrice,
			"output_price":   b.OutputPrice,
		})
		if err != nil {
			log.Printf("[node] insert node_model %s/%s: %v", walletAddr[:16], b.ModelCode, err)
		}
	}
	log.Printf("[node] registered %d models wallet=%s name=%s version=%s",
		len(payload.Bindings), walletAddr[:16], payload.NodeName, payload.Version)
}

// lookupUserIDByWallet finds the user_id for a wallet address from nodes table.
func lookupUserIDByWallet(walletAddr string) int64 {
	v, _ := g.DB().GetValue(context.Background(), "SELECT user_id FROM nodes WHERE wallet_address = ? LIMIT 1", walletAddr)
	if !v.IsEmpty() {
		return v.Int64()
	}
	return 0
}

// ---- Data WebSocket ----

// DataWS handles data-plane WebSocket connections for nodes.
// GET /api/v1/nodes/data
func DataWS(r *ghttp.Request) {
	raw, err := r.WebSocket()
	if err != nil {
		r.Response.WriteJson(g.Map{"code": -1, "message": "websocket upgrade failed"})
		return
	}
	ws := &wsConn{raw}
	defer ws.Close()

	// Auth: receive session token.
	var authMsg struct {
		Type  string `json:"type"`
		Token string `json:"token"`
	}
	if err := ws.ReadJSON(&authMsg); err != nil || authMsg.Type != "auth" || authMsg.Token == "" {
		ws.WriteJSON(g.Map{"type": "error", "error": "expected auth with token"})
		return
	}

	sessionMu.RLock()
	walletAddr, ok := sessionTokens[authMsg.Token]
	sessionMu.RUnlock()
	if !ok {
		ws.WriteJSON(g.Map{"type": "error", "error": "invalid token"})
		return
	}

	// Associate this data connection with the connected node.
	nodesMu.Lock()
	if cn, exists := connected[walletAddr]; exists {
		cn.DataConns = append(cn.DataConns, ws)
	}
	nodesMu.Unlock()

	ws.WriteJSON(g.Map{"type": "auth_ok"})
	log.Printf("[node] data auth wallet=%s", walletAddr[:16])

	// Set up heartbeat detection: 30s ReadDeadline, node pings every 15s.
	ws.SetReadDeadline(time.Now().Add(30 * time.Second))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(30 * time.Second))
		return nil
	})

	// Remove from pool on disconnect.
	defer func() {
		nodesMu.Lock()
		if cn, exists := connected[walletAddr]; exists {
			for i, dc := range cn.DataConns {
				if dc == ws {
					cn.DataConns = append(cn.DataConns[:i], cn.DataConns[i+1:]...)
					break
				}
			}
		}
		nodesMu.Unlock()
		log.Printf("[node] data disconnected wallet=%s", walletAddr[:16])
	}()

	// Read loop: dispatch chunk/done/error to pending responses.
	for {
		var resp WsResponse
		if err := ws.ReadJSON(&resp); err != nil {
			break
		}

		switch resp.Type {
		case "chunk", "done", "error":
			pendMu.Lock()
			ch, has := pendingResps[resp.RequestID]
			pendMu.Unlock()
			if has {
				select {
				case ch <- &resp:
				default:
				}
			}
		default:
			log.Printf("[node] data unexpected type=%s from=%s", resp.Type, walletAddr[:16])
		}
	}
}

// ---- Helpers ----

// SendRequestToNode forwards a request to a connected node and returns the response.
// Used by the relay node_proxy adaptor.
func SendRequestToNode(walletAddr string, requestID string, envelope json.RawMessage) (*WsResponse, error) {
	nodesMu.RLock()
	cn, ok := connected[walletAddr]
	nodesMu.RUnlock()
	if !ok || len(cn.DataConns) == 0 {
		return nil, fmt.Errorf("node %s not connected or no data connections", walletAddr[:16])
	}

	// Pick a data connection (round-robin).
	idx := 0
	for i, dc := range cn.DataConns {
		if dc != nil {
			idx = i
			break
		}
	}
	// Rotate for next call (rough round-robin).
	dc := cn.DataConns[idx]
	nodesMu.Lock()
	if idx+1 < len(cn.DataConns) {
		cn.DataConns = append(cn.DataConns[idx+1:], cn.DataConns[:idx+1]...)
	} else {
		cn.DataConns = append(cn.DataConns[1:], cn.DataConns[0])
	}
	nodesMu.Unlock()

	// Register pending response channel.
	ch := make(chan *WsResponse, 8)
	pendMu.Lock()
	pendingResps[requestID] = ch
	pendMu.Unlock()

	defer func() {
		pendMu.Lock()
		delete(pendingResps, requestID)
		pendMu.Unlock()
	}()

	// Send request over data WS.
	if err := dc.WriteJSON(g.Map{
		"type":       "request",
		"request_id": requestID,
		"data":       envelope,
	}); err != nil {
		return nil, fmt.Errorf("send request to node: %w", err)
	}

	// Wait for response (timeout 30s).
	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("node response timeout")
	}
}

// broadcastNodeCount sends the current node count to all connected nodes.
func broadcastNodeCount() {
	nodesMu.RLock()
	count := len(connected)
	nodesMu.RUnlock()
	if count == 0 {
		return
	}
	msg := g.Map{"type": "node_list", "count": count}
	nodesMu.RLock()
	defer nodesMu.RUnlock()
	for _, n := range connected {
		if n.Signalin != nil {
			n.Signalin.WriteJSON(msg)
		}
	}
}

// GetConnectedCount returns the count of currently connected nodes.
func GetConnectedCount() int {
	nodesMu.RLock()
	defer nodesMu.RUnlock()
	return len(connected)
}

// KickNode sends a kick message to a connected node and disconnects it.
// Used by admin to forcibly disconnect a node.
func KickNode(walletAddr, reason string) {
	nodesMu.RLock()
	cn, ok := connected[walletAddr]
	nodesMu.RUnlock()
	if !ok || cn.Signalin == nil {
		return
	}
	cn.Signalin.WriteJSON(g.Map{"type": "kick", "data": g.Map{"reason": reason}})
	log.Printf("[node] sent kick to %s reason=%s", walletAddr[:16], reason)
	// Close signaling — defer on the WS handler will clean up data conns.
	cn.Signalin.Close()
}

// BroadcastCatalogUpdate sends a catalog_update notification to all connected nodes.
func BroadcastCatalogUpdate() {
	nodesMu.RLock()
	defer nodesMu.RUnlock()
	for _, n := range connected {
		if n.Signalin != nil {
			n.Signalin.WriteJSON(g.Map{"type": "catalog_update"})
		}
	}
	log.Printf("[node] broadcast catalog_update to %d nodes", len(connected))
}

// ---- Binary Attestation ----

const (
	attestExpiry   = 60 * time.Second
	attestSegments = 10
	attestMinLen   = 10
	attestMaxLen   = 20
)

var (
	binariesMu   sync.RWMutex
	binaries     = make(map[string]*officialBinary) // version → binary info

	attestMu     sync.Mutex
	attestStates = make(map[string]*attestState) // challenge_id → state

	attestedMu    sync.RWMutex
	attestedWallets = make(map[string]bool) // wallet_addr → attested (if attestation enabled)
)

type officialBinary struct {
	Version  string
	Size     int64
	SHA256   string
	FilePath string
}

// ReadSegments reads specific byte segments from the binary file on demand.
// Uses random file access — no full binary loading needed.
func (b *officialBinary) ReadSegments(offsets [][2]int) ([]byte, error) {
	f, err := os.Open(b.FilePath)
	if err != nil {
		return nil, fmt.Errorf("open binary: %w", err)
	}
	defer f.Close()

	var buf []byte
	for _, off := range offsets {
		segment := make([]byte, off[1])
		_, err := f.ReadAt(segment, int64(off[0]))
		if err != nil {
			return nil, fmt.Errorf("read offset %d+%d: %w", off[0], off[1], err)
		}
		buf = append(buf, segment...)
	}
	return buf, nil
}

type attestState struct {
	Version    string
	Nonce      []byte
	Offsets    [][2]int
	WalletAddr string
	ExpiresAt  time.Time
}

// LoadAttestationBinary registers an official release binary for attestation.
// Stores only the file path — reads segments on demand during verification.
func LoadAttestationBinary(version, filePath string, size int64, sha256Hex string) error {
	if size <= 0 {
		return fmt.Errorf("invalid binary size %d for version %s", size, version)
	}
	binariesMu.Lock()
	binaries[version] = &officialBinary{
		Version:  version,
		Size:     size,
		SHA256:   sha256Hex,
		FilePath: filePath,
	}
	binariesMu.Unlock()
	log.Printf("[attest] loaded binary version=%s size=%d sha256=%s... path=%s", version, size, sha256Hex[:16], filePath)
	return nil
}

// IsAttestEnabled returns true if at least one official binary is loaded.
func IsAttestEnabled() bool {
	binariesMu.RLock()
	defer binariesMu.RUnlock()
	return len(binaries) > 0
}

// AttestChallenge handles POST /api/v1/nodes/attest/challenge.
// The node sends {version, binary_size, wallet_address} and gets back random offsets.
// If no official binary is loaded (attestation disabled), returns immediately with disabled=true.
func AttestChallenge(r *ghttp.Request) {
	// Passthrough if no binaries loaded.
	binariesMu.RLock()
	noBinaries := len(binaries) == 0
	binariesMu.RUnlock()
	if noBinaries {
		r.Response.WriteJson(g.Map{"code": 0, "message": "ok", "data": g.Map{"disabled": true}})
		return
	}
	var req struct {
		Version    string `json:"version"`
		BinarySize int64  `json:"binary_size"`
		WalletAddr string `json:"wallet_address"`
		Signature  string `json:"signature"`
	}
	if err := r.Parse(&req); err != nil {
		r.Response.WriteJson(g.Map{"code": -1, "message": "bad request: " + err.Error()})
		return
	}
	if req.Version == "" || req.WalletAddr == "" || req.Signature == "" {
		r.Response.WriteJson(g.Map{"code": -1, "message": "version, wallet_address, signature required"})
		return
	}

	// Verify signature over (version + binary_size).
	verifyMsg := fmt.Sprintf("attest-%s-%d-%s", req.Version, req.BinarySize, req.WalletAddr)
	pubBytes, _ := hex.DecodeString(req.WalletAddr)
	sigBytes, _ := hex.DecodeString(req.Signature)
	if len(pubBytes) != ed25519.PublicKeySize || len(sigBytes) != ed25519.SignatureSize ||
		!ed25519.Verify(pubBytes, []byte(verifyMsg), sigBytes) {
		r.Response.WriteJson(g.Map{"code": -1, "message": "signature verification failed"})
		return
	}

	// Look up official binary for this version.
	binariesMu.RLock()
	bin, ok := binaries[req.Version]
	binariesMu.RUnlock()
	if !ok {
		r.Response.WriteJson(g.Map{"code": -1, "message": "unknown version: " + req.Version})
		return
	}
	if req.BinarySize != bin.Size {
		r.Response.WriteJson(g.Map{"code": -1, "message": fmt.Sprintf("binary size mismatch: got %d, expected %d", req.BinarySize, bin.Size)})
		return
	}

	// Generate random nonce.
	nonce := make([]byte, 32)
	rand.Read(nonce)

	// Generate random offsets (10 segments, 10-20 bytes each, non-overlapping).
	offsets := generateOffsets(bin.Size, attestSegments, attestMinLen, attestMaxLen)

	// Store challenge state.
	challengeID := generateToken()
	attestMu.Lock()
	attestStates[challengeID] = &attestState{
		Version:    req.Version,
		Nonce:      nonce,
		Offsets:    offsets,
		WalletAddr: req.WalletAddr,
		ExpiresAt:  time.Now().Add(attestExpiry),
	}
	attestMu.Unlock()

	r.Response.WriteJson(g.Map{"code": 0, "message": "ok", "data": g.Map{
		"challenge_id": challengeID,
		"nonce":        hex.EncodeToString(nonce),
		"offsets":      offsets,
	}})
}

// AttestVerify handles POST /api/v1/nodes/attest/verify.
// The node sends {challenge_id, hmac, wallet_address, signature}.
// If attestation is disabled, automatically passes.
func AttestVerify(r *ghttp.Request) {
	if !IsAttestEnabled() {
		r.Response.WriteJson(g.Map{"code": 0, "message": "ok", "data": g.Map{"verified": true}})
		return
	}
	var req struct {
		ChallengeID string `json:"challenge_id"`
		HMAC        string `json:"hmac"`
		WalletAddr  string `json:"wallet_address"`
		Signature   string `json:"signature"`
	}
	if err := r.Parse(&req); err != nil {
		r.Response.WriteJson(g.Map{"code": -1, "message": "bad request: " + err.Error()})
		return
	}

	// Look up challenge state.
	attestMu.Lock()
	state, ok := attestStates[req.ChallengeID]
	if ok {
		delete(attestStates, req.ChallengeID) // one-time use
	}
	attestMu.Unlock()
	if !ok {
		r.Response.WriteJson(g.Map{"code": -1, "message": "invalid or expired challenge"})
		return
	}
	if time.Now().After(state.ExpiresAt) {
		r.Response.WriteJson(g.Map{"code": -1, "message": "challenge expired"})
		return
	}
	if req.WalletAddr != state.WalletAddr {
		r.Response.WriteJson(g.Map{"code": -1, "message": "wallet mismatch"})
		return
	}

	// Look up official binary.
	binariesMu.RLock()
	bin, ok := binaries[state.Version]
	binariesMu.RUnlock()
	if !ok {
		r.Response.WriteJson(g.Map{"code": -1, "message": "binary not found"})
		return
	}

	// Read byte segments from official binary and compute expected HMAC.
	segments, err := bin.ReadSegments(state.Offsets)
	if err != nil {
		log.Printf("[attest] read segments failed: %v", err)
		r.Response.WriteJson(g.Map{"code": -1, "message": "binary read error"})
		return
	}
	mac := hmac.New(sha256.New, state.Nonce)
	mac.Write(segments)
	expectedHMAC := hex.EncodeToString(mac.Sum(nil))

	if req.HMAC != expectedHMAC {
		log.Printf("[attest] FAILED wallet=%s version=%s", req.WalletAddr[:16], state.Version)
		r.Response.WriteJson(g.Map{"code": -1, "message": "attestation failed — binary mismatch"})
		return
	}

	// Verify signature to bind wallet identity to the proof.
	verifyMsg := fmt.Sprintf("attest-proof-%s-%s", req.ChallengeID, req.HMAC)
	pubBytes, _ := hex.DecodeString(req.WalletAddr)
	sigBytes, _ := hex.DecodeString(req.Signature)
	if !ed25519.Verify(pubBytes, []byte(verifyMsg), sigBytes) {
		r.Response.WriteJson(g.Map{"code": -1, "message": "proof signature invalid"})
		return
	}

	// Mark wallet as attested for WS auth check.
	attestedMu.Lock()
	attestedWallets[req.WalletAddr] = true
	attestedMu.Unlock()

	log.Printf("[attest] PASSED wallet=%s version=%s", req.WalletAddr[:16], state.Version)
	r.Response.WriteJson(g.Map{"code": 0, "message": "ok", "data": g.Map{"verified": true}})
}

// generateOffsets creates N non-overlapping random byte ranges within [0, size).
// Each range has length between minLen and maxLen. Segments are spread across the file.
func generateOffsets(binarySize int64, count, minLen, maxLen int) [][2]int {
	if binarySize < int64(count*minLen) {
		count = int(binarySize) / minLen
		if count < 1 {
			count = 1
			maxLen = int(binarySize)
			minLen = maxLen
		}
	}

	segmentPool := int(binarySize) / count
	if segmentPool < minLen {
		segmentPool = minLen
	}

	offsets := make([][2]int, 0, count)
	used := make(map[int]bool)

	for i := 0; i < count; i++ {
		poolStart := int64(i) * int64(segmentPool)
		jitterMax := int64(segmentPool) - int64(maxLen)
		if jitterMax < 0 {
			jitterMax = 0
		}

		jitter := uniformRand(0, int(jitterMax))
		length := uniformRand(minLen, maxLen)
		start := int(poolStart) + jitter
		if start+length > int(binarySize) {
			start = int(binarySize) - length
		}
		if start < 0 {
			start = 0
		}

		overlap := false
		for _, off := range offsets {
			if (start >= off[0] && start < off[0]+off[1]) ||
				(start+length > off[0] && start+length <= off[0]+off[1]) ||
				(start <= off[0] && start+length >= off[0]+off[1]) {
				overlap = true
				break
			}
		}
		if overlap || used[start] {
			i--
			continue
		}
		used[start] = true
		for j := start; j < start+length; j++ {
			used[j] = true
		}

		offsets = append(offsets, [2]int{start, length})
	}
	return offsets
}

// uniformRand returns a random int in [min, max] using crypto/rand.
func uniformRand(min, max int) int {
	if min >= max {
		return min
	}
	b := make([]byte, 8)
	rand.Read(b)
	v := int(b[0])<<56 | int(b[1])<<48 | int(b[2])<<40 | int(b[3])<<32 |
		int(b[4])<<24 | int(b[5])<<16 | int(b[6])<<8 | int(b[7])
	if v < 0 {
		v = -v
	}
	return min + v%(max-min+1)
}
