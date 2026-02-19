package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	cfg, err := LoadConfig()
	if err != nil {
		LogFatal("system", "config load failed", err)
	}

	InitLogger(cfg)
	initMetrics()

	LogInfo("system", "DNS Security Proxy starting")

	cache := NewCache(cfg)

	valkeyClient, err := NewValkeyClient(cfg)
	if err != nil {
		LogFatal("system", "valkey init failed", err)
	}

	cloudEnricher := NewCloudAPIEnricher(cfg)
	enrichers := []Enricher{cloudEnricher}

	engine := NewCheckEngine(cfg, cache, valkeyClient, enrichers)

	dnsServer := NewDNSServer(engine, cfg)
	go dnsServer.Start()

	httpServer := NewHTTPServer(engine, cfg)
	go httpServer.Start()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	LogInfo("system", "shutdown signal received")

	engine.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)

	LogInfo("system", "shutdown complete")
}
