package main

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	dnsRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "dns_requests_total",
		})

	dnsLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "dns_latency_seconds",
			Buckets: prometheus.DefBuckets,
		})

	cacheHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "dns_cache_hits_total",
		})
)

func initMetrics() {
	prometheus.MustRegister(dnsRequests, dnsLatency, cacheHits)
}

func incrementDNSCounter() {
	dnsRequests.Inc()
}

func observeDNSLatency(d time.Duration) {
	dnsLatency.Observe(d.Seconds())
}

func incrementCacheHit() {
	cacheHits.Inc()
}
