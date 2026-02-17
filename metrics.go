package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "dns_requests_total",
			Help: "Total DNS requests",
		},
	)
)

func initMetrics() {
	prometheus.MustRegister(requestsTotal)
}

func promHandler() http.Handler {
	return promhttp.Handler()
}
