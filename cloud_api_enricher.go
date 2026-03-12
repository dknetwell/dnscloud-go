package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"
)

type CloudAPIEnricher struct {
	cfg     *Config
	client  *http.Client
	limiter *rate.Limiter
}

// Формат ответа CloudAPI:
// <response status="success"><result>{"dns-signature":[{"fqdn":"...","category":5,"ttl":300}]}</result></response>

type cloudAPIXMLResponse struct {
	Status string `xml:"status,attr"`
	Result string `xml:"result"`
}

type cloudAPIJSONResult struct {
	DNSSignature []cloudAPISignature `json:"dns-signature"`
}

type cloudAPISignature struct {
	FQDN     string `json:"fqdn"`
	Category int    `json:"category"`
	TTL      int    `json:"ttl"`
}

func NewCloudAPIEnricher(cfg *Config) *CloudAPIEnricher {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.CloudAPI.InsecureSkipVerify,
		},
	}

	return &CloudAPIEnricher{
		cfg: cfg,
		client: &http.Client{
			Timeout:   time.Duration(cfg.CloudAPI.TimeoutSeconds) * time.Second,
			Transport: tr,
		},
		limiter: rate.NewLimiter(
			rate.Limit(cfg.CloudAPI.RateLimit),
			cfg.CloudAPI.Burst,
		),
	}
}

func (c *CloudAPIEnricher) Name() string {
	return "cloud_api"
}

func (c *CloudAPIEnricher) Enrich(ctx context.Context, domain string, result *DomainResult) error {
	if c.cfg.CloudAPI.Endpoint == "" {
		LogInfoFields("cloud_api", "enrich_skip", map[string]interface{}{
			"domain": domain,
			"reason": "endpoint not configured",
		})
		return nil
	}

	if !c.limiter.Allow() {
		return fmt.Errorf("rate limit exceeded")
	}

	// FIX #4: строим XML cmd корректно.
	// xml.EscapeText защищает от доменов со спецсимволами (<, >, &, ', ")
	// — маловероятно для FQDN, но правильно с точки зрения XML.
	// url.Values.Encode() кодирует весь cmd как параметр URL,
	// а не только домен внутри XML (старое поведение было неверным).
	cmd, err := buildPANOSCmd(domain)
	if err != nil {
		return fmt.Errorf("build cmd: %w", err)
	}

	params := url.Values{}
	params.Set("type", "op")
	params.Set("cmd", cmd)
	reqURL := fmt.Sprintf("%s?%s", c.cfg.CloudAPI.Endpoint, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	// PAN-OS использует X-PAN-KEY, не Bearer
	req.Header.Set("X-PAN-KEY", c.cfg.CloudAPI.APIKey)
	req.Header.Set("Accept", "application/xml")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	// Парсим XML обёртку
	var xmlResp cloudAPIXMLResponse
	if err := xml.NewDecoder(resp.Body).Decode(&xmlResp); err != nil {
		return fmt.Errorf("decode xml: %w", err)
	}

	if xmlResp.Status != "success" {
		return fmt.Errorf("api status: %s", xmlResp.Status)
	}

	// Парсим JSON внутри <result>
	var jsonResult cloudAPIJSONResult
	if err := json.Unmarshal([]byte(xmlResp.Result), &jsonResult); err != nil {
		return fmt.Errorf("decode json result: %w", err)
	}

	if len(jsonResult.DNSSignature) == 0 {
		// Домен не найден в базе — считаем чистым
		result.Category = 0
		result.Action = "allow"
		result.Source = "cloud_api"
		return nil
	}

	sig := jsonResult.DNSSignature[0]

	result.Category = sig.Category
	result.Source = "cloud_api"

	if sig.TTL > 0 {
		result.TTL = sig.TTL
	}

	// Определяем действие по словарю категорий из конфига
	catCfg, known := c.cfg.Categories[sig.Category]
	if known && catCfg.Action == "block" {
		result.Blocked = true
		result.Action = "block"
		// Sinkhole берём из категории, fallback на глобальный
		result.SinkholeIPv4 = catCfg.SinkholeIPv4
		result.SinkholeIPv6 = catCfg.SinkholeIPv6
	} else {
		result.Blocked = false
		result.Action = "allow"
	}

	return nil
}

// buildPANOSCmd формирует XML-команду для PAN-OS API.
// Использует xml.EscapeText для корректного экранирования спецсимволов в домене.
// Для обычных FQDN эскейпинг ничего не меняет, но защищает от edge-case входных данных.
func buildPANOSCmd(domain string) (string, error) {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(domain)); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"<test><dns-proxy><dns-signature><fqdn>%s</fqdn></dns-signature></dns-proxy></test>",
		buf.String(),
	), nil
}
