package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"ai-platform-node/pkg/channel"
	"ai-platform-node/pkg/catalog"
	"ai-platform-node/pkg/combo"
	"ai-platform-node/pkg/db"
	"ai-platform-node/pkg/keychain"
	"ai-platform-node/pkg/modelspec"
	"ai-platform-node/pkg/router"
	"ai-platform-node/pkg/server"
	"ai-platform-node/pkg/tunnel"
	"ai-platform-node/pkg/usagetracker"
	"ai-platform-node/pkg/wallet"

	"ai-platform-node/pkg/attestation"

	"ai-platform/pkg/model"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"gopkg.in/yaml.v3"
)

// NodeVersion is the current node software version.
const NodeVersion = "0.1.0"

//go:embed frontend/dist
var assets embed.FS

// syncedFile wraps *os.File so every write is flushed to disk immediately.
type syncedFile struct{ *os.File }

func (w *syncedFile) Write(p []byte) (int, error) {
	n, err := w.File.Write(p)
	w.File.Sync()
	return n, err
}

// Log levels (numeric, higher = more severe)
const (
	levelDebug = 0
	levelInfo  = 1
	levelWarn  = 2
	levelError = 3
)

var levelNames = map[string]int{
	"debug": levelDebug,
	"info":  levelInfo,
	"warn":  levelWarn,
	"error": levelError,
}

// logLevel is the current minimum level for output (set during startup).
var logLevel = levelDebug

// l writes a formatted log line to the synced file with a level prefix.
func l(prefix, format string, v ...any) {
	log.Printf("["+prefix+"] "+format, v...)
}

// dl writes debug-level log lines — filtered by current logLevel.
func dl(format string, v ...any) {
	if logLevel <= levelDebug {
		l("D", format, v...)
	}
}

// Config holds desktop app configuration loaded from config.yaml.
type Config struct {
	PlatformURL string `yaml:"platform_url"`
	ListenAddr  string `yaml:"listen_addr"`
	LogLevel    string `yaml:"log_level"`
}

func defaultConfig() Config {
	return Config{
		PlatformURL: "http://localhost:8080",
		ListenAddr:  ":20128",
		LogLevel:    "debug",
	}
}

func fluxHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".flux"
	}
	return filepath.Join(home, ".flux")
}

func ensureDir(path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		log.Fatalf("[flux] create directory %s: %v", path, err)
	}
}

func loadConfig(path string) Config {
	cfg := defaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[config] no config file at %s, using defaults: %v", path, err)
		return cfg
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Printf("[config] failed to parse %s: %v, using defaults", path, err)
		return defaultConfig()
	}
	log.Printf("[config] loaded from %s: platform_url=%s listen_addr=%s log_level=%s", path, cfg.PlatformURL, cfg.ListenAddr, cfg.LogLevel)
	return cfg
}

func configPath(fluxDir string) string {
	return filepath.Join(fluxDir, "config.yaml")
}

