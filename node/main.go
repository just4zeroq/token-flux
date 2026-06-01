// Go local node for ai-platform.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ai-platform-node/pkg/db"
	"ai-platform-node/pkg/keychain"
	"ai-platform-node/pkg/router"
	"ai-platform-node/pkg/server"
	"ai-platform-node/pkg/tunnel"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()
	_ = configPath

	log.SetFlags(log.Ltime)
	log.Println("[node] starting ai-platform local node")

	if err := db.Open("node.db"); err != nil {
		log.Fatalf("[node] db open: %v", err)
	}
	log.Println("[node] sqlite opened")

	r := router.New()
	bindings, _ := keychain.SharedBindings()
	for _, b := range bindings {
		r.Register(b.ModelCode, b.KeyHash)
		log.Printf("[node] registered local binding: %s -> %s", b.ModelCode, b.KeyHash[:16]+"...")
	}

	srv := server.New(r)
	go func() {
		if err := srv.Start(":20128"); err != nil {
			log.Printf("[node] http server: %v", err)
		}
	}()

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
		log.Println("[node] no platform config - running in standalone mode")
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("[node] shutting down")
	srv.Stop()
}
