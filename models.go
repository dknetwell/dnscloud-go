package main

import (
	"time"
)

type DomainResult struct {
	Domain    string    `json:"domain"`
	Category  int       `json:"category"`
	Action    string    `json:"action"`
	TTL       int       `json:"ttl"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`

	Blocked      bool   `json:"blocked"`
	SinkholeIPv4 string `json:"sinkhole_ipv4,omitempty"` // выбранный sinkhole для этого домена
	SinkholeIPv6 string `json:"sinkhole_ipv6,omitempty"`
	Negative     bool   `json:"-"`
}

type Stats struct {
	TotalRequests   int64 `json:"total_requests"`
	CacheHits       int64 `json:"cache_hits"`
	CacheMisses     int64 `json:"cache_misses"`
	APICalls        int64 `json:"api_calls"`
	EnrichmentQueue int   `json:"enrichment_queue"`
	AvgLatencyNs    int64 `json:"avg_latency_ns"`
}
