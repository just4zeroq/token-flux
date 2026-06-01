package main

import (
	"embed"
	"log"
	"time"

	"ai-platform-node/internal/combo"
	"ai-platform-node/internal/db"
	"ai-platform-node/internal/keychain"
	"ai-platform-node/internal/router"
	"ai-platform-node/internal/rtk"
	"ai-platform-node/internal/server"
	"ai-platform-node/internal/tunnel"

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
		"running": true, "port": 20128,
		"uptime_sec":   int(time.Since(n.onlineAt).Seconds()),
		"models_count": len(n.rtr.Models()),
		"keys_count":   n.countKeys(),
		"online":       n.tunnel != nil,
	}
}

func (n *NodeService) Stop() string           { n.nodeSrv.Stop(); return "stopped" }
func (n *NodeService) countKeys() int          { keys, _ := keychain.ListKeys(); return len(keys) }
func (n *NodeService) AddKey(label, keyValue, channelID, baseURL string) map[string]any {
	e, err := keychain.AddKey(label, keyValue, channelID, baseURL)
	if err != nil { return map[string]any{"error": err.Error()} }
	return map[string]any{"id": e.ID, "label": e.Label, "channel_id": e.ChannelID, "status": e.Status}
}
func (n *NodeService) ListKeys() []map[string]any {
	keys, _ := keychain.ListKeys()
	if keys == nil { return []map[string]any{} }
	out := make([]map[string]any, len(keys))
	for i, k := range keys {
		out[i] = map[string]any{"id": k.ID, "label": k.Label, "channel_id": k.ChannelID, "base_url": k.BaseURL, "status": k.Status}
	}
	return out
}
func (n *NodeService) DeleteKey(id string) string { db.DB().Exec("DELETE FROM keys WHERE id = ?", id); return "deleted" }
func (n *NodeService) BindModel(keyID, modelCode, modelName string, shared bool) map[string]any {
	b, err := keychain.AddBinding(keyID, modelCode, modelName, shared)
	if err != nil { return map[string]any{"error": err.Error()} }
	if shared { n.rtr.Register(modelCode, b.KeyHash) }
	return map[string]any{"id": b.ID, "model_code": b.ModelCode, "key_hash": b.KeyHash[:16] + "...", "shared": b.Shared}
}
func (n *NodeService) UnbindModel(id string) string { db.DB().Exec("DELETE FROM bindings WHERE id = ?", id); return "unbound" }
func (n *NodeService) ListBindings() []map[string]any {
	rows, _ := db.DB().Query("SELECT id, model_code, model_name, key_hash, shared, created_at FROM bindings")
	if rows == nil { return []map[string]any{} }
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, mc, mn, kh string; var sd, ca int
		rows.Scan(&id, &mc, &mn, &kh, &sd, &ca)
		out = append(out, map[string]any{"id": id, "model_code": mc, "model_name": mn, "key_hash": kh[:16] + "...", "shared": sd == 1})
	}
	return out
}
func (n *NodeService) CreateCombo(name string, models []string, strategy string, sticky int) map[string]any {
	c, err := combo.Create(name, models, strategy, sticky)
	if err != nil { return map[string]any{"error": err.Error()} }
	return map[string]any{"name": c.Name, "models": c.Models, "strategy": c.Strategy, "sticky": c.Sticky}
}
func (n *NodeService) ListCombos() []map[string]any {
	combos, _ := combo.List()
	if combos == nil { return []map[string]any{} }
	out := make([]map[string]any, len(combos))
	for i, c := range combos { out[i] = map[string]any{"name": c.Name, "models": c.Models, "strategy": c.Strategy, "sticky": c.Sticky} }
	return out
}
func (n *NodeService) DeleteCombo(name string) string { combo.Delete(name); return "deleted" }
func (n *NodeService) SetRTK(enabled bool, mode string) string {
	n.rtkCfg = rtk.Config{Enabled: enabled, Mode: mode}
	db.SetConfig("rtk_enabled", boolStr(enabled))
	db.SetConfig("rtk_mode", mode)
	return "ok"
}
func (n *NodeService) SetCaveman(enabled bool) string { db.SetConfig("caveman_enabled", boolStr(enabled)); return "ok" }
func (n *NodeService) GetConfig() map[string]any {
	re, _ := db.GetConfig("rtk_enabled")
	rm, _ := db.GetConfig("rtk_mode")
	ce, _ := db.GetConfig("caveman_enabled")
	return map[string]any{"rtk_enabled": re == "true", "rtk_mode": ifZero(rm, "auto"), "caveman_enabled": ce == "true"}
}
func (n *NodeService) GetUsageStats() map[string]any {
	var tt, tc, ts int64
	db.DB().QueryRow("SELECT COALESCE(COUNT(*),0), COALESCE(SUM(tokens),0), COALESCE(SUM(cost_credits),0) FROM usage_log").Scan(&tc, &tt, &ts)
	rows, _ := db.DB().Query("SELECT model, COUNT(*), SUM(tokens), SUM(cost_credits) FROM usage_log GROUP BY model ORDER BY COUNT(*) DESC LIMIT 10")
	var bm []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() { var m string; var c, t, s int64; rows.Scan(&m, &c, &t, &s); bm = append(bm, map[string]any{"model": m, "calls": c, "tokens": t, "cost": s}) }
	}
	return map[string]any{"total_calls": tc, "total_tokens": tt, "total_cost": ts, "by_model": bm}
}
func (n *NodeService) GetUsageLogs(limit int) []map[string]any {
	if limit <= 0 || limit > 500 { limit = 50 }
	rows, _ := db.DB().Query("SELECT request_id, model, provider, tokens, latency_ms, success, created_at FROM usage_log ORDER BY id DESC LIMIT ?", limit)
	if rows == nil { return []map[string]any{} }
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var rid, m, p string; var t, l int64; var s, ca int
		rows.Scan(&rid, &m, &p, &t, &l, &s, &ca)
		out = append(out, map[string]any{"request_id": rid, "model": m, "provider": p, "tokens": t, "latency_ms": l, "success": s == 1, "created_at": ca})
	}
	return out
}
func boolStr(b bool) string { if b { return "true" }; return "false" }
func ifZero(s, f string) string { if s == "" { return f }; return s }

func main() {
	node := NewNodeService()
	err := wails.Run(&options.App{
		Title: "Token Flux Node", Width: 1200, Height: 800,
		MinWidth: 900, MinHeight: 600,
		AssetServer: &assetserver.Options{Assets: assets},
		Bind: []any{node},
	})
	if err != nil { log.Fatal(err) }
}
