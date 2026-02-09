package clients

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "dnscloud-go/config"
    "dnscloud-go/logger"
)

// CloudAPIChecker проверяет домены через Cloud API
type CloudAPIChecker struct {
    config *config.CloudAPIConfig
    client *http.Client
    name   string
}

// NewCloudAPIChecker создает новый клиент Cloud API
func NewCloudAPIChecker(cfg *config.CloudAPIConfig) *CloudAPIChecker {
    return &CloudAPIChecker{
        config: cfg,
        client: &http.Client{
            Timeout: cfg.Timeout,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
            },
        },
        name: "cloud_api",
    }
}

// Check проверяет домен через Cloud API
func (c *CloudAPIChecker) Check(ctx context.Context, domain string) (*CheckResult, error) {
    start := time.Now()
    
    // Формируем запрос
    req, err := http.NewRequestWithContext(ctx, "GET", c.config.URL, nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    
    // Добавляем заголовки и параметры
    req.Header.Set("X-API-Key", c.config.Key)
    req.Header.Set("Accept", "application/json")
    
    q := req.URL.Query()
    q.Add("domain", domain)
    req.URL.RawQuery = q.Encode()
    
    // Выполняем запрос
    resp, err := c.client.Do(req)
    if err != nil {
        logger.Warn("Cloud API request failed",
            "domain", domain,
            "error", err,
            "duration_ms", time.Since(start).Milliseconds())
        return nil, err
    }
    defer resp.Body.Close()
    
    // Парсим ответ
    var apiResp struct {
        Domain   string `json:"domain"`
        Category int    `json:"category"`
        TTL      int    `json:"ttl"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
        return nil, fmt.Errorf("parse response: %w", err)
    }
    
    logger.Debug("Cloud API check completed",
        "domain", domain,
        "category", apiResp.Category,
        "duration_ms", time.Since(start).Milliseconds())
    
    return &CheckResult{
        Domain:   apiResp.Domain,
        Category: apiResp.Category,
        TTL:      apiResp.TTL,
        Metadata: map[string]interface{}{
            "response_time_ms": time.Since(start).Milliseconds(),
        },
    }, nil
}

// Name возвращает имя источника
func (c *CloudAPIChecker) Name() string {
    return c.name
}

// Timeout возвращает таймаут для этого источника
func (c *CloudAPIChecker) Timeout() time.Duration {
    return c.config.Timeout
}

// IsAvailable проверяет доступность API
func (c *CloudAPIChecker) IsAvailable() bool {
    // Простая проверка ping
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    
    req, _ := http.NewRequestWithContext(ctx, "GET", c.config.URL, nil)
    req.Header.Set("X-API-Key", c.config.Key)
    
    resp, err := c.client.Do(req)
    if err != nil {
        return false
    }
    resp.Body.Close()
    
    return resp.StatusCode == http.StatusOK
}
