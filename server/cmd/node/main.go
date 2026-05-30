// Go local node for ai-platform.
//
// Deployed by suppliers to expose their API keys to the platform
// through a WebSocket tunnel. Handles local routing, multi-account
// fallback, and format translation.
//
// Usage:
//
//	node [--config config.yaml]
//
// The node starts a local HTTP server on :20129 for the admin API
// and an OpenAI-compatible endpoint on :20128 for direct local use.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ai-platform/cmd/node/internal/db"
	"ai-platform/cmd/node/internal/keychain"
	"ai-platform/cmd/node/internal/router"
	"ai-platform/cmd/node/internal/server"
	"ai-platform/cmd/node/internal/tunnel"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	_ = configPath // config file parsing omitted for v1

	log.SetFlags(log.Ltime)
	log.Println("[node] starting ai-platform local node")

	// DB
	if err := db.Open("node.db"); err != nil {
		log.Fatalf("[node] db open: %v", err)
	}
	log.Println("[node] sqlite opened")

	// Router
	r := router.New()

	// Register local shared bindings at startup
	bindings, _ := keychain.SharedBindings()
	for _, b := range bindings {
		r.Register(b.ModelCode, b.KeyHash)
		log.Printf("[node] registered local binding: %s -> %s", b.ModelCode, b.KeyHash[:16]+"...")
	}

	// HTTP Server
	srv := server.New(r)
	go func() {
		if err := srv.Start(":20128"); err != nil {
			log.Printf("[node] http server: %v", err)
		}
	}()

	// Tunnel: connect to platform if configured
	nodeID, _ := db.GetConfig("node_id")
	platformURL, _ := db.GetConfig("platform_url")
	nodeSecret, _ := db.GetConfig("node_secret")

	if nodeID != "" && platformURL != "" {
		client := tunnel.NewClient(platformURL, nodeID, nodeSecret, r)
		go func() {
			if err := client.Connect(context.Background()); err != nil {
				log.Printf("[node] platform connect: %v", err)
			}
		}()
	} else {
		log.Println("[node] no platform config — running in standalone mode")
	}

	// Wait for shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("[node] shutting down")
	srv.Stop()
}
