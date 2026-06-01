package main

import (
	"context"
	"embed"
	"log"

	"ai-platform/cmd/node/internal/db"
	"ai-platform/cmd/node/internal/keychain"
	"ai-platform/cmd/node/internal/router"
	"ai-platform/cmd/node/internal/server"
	"ai-platform/cmd/node/internal/tunnel"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed frontend/dist
var assets embed.FS

// NodeService exposes local node control to the React frontend via Wails Bind.
type NodeService struct {
	nodeSrv    *server.Server
	router     *router.Router
	tunnel     *tunnel.Client
}

// NewNodeService creates the node and starts it.
func NewNodeService() *NodeService {
	db.Open("node.db")
	r := router.New()

	// Register local shared bindings
	bindings, _ := keychain.SharedBindings()
	for _, b := range bindings {
		r.Register(b.ModelCode, b.KeyHash)
	}

	srv := server.New(r)
	go srv.Start(":20128")

	// WS tunnel (optional)
	nodeID, _ := db.GetConfig("node_id")
	platformURL, _ := db.GetConfig("platform_url")
	nodeSecret, _ := db.GetConfig("node_secret")
	var cli *tunnel.Client
	if nodeID != "" && platformURL != "" {
		cli = tunnel.NewClient(platformURL, nodeID, nodeSecret, r)
		go func() {
			if err := cli.Connect(context.Background()); err != nil {
				log.Printf("platform connect: %v", err)
			}
		}()
	}

	return &NodeService{nodeSrv: srv, router: r, tunnel: cli}
}

// Start starts the local node HTTP server.
func (n *NodeService) Start() string {
	// Already started in NewNodeService
	return "running"
}

// Status returns node status.
func (n *NodeService) Status() map[string]any {
	return map[string]any{"running": true, "port": 20128}
}

// Stop stops the node server.
func (n *NodeService) Stop() string {
	n.nodeSrv.Stop()
	return "stopped"
}

// AddKey adds a local API key via keychain.
func (n *NodeService) AddKey(label, keyValue, channelID, baseURL string) map[string]any {
	e, err := keychain.AddKey(label, keyValue, channelID, baseURL)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return map[string]any{"id": e.ID, "label": e.Label}
}

// ListKeys returns all local keys.
func (n *NodeService) ListKeys() []map[string]any {
	keys, _ := keychain.ListKeys()
	out := make([]map[string]any, len(keys))
	for i, k := range keys {
		out[i] = map[string]any{"id": k.ID, "label": k.Label, "channel_id": k.ChannelID, "status": k.Status}
	}
	return out
}

// BindModel binds a key to a platform model.
func (n *NodeService) BindModel(keyID, modelCode, modelName string, shared bool) map[string]any {
	b, err := keychain.AddBinding(keyID, modelCode, modelName, shared)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	if shared {
		n.router.Register(modelCode, b.KeyHash)
	}
	return map[string]any{"id": b.ID, "model_code": b.ModelCode, "key_hash": b.KeyHash[:16] + "..."}
}

func main() {
	node := NewNodeService()

	err := wails.Run(&options.App{
		Title:  "Token Flux Node",
		Width:  1200,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Bind: []any{node},
	})
	if err != nil {
		log.Fatal(err)
	}
}
