package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func main() {
    // Загрузка конфигурации
    if err := loadConfig(); err != nil {
        fmt.Printf("❌ Failed to load config: %v\n", err)
        os.Exit(1)
    }
    
    cfg := getConfig()
    
    // Инициализация логгера
    initLogger(cfg.LogLevel)
    
    logInfo("🚀 Starting DNS Security Proxy",
        "version", "1.0.0",
        "dns_listen", cfg.DNSListen,
        "http_listen", cfg.HTTPListen,
        "sla_timeout", cfg.Timeouts.Total.String())
    
    // Инициализация метрик
    initMetrics()
    
    // Инициализация кеша
    cache := newCacheManager()
    defer cache.shutdown()
    
    // Инициализация Cloud API клиента
    apiClient := newCloudAPIClient(&cfg.CloudAPI)
    
    // Инициализация движка проверок
    engine := newCheckEngine(cache, apiClient)
    
    // Запуск DNS сервера (UDP и TCP)
    dnsServer := newDNSServer(cfg.DNSListen, engine)
    go func() {
        logInfo("Starting DNS server", "address", cfg.DNSListen)
        if err := dnsServer.start(); err != nil {
            logError("DNS server failed", err)
            os.Exit(1)
        }
    }()
    
    // Запуск HTTP сервера для метрик
    httpServer := newHTTPServer(cfg.HTTPListen, engine)
    go func() {
        logInfo("Starting HTTP server", "address", cfg.HTTPListen)
        if err := httpServer.start(); err != nil {
            logError("HTTP server failed", err)
        }
    }()
    
    // Ожидание сигналов завершения
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    logInfo("✅ Services started successfully")
    logInfo("📊 Metrics: http://" + cfg.HTTPListen + "/metrics")
    logInfo("❤️  Health: http://" + cfg.HTTPListen + "/health")
    
    select {
    case sig := <-sigChan:
        logInfo("🛑 Received signal, shutting down", "signal", sig.String())
        
        // Graceful shutdown
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        dnsServer.shutdown(ctx)
        httpServer.shutdown(ctx)
        
    case <-time.After(1 * time.Hour):
        logInfo("Timeout reached")
    }
    
    logInfo("👋 Shutdown completed")
}
