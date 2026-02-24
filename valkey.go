package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type ValkeyClient struct {
	client *redis.Client
	queue  chan kvPair
}

type kvPair struct {
	key   string
	value *DomainResult
}

func NewValkeyClient(cfg *Config) (*ValkeyClient, error) {

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Valkey.Address,
		Password: cfg.Valkey.Password,
		DB:       cfg.Valkey.DB,
	})

	v := &ValkeyClient{
		client: rdb,
		queue:  make(chan kvPair, 1000),
	}

	go v.writer()

	return v, nil
}

func (v *ValkeyClient) writer() {

	for job := range v.queue {

		data, _ := json.Marshal(job.value)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

		v.client.Set(ctx,
			job.key,
			data,
			time.Duration(job.value.TTL)*time.Second)

		cancel()
	}
}

func (v *ValkeyClient) SetAsync(key string, value *DomainResult) {

	select {
	case v.queue <- kvPair{key: key, value: value}:
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
