package main

import (
    "context"
    "crypto/tls"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"
    "golang.org/x/time/rate"
)

// CloudAPIClient - клиент Cloud API
type CloudAPIClient struct {
    config *CloudAPIConfig
    client *http.Client
    limiter *rate.Limiter
}

func newCloudAPIClient(config *CloudAPIConfig) *CloudAPIClient {
    transport := &http.Transport{
        TLSClientConfig: &tls.Config{
            InsecureSkipVerify: true,
        },
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        // Увеличиваем таймауты для медленного API
        ResponseHeaderTimeout: 30 * time.Second,
        ExpectContinueTimeout: 1 * time.Second,
    }

    return &CloudAPIClient{
        config: config,
        client: client,
        limiter: rate.NewLimiter(rate.Limit(config.RateLimit), config.Burst),
    }
}

func (c *CloudAPIClient) check(ctx context.Context, domain string) (*APIResponse, error) {
        // Применяем rate limiting
    if err := c.limiter.Wait(ctx); err != nil {
        return nil, fmt.Errorf("rate limit exceeded: %w", err)
    }
    start := time.Now()

    cleanDomain := strings.TrimSuffix(domain, ".")
    xmlQuery := fmt.Sprintf(`<test><dns-proxy><dns-signature><fqdn>%s</fqdn></dns-signature></dns-proxy></test>`, cleanDomain)
    
    url := fmt.Sprintf("%s?type=op&cmd=%s", c.config.URL, xmlQuery)
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    req.Header.Set("X-PAN-KEY", c.config.Key)
    req.Header.Set("Accept", "application/json")

    // Добавляем таймаут на соединение
    logDebug("Cloud API request", "url", url, "domain", cleanDomain)
    
    resp, err := c.client.Do(req)
    if err != nil {
        // Не логируем как ошибку, это нормально для фоновой проверки
        logDebug("Cloud API request failed",
            "domain", cleanDomain,
            "error", err,
            "duration_ms", time.Since(start).Milliseconds())
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        logDebug("Cloud API returned non-OK status",
            "domain", cleanDomain,
            "status", resp.StatusCode)
        return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response: %w", err)
    }

    // Парсим ответ
    bodyStr := string(body)
    category := 0
    ttl := 300
    
    // Ищем категорию
    if strings.Contains(bodyStr, `"category":1`) || strings.Contains(bodyStr, `"category": 1`) {
        category = 1
    } else if strings.Contains(bodyStr, `"category":2`) || strings.Contains(bodyStr, `"category": 2`) {
        category = 2
    } else if strings.Contains(bodyStr, `"category":3`) || strings.Contains(bodyStr, `"category": 3`) {
        category = 3
    } else if strings.Contains(bodyStr, `"category":8`) || strings.Contains(bodyStr, `"category": 8`) {
        category = 8
    } else if strings.Contains(bodyStr, `"category":9`) || strings.Contains(bodyStr, `"category": 9`) {
        category = 9
    }

    // Ищем TTL
    if idx := strings.Index(bodyStr, `"ttl":`); idx != -1 {
        endIdx := strings.Index(bodyStr[idx:], ",")
        if endIdx == -1 {
            endIdx = strings.Index(bodyStr[idx:], "}")
        }
        if endIdx != -1 {
            ttlStr := bodyStr[idx+6 : idx+endIdx]
            ttlStr = strings.Trim(ttlStr, `" `)
            fmt.Sscanf(ttlStr, "%d", &ttl)
        }
    }

    result := &APIResponse{
        Domain:   cleanDomain,
        Category: category,
        TTL:      ttl,
    }

    logDebug("Cloud API check successful",
        "domain", cleanDomain,
        "category", category,
        "ttl", ttl,
        "duration_ms", time.Since(start).Milliseconds())

    return result, nil
}
