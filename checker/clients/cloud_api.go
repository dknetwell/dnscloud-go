package clients

import (
    "context"
    "crypto/tls"
    "encoding/json"
    "encoding/xml"
    "fmt"
    "io"
    "net/http"
    "time"

    "config"
    "logger"
)

type CloudAPIClient struct {
    config *config.CloudAPIConfig
    client *http.Client
}

type APIResponse struct {
    XMLName  xml.Name `xml:"response"`
    Status   string   `xml:"status,attr"`
    Result   string   `xml:"result"`
}

type ParsedResult struct {
    DNSSignature []struct {
        FQDN     string `json:"fqdn"`
        Category int    `json:"category"`
        TTL      int    `json:"ttl"`
    } `json:"dns-signature"`
}

func NewCloudAPIClient(cfg *config.CloudAPIConfig) *CloudAPIClient {
    return &CloudAPIClient{
        config: cfg,
        client: &http.Client{
            Timeout: time.Duration(cfg.Timeout) * time.Millisecond,
            Transport: &http.Transport{
                TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
            },
        },
    }
}

func (c *CloudAPIClient) Check(ctx context.Context, domain string) (*APIResponse, error) {
    start := time.Now()

    // Формируем XML запрос
    xmlCmd := fmt.Sprintf(
        `<test><dns-proxy><dns-signature><fqdn>%s</fqdn></dns-signature></dns-proxy></test>`,
        domain)

    // Создаем запрос
    req, err := http.NewRequestWithContext(ctx, "GET", c.config.URL, nil)
    if err != nil {
        return nil, err
    }

    // Добавляем параметры
    q := req.URL.Query()
    q.Add("type", "op")
    q.Add("cmd", xmlCmd)
    req.URL.RawQuery = q.Encode()

    // Добавляем заголовки
    req.Header.Set("X-PAN-KEY", c.config.Key)
    req.Header.Set("Accept", "application/json")

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

    // Читаем ответ
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    logger.Debug("Cloud API response",
        "domain", domain,
        "status", resp.StatusCode,
        "duration_ms", time.Since(start).Milliseconds())

    // Парсим XML
    var apiResp APIResponse
    if err := xml.Unmarshal(body, &apiResp); err != nil {
        return nil, err
    }

    if apiResp.Status != "success" {
        return nil, fmt.Errorf("API returned status: %s", apiResp.Status)
    }

    // Парсим JSON внутри XML
    var parsed ParsedResult
    if err := json.Unmarshal([]byte(apiResp.Result), &parsed); err != nil {
        return nil, err
    }

    if len(parsed.DNSSignature) == 0 {
        return nil, fmt.Errorf("no signature in response")
    }

    sig := parsed.DNSSignature[0]

    return &APIResponse{
        Domain:   sig.FQDN,
        Category: sig.Category,
        TTL:      sig.TTL,
    }, nil
}