func writeDefaultConfig(path string, cfg Config) {
	if _, err := os.Stat(path); err == nil {
		return
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		log.Printf("[config] failed to marshal default config: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("[config] failed to write %s: %v", path, err)
		return
	}
	log.Printf("[config] wrote default config to %s", path)
}

type NodeService struct {
	nodeSrv     *server.Server
	rtr         *router.Router
	onlineAt    time.Time
	platformURL string
	logLevel    string
	configPath  string
	httpClient  *http.Client
	fluxDir     string
	wallet      *wallet.Wallet
	tunnel      *tunnel.Tunnel
	nodeName    string
}

func NewNodeService(cfg Config, fluxDir string) *NodeService {
	dbPath := filepath.Join(fluxDir, "node.db")
	if err := db.Open(dbPath); err != nil {
		log.Fatalf("[flux] open db %s: %v", dbPath, err)
	}
	r := router.New()
	bindings, _ := keychain.SharedBindings()
	for _, b := range bindings {
		r.Register(b.ModelCode, b.KeyHash)
	}
	srv := server.New(r)
	go srv.Start(cfg.ListenAddr)

	// Start platform catalog sync (every 10 min).
	go startSync(cfg.PlatformURL)

	// Load wallet from DB if exists (non-fatal if missing).
	var w *wallet.Wallet
	if wallet.HasActiveWallet(db.DB()) {
		if wl, err := wallet.LoadFromDB(db.DB()); err == nil {
			w = wl
			l("I", "wallet loaded address=%s host=%s", wl.Address(), wallet.HostInfo())
		} else {
			l("W", "load wallet from DB failed: %v", err)
		}
	}

	return &NodeService{
		nodeSrv:     srv,
		rtr:         r,
		onlineAt:    time.Now(),
		platformURL: cfg.PlatformURL,
		logLevel:    cfg.LogLevel,
		configPath:  filepath.Join(fluxDir, "config.yaml"),
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		fluxDir:     fluxDir,
		wallet:      w,
	}
}

// ---- Node status ----

func (n *NodeService) Status() map[string]any {
	online := false
	if n.tunnel != nil {
		st := n.tunnel.GetStatus()
		online = st.Connected
	}
	return map[string]any{
		"running": true, "port": 20128,
		"uptime_sec":   int(time.Since(n.onlineAt).Seconds()),
		"models_count": len(n.rtr.Models()),
		"keys_count":   n.countKeys(),
		"online":       online,
	}
}
func (n *NodeService) Stop() string { n.nodeSrv.Stop(); return "stopped" }
func (n *NodeService) countKeys() int {
	keys, _ := keychain.ListKeys()
	return len(keys)
}

// ---- Diagnostics ----

// Ping tests basic Wails runtime connectivity.
func (n *NodeService) Ping() string {
	l("I", "pong")
	return "pong"
}

// ---- Platform config ----

func (n *NodeService) GetPlatformURL() string {
	return n.platformURL
}

// ---- Upstream API keys (local DB) ----

func (n *NodeService) AddKey(name, keyValue, baseURL string, channelID int64) map[string]any {
	e, err := keychain.AddKey(name, keyValue, baseURL, channelID)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return map[string]any{"id": e.ID, "name": e.Name, "channel_id": e.ChannelID, "status": e.Status}
}

func (n *NodeService) ListKeys() []map[string]any {
	keys, _ := keychain.ListKeys()
	if keys == nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, len(keys))
	for i, k := range keys {
		out[i] = map[string]any{
			"id": k.ID, "name": k.Name, "key_masked": k.KeyMasked,
			"channel_id": k.ChannelID, "base_url": k.BaseURL, "status": k.Status,
		}
	}
	return out
}

func (n *NodeService) DeleteKey(id int64) string {
	db.DB().Exec("DELETE FROM llm_model_keys WHERE id = ?", id)
	db.DB().Exec("DELETE FROM llm_model_key_models WHERE model_key_id = ?", id)
	return "deleted"
}

// ---- Key-Model bindings ----

func (n *NodeService) BindModel(keyID int64, modelCode, upstreamModelName string, shared bool) map[string]any {
	b, err := keychain.AddBinding(keyID, modelCode, upstreamModelName, 0, shared, 0, 1)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	if shared {
		n.rtr.Register(modelCode, b.KeyHash)
	}
	return map[string]any{
		"id": b.ID, "model_code": b.ModelCode,
		"upstream_model_name": b.UpstreamModelName,
		"key_hash":            b.KeyHash[:16] + "...",
		"shared":              b.Shared,
	}
}

func (n *NodeService) UnbindModel(id int64) string {
	db.DB().Exec("DELETE FROM llm_model_key_models WHERE id = ?", id)
	return "unbound"
}

func (n *NodeService) ListBindings() []map[string]any {
	rows, _ := db.DB().Query(
		`SELECT b.id, b.model_key_id, b.model_spec_id, b.upstream_model_name,
		 b.model_code, b.key_hash, b.priority, b.weight, b.status, b.last_error, b.created_at,
		 k.name, k.key_masked, k.base_url
		 FROM llm_model_key_models b
		 LEFT JOIN llm_model_keys k ON b.model_key_id = k.id`)
	if rows == nil {
		return []map[string]any{}
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, mkid, msid, p, w, ca int64
		var umn, mc, kh, st, le, kn, km, bu string
		rows.Scan(&id, &mkid, &msid, &umn, &mc, &kh, &p, &w, &st, &le, &ca, &kn, &km, &bu)
		khDisplay := kh
		if len(kh) > 16 {
			khDisplay = kh[:16] + "..."
		}
		out = append(out, map[string]any{
			"id": id, "model_key_id": mkid, "model_spec_id": msid,
			"upstream_model_name": umn, "model_code": mc,
			"key_hash":            khDisplay, "priority": p, "weight": w,
			"status": st, "last_error": le,
			"key_name": kn, "key_masked": km, "base_url": bu,
		})
	}
	return out
}

