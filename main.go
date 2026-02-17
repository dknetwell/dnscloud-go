package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var netResolver *net.Resolver

func main() {

	cfg, err := LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	InitLogger(cfg)
	initMetrics()

	// L1 cache (Ristretto)
	cache := NewCache(cfg)

	// Valkey client (L2 cache)
	valkeyClient, err := NewValkeyClient(cfg)
	if err != nil {
		log.Fatalf("valkey connection error: %v", err)
	}

	// Enrichers
	enrichers := []Enricher{
		NewCloudAPIEnricher(cfg),
	}

	engine := NewCheckEngine(cfg, cache, valkeyClient, enrichers)

	netResolver = &net.Resolver{
		PreferGo: true,
	}

	// DNS server
	dnsServer := NewDNSServer(engine, cfg)
	go dnsServer.Start()

	// HTTP server (stats + metrics)
	httpServer := NewHTTPServer(engine, cfg)
	go httpServer.Start()

	LogInfo("system", "DNS Security Proxy started")

	// graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	LogInfo("system", "Shutting down...")
	engine.Shutdown()
}
