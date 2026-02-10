package main

import (
    "context"
    "crypto/tls"
    "encoding/json"
    "encoding/xml"
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
            InsecureSkipVerify: true, // Игнорируем ошибки сертификатов
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

    // Формируем XML запрос согласно curl примеру
    xmlQuery := fmt.Sprintf(`<test><dns-proxy><dns-signature><fqdn>%s</fqdn></dns-signature></dns-proxy></test>`, cleanDomain)
    
    // Формируем запрос
    req, err := http.NewRequestWithContext(ctx, "GET", c.config.URL, nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    // Добавляем заголовки согласно curl примеру
    req.Header.Set("X-PAN-KEY", c.config.Key)
    req.Header.Set("Accept", "application/json")

    // Добавляем параметры согласно curl примеру
    q := req.URL.Query()
    q.Add("type", "op")
    q.Add("cmd", xmlQuery)
    req.URL.RawQuery = q.Encode()

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

    // Парсим XML ответ
    result, err := parseXMLResponse(body)
    if err != nil {
        return nil, fmt.Errorf("parse response: %w", err)
    }

    // Устанавливаем домен в результат
    result.Domain = cleanDomain

    logDebug("Cloud API check completed",
        "domain", cleanDomain,
        "category", result.Category,
        "duration_ms", time.Since(start).Milliseconds())

    return result, nil
}

// Структуры для парсинга XML
type XMLResponse struct {
    Result struct {
        Content string `xml:",innerxml"`
    } `xml:"result"`
}

type DNSResult struct {
    Fqdn     string `json:"fqdn"`
    Category int    `json:"category"`
    TTL      int    `json:"ttl"`
}

func parseXMLResponse(body []byte) (*APIResponse, error) {
    // Сначала парсим XML
    var xmlResp XMLResponse
    if err := xml.Unmarshal(body, &xmlResp); err != nil {
        return nil, fmt.Errorf("unmarshal XML: %w", err)
    }

    // Контент внутри тега result - это JSON строка
    // Очищаем возможные символы переноса строки
    jsonContent := strings.TrimSpace(xmlResp.Result.Content)
    
    // Парсим JSON
    var apiResp struct {
        DNSSignature []DNSResult `json:"dns-signature"`
    }
    
    if err := json.Unmarshal([]byte(jsonContent), &apiResp); err != nil {
        return nil, fmt.Errorf("unmarshal JSON: %w", err)
    }

    if len(apiResp.DNSSignature) == 0 {
        return nil, fmt.Errorf("no DNS signature in response")
    }

    result := &APIResponse{
        Category: apiResp.DNSSignature[0].Category,
        TTL:      apiResp.DNSSignature[0].TTL,
    }

    return result, nil
}
