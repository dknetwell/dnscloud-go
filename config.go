package main

import (
	"os"
	"strconv"
)

type Config struct {
	DNS struct {
		ListenUDP     string
		ListenTCP     string
		MaxPacketSize int
		SinkholeIPv4  string
		SinkholeIPv6  string
	}

	HTTP struct {
		Listen string
	}

	Cache struct {
		MaxCost     int64
		NumCounters int64
		BufferItems int64
	}

	Valkey struct {
		Address  string
		Password string
		DB       int
	}

	Engine struct {
		WorkerCount int
		QueueSize   int
	}

	CloudAPI struct {
		Endpoint           string
		APIKey             string
		TimeoutSeconds     int
		RateLimit          int
		Burst              int
		InsecureSkipVerify bool
	}
}

func LoadConfig() (*Config, error) {

	cfg := &Config{}

	// DNS
	cfg.DNS.ListenUDP = getEnv("DNS_LISTEN_UDP", ":53")
	cfg.DNS.ListenTCP = getEnv("DNS_LISTEN_TCP", ":53")
	cfg.DNS.MaxPacketSize = getEnvInt("DNS_MAX_PACKET", 1232)
	cfg.DNS.SinkholeIPv4 = getEnv("DNS_SINKHOLE_IPV4", "0.0.0.0")
	cfg.DNS.SinkholeIPv6 = getEnv("DNS_SINKHOLE_IPV6", "::")

	// HTTP
	cfg.HTTP.Listen = getEnv("HTTP_LISTEN", ":8080")

	// Cache
	cfg.Cache.MaxCost = getEnvInt64("CACHE_MAX_COST", 1<<30)
	cfg.Cache.NumCounters = getEnvInt64("CACHE_NUM_COUNTERS", 1e7)
	cfg.Cache.BufferItems = getEnvInt64("CACHE_BUFFER_ITEMS", 64)

	// Valkey
	cfg.Valkey.Address = getEnv("VALKEY_ADDR", "valkey:6379")
	cfg.Valkey.Password = os.Getenv("VALKEY_PASSWORD")
	cfg.Valkey.DB = getEnvInt("VALKEY_DB", 0)

	// Engine
	cfg.Engine.WorkerCount = getEnvInt("ENGINE_WORKERS", 100)
	cfg.Engine.QueueSize = getEnvInt("ENGINE_QUEUE", 1000)

	// CloudAPI
	cfg.CloudAPI.Endpoint = getEnv("CLOUDAPI_ENDPOINT", "")
	cfg.CloudAPI.APIKey = os.Getenv("CLOUDAPI_APIKEY")
	cfg.CloudAPI.TimeoutSeconds = getEnvInt("CLOUDAPI_TIMEOUT", 2)
	cfg.CloudAPI.RateLimit = getEnvInt("CLOUDAPI_RPS", 500)
	cfg.CloudAPI.Burst = getEnvInt("CLOUDAPI_BURST", 100)
	cfg.CloudAPI.InsecureSkipVerify = getEnvBool("CLOUDAPI_INSECURE", false)

	return cfg, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getEnvInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