// ---- Combos ----

func (n *NodeService) CreateCombo(name string, models []string, strategy string, sticky int) map[string]any {
	c, err := combo.Create(name, models, strategy, sticky)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return map[string]any{"name": c.Name, "models": c.Models, "strategy": c.Strategy, "sticky": c.Sticky}
}
func (n *NodeService) ListCombos() []map[string]any {
	combos, _ := combo.List()
	if combos == nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, len(combos))
	for i, c := range combos {
		out[i] = map[string]any{"name": c.Name, "models": c.Models, "strategy": c.Strategy, "sticky": c.Sticky}
	}
	return out
}
func (n *NodeService) DeleteCombo(name string) string { combo.Delete(name); return "deleted" }

// ---- Local channels CRUD ----

func (n *NodeService) ListChannels() []map[string]any {
	list, err := channel.List()
	if err != nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, len(list))
	for i, c := range list {
		out[i] = map[string]any{
			"id":            c.ID,
			"code":          c.Code,
			"name":          c.Name,
			"description":   c.Description,
			"provider_type": c.ProviderType,
			"protocols":     c.Protocols,
			"status":        c.Status,
			"source":        c.Source,
			"created_at":    c.CreatedAt,
			"updated_at":    c.UpdatedAt,
		}
	}
	return out
}

func (n *NodeService) ListChannelModels(channelID int64) []map[string]any {
	list, err := channel.ListChannelModels(channelID)
	if err != nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, len(list))
	for i, b := range list {
		out[i] = map[string]any{
			"id":                b.ID,
			"channel_id":        b.ChannelID,
			"model_spec_id":     b.ModelSpecID,
			"upstream_model_name": b.UpstreamModelName,
			"model_name":        b.ModelName,
			"model_code":        b.ModelCode,
			"developer_name":    b.DeveloperName,
			"source":            b.Source,
			"created_at":        b.CreatedAt,
		}
	}
	return out
}

func (n *NodeService) CreateChannel(code, name, description string, providerType int, protocols []any) map[string]any {
	entries, err := decodeProtocolEntries(protocols)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	c, err := channel.Create(code, name, description, model.ProviderType(providerType), entries)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return map[string]any{
		"id": c.ID, "code": c.Code, "name": c.Name,
		"description": c.Description, "provider_type": c.ProviderType,
		"protocols": c.Protocols, "status": c.Status,
	}
}

func (n *NodeService) UpdateChannel(id int64, name string, protocols []any, status string) map[string]any {
	entries, err := decodeProtocolEntries(protocols)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	c, err := channel.Update(id, name, entries, status)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return map[string]any{
		"id": c.ID, "code": c.Code, "name": c.Name,
		"protocols": c.Protocols, "status": c.Status,
	}
}

func (n *NodeService) DeleteChannel(id int64) string {
	if err := channel.Delete(id); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "deleted"
}


