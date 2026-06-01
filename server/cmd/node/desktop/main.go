package main

import (
	"embed"
	"log"
	"time"

	"ai-platform/cmd/node/internal/combo"
	"ai-platform/cmd/node/internal/db"
	"ai-platform/cmd/node/internal/keychain"
	"ai-platform/cmd/node/internal/router"
	"ai-platform/cmd/node/internal/rtk"
	"ai-platform/cmd/node/internal/server"
	"ai-platform/cmd/node/internal/tunnel"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed frontend/dist
var assets embed.FS

type NodeService struct {
	nodeSrv *server.Server
	rtr     *router.Router
	tunnel  *tunnel.Client
	rtkCfg  rtk.Config
	onlineAt time.Time
}

func NewNodeService() *NodeService {
	db.Open("node.db")
	r := router.New()
	bindings, _ := keychain.SharedBindings()
	for _, b := range bindings {
		r.Register(b.ModelCode, b.KeyHash)
	}
	srv := server.New(r)
	go srv.Start(":20128")

	return &NodeService{nodeSrv: srv, rtr: r, onlineAt: time.Now()}
}

func (n *NodeService) Status() map[string]any {
	return map[string]any{
		"running":      true, "port": 20128,
		"uptime_sec":   int(time.Since(n.onlineAt).Seconds()),
		"models_count": len(n.rtr.Models()),
		"keys_count":   n.countKeys(),
		"online":       n.tunnel != nil,
	}
}

func (n *NodeService) Stop() string { n.nodeSrv.Stop(); return "stopped" }

func (n *NodeService) countKeys() int {
	keys, _ := keychain.ListKeys()
	return len(keys)
}

func (n *NodeService) AddKey(label, keyValue, channelID, baseURL string) map[string]any {
	e, err := keychain.AddKey(label, keyValue, channelID, baseURL)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return map[string]any{"id": e.ID, "label": e.Label, "channel_id": e.ChannelID, "status": e.Status}
}

func (n *NodeService) ListKeys() []map[string]any {
	keys, err := keychain.ListKeys()
	if err != nil || keys == nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, len(keys))
	for i, k := range keys {
		out[i] = map[string]any{
			"id": k.ID, "label": k.Label, "channel_id": k.ChannelID,
			"base_url": k.BaseURL, "status": k.Status, "created_at": k.CreatedAt,
		}
	}
	return out
}

func (n *NodeService) DeleteKey(id string) string {
	db.DB().Exec("DELETE FROM keys WHERE id = ?", id)
	return "deleted"
}

func (n *NodeService) BindModel(keyID, modelCode, modelName string, shared bool) map[string]any {
	b, err := keychain.AddBinding(keyID, modelCode, modelName, shared)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	if shared {
		n.rtr.Register(modelCode, b.KeyHash)
	}
	return map[string]any{"id": b.ID, "model_code": b.ModelCode, "key_hash": b.KeyHash[:16] + "...", "shared": b.Shared}
}

func (n *NodeService) UnbindModel(id string) string {
	db.DB().Exec("DELETE FROM bindings WHERE id = ?", id)
	return "unbound"
}

func (n *NodeService) ListBindings() []map[string]any {
	rows, err := db.DB().Query("SELECT id, key_id, model_code, model_name, key_hash, shared, created_at FROM bindings")
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, keyID, modelCode, modelName, keyHash string
		var shared, createdAt int
		rows.Scan(&id, &keyID, &modelCode, &modelName, &keyHash, &shared, &createdAt)
		out = append(out, map[string]any{
			"id": id, "model_code": modelCode, "model_name": modelName,
			"key_hash": keyHash[:16] + "...", "shared": shared == 1, "created_at": createdAt,
		})
	}
	return out
}

func (n *NodeService) CreateCombo(name string, models []string, strategy string, sticky int) map[string]any {
	c, err := combo.Create(name, models, strategy, sticky)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return map[string]any{"name": c.Name, "models": c.Models, "strategy": c.Strategy, "sticky": c.Sticky}
}

func (n *NodeService) ListCombos() []map[string]any {
	combos, err := combo.List()
	if err != nil || combos == nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, len(combos))
	for i, c := range combos {
		out[i] = map[string]any{"name": c.Name, "models": c.Models, "strategy": c.Strategy, "sticky": c.Sticky}
	}
	return out
}

func (n *NodeService) DeleteCombo(name string) string { combo.Delete(name); return "deleted" }

func (n *NodeService) SetRTK(enabled bool, mode string) string {
	n.rtkCfg = rtk.Config{Enabled: enabled, Mode: mode}
	db.SetConfig("rtk_enabled", boolStr(enabled))
	db.SetConfig("rtk_mode", mode)
	return "ok"
}

func (n *NodeService) SetCaveman(enabled bool) string {
	db.SetConfig("caveman_enabled", boolStr(enabled))
	return "ok"
}

func (n *NodeService) GetConfig() map[string]any {
	rtkEnabled, _ := db.GetConfig("rtk_enabled")
	rtkMode, _ := db.GetConfig("rtk_mode")
	caveman, _ := db.GetConfig("caveman_enabled")
	return map[string]any{
		"rtk_enabled": rtkEnabled == "true",
		"rtk_mode":    ifZero(rtkMode, "auto"),
		"caveman_enabled": caveman == "true",
	}
}

func (n *NodeService) GetUsageStats() map[string]any {
	var totalTokens, totalCalls, totalCost int64
	db.DB().QueryRow("SELECT COALESCE(COUNT(*),0), COALESCE(SUM(tokens),0), COALESCE(SUM(cost_credits),0) FROM usage_log").Scan(&totalCalls, &totalTokens, &totalCost)
	rows, _ := db.DB().Query("SELECT model, COUNT(*), SUM(tokens), SUM(cost_credits) FROM usage_log GROUP BY model ORDER BY COUNT(*) DESC LIMIT 10")
	var byModel []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var model string; var calls, tokens, cost int64
			rows.Scan(&model, &calls, &tokens, &cost)
			byModel = append(byModel, map[string]any{"model": model, "calls": calls, "tokens": tokens, "cost": cost})
		}
	}
	return map[string]any{"total_calls": totalCalls, "total_tokens": totalTokens, "total_cost": totalCost, "by_model": byModel}
}

func (n *NodeService) GetUsageLogs(limit int) []map[string]any {
	if limit <= 0 || limit > 500 { limit = 50 }
	rows, _ := db.DB().Query("SELECT request_id, model, provider, tokens, input_tokens, output_tokens, cost_credits, latency_ms, success, created_at FROM usage_log ORDER BY id DESC LIMIT ?", limit)
	if rows == nil { return []map[string]any{} }
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var rid, model, provider string; var tokens, inT, outT, cost, lat int64; var success int; var created int64
		rows.Scan(&rid, &model, &provider, &tokens, &inT, &outT, &cost, &lat, &success, &created)
		out = append(out, map[string]any{
			"request_id": rid, "model": model, "provider": provider, "tokens": tokens,
			"input_tokens": inT, "output_tokens": outT, "cost": cost, "latency_ms": lat,
			"success": success == 1, "created_at": created,
		})
	}
	return out
}

func boolStr(b bool) string { if b { return "true" }; return "false" }
func ifZero(s, fallback string) string { if s == "" { return fallback }; return s }

func main() {
	node := NewNodeService()
	err := wails.Run(&options.App{
		Title: "Token Flux Node", Width: 1200, Height: 800, MinWidth: 900, MinHeight: 600,
		AssetServer: &assetserver.Options{Assets: assets},
		Bind: []any{node},
	})
	if err != nil { log.Fatal(err) }
}
