package main

import (
    "context"
    "time"
)

// CheckEngine - движок проверок с контролем SLA
type CheckEngine struct {
    cache     *CacheManager
    apiClient *CloudAPIClient
    metrics   *MetricsCollector
}

func newCheckEngine(cache *CacheManager, apiClient *CloudAPIClient) *CheckEngine {
    return &CheckEngine{
        cache:     cache,
        apiClient: apiClient,
        metrics:   newMetricsCollector(),
    }
}

func (e *CheckEngine) checkDomain(ctx context.Context, domain string) (*DomainResult, error) {
    start := time.Now()
    
    // Устанавливаем общий таймаут SLA
    slaCtx, cancel := context.WithTimeout(ctx, getConfig().Timeouts.Total)
    defer cancel()
    
    // Канал для результата
    resultChan := make(chan *DomainResult, 1)
    errorChan := make(chan error, 1)
    
    // Запускаем проверку в горутине
    go e.processCheck(slaCtx, domain, resultChan, errorChan)
    
    // Ждем результат или таймаут SLA
    select {
    case result := <-resultChan:
        result.ProcessingTime = time.Since(start)
        
        // Записываем метрику
        e.metrics.recordCheckDuration(result.ProcessingTime, result.Source)
        
        // Логируем если близко к лимиту
        if result.ProcessingTime > 80*time.Millisecond {
            logWarn("Check close to SLA limit",
                "domain", domain,
                "duration", result.ProcessingTime.String(),
                "sla_limit", getConfig().Timeouts.Total.String())
        }
        
        return result, nil
        
    case err := <-errorChan:
        return e.createFallbackResult(domain, "error"), err
        
    case <-slaCtx.Done():
        // SLA ТАЙМАУТ - возвращаем fallback
        logWarn("SLA timeout reached",
            "domain", domain,
            "timeout", getConfig().Timeouts.Total.String())
        
        e.metrics.incTimeout()
        return e.createFallbackResult(domain, "timeout"), nil
    }
}

func (e *CheckEngine) processCheck(ctx context.Context, domain string, 
    resultChan chan<- *DomainResult, errorChan chan<- error) {
    
    // 1. Проверка кеша (быстрый путь)
    if cached := e.cache.get(domain); cached != nil {
        cached.Source = "cache"
        resultChan <- cached
        e.metrics.incCacheHit()
        return
    }
    
    e.metrics.incCacheMiss()
    
    // 2. Проверка через Cloud API
    apiResult, err := e.apiClient.check(ctx, domain)
    if err != nil {
        logWarn("Cloud API check failed",
            "domain", domain,
            "error", err)
        
        // Возвращаем fallback результат
        resultChan <- e.createFallbackResult(domain, "api_error")
        return
    }
    
    // 3. Создаем результат
    result := e.createResultFromAPI(apiResult)
    
    // 4. Кешируем результат (асинхронно)
    go e.cache.set(domain, result)
    
    resultChan <- result
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

func (e *CheckEngine) createFallbackResult(domain, reason string) *DomainResult {
    return &DomainResult{
        Domain:   domain,
        Action:   "allow",  // В случае ошибки разрешаем
        Category: 0,
        TTL:      getTTLByCategory(0),
        Source:   "fallback:" + reason,
        Timestamp: time.Now(),
    }
}

func (e *CheckEngine) getStats() *Stats {
    return e.metrics.getStats()
}
