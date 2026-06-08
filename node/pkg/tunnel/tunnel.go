package tunnel

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"ai-platform-node/pkg/keychain"
	"ai-platform-node/pkg/router"
	"ai-platform-node/pkg/types"
	"ai-platform-node/pkg/usagetracker"

	"github.com/gorilla/websocket"
)

// Status holds current tunnel connection state.
type Status struct {
	Connected  bool   `json:"connected"`
	WalletAddr string `json:"wallet_address,omitempty"`
	Platform   string `json:"platform,omitempty"`
	Uptime     int64  `json:"uptime_sec,omitempty"`
	PeerCount  int    `json:"peer_count,omitempty"`
}

// Signer abstracts Ed25519 signing so tunnel doesn't import wallet directly.
type Signer interface {
	Address() string
	Sign(msg []byte) []byte
}

// ---- Tunnel (high-level, manages signaling + data pool) ----

type Tunnel struct {
	platformURL string
	signer      Signer
	router      *router.Router
	sig         *signalingClient
	data        *dataPool
	mu          sync.Mutex
	started     bool
	stopCh      chan struct{}
	kicked      chan struct{}
	nodeName    string
	nodeVersion string
}

// NewClient creates a new tunnel with signaling + data pool.
func NewClient(platformURL string, signer Signer, r *router.Router, nodeName, nodeVersion string) *Tunnel {
	pu := strings.TrimRight(platformURL, "/")
	return &Tunnel{
		platformURL: pu,
		signer:      signer,
		router:      r,
		stopCh:      make(chan struct{}),
		kicked:      make(chan struct{}),
		nodeName:    nodeName,
		nodeVersion: nodeVersion,
	}
}

// IsKicked returns true if the platform kicked this node.
func (t *Tunnel) IsKicked() bool {
	select {
	case <-t.kicked:
		return true
	default:
		return false
	}
}

// Connect performs initial connection: signaling auth + data pool auth.
func (t *Tunnel) Connect(ctx context.Context) error {
	// Close old connections before creating new ones.
	t.mu.Lock()
	if t.data != nil {
		t.data.closeAll()
		t.data = nil
	}
	if t.sig != nil {
		t.sig.close()
		t.sig = nil
	}
	t.mu.Unlock()

	t.mu.Lock()
	t.sig = &signalingClient{
		platformURL: t.platformURL,
		signer:      t.signer,
		onKick:      t.onKick,
	}
	t.mu.Unlock()

	// Connect signaling.
	if err := t.sig.connect(ctx); err != nil {
		return fmt.Errorf("signaling connect: %w", err)
	}

	token := t.sig.sessionToken
	log.Printf("[tunnel] signaling connected, token=%s...", token[:16])

	// Connect data pool.
	t.mu.Lock()
	t.data = &dataPool{
		platformURL: t.platformURL,
		size:        4,
		signer:      t.signer,
		router:      t.router,
	}
	t.mu.Unlock()

	if err := t.data.connectAll(ctx, token); err != nil {
		log.Printf("[tunnel] data pool partial connect: %v", err)
	}

	// Register shared bindings (with node name and version).
	if err := t.sig.registerBindings(t.router, t.nodeName, t.nodeVersion); err != nil {
		log.Printf("[tunnel] register bindings: %v", err)
	}

	return nil
}

// onKick is called when the platform sends a kick message.
func (t *Tunnel) onKick(reason string) {
	log.Printf("[tunnel] KICKED by platform: %s", reason)
	close(t.kicked)
	t.mu.Lock()
	if t.sig != nil {
		t.sig.close()
	}
	if t.data != nil {
		t.data.closeAll()
	}
	t.mu.Unlock()
}

