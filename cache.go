package main

import (
	"time"

	"github.com/dgraph-io/ristretto"
)

type Cache struct {
	store *ristretto.Cache
}

func NewCache(cfg *Config) *Cache {

	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,
		MaxCost:     cfg.Cache.MaxCost,
		BufferItems: 64,
	})

	return &Cache{store: cache}
}

func (c *Cache) Get(key string) (*DomainResult, bool) {

	val, ok := c.store.Get(key)
	if !ok {
		return nil, false
	}
	return val.(*DomainResult), true
}

func (c *Cache) Set(key string, value *DomainResult) {

	cost := int64(len(key) + 64)
	c.store.SetWithTTL(key, value, cost, time.Duration(value.TTL)*time.Second)
}

func (c *Cache) SetNegative(key string, ttl time.Duration) {
	c.store.SetWithTTL(key, &DomainResult{Domain: key}, 1, ttl)
}
