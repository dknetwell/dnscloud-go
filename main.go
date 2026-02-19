package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dnscloud-go/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	log.Println("DNS Security Proxy starting...")

	// DNS сервер
	go startDNSServer(cfg)

	// HTTP сервер (health + metrics)
	go startHTTPServer(cfg)

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down gracefully...")
}

func startDNSServer(cfg *config.Config) {
	addr := cfg.Server.DNSListenAddr

	ln, err := net.ListenPacket("udp", addr)
	if err != nil {
		log.Fatalf("failed to start DNS server: %v", err)
	}
	defer ln.Close()

	log.Printf("DNS server started on %s\n", addr)

	buf := make([]byte, 512)
	for {
		_, _, err := ln.ReadFrom(buf)
		if err != nil {
			log.Println("DNS read error:", err)
			continue
		}
	}
}

func startHTTPServer(cfg *config.Config) {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:    cfg.Server.HTTPListenAddr,
		Handler: mux,
	}

	log.Printf("HTTP server started on %s\n", cfg.Server.HTTPListenAddr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}
}
