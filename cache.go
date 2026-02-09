package main

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"
    
    "github.com/dgraph-io/ristretto"
    "github.com/go-redis/redis/v8"
)

// CacheManager - менеджер кеша (memory + valkey)
type CacheManager struct {
    memoryCache *ristretto.Cache
    valkey      *redis.Client
    strategy    string
    mu          sync.RWMutex
}

func newCacheManager() *CacheManager {
    cfg := getConfig()
    
    manager := &CacheManager{
        strategy: cfg.Cache.Strategy,
    }
    
    // Инициализируем memory кеш
    if cfg.Cache.Strategy == "hybrid" || cfg.Cache.Strategy == "memory_only" {
        memoryCache, err := ristretto.NewCache(&ristretto.Config{
            NumCounters: 1_000_000,
            MaxCost:     int64(cfg.Cache.MemoryMaxSize) << 20,
            BufferItems: 64,
        })
        
        if err != nil {
            logError("Failed to create memory cache", err)
        } else {
            manager.memoryCache = memoryCache
        }
    }
    
    // Инициализируем Valkey
    if cfg.Cache.Strategy == "hybrid" || cfg.Cache.Strategy == "valkey_only" {
        valkeyClient := redis.NewClient(&redis.Options{
            Addr:     cfg.Cache.ValkeyAddr,
            Password: cfg.Cache.ValkeyPass,
            DB:       0,
            PoolSize: 100,
        })
        
        // Проверяем соединение
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        
        if err := valkeyClient.Ping(ctx).Err(); err != nil {
            logError("Failed to connect to Valkey", err)
        } else {
            manager.valkey = valkeyClient
            logInfo("Valkey cache connected", "address", cfg.Cache.ValkeyAddr)
        }
    }
    
    logInfo("Cache manager initialized",
        "strategy", cfg.Cache.Strategy,
        "memory_enabled", manager.memoryCache != nil,
        "valkey_enabled", manager.valkey != nil)
    
    return manager
}

func (c *CacheManager) get(domain string) *DomainResult {
    // 1. Пробуем memory кеш
    if c.memoryCache != nil && (c.strategy == "hybrid" || c.strategy == "memory_only") {
        if val, found := c.memoryCache.Get(domain); found {
            if result, ok := val.(*DomainResult); ok {
                logDebug("Memory cache hit", "domain", domain)
                return result
            }
        }
    }
    
    // 2. Пробуем Valkey
    if c.valkey != nil && (c.strategy == "hybrid" || c.strategy == "valkey_only") {
        ctx, cancel := context.WithTimeout(context.Background(), getConfig().Timeouts.CacheRead)
        defer cancel()
        
        data, err := c.valkey.Get(ctx, cacheKey(domain)).Bytes()
        if err == nil {
            var result DomainResult
            if err := json.Unmarshal(data, &result); err == nil {
                // Сохраняем в memory кеш для следующих запросов
                if c.memoryCache != nil {
                    c.memoryCache.SetWithTTL(domain, &result, 1, time.Duration(result.TTL)*time.Second)
                }
                
                logDebug("Valkey cache hit", "domain", domain)
                return &result
            }
        }
    }
    
    return nil
}

func (c *CacheManager) set(domain string, result *DomainResult) {
    result.Timestamp = time.Now()
    ttl := time.Duration(result.TTL) * time.Second
    
    // 1. Сохраняем в memory кеш
    if c.memoryCache != nil && (c.strategy == "hybrid" || c.strategy == "memory_only") {
        c.memoryCache.SetWithTTL(domain, result, 1, ttl)
    }
    
    // 2. Асинхронно сохраняем в Valkey
    if c.valkey != nil && (c.strategy == "hybrid" || c.strategy == "valkey_only") {
        go func() {
            data, err := json.Marshal(result)
            if err != nil {
                logError("Failed to marshal result for cache", err, "domain", domain)
                return
            }
            
            ctx, cancel := context.WithTimeout(context.Background(), getConfig().Timeouts.CacheWrite)
            defer cancel()
            
            if err := c.valkey.SetEx(ctx, cacheKey(domain), data, ttl).Err(); err != nil {
                logWarn("Failed to cache result in Valkey", "domain", domain, "error", err)
            }
        }()
    }
}

func (c *CacheManager) shutdown() {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    if c.memoryCache != nil {
        c.memoryCache.Close()
    }
    
    if c.valkey != nil {
        c.valkey.Close()
    }
    
    logInfo("Cache manager shutdown complete")
}

func cacheKey(domain string) string {
    return fmt.Sprintf("dns:%s", domain)
}
