package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
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

	if !c.limiter.Allow() {
		return nil
	}

	req, _ := http.NewRequestWithContext(ctx,
		"GET",
		c.cfg.CloudAPI.Endpoint+"?domain="+domain,
		nil)

	req.Header.Set("Authorization", c.cfg.CloudAPI.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var apiResp CloudAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}

	result.Category = apiResp.Category
	result.Action = apiResp.Action
	result.TTL = apiResp.TTL
	result.Source = "cloud_api"

	if apiResp.Category != 0 {
		result.Blocked = true
	}

	return nil
}