// ConnectWithRetry starts the reconnection loop in the background.
// Blocks until first successful connection, then returns.
func (t *Tunnel) ConnectWithRetry(ctx context.Context) {
	t.mu.Lock()
	if t.started {
		t.mu.Unlock()
		return
	}
	t.started = true
	t.mu.Unlock()

	go func() {
		for {
			select {
			case <-t.stopCh:
				return
			case <-t.kicked:
				log.Println("[tunnel] stopped reconnect loop (kicked)")
				return
			default:
			}

			if err := t.Connect(ctx); err != nil {
				log.Printf("[tunnel] connect failed: %v (retry 10s)", err)
				select {
				case <-t.stopCh:
					return
				case <-t.kicked:
					return
				case <-time.After(10 * time.Second):
				}
				continue
			}

			// Connected — wait for signaling disconnect.
			t.mu.Lock()
			sig := t.sig
			t.mu.Unlock()

			if sig != nil {
				sig.waitDisconnect()
				// Check if this disconnect was due to kick.
				if t.IsKicked() {
					log.Println("[tunnel] disconnect was kick, not reconnecting")
					return
				}
				log.Println("[tunnel] signaling disconnected, reconnecting...")
			}

			select {
			case <-t.stopCh:
				return
			case <-t.kicked:
				return
			case <-time.After(5 * time.Second):
			}
		}
	}()
}

// Stop signals all loops to stop and closes connections.
func (t *Tunnel) Stop() {
	close(t.stopCh)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.sig != nil {
		t.sig.close()
	}
	if t.data != nil {
		t.data.closeAll()
	}
}

// GetStatus returns the current tunnel status.
func (t *Tunnel) GetStatus() *Status {
	t.mu.Lock()
	defer t.mu.Unlock()

	s := &Status{
		WalletAddr: t.signer.Address(),
		Platform:   t.platformURL,
		Connected:  false,
	}
	if t.sig != nil {
		st := t.sig.getStatus()
		s.Connected = st.Connected
		s.Uptime = st.Uptime
	}
	if t.sig != nil {
		s.PeerCount = t.sig.peerCount
	}
	return s
}

// UpdateBindings re-registers shared bindings on the signaling channel.
func (t *Tunnel) UpdateBindings(ctx context.Context) {
	t.mu.Lock()
	sig := t.sig
	t.mu.Unlock()
	if sig != nil {
		sig.registerBindings(t.router, t.nodeName, t.nodeVersion)
	}
}

// ---- Signaling Client ----

type signalingClient struct {
	platformURL  string
	signer       Signer
	conn         *websocket.Conn
	mu           sync.Mutex
	connectedAt  time.Time
	sessionToken string
	peerCount    int
	disconnectCh chan struct{}
	onKick       func(reason string)
}

func (s *signalingClient) connect(ctx context.Context) error {
	s.mu.Lock()
	if s.conn != nil {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	wsURL := s.platformURL + "/api/v1/nodes/ws"
	u, err := url.Parse(wsURL)
	if err == nil {
		u.Scheme = "ws"
		wsURL = u.String()
	}
	log.Printf("[tunnel] signaling dial %s", wsURL)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}

	s.mu.Lock()
	s.conn = conn
	s.connectedAt = time.Now()
	s.disconnectCh = make(chan struct{})
	s.mu.Unlock()

	// Step 1: send wallet address.
	addr := s.signer.Address()
	if err := conn.WriteJSON(types.WSMessage{Type: "auth", NodeID: addr}); err != nil {
		s.close()
		return fmt.Errorf("auth send: %w", err)
	}

	// Step 2: receive challenge.
	var challenge types.WSMessage
	if err := conn.ReadJSON(&challenge); err != nil {
		s.close()
		return fmt.Errorf("challenge read: %w", err)
	}
	if challenge.Type != "challenge" {
		s.close()
		return fmt.Errorf("expected challenge, got %s: %s", challenge.Type, challenge.Error)
	}

	// Step 3: sign nonce.
	nonceBytes, err := hex.DecodeString(challenge.Nonce)
	if err != nil {
		s.close()
		return fmt.Errorf("bad nonce hex: %w", err)
	}
	sig := s.signer.Sign(nonceBytes)
	sigHex := hex.EncodeToString(sig)

	if err := conn.WriteJSON(types.WSMessage{Type: "auth_response", Data: json.RawMessage(`"` + sigHex + `"`)}); err != nil {
		s.close()
		return fmt.Errorf("auth response send: %w", err)
	}

	// Step 4: read auth_ok with session_token.
	var authResp struct {
		Type         string `json:"type"`
		SessionToken string `json:"session_token"`
		Error        string `json:"error,omitempty"`
	}
	if err := conn.ReadJSON(&authResp); err != nil {
		s.close()
		return fmt.Errorf("auth result read: %w", err)
	}
	if authResp.Type != "auth_ok" {
		s.close()
		return fmt.Errorf("auth failed: %s", authResp.Error)
	}

	s.sessionToken = authResp.SessionToken
	log.Printf("[tunnel] signaling auth ok addr=%s token=%s...", addr[:16], s.sessionToken[:16])

	// Start read and heartbeat loops.
	go s.readLoop()
	go s.heartbeatLoop()

	return nil
}

