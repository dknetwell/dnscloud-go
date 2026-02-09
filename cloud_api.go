package main

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// CloudAPIClient - клиент Cloud API
type CloudAPIClient struct {
    config *CloudAPIConfig
    client *http.Client
}

func newCloudAPIClient(config *CloudAPIConfig) *CloudAPIClient {
    return &CloudAPIClient{
        config: config,
        client: &http.Client{
            Timeout: config.Timeout,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
            },
        },
    }
}

func (c *CloudAPIClient) check(ctx context.Context, domain string) (*APIResponse, error) {
    start := time.Now()
    
    // Формируем запрос
    req, err := http.NewRequestWithContext(ctx, "GET", c.config.URL, nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    
    // Добавляем заголовки
    req.Header.Set("X-API-Key", c.config.Key)
    req.Header.Set("Accept", "application/json")
    
    q := req.URL.Query()
    q.Add("domain", domain)
    req.URL.RawQuery = q.Encode()
    
    // Выполняем запрос
    resp, err := c.client.Do(req)
    if err != nil {
        logWarn("Cloud API request failed",
            "domain", domain,
            "error", err,
            "duration_ms", time.Since(start).Milliseconds())
        return nil, err
    }
    defer resp.Body.Close()
    
    // Парсим ответ
    var apiResp APIResponse
    
    if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
        return nil, fmt.Errorf("parse response: %w", err)
    }
    
    logDebug("Cloud API check completed",
        "domain", domain,
        "category", apiResp.Category,
        "duration_ms", time.Since(start).Milliseconds())
    
    return &apiResp, nil
}