func (n *NodeService) BindChannelModels(channelID int64, modelIDs []int64) string {
	if err := channel.BindChannelModels(channelID, modelIDs); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

func (n *NodeService) UnbindChannelModel(bindingID int64) string {
	if err := channel.UnbindChannelModel(bindingID); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

func decodeProtocolEntries(raw []any) ([]model.ProtocolEntry, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var entries []model.ProtocolEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// ---- Local model specs CRUD ----

func (n *NodeService) ListModelSpecs() []map[string]any {
	list, err := modelspec.List()
	if err != nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, len(list))
	for i, s := range list {
		out[i] = map[string]any{
			"id":                 s.ID,
			"developer_name":     s.DeveloperName,
			"model_name":         s.ModelName,
			"model_code":         s.ModelCode,
			"display_name":       s.DisplayName,
			"model_family":       s.ModelFamily,
			"description":        s.Description,
			"capabilities":       s.Capabilities,
			"context_window":     s.ContextWindow,
			"max_input_tokens":   s.MaxInputTokens,
			"max_output_tokens":  s.MaxOutputTokens,
			"supports_stream":    s.SupportsStream,
			"supports_tools":     s.SupportsTools,
			"supports_vision":    s.SupportsVision,
			"supports_json_mode": s.SupportsJsonMode,
			"supports_reasoning": s.SupportsReasoning,
			"supports_logprobs":  s.SupportsLogprobs,
			"status":             s.Status,
				"source":             s.Source,
		}
	}
	return out
}

func (n *NodeService) CreateModelSpec(spec map[string]any) map[string]any {
	devName, _ := spec["developer_name"].(string)
	modelName, _ := spec["model_name"].(string)
	modelCode, _ := spec["model_code"].(string)
	displayName, _ := spec["display_name"].(string)
	modelFamily, _ := spec["model_family"].(string)
	desc, _ := spec["description"].(string)
	capsJSON, _ := spec["capabilities_json"].(string)
	ctxWin, _ := toInt(spec["context_window"])
	maxIn, _ := toInt(spec["max_input_tokens"])
	maxOut, _ := toInt(spec["max_output_tokens"])
	s, _ := toBool(spec["supports_stream"])
	t, _ := toBool(spec["supports_tools"])
	v, _ := toBool(spec["supports_vision"])
	j, _ := toBool(spec["supports_json_mode"])
	r, _ := toBool(spec["supports_reasoning"])
	l, _ := toBool(spec["supports_logprobs"])

	specObj, err := modelspec.Create(devName, modelName, modelCode, displayName, modelFamily, desc, capsJSON, ctxWin, maxIn, maxOut, s, t, v, j, r, l)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return map[string]any{
		"id": specObj.ID, "model_code": specObj.ModelCode,
		"model_name": specObj.ModelName, "status": specObj.Status,
	}
}

func (n *NodeService) DeleteModelSpec(id int64) string {
	if err := modelspec.Delete(id); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "deleted"
}

func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case float64:
		return int(x), true
	case int64:
		return int(x), true
	}
	return 0, false
}

func toBool(v any) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case int:
		return x == 1, true
	case int64:
		return x == 1, true
	case float64:
		return x == 1, true
	}
	return false, false
}

// ---- Node API keys (local gateway keys) ----

func (n *NodeService) ListNodeApiKeys() []map[string]any {
	rows, err := db.DB().Query("SELECT id, key_prefix, key_value, label, status, last_used_at, created_at FROM node_api_keys ORDER BY created_at DESC")
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	var keys []map[string]any
	for rows.Next() {
		var id int64
		var prefix, keyVal, label, status string
		var lastUsed, createdAt int64
		rows.Scan(&id, &prefix, &keyVal, &label, &status, &lastUsed, &createdAt)
		keys = append(keys, map[string]any{
			"id": id, "key_prefix": prefix, "key": keyVal,
			"label": label, "status": status,
			"last_used_at": lastUsed, "created_at": createdAt,
		})
	}
	if keys == nil {
		keys = []map[string]any{}
	}
	return keys
}

func (n *NodeService) CreateNodeApiKey(label string) map[string]any {
	key, err := keychain.GenerateAPIKey(label)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return map[string]any{
		"id": key.ID, "key": key.Key,
		"key_prefix": key.KeyPrefix, "label": key.Label, "status": key.Status,
	}
}

func (n *NodeService) DeleteNodeApiKey(id int64) string {
	db.DB().Exec("DELETE FROM node_api_keys WHERE id = ?", id)
	return "deleted"
}

// ---- Platform catalog fetch (backend-encapsulated HTTP) ----

func (n *NodeService) FetchCatalogProviders() []map[string]any {
	var result []map[string]any
	n.fetchFromPlatform("/api/v1/catalog/providers", &result)
	return result
}

func (n *NodeService) FetchCatalogModels() []map[string]any {
	var result []map[string]any
	n.fetchFromPlatform("/api/v1/catalog/models", &result)
	return result
}

func (n *NodeService) FetchCatalogChannels() []map[string]any {
	var result []map[string]any
	n.fetchFromPlatform("/api/v1/catalog/channels", &result)
	return result
}