func (s *signalingClient) registerBindings(r *router.Router, nodeName, nodeVersion string) error {
	bindings, _ := keychain.SharedBindings()
	if len(bindings) == 0 {
		return nil
	}

	type extRegisterPayload struct {
		NodeName string              `json:"node_name"`
		Version  string              `json:"version"`
		Bindings []types.ModelBinding `json:"bindings"`
	}

	payload := extRegisterPayload{
		NodeName: nodeName,
		Version:  nodeVersion,
	}
	for _, b := range bindings {
		payload.Bindings = append(payload.Bindings, types.ModelBinding{
			ModelCode:   b.ModelCode,
			KeyHash:     b.KeyHash,
			InputPrice:  b.InputPrice,
			OutputPrice: b.OutputPrice,
		})
	}
	regData, _ := json.Marshal(payload)

	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("not connected")
	}

	if err := conn.WriteJSON(types.WSMessage{Type: "register", Data: regData}); err != nil {
		return fmt.Errorf("register send: %w", err)
	}
	log.Printf("[tunnel] registered %d shared bindings name=%s version=%s", len(bindings), nodeName, nodeVersion)

	for _, b := range bindings {
		r.Register(b.ModelCode, b.KeyHash)
	}
	return nil
}

func (s *signalingClient) readLoop() {
	for {
		s.mu.Lock()
		conn := s.conn
		s.mu.Unlock()
		if conn == nil {
			return
		}

		var msg struct {
			Type  string          `json:"type"`
			Data  json.RawMessage `json:"data,omitempty"`
			Error string          `json:"error,omitempty"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			log.Println("[tunnel] signaling read error:", err)
			s.close()
			return
		}

		switch msg.Type {
		case "pong":
		case "node_list":
			var list struct{ Count int `json:"count"` }
			if msg.Data != nil {
				json.Unmarshal(msg.Data, &list)
			}
			s.mu.Lock()
			s.peerCount = list.Count
			s.mu.Unlock()
		case "registered":
			log.Println("[tunnel] platform confirmed registration")
		case "catalog_update":
			log.Println("[tunnel] platform catalog updated — consider refreshing local cache")
		case "kick":
			var kickMsg struct{ Reason string `json:"reason,omitempty"` }
			if msg.Data != nil {
				json.Unmarshal(msg.Data, &kickMsg)
			}
			if s.onKick != nil {
				s.onKick(kickMsg.Reason)
			}
			return
		case "error":
			log.Println("[tunnel] signaling error:", msg.Error)
		default:
			log.Println("[tunnel] signaling unexpected msg type:", msg.Type)
		}
	}
}

func (s *signalingClient) heartbeatLoop() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			s.mu.Lock()
			conn := s.conn
			s.mu.Unlock()
			if conn != nil {
				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := conn.WriteJSON(types.WSMessage{Type: "ping"}); err != nil {
					log.Printf("[tunnel] signaling ping error, closing: %v", err)
					s.close()
					return
				}
			}
		case <-s.waitDisconnectCh():
			return
		}
	}
}

func (s *signalingClient) waitDisconnect() {
	ch := s.getDisconnectCh()
	if ch != nil {
		<-ch
	}
}

func (s *signalingClient) getDisconnectCh() chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.disconnectCh
}

func (s *signalingClient) waitDisconnectCh() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.disconnectCh == nil {
		return nil
	}
	return s.disconnectCh
}

func (s *signalingClient) getStatus() *Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return &Status{Connected: false}
	}
	return &Status{
		Connected: true,
		Uptime:    int64(time.Since(s.connectedAt).Seconds()),
	}
}

func (s *signalingClient) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	if s.disconnectCh != nil {
		close(s.disconnectCh)
		s.disconnectCh = nil
	}
}

// ---- Data Pool ----

type dataPool struct {
	platformURL string
	size        int
	token       string
	signer      Signer
	conns       []*websocket.Conn
	mu          sync.Mutex
	router      *router.Router
	closing     bool
}

func (p *dataPool) connectAll(ctx context.Context, token string) error {
	wsURL := p.platformURL + "/api/v1/nodes/data"
	u, err := url.Parse(wsURL)
	if err == nil {
		u.Scheme = "ws"
		wsURL = u.String()
	}
	log.Printf("[tunnel] data pool dialing %d connections", p.size)

	p.mu.Lock()
	p.closing = false
	p.token = token
	p.mu.Unlock()

	var conns []*websocket.Conn
	for i := 0; i < p.size; i++ {
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
		if err != nil {
			log.Printf("[tunnel] data conn %d dial: %v", i, err)
			continue
		}

		// Auth with session token.
		authMsg := map[string]string{"type": "auth", "token": token}
		if err := conn.WriteJSON(authMsg); err != nil {
			conn.Close()
			log.Printf("[tunnel] data conn %d auth send: %v", i, err)
			continue
		}

		var authResp types.WSMessage
		if err := conn.ReadJSON(&authResp); err != nil || authResp.Type != "auth_ok" {
			conn.Close()
			log.Printf("[tunnel] data conn %d auth failed: %v %s", i, err, authResp.Error)
			continue
		}

		conns = append(conns, conn)
		log.Printf("[tunnel] data conn %d connected", i)
	}

	p.mu.Lock()
	p.conns = conns
	p.mu.Unlock()

	// Start read loops and heartbeat goroutines for each connection.
	for _, c := range conns {
		go p.readLoop(c)
		go p.heartbeatLoop(c)
	}

	if len(conns) == 0 {
		return fmt.Errorf("no data connections established")
	}
	log.Printf("[tunnel] data pool ready: %d/%d", len(conns), p.size)
	return nil
}

func (p *dataPool) heartbeatLoop(conn *websocket.Conn) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.mu.Lock()
			closing := p.closing
			p.mu.Unlock()
			if closing {
				return
			}
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (p *dataPool) readLoop(conn *websocket.Conn) {
	for {
		var msg struct {
			Type      string          `json:"type"`
			RequestID string          `json:"request_id"`
			Data      json.RawMessage `json:"data"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			log.Println("[tunnel] data read error:", err)
			break
		}

		var env types.RequestEnvelope
		if err := json.Unmarshal(msg.Data, &env); err != nil {
			log.Println("[tunnel] data unmarshal error:", err)
			continue
		}

		// Handle incoming request.
		if env.Request.Stream {
			p.handleStreamRequest(conn, &env)
		} else {
			p.handleRequest(conn, &env)
		}
	}

	// Remove dead conn from pool and close it.
	p.mu.Lock()
	for i, c := range p.conns {
		if c == conn {
			p.conns = append(p.conns[:i], p.conns[i+1:]...)
			break
		}
	}
	closing := p.closing
	p.mu.Unlock()
	conn.Close()

	// Auto-replenish if pool not shutting down.
	if !closing {
		log.Println("[tunnel] data conn lost, replenishing...")
		go p.replenishOne()
	}
}

