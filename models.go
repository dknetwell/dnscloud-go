package main

import (
	"net"
	"time"
)

type DomainResult struct {
	Domain    string    `json:"domain"`
	Category  int       `json:"category"`
	Action    string    `json:"action"`
	TTL       int       `json:"ttl"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`

	Blocked  bool   `json:"blocked"`
	RealIP   net.IP `json:"-"`
	RealIPv6 net.IP `json:"-"`
	Negative bool   `json:"-"`
}

type Stats struct {
	TotalRequests   int64         `json:"total_requests"`
	CacheHits       int64         `json:"cache_hits"`
	CacheMisses     int64         `json:"cache_misses"`
	APICalls        int64         `json:"api_calls"`
	EnrichmentQueue int           `json:"enrichment_queue"`
	AvgLatency      time.Duration `json:"avg_latency"`
}