func (n *NodeService) fetchFromPlatform(path string, out *[]map[string]any) {
	url := n.platformURL + path
	dl("platform GET %s", url)

	resp, err := n.httpClient.Get(url)
	if err != nil {
		l("E", "platform GET %s failed: %v", url, err)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		l("E", "platform GET %s read body failed: %v", url, err)
		return
	}
	l("I", "platform GET %s status=%d body=%s", url, resp.StatusCode, truncate(string(body), 500))

	if resp.StatusCode != http.StatusOK {
		return
	}
	// gfast-style: {code:0, data:{list:[...]}}
	type envData struct {
		List []map[string]any `json:"list"`
	}
	var envelope struct {
		Code    int     `json:"code"`
		Message string  `json:"message"`
		Data    envData `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Code == 0 {
		if envelope.Data.List != nil {
			*out = envelope.Data.List
		} else {
			*out = []map[string]any{}
		}
		l("I", "platform GET %s: parsed %d items", url, len(*out))
		return
	}
	// Fallback: direct array
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		*out = arr
		l("I", "platform GET %s: parsed array %d items", url, len(arr))
		return
	}
	// Fallback: {data:[...]}
	var dataWrap struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &dataWrap); err == nil {
		*out = dataWrap.Data
		l("I", "platform GET %s: parsed data-wrap %d items", url, len(*out))
		return
	}
	l("I", "platform GET %s: unparseable response", url)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func (n *NodeService) GetConfig() map[string]any {
		return map[string]any{
		"platform_url":    n.platformURL,
		"log_level":       n.logLevel,
		"config_path":     n.configPath,
	}
}

// SaveConfig persists config.yaml and updates NodeService in-memory fields.
func (n *NodeService) SaveConfig(cfg map[string]any) string {
	if v, ok := cfg["platform_url"].(string); ok && v != "" {
		n.platformURL = v
	}
	if v, ok := cfg["log_level"].(string); ok && v != "" {
		n.logLevel = v
	}
	yamlCfg := Config{
		PlatformURL: n.platformURL,
		ListenAddr:  ":20128",
		LogLevel:    n.logLevel,
	}
	data, err := yaml.Marshal(yamlCfg)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if err := os.WriteFile(n.configPath, data, 0644); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	l("I", "config saved platform_url=%s log_level=%s", n.platformURL, n.logLevel)
	return "ok"
}

// ---- Usage stats ----

func (n *NodeService) GetUsageStats() map[string]any {
	var tt, tc, ts int64
	db.DB().QueryRow(
		"SELECT COALESCE(COUNT(*),0), COALESCE(SUM(total_tokens),0), COALESCE(SUM(cost_credits),0) FROM llm_usage_records",
	).Scan(&tc, &tt, &ts)
	rows, _ := db.DB().Query(
		"SELECT capability, COUNT(*), SUM(total_tokens), SUM(cost_credits) FROM llm_usage_records GROUP BY capability ORDER BY COUNT(*) DESC LIMIT 10")
	var bm []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var m string
			var c, t, s int64
			rows.Scan(&m, &c, &t, &s)
			bm = append(bm, map[string]any{"capability": m, "calls": c, "tokens": t, "cost": s})
		}
	}
	return map[string]any{"total_calls": tc, "total_tokens": tt, "total_cost": ts, "by_model": bm}
}

func (n *NodeService) GetUsageLogs(limit int) []map[string]any {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, _ := db.DB().Query(
		`SELECT request_id, capability, total_tokens, latency_ms, status, created_at
		 FROM llm_usage_records ORDER BY id DESC LIMIT ?`, limit)
	if rows == nil {
		return []map[string]any{}
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var rid, cap, st string
		var t, l, ca int64
		rows.Scan(&rid, &cap, &t, &l, &st, &ca)
		out = append(out, map[string]any{
			"request_id": rid, "capability": cap, "tokens": t,
			"latency_ms": l, "status": st, "created_at": ca,
		})
	}
	return out
}

// ---- Wallet ----

func (n *NodeService) WalletStatus() map[string]any {
	hasActive := wallet.HasActiveWallet(db.DB())
	m := map[string]any{
		"has_wallet":      hasActive,
		"wallet_name":     "",
		"wallet_address":  "",
		"wallet_status":   "none",
		"host":            wallet.HostInfo(),
	}
	if n.wallet != nil {
		m["wallet_address"] = n.wallet.Address()
		m["wallet_name"] = n.wallet.Name
		m["wallet_status"] = n.wallet.Status
	}
	reg, _ := db.GetConfig("registered")
	m["registered"] = reg == "true"
	return m
}

func (n *NodeService) SetupWallet() map[string]any {
	return n.CreateWallet("")
}

func (n *NodeService) CreateWallet(name string) map[string]any {
	if n.wallet != nil {
		return map[string]any{"error": "wallet already loaded", "wallet_address": n.wallet.Address()}
	}
	w, err := wallet.Generate()
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	w.Name = name
	if w.Name == "" {
		w.Name = "default"
	}
	if err := w.Save(db.DB()); err != nil {
		return map[string]any{"error": err.Error()}
	}
	n.wallet = w
	l("I", "wallet created address=%s name=%s", w.Address(), w.Name)
	return map[string]any{
		"has_wallet":     true,
		"wallet_address": w.Address(),
		"wallet_name":    w.Name,
		"seed_hex":       w.SeedHex(),
		"host":           wallet.HostInfo(),
		"backup_warning": "SAVE THIS SEED — it controls your node and earnings. Lost seed = lost access forever.",
	}
}

// ExportSeed returns the wallet seed hex for backup (only if wallet is loaded).
func (n *NodeService) ExportSeed() map[string]any {
	if n.wallet == nil {
		return map[string]any{"error": "no active wallet"}
	}
	return map[string]any{
		"seed_hex":       n.wallet.SeedHex(),
		"wallet_address": n.wallet.Address(),
		"backup_warning": "SAVE THIS SEED — it controls your node and earnings. Lost seed = lost access forever.",
	}
}

// registerOnPlatform registers the wallet on platform via POST /api/v1/nodes/register.
// Returns empty on success, error message on failure.
func (n *NodeService) registerOnPlatform() string {
	addr := n.wallet.Address()
	msg := "flux-node-register-" + addr
	sig := hex.EncodeToString(n.wallet.Sign([]byte(msg)))
	regBody := map[string]any{
		"wallet_address": addr,
		"signature":      sig,
		"name":           n.nodeName,
	}
	regJSON, _ := json.Marshal(regBody)

	url := n.platformURL + "/api/v1/nodes/register"
	resp, err := n.httpClient.Post(url, "application/json", bytes.NewReader(regJSON))
	if err != nil {
		return "error: register request failed: " + err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Sprintf("error: register status=%d body=%s", resp.StatusCode, truncate(string(body), 200))
	}

	db.SetConfig("registered", "true")
	db.SetConfig("network.auto.join", "true")
	l("I", "registered on platform %s wallet=%s name=%s", n.platformURL, addr, n.nodeName)
	return ""
}

// ---- Network ----

func (n *NodeService) JoinNetwork(name string) string {
	if n.wallet == nil {
		return "error: no active wallet, create wallet first"
	}
	if name != "" {
		n.nodeName = name
	}
	if n.nodeName == "" {
		n.nodeName = n.wallet.Name
	}

	// Register on platform.
	if r := n.registerOnPlatform(); r != "" {
		return r
	}

	// Binary attestation — platform decides if it's needed.
	if ok, errMsg := n.doAttestation(); !ok {
		l("E", "attestation failed: %s", errMsg)
		return "error: " + errMsg
	}

	// Connect tunnel.
	return n.connectTunnel()
}

// doAttestation runs binary integrity verification against the platform.
// Returns (true, "") on success or (false, errorMsg) on failure.
func (n *NodeService) doAttestation() (bool, string) {
	exePath, err := os.Executable()
	if err != nil {
		return false, "cannot determine binary path: " + err.Error()
	}

	binarySize, err := attestation.SelfSize(exePath)
	if err != nil {
		return false, "read binary size: " + err.Error()
	}

	addr := n.wallet.Address()

	// Step 1: Request challenge.
	sigMsg := fmt.Sprintf("attest-%s-%d-%s", NodeVersion, binarySize, addr)
	sig := hex.EncodeToString(n.wallet.Sign([]byte(sigMsg)))
	challengeBody := map[string]any{
		"version":        NodeVersion,
		"binary_size":    binarySize,
		"wallet_address": addr,
		"signature":      sig,
	}
	body, _ := json.Marshal(challengeBody)

	url := n.platformURL + "/api/v1/nodes/attest/challenge"
	resp, err := n.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return false, "attest challenge request: " + err.Error()
	}
	defer resp.Body.Close()

	var challResult struct {
		Code int `json:"code"`
		Data struct {
			Disabled    bool       `json:"disabled,omitempty"`
			ChallengeID string     `json:"challenge_id,omitempty"`
			Nonce       string     `json:"nonce,omitempty"`
			Offsets     [][2]int   `json:"offsets,omitempty"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&challResult)
	if challResult.Code != 0 {
		return false, fmt.Sprintf("attest challenge failed: code=%d", challResult.Code)
	}

	// Attestation disabled on platform — skip proof.
	if challResult.Data.Disabled {
		l("I", "attestation disabled on platform, skipping")
		return true, ""
	}

	// Step 2: Compute proof from local binary.
	challenge := &attestation.Challenge{
		Version: NodeVersion,
		Nonce:   challResult.Data.Nonce,
		Offsets: challResult.Data.Offsets,
	}
	proof, err := attestation.ComputeProof(exePath, challenge)
	if err != nil {
		return false, "compute proof: " + err.Error()
	}

	// Step 3: Submit proof for verification.
	proofSig := fmt.Sprintf("attest-proof-%s-%s", challResult.Data.ChallengeID, proof.HMAC)
	proofSigHex := hex.EncodeToString(n.wallet.Sign([]byte(proofSig)))
	verifyBody := map[string]any{
		"challenge_id":   challResult.Data.ChallengeID,
		"hmac":           proof.HMAC,
		"wallet_address": addr,
		"signature":      proofSigHex,
	}
	vBody, _ := json.Marshal(verifyBody)

	url2 := n.platformURL + "/api/v1/nodes/attest/verify"
	vResp, err := n.httpClient.Post(url2, "application/json", bytes.NewReader(vBody))
	if err != nil {
		return false, "attest verify request: " + err.Error()
	}
	defer vResp.Body.Close()

	var verifyResult struct {
		Code int `json:"code"`
		Data struct {
			Verified bool `json:"verified"`
		} `json:"data"`
	}
	json.NewDecoder(vResp.Body).Decode(&verifyResult)
	if verifyResult.Code != 0 || !verifyResult.Data.Verified {
		return false, "binary attestation failed — binary may be modified"
	}

	l("I", "attestation passed version=%s size=%d", NodeVersion, binarySize)
	return true, ""
}

