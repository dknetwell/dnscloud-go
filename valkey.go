package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type ValkeyClient struct {
	client *redis.Client
	cfg    *Config
	queue  chan kvPair
}

type kvPair struct {
	key      string
	value    *DomainResult
	cacheTTL int // TTL для Valkey из конфига категории
}

func NewValkeyClient(cfg *Config) (*ValkeyClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Valkey.Address,
		Password: cfg.Valkey.Password,
		DB:       cfg.Valkey.DB,
	})

	v := &ValkeyClient{
		client: rdb,
		cfg:    cfg,
		queue:  make(chan kvPair, 1000),
	}

	go v.writer()
	return v, nil
}

func (v *ValkeyClient) writer() {
	for job := range v.queue {
		data, _ := json.Marshal(job.value)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		v.client.Set(ctx, job.key, data, time.Duration(job.cacheTTL)*time.Second)
		cancel()
	}
}

// SetAsync записывает результат в Valkey с TTL из словаря категорий.
// Если категория не найдена — используется TTL из поля result.TTL.
func (v *ValkeyClient) SetAsync(key string, value *DomainResult) {
	cacheTTL := value.TTL // fallback

	if catCfg, ok := v.cfg.Categories[value.Category]; ok && catCfg.CacheTTL > 0 {
		cacheTTL = catCfg.CacheTTL
	}

	select {
	case v.queue <- kvPair{key: key, value: value, cacheTTL: cacheTTL}:
	default:
		// drop if full
	}
}

func (v *ValkeyClient) Get(key string) (*DomainResult, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	data, err := v.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}

	var r DomainResult
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, false
	}

	return &r, true
}
