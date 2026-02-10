package main

import (
    "context"
    "time"
)

// CheckEngine - движок проверок
type CheckEngine struct {
    Cache     *CacheManager
    apiClient *CloudAPIClient
    metrics   *MetricsCollector
}

func newCheckEngine(cache *CacheManager, apiClient *CloudAPIClient) *CheckEngine {
    return &CheckEngine{
        Cache:     cache,
        apiClient: apiClient,
        metrics:   newMetricsCollector(),
    }
}

// checkDomainForCache - проверка домена только для обогащения кеша
// Используется в фоне после ответа клиенту
func (e *CheckEngine) checkDomainForCache(ctx context.Context, domain string) (*DomainResult, error) {
    start := time.Now()

    // Пробуем кеш
    if cached := e.cache.get(domain); cached != nil {
        return cached, nil
    }

    // Запрашиваем Cloud API с увеличенным таймаутом
    apiResult, err := e.apiClient.check(ctx, domain)
    if err != nil {
        logWarn("Background API check failed",
            "domain", domain,
            "error", err,
            "duration_ms", time.Since(start).Milliseconds())
        return nil, err
    }

    // Создаем результат
    result := e.createResultFromAPI(apiResult)

    // Сохраняем в кеш
    e.cache.set(domain, result)

    logDebug("Background API check completed",
        "domain", domain,
        "category", result.Category,
        "duration_ms", time.Since(start).Milliseconds())

    return result, nil
}

// checkDomain - основная проверка (используется если есть в кеше)
func (e *CheckEngine) checkDomain(ctx context.Context, domain string) (*DomainResult, error) {
    // В основном только проверяем кеш
    if cached := e.cache.get(domain); cached != nil {
        e.metrics.incCacheHit()
        return cached, nil
    }

    e.metrics.incCacheMiss()
    
    // Если нет в кеше, возвращаем разрешенный результат
    // Cloud API будет проверен в фоне
    return &DomainResult{
        Domain:   domain,
        Action:   "allow",
        Category: 0,
        TTL:      getTTLByCategory(0),
        Source:   "cache_miss_fallback",
        Timestamp: time.Now(),
    }, nil
}

func (e *CheckEngine) createResultFromAPI(apiResult *APIResponse) *DomainResult {
    result := &DomainResult{
        Domain:   apiResult.Domain,
        Category: apiResult.Category,
        TTL:      getTTLByCategory(apiResult.Category),
        Source:   "cloud_api",
        Timestamp: time.Now(),
    }

    // Определяем действие по категории
    if apiResult.Category == 0 || apiResult.Category == 9 {
        result.Action = "allow"
    } else {
        result.Action = "block"
        result.IP = getSinkholeIP(apiResult.Category)
    }

    return result
}

func (e *CheckEngine) getStats() *Stats {
    return e.metrics.getStats()
}
