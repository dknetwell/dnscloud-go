package checker

import (
    "context"
    "fmt"
    "time"

    "dnscloud-go/config"
    "dnscloud-go/logger"
    "dnscloud-go/server"
)

// Engine - основной движок проверок с контролем SLA
type Engine struct {
    cache    *CacheManager
    cloudAPI *CloudAPIClient
    metrics  *Metrics
}

// NewEngine создает новый движок проверок
func NewEngine() *Engine {
    cfg := config.Get()
    
    engine := &Engine{
        cache:    NewCacheManager(),
        cloudAPI: NewCloudAPIClient(cfg.CloudAPI),
        metrics:  NewMetrics(),
    }
    
    logger.Info("Checker engine initialized",
        "sla_timeout", cfg.Timeouts.Total,
        "cache_strategy", cfg.Cache.Strategy)
    
    return engine
}

// Check выполняет проверку домена с контролем SLA
func (e *Engine) Check(ctx context.Context, domain string) (*DomainResult, error) {
    start := time.Now()
    
    // Устанавливаем общий таймаут SLA
    ctx, cancel := context.WithTimeout(ctx, config.Get().Timeouts.Total)
    defer cancel()
    
    // Канал для результата
    resultChan := make(chan *DomainResult, 1)
    errorChan := make(chan error, 1)
    
    // Запускаем проверку в горутине
    go e.processCheck(ctx, domain, resultChan, errorChan)
    
    // Ждем результат или таймаут SLA
    select {
    case result := <-resultChan:
        // Успешный ответ в рамках SLA
        duration := time.Since(start)
        result.ProcessingTime = duration
        
        // Записываем метрику времени выполнения
        e.metrics.RecordCheckDuration(duration, result.Source)
        
        // Логируем если близко к лимиту
        if duration > 80*time.Millisecond {
            logger.Warn("Check close to SLA limit",
                "domain", domain,
                "duration", duration,
                "sla_limit", config.Get().Timeouts.Total)
        }
        
        return result, nil
        
    case err := <-errorChan:
        // Ошибка при проверке
        return e.createFallbackResult(domain, "error"), err
        
    case <-ctx.Done():
        // SLA ТАЙМАУТ - возвращаем fallback
        logger.Warn("SLA timeout reached",
            "domain", domain,
            "timeout", config.Get().Timeouts.Total)
        
        e.metrics.IncTimeout()
        return e.createFallbackResult(domain, "timeout"), nil
    }
}

// processCheck выполняет фактическую проверку
func (e *Engine) processCheck(ctx context.Context, domain string, 
    resultChan chan<- *DomainResult, errorChan chan<- error) {
    
    // 1. Проверка кеша (быстрый путь)
    if cached := e.cache.Get(domain); cached != nil {
        cached.Source = "cache"
        resultChan <- cached
        e.metrics.IncCacheHit()
        return
    }
    
    e.metrics.IncCacheMiss()
    
    // 2. Проверка через Cloud API
    apiResult, err := e.cloudAPI.Check(ctx, domain)
    if err != nil {
        logger.Warn("Cloud API check failed",
            "domain", domain,
            "error", err)
        
        // Возвращаем fallback результат
        resultChan <- e.createFallbackResult(domain, "api_error")
        return
    }
    
    // 3. Создаем результат
    result := e.createResultFromAPI(apiResult)
    
    // 4. Кешируем результат (асинхронно)
    go e.cache.Set(domain, result)
    
    resultChan <- result
}

// createResultFromAPI преобразует ответ API в DomainResult
func (e *Engine) createResultFromAPI(apiResult *APIResponse) *DomainResult {
    result := &DomainResult{
        Domain:   apiResult.Domain,
        Category: apiResult.Category,
        TTL:      config.GetTTLByCategory(apiResult.Category),
        Source:   "cloud_api",
    }
    
    // Определяем действие по категории
    if apiResult.Category == 0 || apiResult.Category == 9 {
        result.Action = "allow"
    } else {
        result.Action = "block"
        result.IP = config.GetSinkholeIP(apiResult.Category)
    }
    
    return result
}

// createFallbackResult создает fallback результат
func (e *Engine) createFallbackResult(domain, reason string) *DomainResult {
    return &DomainResult{
        Domain:   domain,
        Action:   "allow",  // В случае ошибки разрешаем
        Category: 0,
        TTL:      config.GetTTLByCategory(0),
        Source:   "fallback:" + reason,
    }
}

// Shutdown корректно останавливает движок
func (e *Engine) Shutdown() {
    if e.cache != nil {
        e.cache.Shutdown()
    }
    logger.Info("Checker engine shutdown complete")
}
