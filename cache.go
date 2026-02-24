package main

import (
	"encoding/json"
	"time"

	"github.com/dgraph-io/ristretto"
)

type Cache struct {
	rc  *ristretto.Cache
	cfg *Config
}

func NewCache(cfg *Config) *Cache {

	rc, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,
		MaxCost:     cfg.Cache.MaxCost,
		BufferItems: 64,
	})

	return &Cache{
		rc:  rc,
		cfg: cfg,
	}
}

func (c *Cache) Get(key string) (*DomainResult, bool) {

	v, ok := c.rc.Get(key)
	if !ok {
		return nil, false
	}

	return v.(*DomainResult), true
}

func (c *Cache) Set(key string, value *DomainResult) {

	data, _ := json.Marshal(value)
	cost := int64(len(data))

	c.rc.SetWithTTL(key, value, cost,
		time.Duration(value.TTL)*time.Second)
}
