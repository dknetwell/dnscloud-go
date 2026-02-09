package cache

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"

    "github.com/dgraph-io/ristretto"
    "github.com/go-redis/redis/v8"
    "github.com/patrickmn/go-cache"

    "dnscloud-go/checker/models"
    "dnscloud-go/config"
    "dnscloud-go/logger"
)

// CacheManager управляет многоуровневым кешем
type CacheManager struct {
    memoryCache *ristretto.Cache
    valkey      *redis.Client
    strategy    string
    mu          sync.RWMutex
}

// NewCacheManager создает новый менеджер кеша
func NewCacheManager() *CacheManager {
    cfg := config.Get()
    
    manager := &CacheManager{
        strategy: cfg.Cache.Strategy,
    }
    
    // Инициализируем memory кеш
    if cfg.Cache.Strategy == "hybrid" || cfg.Cache.Strategy == "memory_only" {
        memoryCache, err := ristretto.NewCache(&ristretto.Config{
            NumCounters: 1_000_000, // 1M счетчиков
            MaxCost:     int64(cfg.Cache.Memory.MaxSizeMB) << 20,
            BufferItems: 64,
        })
        
        if err != nil {
            logger.Error("Failed to create memory cache", "error", err)
        } else {
            manager.memoryCache = memoryCache
        }
    }
    
    // Инициализируем Valkey
    if cfg.Cache.Strategy == "hybrid" || cfg.Cache.Strategy == "valkey_only" {
        valkeyClient := redis.NewClient(&redis.Options{
            Addr:     cfg.Cache.Valkey.Address,
            Password: cfg.Cache.Valkey.Password,
            DB:       0,
            PoolSize: cfg.Cache.Valkey.PoolSize,
        })
        
        // Проверяем соединение
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        
        if err := valkeyClient.Ping(ctx).Err(); err != nil {
            logger.Error("Failed to connect to Valkey", "error", err)
        } else {
            manager.valkey = valkeyClient
            logger.Info("Valkey cache connected", "address", cfg.Cache.Valkey.Address)
        }
    }
    
    logger.Info("Cache manager initialized",
        "strategy", cfg.Cache.Strategy,
        "memory_enabled", manager.memoryCache != nil,
        "valkey_enabled", manager.valkey != nil)
    
    return manager
}

// Get получает значение из кеша
func (c *CacheManager) Get(domain string) *models.DomainResult {
    // 1. Пробуем memory кеш
    if c.memoryCache != nil && (c.strategy == "hybrid" || c.strategy == "memory_only") {
        if val, found := c.memoryCache.Get(domain); found {
            if result, ok := val.(*models.DomainResult); ok {
                logger.Debug("Memory cache hit", "domain", domain)
                return result
            }
        }
    }
    
    // 2. Пробуем Valkey
    if c.valkey != nil && (c.strategy == "hybrid" || c.strategy == "valkey_only") {
        ctx, cancel := context.WithTimeout(context.Background(), 
            config.Get().Cache.Valkey.ReadTimeout)
        defer cancel()
        
        data, err := c.valkey.Get(ctx, cacheKey(domain)).Bytes()
        if err == nil {
            var result models.DomainResult
            if err := json.Unmarshal(data, &result); err == nil {
                // Сохраняем в memory кеш для следующих запросов
                if c.memoryCache != nil {
                    c.memoryCache.SetWithTTL(domain, &result, 1,
                        time.Duration(result.TTL)*time.Second)
                }
                
                logger.Debug("Valkey cache hit", "domain", domain)
                return &result
            }
        }
    }
    
    return nil
}

// Set сохраняет значение в кеш
func (c *CacheManager) Set(domain string, result *models.DomainResult) {
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
                logger.Error("Failed to marshal result for cache",
                    "domain", domain, "error", err)
                return
            }
            
            ctx, cancel := context.WithTimeout(context.Background(),
                config.Get().Cache.Valkey.WriteTimeout)
            defer cancel()
            
            if err := c.valkey.SetEx(ctx, cacheKey(domain), data, ttl).Err(); err != nil {
                logger.Warn("Failed to cache result in Valkey",
                    "domain", domain, "error", err)
            }
        }()
    }
}

// Shutdown корректно останавливает кеш
func (c *CacheManager) Shutdown() {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    if c.memoryCache != nil {
        c.memoryCache.Close()
    }
    
    if c.valkey != nil {
        c.valkey.Close()
    }
    
    logger.Info("Cache manager shutdown complete")
}

func cacheKey(domain string) string {
    return fmt.Sprintf("dns:%s", domain)
}
