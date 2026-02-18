package main

import (
	"flag"
	"log"

	"ns116/internal/config"
	"ns116/internal/server"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Println("=== NS116 — DNS Manager ===")
	log.Printf("Version: %s", version)
	log.Printf("Listening on %s:%d", cfg.Server.Host, cfg.Server.Port)

	if len(cfg.HostedZones) > 0 {
		log.Printf("Managing %d hosted zone(s)", len(cfg.HostedZones))
	} else {
		log.Println("Managing ALL hosted zones in the account")
	}

	if err := server.Start(cfg, version); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
