package main

import (
    "context"
    "crypto/tls"
    "encoding/json"
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

    // Выполняем запрос с логированием URL для отладки
    logDebug("Cloud API request", "url", req.URL.String(), "domain", cleanDomain)
    
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
    result, err := parseAPIResponse(body, cleanDomain)
    if err != nil {
        logWarn("Failed to parse API response",
            "domain", cleanDomain,
            "error", err,
            "response", string(body))
        return nil, err
    }

    logDebug("Cloud API check completed",
        "domain", cleanDomain,
        "category", result.Category,
        "duration_ms", time.Since(start).Milliseconds())

    return result, nil
}

// Структуры для парсинга ответа
type PANResponse struct {
    Response struct {
        Status string `json:"status"`
        Result struct {
            Content string `json:"-"`
        } `json:"result"`
    } `json:"response"`
}

type DNSSignature struct {
    Fqdn     string `json:"fqdn"`
    Category int    `json:"category"`
    TTL      int    `json:"ttl"`
}

func parseAPIResponse(body []byte, domain string) (*APIResponse, error) {
    responseStr := string(body)
    
    // Пробуем парсить как JSON
    var panResp PANResponse
    if err := json.Unmarshal(body, &panResp); err != nil {
        // Если не JSON, пробуем найти категорию в строке
        return parseResponseHeuristic(responseStr, domain)
    }

    // Если ответ успешный, но нет данных - возвращаем категорию 0 (разрешено)
    if panResp.Response.Status != "success" {
        return &APIResponse{
            Domain:   domain,
            Category: 0, // Разрешено по умолчанию
            TTL:      300,
        }, nil
    }

    // Пробуем извлечь JSON из Content
    var dnsData struct {
        DNSSignature []DNSSignature `json:"dns-signature"`
    }
    
    if err := json.Unmarshal([]byte(panResp.Response.Result.Content), &dnsData); err != nil {
        // Если не удалось, используем эвристический парсинг
        return parseResponseHeuristic(panResp.Response.Result.Content, domain)
    }

    if len(dnsData.DNSSignature) == 0 {
        return &APIResponse{
            Domain:   domain,
            Category: 0,
            TTL:      300,
        }, nil
    }

    return &APIResponse{
        Domain:   domain,
        Category: dnsData.DNSSignature[0].Category,
        TTL:      dnsData.DNSSignature[0].TTL,
    }, nil
}

func parseResponseHeuristic(responseStr, domain string) (*APIResponse, error) {
    // Простой эвристический парсинг
    category := 0 // По умолчанию разрешено
    ttl := 300    // По умолчанию TTL

    // Ищем категорию в ответе
    if strings.Contains(responseStr, `"category":1`) || strings.Contains(responseStr, "category\": 1") {
        category = 1
    } else if strings.Contains(responseStr, `"category":2`) || strings.Contains(responseStr, "category\": 2") {
        category = 2
    } else if strings.Contains(responseStr, `"category":3`) || strings.Contains(responseStr, "category\": 3") {
        category = 3
    } else if strings.Contains(responseStr, `"category":8`) || strings.Contains(responseStr, "category\": 8") {
        category = 8
    } else if strings.Contains(responseStr, `"category":9`) || strings.Contains(responseStr, "category\": 9") {
        category = 9
    }

    return &APIResponse{
        Domain:   domain,
        Category: category,
        TTL:      ttl,
    }, nil
}
