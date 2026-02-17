package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type Enricher interface {
	Name() string
	Enrich(ctx context.Context, domain string, result *DomainResult) error
}

type CheckEngine struct {
	cfg        *Config
	cache      *Cache
	enrichers  []Enricher
	sf         singleflight.Group
	jobQueue   chan enrichmentJob
	wg         sync.WaitGroup
}

type enrichmentJob struct {
	ctx    context.Context
	domain string
	result *DomainResult
}

func NewCheckEngine(cfg *Config, cache *Cache, enrichers []Enricher) *CheckEngine {

	engine := &CheckEngine{
		cfg:       cfg,
		cache:     cache,
		enrichers: enrichers,
		jobQueue:  make(chan enrichmentJob, cfg.Engine.WorkerQueueSize),
	}

	for i := 0; i < cfg.Engine.WorkerCount; i++ {
		engine.wg.Add(1)
		go engine.worker()
	}

	return engine
}

func (e *CheckEngine) worker() {
	defer e.wg.Done()
	for job := range e.jobQueue {
		for _, enricher := range e.enrichers {
			_ = enricher.Enrich(job.ctx, job.domain, job.result)
		}
	}
}

func (e *CheckEngine) CheckDomain(ctx context.Context, domain string) (*DomainResult, error) {

	if cached, ok := e.cache.Get(domain); ok {
		incrementCacheHit()
		return cached, nil
	}

	val, err, _ := e.sf.Do(domain, func() (interface{}, error) {

		result := &DomainResult{
			Domain: domain,
		}

		err := e.resolveRealIP(ctx, result)
		if err != nil {
			return nil, err
		}

		select {
		case e.jobQueue <- enrichmentJob{
			ctx:    ctx,
			domain: domain,
			result: result,
		}:
		default:
			// очередь переполнена — деградируем gracefully
		}

		e.applyTTLPolicy(result)
		e.cache.Set(domain, result)

		return result, nil
	})

	if err != nil {
		e.cache.SetNegative(domain, 20*time.Second)
		return nil, err
	}

	return val.(*DomainResult), nil
}

func (e *CheckEngine) resolveRealIP(ctx context.Context, result *DomainResult) error {

	ips, err := netResolver.LookupIPAddr(ctx, result.Domain)
	if err != nil {
		return err
	}

	for _, ip := range ips {
		if ip.IP.To4() != nil {
			result.RealIP = ip.IP
		} else {
			result.RealIPv6 = ip.IP
		}
	}

	if result.RealIP == nil && result.RealIPv6 == nil {
		return errors.New("no ip resolved")
	}

	return nil
}

func (e *CheckEngine) applyTTLPolicy(result *DomainResult) {

	ttl := result.TTL
	if ttl == 0 {
		ttl = e.cfg.TTL.Default
	}

	if ttl < e.cfg.TTL.Min {
		ttl = e.cfg.TTL.Min
	}
	if ttl > e.cfg.TTL.Max {
		ttl = e.cfg.TTL.Max
	}

	result.TTL = ttl
}