func (n *NodeService) connectTunnel() string {
	n.tunnel = tunnel.NewClient(n.platformURL, n.wallet, n.rtr, n.nodeName, NodeVersion)
	ctx := context.Background()
	if err := n.tunnel.Connect(ctx); err != nil {
		l("E", "tunnel connect failed: %v", err)
		return "error: tunnel connect: " + err.Error()
	}
	go n.tunnel.ConnectWithRetry(ctx)
	return "ok"
}

func (n *NodeService) ConnectToNetwork() string {
	if n.wallet == nil {
		return "error: no active wallet"
	}
	if n.tunnel != nil {
		st := n.tunnel.GetStatus()
		if st.Connected {
			return "already connected"
		}
	}
	return n.connectTunnel()
}

func (n *NodeService) DisconnectFromNetwork() string {
	if n.tunnel != nil {
		n.tunnel.Stop()
		n.tunnel = nil
	}
	l("I", "disconnected from network (wallet kept)")
	return "ok"
}

func (n *NodeService) LeaveNetwork() string {
	if n.tunnel != nil {
		n.tunnel.Stop()
		n.tunnel = nil
	}
	wallet.MarkInactive(db.DB())
	n.wallet = nil
	db.SetConfig("registered", "false")
	l("I", "left network — wallet marked inactive")
	return "ok"
}

