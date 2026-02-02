package checker

import (
    "context"
    "sync"
    "time"
    
    "github.com/proxy/dnscloud-go/config"
    "github.com/proxy/dnscloud-go/logger"
)

type Engine struct {
    cache       *CacheManager
    cloudAPI    *CloudAPIClient
    cloudDNS    *CloudDNSClient
    fastDNS     *FastDNSClient
    rateLimiter *RateLimiter
    enricher    *Enricher
}

func NewEngine() *Engine {
    cfg := config.Get()
    
    return &Engine{
        cache:       NewCacheManager(),
        cloudAPI:    NewCloudAPIClient(cfg.CloudAPI),
        cloudDNS:    NewCloudDNSClient(cfg.CloudDNS),
        fastDNS:     NewFastDNSClient(cfg.FallbackDNS),
        rateLimiter: NewRateLimiter(cfg.CloudAPI.RateLimit, cfg.CloudAPI.Burst),
        enricher:    NewEnricher(),
    }
}

func (e *Engine) Check(ctx context.Context, domain string) (*DomainResult, error) {
    // 1. Проверка кеша
    if cached := e.cache.Get(domain); cached != nil {
        return cached, nil
    }
    
    // 2. Параллельные проверки с таймаутом 95ms
    ctx, cancel := context.WithTimeout(ctx, 95*time.Millisecond)
    defer cancel()
    
    // 3. Запускаем проверки
    apiChan := make(chan *APIResponse, 1)
    dnsChan := make(chan *DNSResponse, 1)
    
    go e.checkCloudAPI(ctx, domain, apiChan)
    go e.checkCloudDNS(ctx, domain, dnsChan)
    
    // 4. Ждем первый успешный результат
    var result *DomainResult
    
    select {
    case apiResp := <-apiChan:
        // Cloud API ответил первым
        result = e.createResultFromAPI(apiResp)
        
        // Сразу отвечаем клиенту (с IP из fast DNS если есть)
        if result.IP == "" && result.Action == "allow" {
            // Пробуем быстро получить IP
            ip, _ := e.fastDNS.LookupA(context.Background(), domain)
            result.IP = ip
        }
        
        // В фоне ждем Cloud DNS для обогащения
        go e.waitForDNSAndEnrich(domain, dnsChan, result)
        
    case dnsResp := <-dnsChan:
        // Cloud DNS ответил первым
        result = e.createResultFromDNS(dnsResp)
        
        // Сразу отвечаем клиенту
        // В фоне ждем Cloud API для обогащения категорией
        go e.waitForAPIAndEnrich(domain, apiChan, result)
        
    case <-ctx.Done():
        // ТАЙМАУТ - только тогда используем fallback
        result = e.fallbackCheck(context.Background(), domain)
    }
    
    // 5. Кешируем результат (даже если не полный)
    e.cache.Set(domain, result)
    
    return result, nil
}

func (e *Engine) processResults(domain string, 
    apiResult *APIResponse, apiErr error,
    dnsResult *DNSResponse, dnsErr error) *DomainResult {
    
    // Логируем результаты проверок
    logger.Debug("Processing check results",
        "domain", domain,
        "api_success", apiErr == nil,
        "dns_success", dnsErr == nil)
    
    // Сценарий 1: Cloud API успел
    if apiResult != nil && apiErr == nil {
        result := e.createResultFromAPI(apiResult)
        
        // Обогащаем IP если Cloud DNS тоже успел
        if dnsResult != nil && dnsErr == nil {
            result.IP = dnsResult.IP
            result.Source = "enriched_api_dns"
            result.TTL = e.adjustTTL(result.TTL, true)
        } else {
            // Получаем IP из fast DNS
            ip, err := e.fastDNS.LookupA(context.Background(), domain)
            if err == nil {
                result.IP = ip
                result.Source = "enriched_api_fastdns"
            } else {
                result.Source = "cloud_api"
            }
        }
        
        return result
    }
    
    // Сценарий 2: Cloud DNS успел
    if dnsResult != nil && dnsErr == nil {
        result := e.createResultFromDNS(dnsResult)
        
        // Если API потом ответил, обогащаем категорией
        if apiResult != nil && apiErr == nil {
            result.Category = apiResult.Category
            result.TTL = config.GetTTLByCategory(apiResult.Category)
            result.Source = "enriched_dns_api"
            result.TTL = e.adjustTTL(result.TTL, true)
        }
        
        return result
    }
    
    // Сценарий 3: Обе проверки не удались
    logger.Warn("Both checks failed, using fallback",
        "domain", domain,
        "api_error", apiErr,
        "dns_error", dnsErr)
    
    return e.fallbackCheck(context.Background(), domain)
}

func (e *Engine) createResultFromAPI(apiResult *APIResponse) *DomainResult {
    result := &DomainResult{
        Domain:   apiResult.Domain,
        Category: apiResult.Category,
        TTL:      config.GetTTLByCategory(apiResult.Category),
    }
    
    // Определяем action по категории
    if apiResult.Category == 0 || apiResult.Category == 7 || apiResult.Category == 9 {
        result.Action = "allow"
    } else if apiResult.Category == 4 || apiResult.Category == 5 || apiResult.Category == 6 {
        result.Action = "allow" // но с коротким TTL
    } else {
        result.Action = "block"
        result.IP = config.GetSinkholeIP(apiResult.Category)
    }
    
    return result
}

func (e *Engine) createResultFromDNS(dnsResult *DNSResponse) *DomainResult {
    result := &DomainResult{
        Domain: dnsResult.Domain,
        IP:     dnsResult.IP,
        TTL:    uint32(dnsResult.TTL),
    }
    
    // Cloud DNS вернул sinkhole
    if dnsResult.IsSinkhole {
        result.Action = "block"
        result.Category = 1 // предполагаем malware
        result.IP = config.GetSinkholeIP(1)
        result.Source = "cloud_dns"
    } else {
        result.Action = "allow"
        result.Category = 0 // предполагаем benign
        result.Source = "cloud_dns"
    }
    
    return result
}

func (e *Engine) fallbackCheck(ctx context.Context, domain string) *DomainResult {
    ip, err := e.fastDNS.LookupA(ctx, domain)
    
    result := &DomainResult{
        Domain: domain,
        Action: "allow",
        Category: 0,
        Source: "fallback",
        TTL: config.GetTTLByCategory(0),
    }
    
    if err == nil {
        result.IP = ip
    }
    
    return result
}

func (e *Engine) adjustTTL(ttl uint32, enriched bool) uint32 {
    cfg := config.Get()
    
    // Применяем ограничения
    if ttl < uint32(cfg.TTL.Min) {
        ttl = uint32(cfg.TTL.Min)
    }
    if ttl > uint32(cfg.TTL.Max) {
        ttl = uint32(cfg.TTL.Max)
    }
    
    // Увеличиваем TTL для обогащенных ответов
    if enriched && cfg.TTL.EnrichedMultiplier > 1.0 {
        ttl = uint32(float64(ttl) * cfg.TTL.EnrichedMultiplier)
        if ttl > uint32(cfg.TTL.Max) {
            ttl = uint32(cfg.TTL.Max)
        }
    }
    
    return ttl
}
