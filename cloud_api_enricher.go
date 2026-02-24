package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
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

type CloudAPIResponse struct {
	Domain   string `json:"domain"`
	Category int    `json:"category"`
	TTL      int    `json:"ttl"`
	Action   string `json:"action"`
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
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var apiResp CloudAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	result.Category = apiResp.Category
	result.Action = apiResp.Action
	result.Source = "cloud_api"

	if apiResp.TTL > 0 {
		result.TTL = apiResp.TTL
	}

	if apiResp.Category != 0 {
		result.Blocked = true
	}

	return nil
}
