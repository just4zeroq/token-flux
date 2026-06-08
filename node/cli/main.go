package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"ai-platform-node/pkg/db"
	"ai-platform-node/pkg/keychain"
	"ai-platform-node/pkg/router"
	"ai-platform-node/pkg/server"
)

const (
	version = "0.1.0"
	apiBase = "http://localhost:20128"
)

var rootCmd = &cobra.Command{
	Use:   "node",
	Short: "Token Flux Node — local AI gateway",
	RunE:  runStart,
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the node server",
	RunE:  runStart,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show node server status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return apiGet("/api/node/status")
	},
}

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "List upstream API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		return apiGet("/api/node/keys")
	},
}

var channelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "List provider channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		return apiGet("/api/node/channels")
	},
}

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show usage statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		return apiGet("/api/node/usage")
	},
}

var apiKeyCmd = &cobra.Command{
	Use:   "api-key",
	Short: "Manage gateway API keys",
}

var apiKeyGenCmd = &cobra.Command{
	Use:   "generate [label]",
	Short: "Generate a new gateway API key",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		label := ""
		if len(args) > 0 {
			label = args[0]
		}
		return apiPost("/api/node/api-keys", map[string]string{"label": label})
	},
}

var apiKeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List gateway API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		return apiGet("/api/node/api-keys")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Token Flux Node v%s\n", version)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd, statusCmd, keysCmd, channelsCmd, usageCmd, apiKeyCmd, versionCmd)
	apiKeyCmd.AddCommand(apiKeyGenCmd, apiKeyListCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runStart(cmd *cobra.Command, args []string) error {
	log.SetFlags(log.Ltime)
	log.Println("[node] starting Token Flux Node v" + version)

	if err := db.Open("node.db"); err != nil {
		return fmt.Errorf("db open: %w", err)
	}
	log.Println("[node] sqlite opened")

	r := router.New()
	bindings, _ := keychain.SharedBindings()
	for _, b := range bindings {
		r.Register(b.ModelCode, b.KeyHash)
		log.Printf("[node] registered: %s", b.ModelCode)
	}

	srv := server.New(r)
	go func() {
		if err := srv.Start(":20128"); err != nil {
			log.Printf("[node] http server: %v", err)
		}
	}()
	log.Println("[node] listening on :20128")

	// Platform tunnel not available in CLI mode (use desktop app for network join).

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("[node] shutting down")
	srv.Stop()
	return nil
}

// ---- HTTP helpers for CLI subcommands ----

func apiGet(path string) error {
	resp, err := http.Get(apiBase + path)
	if err != nil {
		return fmt.Errorf("connect to node (is it running on %s?): %w", apiBase, err)
	}
	defer resp.Body.Close()

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	b, _ := json.MarshalIndent(raw, "", "  ")
	fmt.Println(string(b))
	return nil
}

func apiPost(path string, body map[string]string) error {
	b, _ := json.Marshal(body)
	resp, err := http.Post(apiBase+path, "application/json", strings.NewReader(string(b)))
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer resp.Body.Close()

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	// If generated key returned, show prominently.
	if key, ok := raw["key"]; ok {
		fmt.Println("")
		fmt.Println("  ┌─────────────────────────────────────────────────────────────┐")
		fmt.Println("  │  NEW API KEY — copy this now, it won't be shown again!      │")
		fmt.Println("  └─────────────────────────────────────────────────────────────┘")
		fmt.Printf("\n  %s\n\n", key)
		return nil
	}

	b, _ = json.MarshalIndent(raw, "", "  ")
	fmt.Println(string(b))
	return nil
}
