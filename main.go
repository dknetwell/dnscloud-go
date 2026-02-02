package main

import (
    "os"
    "os/signal"
    "syscall"
    
    "github.com/coredns/coredns/core/dnsserver"
    "github.com/coredns/coredns/coremain"
    
    _ "github.com/dknetwell/dnscloud-go/plugin/security"
    
    "github.com/dknetwell/dnscloud-go/config"
    "github.com/dknetwell/dnscloud-go/logger"
)

func init() {
    // Инициализация конфига
    if err := config.Load(); err != nil {
        panic(err)
    }
    
    // Инициализация логгера
    logger.Init(config.Get().Logging)
    
    // Регистрация плагина
    dnsserver.Directives = append(dnsserver.Directives, "security")
}

func main() {
    // Захват сигналов для graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        logger.Info("Shutdown signal received")
        // Cleanup логика
        os.Exit(0)
    }()
    
    // Запуск CoreDNS
    coremain.Run()
}
