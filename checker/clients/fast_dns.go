package clients

import (
    "context"
    "net"
    "time"
    
    "github.com/dgraph-io/ristretto"
    "github.com/proxy/dnscloud-go/config"
    "github.com/proxy/dnscloud-go/logger"
)

type FastDNSClient struct {
    resolver *net.Resolver
    cache    *ristretto.Cache
    timeout  time.Duration
}

func NewFastDNSClient(cfg *config.FallbackDNSConfig) *FastDNSClient {
    // Создаем кеш для быстрых DNS запросов
    cache, _ := ristretto.NewCache(&ristretto.Config{
        NumCounters: 100000,
        MaxCost:     50 << 20, // 50MB
        BufferItems: 64,
    })
    
    // Создаем резолвер с указанными серверами
    resolver := &net.Resolver{
        PreferGo: true,
        Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
            d := net.Dialer{
                Timeout: time.Duration(cfg.Timeout) * time.Millisecond,
            }
            // Используем первый fallback сервер
            return d.DialContext(ctx, "udp", cfg.Servers[0])
        },
    }
    
    return &FastDNSClient{
        resolver: resolver,
        cache:    cache,
        timeout:  time.Duration(cfg.Timeout) * time.Millisecond,
    }
}

func (f *FastDNSClient) LookupA(ctx context.Context, domain string) (string, error) {
    start := time.Now()
    
    // Проверяем кеш
    if ip, found := f.cache.Get(domain); found {
        logger.Debug("Fast DNS cache hit",
            "domain", domain,
            "ip", ip)
        return ip.(string), nil
    }
    
    // Устанавливаем таймаут
    ctx, cancel := context.WithTimeout(ctx, f.timeout)
    defer cancel()
    
    // Выполняем DNS запрос
    ips, err := f.resolver.LookupIP(ctx, "ip4", domain)
    if err != nil {
        logger.Warn("Fast DNS lookup failed",
            "domain", domain,
            "error", err,
            "duration_ms", time.Since(start).Milliseconds())
        return "", err
    }
    
    if len(ips) == 0 {
        return "", fmt.Errorf("no IP found for domain")
    }
    
    ip := ips[0].String()
    
    // Кешируем результат с коротким TTL
    f.cache.SetWithTTL(domain, ip, 1, 60*time.Second)
    
    logger.Debug("Fast DNS lookup completed",
        "domain", domain,
        "ip", ip,
        "duration_ms", time.Since(start).Milliseconds())
    
    return ip, nil
}
