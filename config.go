package main

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {

	Logging struct {
		Level  string `yaml:"level"`
		Syslog bool   `yaml:"syslog"`
	} `yaml:"logging"`

	DNS struct {
		ListenUDP     string   `yaml:"listen_udp"`
		ListenTCP     string   `yaml:"listen_tcp"`
		Upstream      []string `yaml:"upstream"`
		SinkholeIPv4  string   `yaml:"sinkhole_ipv4"`
		SinkholeIPv6  string   `yaml:"sinkhole_ipv6"`
		MaxPacketSize int      `yaml:"max_packet_size"`
	} `yaml:"dns"`

	CloudAPI struct {
		Endpoint           string  `yaml:"endpoint"`
		APIKey             string  `yaml:"api_key"`
		InsecureSkipVerify bool    `yaml:"insecure_skip_verify"`
		RateLimit          float64 `yaml:"rate_limit"`
		Burst              int     `yaml:"burst"`
		TimeoutSeconds     int     `yaml:"timeout_seconds"`
	} `yaml:"cloud_api"`

	TTL struct {
		Default int `yaml:"default"`
		Min     int `yaml:"min"`
		Max     int `yaml:"max"`
	} `yaml:"ttl"`

	Cache struct {
		MaxCost int64 `yaml:"max_cost"`
	} `yaml:"cache"`

	Valkey struct {
		Address  string `yaml:"address"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"valkey"`

	Engine struct {
		WorkerCount     int `yaml:"worker_count"`
		WorkerQueueSize int `yaml:"worker_queue_size"`
	} `yaml:"engine"`

	HTTP struct {
		Listen string `yaml:"listen"`
	} `yaml:"http"`
}

func LoadConfig() (*Config, error) {

	cfg := &Config{}

	// 1️⃣ YAML optional
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			_ = yaml.Unmarshal(data, cfg)
		}
	}

	// 2️⃣ ENV overrides (production-first)

	// Logging
	cfg.Logging.Level = getEnv("LOG_LEVEL", cfg.Logging.Level)
	cfg.Logging.Syslog = getEnvBool("LOG_SYSLOG", cfg.Logging.Syslog)

	// DNS
	cfg.DNS.ListenUDP = getEnv("DNS_LISTEN_UDP", defaultStr(cfg.DNS.ListenUDP, ":53"))
	cfg.DNS.ListenTCP = getEnv("DNS_LISTEN_TCP", defaultStr(cfg.DNS.ListenTCP, ":53"))
	cfg.DNS.MaxPacketSize = getEnvInt("DNS_MAX_PACKET", defaultInt(cfg.DNS.MaxPacketSize, 1232))
	cfg.DNS.SinkholeIPv4 = getEnv("DNS_SINKHOLE_IPV4", defaultStr(cfg.DNS.SinkholeIPv4, "0.0.0.0"))
	cfg.DNS.SinkholeIPv6 = getEnv("DNS_SINKHOLE_IPV6", defaultStr(cfg.DNS.SinkholeIPv6, "::"))

	// HTTP
	cfg.HTTP.Listen = getEnv("HTTP_LISTEN", defaultStr(cfg.HTTP.Listen, ":8080"))

	// Valkey
	cfg.Valkey.Address = getEnv("VALKEY_ADDR", defaultStr(cfg.Valkey.Address, "valkey:6379"))
	cfg.Valkey.Password = getEnv("VALKEY_PASSWORD", cfg.Valkey.Password)
	cfg.Valkey.DB = getEnvInt("VALKEY_DB", cfg.Valkey.DB)

	// Engine
	cfg.Engine.WorkerCount = getEnvInt("ENGINE_WORKERS", defaultInt(cfg.Engine.WorkerCount, 100))
	cfg.Engine.WorkerQueueSize = getEnvInt("ENGINE_QUEUE", defaultInt(cfg.Engine.WorkerQueueSize, 1000))

	// CloudAPI
	cfg.CloudAPI.Endpoint = getEnv("CLOUDAPI_ENDPOINT", cfg.CloudAPI.Endpoint)
	cfg.CloudAPI.APIKey = getEnv("CLOUDAPI_APIKEY", cfg.CloudAPI.APIKey)
	cfg.CloudAPI.TimeoutSeconds = getEnvInt("CLOUDAPI_TIMEOUT", defaultInt(cfg.CloudAPI.TimeoutSeconds, 2))
	cfg.CloudAPI.RateLimit = getEnvFloat("CLOUDAPI_RPS", cfg.CloudAPI.RateLimit)
	cfg.CloudAPI.Burst = getEnvInt("CLOUDAPI_BURST", defaultInt(cfg.CloudAPI.Burst, 100))
	cfg.CloudAPI.InsecureSkipVerify = getEnvBool("CLOUDAPI_INSECURE", cfg.CloudAPI.InsecureSkipVerify)

	// TTL
	cfg.TTL.Default = getEnvInt("TTL_DEFAULT", defaultInt(cfg.TTL.Default, 300))
	cfg.TTL.Min = getEnvInt("TTL_MIN", defaultInt(cfg.TTL.Min, 60))
	cfg.TTL.Max = getEnvInt("TTL_MAX", defaultInt(cfg.TTL.Max, 86400))

	// Cache
	cfg.Cache.MaxCost = getEnvInt64("CACHE_MAX_COST", defaultInt64(cfg.Cache.MaxCost, 1<<30))

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

func getEnvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
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

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func defaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func defaultInt64(v, def int64) int64 {
	if v == 0 {
		return def
	}
	return v
}
