package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	cfg, err := LoadConfig("config/config.yaml")
	if err != nil {
		panic(err)
	}

	InitLogger(cfg)
	initMetrics()

	LogInfo("system", "DNS Security Proxy starting")

	// Cache
	cache := NewCache(cfg)

	// Valkey
	valkeyClient, err := NewValkeyClient(cfg)
	if err != nil {
		panic(err)
	}

	// Enrichers
	cloudEnricher := NewCloudAPIEnricher(cfg)
	enrichers := []Enricher{
		cloudEnricher,
	}

	// Engine
	engine := NewCheckEngine(cfg, cache, valkeyClient, enrichers)

	// DNS server
	dnsServer := NewDNSServer(engine, cfg)
	go dnsServer.Start()

	// HTTP server
	httpServer := NewHTTPServer(engine, cfg)
	go httpServer.Start()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	LogInfo("system", "Shutting down")

	engine.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	httpServer.Shutdown(ctx)

	LogInfo("system", "Shutdown complete")
}
