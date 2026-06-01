package tunnel

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"ai-platform-node/internal/db"
	"ai-platform-node/internal/keychain"
	"ai-platform-node/internal/router"
	"ai-platform-node/internal/types"

	"github.com/gorilla/websocket"
)

type Client struct {
	platformURL string
	nodeID      string
	nodeSecret  string
	conn        *websocket.Conn
	mu          sync.Mutex
	router      *router.Router
	pending     map[string]chan []byte // requestID -> response channel
	pmu         sync.RWMutex
}

func NewClient(platformURL, nodeID, nodeSecret string, r *router.Router) *Client {
	return &Client{
		platformURL: platformURL,
		nodeID:      nodeID,
		nodeSecret:  nodeSecret,
		router:      r,
		pending:     make(map[string]chan []byte),
	}
}

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.platformURL+"/api/v1/nodes/ws", nil)
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}
	c.conn = conn

	// Auth
	sig := hmacSHA256(c.nodeID, c.nodeSecret)
	authPayload, _ := json.Marshal(types.AuthPayload{NodeID: c.nodeID, Signature: sig})
	if err := conn.WriteJSON(types.WSMessage{Type: "auth", Data: authPayload}); err != nil {
		return fmt.Errorf("auth send: %w", err)
	}

	var authResp types.WSMessage
	if err := conn.ReadJSON(&authResp); err != nil {
		return fmt.Errorf("auth read: %w", err)
	}
	if authResp.Type != "auth_ok" {
		return fmt.Errorf("auth failed: %s", authResp.Error)
	}
	log.Println("[tunnel] authenticated as", c.nodeID)

	// Register shared bindings
	bindings, _ := keychain.SharedBindings()
	if len(bindings) > 0 {
		reg := types.RegisterPayload{}
		for _, b := range bindings {
			// We don't know actual prices here - they'd come from local config
			reg.Bindings = append(reg.Bindings, types.ModelBinding{
				ModelCode:  b.ModelCode,
				KeyHash:    b.KeyHash,
			})
		}
		regData, _ := json.Marshal(reg)
		conn.WriteJSON(types.WSMessage{Type: "register", Data: regData})
		log.Printf("[tunnel] registered %d shared bindings", len(bindings))

		// Register with router
		for _, b := range bindings {
			c.router.Register(b.ModelCode, b.KeyHash)
		}
	}

	go c.readLoop(ctx)
	go c.heartbeatLoop(ctx)

	return nil
}

func (c *Client) readLoop(ctx context.Context) {
	for {
		var msg types.WSMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			log.Println("[tunnel] read error:", err)
			return
		}

		switch msg.Type {
		case "pong":
			// ignore
		case "request":
			c.handleRequest(ctx, &msg)
		case "registered":
			log.Println("[tunnel] platform confirmed registration")
		case "error":
			log.Println("[tunnel] platform error:", msg.Error)
		default:
			log.Println("[tunnel] unexpected message type:", msg.Type)
		}
	}
}

func (c *Client) handleRequest(ctx context.Context, msg *types.WSMessage) {
	var env types.RequestEnvelope
	if err := json.Unmarshal(msg.Data, &env); err != nil {
		log.Println("[tunnel] bad request payload:", err)
		return
	}

	if env.Request.Stream {
		chunks, errCh := c.router.RouteStream(ctx, &env)
		for {
			select {
			case chunk, ok := <-chunks:
				if !ok {
					c.conn.WriteJSON(types.WSMessage{Type: "done", RequestID: env.RequestID})
					return
				}
				chunkJSON, _ := json.Marshal(chunk)
				c.conn.WriteJSON(types.WSMessage{Type: "chunk", RequestID: env.RequestID, Data: chunkJSON})
			case err := <-errCh:
				c.conn.WriteJSON(types.WSMessage{Type: "error", RequestID: env.RequestID, Error: err.Error()})
				return
			case <-ctx.Done():
				return
			}
		}
	} else {
		resp, err := c.router.Route(ctx, &env)
		if err != nil {
			c.conn.WriteJSON(types.WSMessage{Type: "error", RequestID: env.RequestID, Error: err.Error()})
			return
		}
		respJSON, _ := json.Marshal(resp)
		c.conn.WriteJSON(types.WSMessage{Type: "done", RequestID: env.RequestID, Data: respJSON})
	}
}

func (c *Client) heartbeatLoop(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			c.mu.Lock()
			if c.conn != nil {
				c.conn.WriteJSON(types.WSMessage{Type: "ping"})
			}
			c.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func hmacSHA256(nodeID, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(nodeID))
	return hex.EncodeToString(h.Sum(nil))
}

// SetPlatformConfig stores node credentials for later restarts.
func (c *Client) SetPlatformConfig(url, nodeID, secret string) {
	db.SetConfig("platform_url", url)
	db.SetConfig("node_id", nodeID)
	db.SetConfig("node_secret", secret)
}
