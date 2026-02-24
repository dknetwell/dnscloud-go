package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "dns_requests_total",
			Help: "Total DNS requests received",
		},
	)

	requestsBlocked = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "dns_requests_blocked_total",
			Help: "Total DNS requests blocked (sinkholed)",
		},
	)

	cacheHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dns_cache_hits_total",
			Help: "Cache hits by layer (l1, l2)",
		},
		[]string{"layer"},
	)

	enricherCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dns_enricher_calls_total",
			Help: "Enricher calls by name and status",
		},
		[]string{"enricher", "status"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dns_request_duration_ms",
			Help:    "DNS request processing latency in milliseconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 25, 50, 100, 250, 500},
		},
		[]string{"blocked"},
	)

	enricherDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dns_enricher_duration_ms",
			Help:    "Enricher call latency in milliseconds",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
		},
		[]string{"enricher"},
	)

	enricherQueueSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "dns_enricher_queue_size",
			Help: "Current enrichment job queue size",
		},
	)
)

func initMetrics() {
	prometheus.MustRegister(
		requestsTotal,
		requestsBlocked,
		cacheHitsTotal,
		enricherCallsTotal,
		requestDuration,
		enricherDuration,
		enricherQueueSize,
	)
}

func promHandler() http.Handler {
	return promhttp.Handler()
}