func (n *NodeService) NetworkStatus() map[string]any {
	m := map[string]any{
		"connected":    false,
		"has_wallet":   wallet.HasActiveWallet(db.DB()),
		"auto_join":    false,
	}
	aj, _ := db.GetConfig("network.auto.join")
	m["auto_join"] = aj == "true"

	if n.wallet != nil {
		m["wallet_address"] = n.wallet.Address()
		m["wallet_name"] = n.wallet.Name
	}
	if n.nodeName != "" {
		m["node_name"] = n.nodeName
	}
	if n.tunnel != nil {
		st := n.tunnel.GetStatus()
		m["connected"] = st.Connected
		m["uptime_sec"] = st.Uptime
		m["platform"] = st.Platform
		m["peer_count"] = st.PeerCount
	}
	m["shared_count"] = len(n.rtr.Models())

	// Token totals
	if tt, err := usagetracker.TokenSummary(); err == nil {
		m["input_tokens"] = tt.InputTokens
		m["output_tokens"] = tt.OutputTokens
	}
	return m
}

func (n *NodeService) FetchPeerCount() int {
	url := n.platformURL + "/api/v1/nodes/count"
	resp, err := n.httpClient.Get(url)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var r struct {
		Count int `json:"count"`
	}
	json.NewDecoder(resp.Body).Decode(&r)
	return r.Count
}

