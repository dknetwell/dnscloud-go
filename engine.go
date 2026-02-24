package main

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

type enrichmentJob struct {
	domain string
	result *DomainResult
}

type CheckEngine struct {
	cfg      *Config
	cache    *Cache
	valkey   *ValkeyClient
	enrichers []Enricher

	jobs      chan enrichmentJob
	wg        sync.WaitGroup
	sf        singleflight.Group

	stats Stats
}

func NewCheckEngine(cfg *Config, cache *Cache, valkey *ValkeyClient, enrichers []Enricher) *CheckEngine {

	e := &CheckEngine{
		cfg:       cfg,
		cache:     cache,
		valkey:    valkey,
		enrichers: enrichers,
		jobs:      make(chan enrichmentJob, cfg.Engine.WorkerQueueSize),
	}

	for i := 0; i < cfg.Engine.WorkerCount; i++ {
		e.wg.Add(1)
		go e.worker()
	}

	return e
}

func (e *CheckEngine) Shutdown() {
	close(e.jobs)
	e.wg.Wait()
}

func (e *CheckEngine) worker() {
	defer e.wg.Done()

	for job := range e.jobs {
		ctx, cancel := context.WithTimeout(context.Background(),
			time.Duration(e.cfg.CloudAPI.TimeoutSeconds)*time.Second)

		for _, enricher := range e.enrichers {
			enricher.Enrich(ctx, job.domain, job.result)
		}

		cancel()

		e.cache.Set(job.domain, job.result)
		e.valkey.SetAsync(job.domain, job.result)

		atomic.AddInt64(&e.stats.APICalls, 1)
	}
}

func (e *CheckEngine) CheckDomain(domain string) (*DomainResult, error) {

	start := time.Now()
	atomic.AddInt64(&e.stats.TotalRequests, 1)

	// L1 cache
	if r, ok := e.cache.Get(domain); ok {
		atomic.AddInt64(&e.stats.CacheHits, 1)
		return r, nil
	}

	atomic.AddInt64(&e.stats.CacheMisses, 1)

	// L2 cache (Valkey)
	if r, ok := e.valkey.Get(domain); ok {
		e.cache.Set(domain, r)
		return r, nil
	}

	// singleflight
	v, _, _ := e.sf.Do(domain, func() (interface{}, error) {

		result := &DomainResult{
			Domain:    domain,
			Category:  0,
			Action:    "allow",
			TTL:       e.cfg.TTL.Default,
			Source:    "engine",
			Timestamp: time.Now(),
		}

		select {
		case e.jobs <- enrichmentJob{domain: domain, result: result}:
		default:
			// queue full → negative short TTL
			result.Negative = true
			result.TTL = 10
		}

		return result, nil
	})

	res := v.(*DomainResult)

	// clamp TTL
	if res.TTL < e.cfg.TTL.Min {
		res.TTL = e.cfg.TTL.Min
	}
	if res.TTL > e.cfg.TTL.Max {
		res.TTL = e.cfg.TTL.Max
	}

	res.Timestamp = time.Now()
	atomic.AddInt64(&e.stats.AvgLatencyNs,
        time.Since(start).Nanoseconds())

	return res, nil
}

func (e *CheckEngine) GetStats() Stats {

	s := e.stats
	s.EnrichmentQueue = len(e.jobs)

	total := atomic.LoadInt64(&s.TotalRequests)
	if total > 0 {
		avg := atomic.LoadInt64(&s.AvgLatencyNs) / total
		s.AvgLatencyNs = avg
	}

	return s
}
