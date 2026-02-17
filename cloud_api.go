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

type cloudResponse struct {
	Blocked  bool   `json:"blocked"`
	Category int    `json:"category"`
	TTL      int    `json:"ttl"`
	CNAME    string `json:"cname"`
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
			Timeout:   2 * time.Second,
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

	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}

	req, _ := http.NewRequestWithContext(ctx,
		"GET",
		c.cfg.CloudAPI.Endpoint+"?domain="+domain,
		nil,
	)

	req.Header.Set("Authorization", "Bearer "+c.cfg.CloudAPI.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var parsed cloudResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return err
	}

	result.Blocked = parsed.Blocked
	result.TTL = parsed.TTL
	result.CNAME = parsed.CNAME

	return nil
}
