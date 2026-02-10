package main

import (
    "context"
    "crypto/tls"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"
)

// CloudAPIClient - клиент Cloud API
type CloudAPIClient struct {
    config *CloudAPIConfig
    client *http.Client
}

func newCloudAPIClient(config *CloudAPIConfig) *CloudAPIClient {
    // Создаем транспорт с игнорированием проверки сертификатов
    transport := &http.Transport{
        TLSClientConfig: &tls.Config{
            InsecureSkipVerify: true,
        },
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    }

    return &CloudAPIClient{
        config: config,
        client: &http.Client{
            Timeout:   config.Timeout,
            Transport: transport,
        },
    }
}

func (c *CloudAPIClient) check(ctx context.Context, domain string) (*APIResponse, error) {
    start := time.Now()

    // Очищаем домен от точки в конце
    cleanDomain := strings.TrimSuffix(domain, ".")

    // Формируем XML запрос
    xmlQuery := fmt.Sprintf(`<test><dns-proxy><dns-signature><fqdn>%s</fqdn></dns-signature></dns-proxy></test>`, cleanDomain)
    
    // Формируем URL
    url := fmt.Sprintf("%s?type=op&cmd=%s", c.config.URL, xmlQuery)
    
    // Формируем запрос
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    // Добавляем заголовки
    req.Header.Set("X-PAN-KEY", c.config.Key)
    req.Header.Set("Accept", "application/json")

    // Выполняем запрос
    resp, err := c.client.Do(req)
    if err != nil {
        logWarn("Cloud API request failed",
            "domain", cleanDomain,
            "error", err,
            "duration_ms", time.Since(start).Milliseconds())
        return nil, err
    }
    defer resp.Body.Close()

    // Проверяем статус код
    if resp.StatusCode != http.StatusOK {
        logWarn("Cloud API returned non-OK status",
            "domain", cleanDomain,
            "status", resp.StatusCode,
            "duration_ms", time.Since(start).Milliseconds())
        return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
    }

    // Читаем тело ответа
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response: %w", err)
    }

    // Парсим ответ
    bodyStr := string(body)
    category := 0 // По умолчанию разрешено
    ttl := 300    // По умолчанию TTL
    
    // Ищем категорию в ответе
    if strings.Contains(bodyStr, `"category":1`) {
        category = 1
    } else if strings.Contains(bodyStr, `"category":2`) {
        category = 2
    } else if strings.Contains(bodyStr, `"category":3`) {
        category = 3
    } else if strings.Contains(bodyStr, `"category":8`) {
        category = 8
    } else if strings.Contains(bodyStr, `"category":9`) {
        category = 9
    }

    // Простой парсинг для TTL
    if idx := strings.Index(bodyStr, `"ttl":`); idx != -1 {
        endIdx := strings.Index(bodyStr[idx:], ",")
        if endIdx == -1 {
            endIdx = strings.Index(bodyStr[idx:], "}")
        }
        if endIdx != -1 {
            ttlStr := bodyStr[idx+6 : idx+endIdx]
            // Убираем возможные кавычки и пробелы
            ttlStr = strings.Trim(ttlStr, `" `)
            // Пробуем преобразовать в число
            fmt.Sscanf(ttlStr, "%d", &ttl)
        }
    }

    result := &APIResponse{
        Domain:   cleanDomain,
        Category: category,
        TTL:      ttl,
    }

    logDebug("Cloud API check completed",
        "domain", cleanDomain,
        "category", category,
        "ttl", ttl,
        "duration_ms", time.Since(start).Milliseconds())

    return result, nil
}
