package cache

import (
    "time"
    
    "github.com/dgraph-io/ristretto"
    "github.com/go-redis/redis/v8"
    
    "github.com/dknetwell/dnscloud-go/checker/models"
    "github.com/dknetwell/dnscloud-go/config"
    "github.com/dknetwell/dnscloud-go/logger"
)

type CacheManager struct {
    memoryCache *ristretto.Cache
    valkey      *redis.Client
    strategy    string
}

func NewCacheManager() *CacheManager {
    cfg := config.Get()
    
    // Инициализируем memory кеш
    memoryCache, err := ristretto.NewCache(&ristretto.Config{
        NumCounters: cfg.Cache.Memory.NumCounters,
        MaxCost:     int64(cfg.Cache.Memory.MaxSizeMB) << 20,
        BufferItems: cfg.Cache.Memory.BufferItems,
        Cost: func(value interface{}) int64 {
            if cfg.Cache.Memory.CostEstimation {
                // Оцениваем размер структуры
                if result, ok := value.(*models.DomainResult); ok {
                    return int64(len(result.Domain) + len(result.IP) + 16)
                }
            }
            return 1
        },
    })
    
    if err != nil {
        logger.Error("Failed to create memory cache", "error", err)
        return nil
    }
    
    // Инициализируем Valkey если нужно
    var valkeyClient *redis.Client
    if cfg.Cache.Strategy == "hybrid" || cfg.Cache.Strategy == "valkey_only" {
        valkeyClient = redis.NewClient(&redis.Options{
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
            valkeyClient = nil
        }
    }
    
    return &CacheManager{
        memoryCache: memoryCache,
        valkey:      valkeyClient,
        strategy:    cfg.Cache.Strategy,
    }
}

func (c *CacheManager) Get(domain string) *models.DomainResult {
    // 1. Проверяем memory кеш
    if val, found := c.memoryCache.Get(domain); found {
        if result, ok := val.(*models.DomainResult); ok {
            logger.Debug("Memory cache hit", "domain", domain)
            return result
        }
    }
    
    // 2. Проверяем Valkey если есть
    if c.valkey != nil && (c.strategy == "hybrid" || c.strategy == "valkey_only") {
        ctx, cancel := context.WithTimeout(context.Background(), 
            config.Get().Cache.Valkey.ReadTimeout)
        defer cancel()
        
        data, err := c.valkey.Get(ctx, cacheKey(domain)).Bytes()
        if err == nil {
            var result models.DomainResult
            if err := json.Unmarshal(data, &result); err == nil {
                // Сохраняем в memory кеш для будущих запросов
                c.memoryCache.SetWithTTL(domain, &result, 1, 
                    time.Duration(result.TTL)*time.Second)
                
                logger.Debug("Valkey cache hit", "domain", domain)
                return &result
            }
        }
    }
    
    return nil
}

func (c *CacheManager) Set(domain string, result *models.DomainResult) {
    // Устанавливаем timestamp
    result.Timestamp = time.Now()
    
    ttl := time.Duration(result.TTL) * time.Second
    
    // 1. Сохраняем в memory кеш
    cost := int64(1)
    if result.Action == "block" {
        cost = 2 // Блокировки более важны
    }
    
    c.memoryCache.SetWithTTL(domain, result, cost, ttl)
    
    // 2. Асинхронно сохраняем в Valkey если нужно
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

func cacheKey(domain string) string {
    return "dns:" + domain
}