// replenishOne dials one new data connection and adds it to the pool.
func (p *dataPool) replenishOne() {
	wsURL := p.platformURL + "/api/v1/nodes/data"
	u, err := url.Parse(wsURL)
	if err == nil {
		u.Scheme = "ws"
		wsURL = u.String()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		log.Printf("[tunnel] replenish dial: %v", err)
		return
	}

	p.mu.Lock()
	token := p.token
	p.mu.Unlock()

	authMsg := map[string]string{"type": "auth", "token": token}
	if err := conn.WriteJSON(authMsg); err != nil {
		conn.Close()
		log.Printf("[tunnel] replenish auth: %v", err)
		return
	}

	var authResp types.WSMessage
	if err := conn.ReadJSON(&authResp); err != nil || authResp.Type != "auth_ok" {
		conn.Close()
		return
	}

	p.mu.Lock()
	if p.closing {
		p.mu.Unlock()
		conn.Close()
		return
	}
	p.conns = append(p.conns, conn)
	p.mu.Unlock()

	go p.readLoop(conn)
	go p.heartbeatLoop(conn)
	log.Printf("[tunnel] data conn replenished (pool size: %d)", len(p.conns))
}

func (p *dataPool) handleRequest(conn *websocket.Conn, env *types.RequestEnvelope) {
	resp, err := p.getRouter().Route(context.Background(), env)
	if err != nil {
		conn.WriteJSON(types.WSMessage{Type: "error", RequestID: env.RequestID, Error: err.Error()})
		return
	}
	respJSON, _ := json.Marshal(resp)
	conn.WriteJSON(types.WSMessage{Type: "done", RequestID: env.RequestID, Data: respJSON})

	// Record usage.
	if resp.Usage.TotalTokens > 0 {
		entry, _, err := keychain.FindKeyByHash(env.KeyHash)
		if err == nil {
			usagetracker.Insert(&usagetracker.Record{
				RequestID:    env.RequestID,
				ModelKeyID:   entry.ID,
				ChannelID:    entry.ChannelID,
				Capability:   "chat",
				InputTokens:  resp.Usage.PromptTokens,
				OutputTokens: resp.Usage.CompletionTokens,
				TotalTokens:  resp.Usage.TotalTokens,
				Status:       "success",
				Source:       "tunnel",
			})
		}
	}
}

func (p *dataPool) handleStreamRequest(conn *websocket.Conn, env *types.RequestEnvelope) {
	chunks, errCh := p.getRouter().RouteStream(context.Background(), env)
	for {
		select {
		case chunk, ok := <-chunks:
			if !ok {
				conn.WriteJSON(types.WSMessage{Type: "done", RequestID: env.RequestID})
				return
			}
			chunkJSON, _ := json.Marshal(chunk)
			conn.WriteJSON(types.WSMessage{Type: "chunk", RequestID: env.RequestID, Data: chunkJSON})
		case err := <-errCh:
			conn.WriteJSON(types.WSMessage{Type: "error", RequestID: env.RequestID, Error: err.Error()})
			return
		}
	}
}

func (p *dataPool) getRouter() *router.Router {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.router
}

// SetRouter updates the router reference for data pool (used during reconnect).
func (p *dataPool) SetRouter(r *router.Router) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.router = r
}

func (p *dataPool) closeAll() {
	p.mu.Lock()
	p.closing = true
	for _, c := range p.conns {
		if c != nil {
			c.Close()
		}
	}
	p.conns = nil
	p.mu.Unlock()
}