func (n *NodeService) GetTunnelUsage() map[string]any {
	var totalReqs, totalTokens int64
	db.DB().QueryRow(
		"SELECT COALESCE(COUNT(*),0), COALESCE(SUM(total_tokens),0) FROM llm_usage_records WHERE source = 'tunnel'",
	).Scan(&totalReqs, &totalTokens)

	rows, err := db.DB().Query(
		"SELECT ms.model_name, ms.model_code, COUNT(*), SUM(r.total_tokens)" +
		" FROM llm_usage_records r" +
		" LEFT JOIN llm_model_key_models mk ON r.model_key_id = mk.model_key_id" +
		" LEFT JOIN llm_model_specs ms ON mk.model_spec_id = ms.id" +
		" WHERE r.source = 'tunnel'" +
		" GROUP BY ms.model_name ORDER BY COUNT(*) DESC LIMIT 20")
	var breakdown []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var name, code string
			var reqs, toks int64
			if err := rows.Scan(&name, &code, &reqs, &toks); err == nil {
				breakdown = append(breakdown, map[string]any{
					"model_name":  name,
					"model_code":  code,
					"requests":    reqs,
					"total_tokens": toks,
				})
			}
		}
	}
	if err != nil || breakdown == nil {
		breakdown = []map[string]any{}
	}

	return map[string]any{
		"total_requests": totalReqs,
		"total_tokens":  totalTokens,
		"model_breakdown": breakdown,
	}
}
func (n *NodeService) GetRecentRequests(limit int) []map[string]any {
	records, err := usagetracker.RecentRequests(limit)
	if err != nil {
		l("E", "get recent requests: %v", err)
		return []map[string]any{}
	}
	out := make([]map[string]any, len(records))
	for i, r := range records {
		out[i] = map[string]any{
			"request_id":     r.RequestID,
			"model_code":     r.ModelCode,
			"model_name":     r.ModelName,
			"upstream_model": r.UpstreamModel,
			"input_tokens":   r.InputTokens,
			"output_tokens":  r.OutputTokens,
			"total_tokens":   r.TotalTokens,
			"status":         r.Status,
			"source":         r.Source,
			"created_at":     r.CreatedAt,
		}
	}
	return out
}


func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
func ifZero(s, f string) string {
	if s == "" {
		return f
	}
	return s
}

// startSync runs a periodic platform catalog sync in a background goroutine.
func startSync(platformURL string) {
	// Update catalog package default URL for SyncAll.
	db.SetConfig("platform_url", platformURL)
	l("I", "catalog sync started interval=10m url=%s", platformURL)

	// Run an initial sync immediately.
	if err := catalog.SyncAll(); err != nil {
		l("W", "catalog initial sync failed: %v", err)
	} else {
		l("I", "catalog initial sync complete")
	}

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if err := catalog.SyncAll(); err != nil {
			l("W", "catalog sync failed: %v", err)
		} else {
			l("I", "catalog sync complete")
		}
	}
}

func main() {
	fluxDir := fluxHome()
	ensureDir(fluxDir)

	cfg := loadConfig(configPath(fluxDir))

	// Open log file and redirect Go's standard logger to it.
	logPath := filepath.Join(fluxDir, "desktop.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("[flux] failed to open log %s, falling back to stderr: %v", logPath, err)
	} else {
		f.WriteString(time.Now().Format(time.StampMilli) + " [I] flux boot: opening log\n")
		log.SetOutput(&syncedFile{f})
	}
	logLevel = levelNames[cfg.LogLevel]
	l("I", "flux started dir=%s log_level=%s", fluxDir, cfg.LogLevel)

	writeDefaultConfig(configPath(fluxDir), cfg)
	node := NewNodeService(cfg, fluxDir)

	err = wails.Run(&options.App{
		Title:             "Token Flux Node",
		Width:             1200,
		Height:            800,
		MinWidth:          900,
		MinHeight:         600,
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Bind: []any{node},
	})
	if err != nil {
		log.Fatal(err)
	}
}
