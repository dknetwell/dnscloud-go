package main

import (
    "context"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/miekg/dns"
    "github.com/prometheus/client_golang/prometheus/promhttp"

    "dnscloud-go/config"
    "dnscloud-go/logger"
    "dnscloud-go/server"
    "dnscloud-go/checker"
)

func main() {
    // Загрузка конфигурации
    if err := config.Load(); err != nil {
        logger.Fatal("Failed to load config", "error", err)
    }

    cfg := config.Get()
    
    // Инициализация логгера
    logger.Init(cfg.Logging)
    
    logger.Info("🚀 Starting DNS Security Proxy",
        "version", "1.0.0",
        "listen_dns", ":5353",
        "listen_http", ":8054",
        "sla_timeout", cfg.Timeouts.Total)

    // Создание движка проверок
    engine := checker.NewEngine()
    defer engine.Shutdown()

    // Создание DNS сервера
    dnsServer := server.NewDNSServer(cfg.Server, engine)
    
    // Создание HTTP сервера для метрик и health
    httpServer := server.NewHTTPServer(cfg.Server)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Запуск серверов
    go func() {
        if err := dnsServer.Start(ctx); err != nil {
            logger.Error("DNS server failed", "error", err)
            cancel()
        }
    }()

    go func() {
        if err := httpServer.Start(ctx); err != nil {
            logger.Error("HTTP server failed", "error", err)
            cancel()
        }
    }()

    // Ожидание сигналов завершения
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    logger.Info("✅ Services started successfully")
    logger.Info("📊 Metrics: http://localhost:8054/metrics")
    logger.Info("❤️  Health: http://localhost:8054/health")

    select {
    case sig := <-sigChan:
        logger.Info("🛑 Received signal, shutting down", "signal", sig)
        cancel()
        
        // Graceful shutdown (30 секунд)
        shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer shutdownCancel()
        
        dnsServer.Shutdown(shutdownCtx)
        httpServer.Shutdown(shutdownCtx)
        
    case <-ctx.Done():
        logger.Info("Context cancelled, shutting down")
    }

    logger.Info("👋 Shutdown completed")
}
