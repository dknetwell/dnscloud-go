package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type CloudAPIEnricher struct {
	cfg     *Config
	client  *http.Client
	limiter *rate.Limiter
}

// XML структура ответа:
// <response status="success"><result>{"dns-signature":[...]}</result></response>

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
		return nil
	}

	if !c.limiter.Allow() {
		return fmt.Errorf("rate limit exceeded")
	}

	req, err := http.NewRequestWithContext(ctx,
		"GET",
		c.cfg.CloudAPI.Endpoint+"?domain="+domain,
		nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.CloudAPI.APIKey)
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
		return nil
	}

	sig := jsonResult.DNSSignature[0]

	result.Category = sig.Category
	result.Source = "cloud_api"

	if sig.TTL > 0 {
		result.TTL = sig.TTL
	}

	// Определяем действие по категории через конфиг
	catCfg, known := c.cfg.Categories[sig.Category]
	if known && catCfg.Action == "block" {
		result.Blocked = true
		result.Action = "block"
		result.SinkholeIPv4 = catCfg.SinkholeIPv4
		result.SinkholeIPv6 = catCfg.SinkholeIPv6
	} else {
		result.Blocked = false
		result.Action = "allow"
	}

	return nil
}
