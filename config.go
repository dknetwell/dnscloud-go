package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// CategoryConfig — настройки для одной категории угроз
type CategoryConfig struct {
	Name         string `yaml:"name"`
	Action       string `yaml:"action"`        // "block" | "allow"
	SinkholeIPv4 string `yaml:"sinkhole_ipv4"` // переопределяет глобальный; "" = fallback
	SinkholeIPv6 string `yaml:"sinkhole_ipv6"`
	CacheTTL     int    `yaml:"cache_ttl"`     // TTL записи в Valkey (секунды)
}

type Config struct {
	Logging struct {
		Level string `yaml:"level"`
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

	// Словарь категорий: ключ — int из CloudAPI ответа.
	// Полный список и значения — в config.yaml, секция categories.
	Categories map[int]CategoryConfig `yaml:"categories"`

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
		Listen string `yaml:"http"`
	} `yaml:"http"`
}

func LoadConfig() (*Config, error) {
	cfg := &Config{}

	// Шаг 1 — читаем YAML (обязательный, путь из ENV или дефолт)
	configPath := getEnv("CONFIG_PATH", "/app/config/config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", configPath, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Шаг 2 — переменные окружения переопределяют YAML
	// Только секреты и инфраструктурные параметры которые меняются между окружениями.
	// Всё остальное (категории, TTL, воркеры) — только в config.yaml.

	// CloudAPI — секреты всегда в .env, не в yaml
	if v := os.Getenv("CLOUDAPI_ENDPOINT"); v != "" {
		cfg.CloudAPI.Endpoint = v
	}
	if v := os.Getenv("CLOUDAPI_APIKEY"); v != "" {
		cfg.CloudAPI.APIKey = v
	}
	if v := os.Getenv("CLOUDAPI_INSECURE"); v != "" {
		cfg.CloudAPI.InsecureSkipVerify = getEnvBool("CLOUDAPI_INSECURE", false)
	}
	if v := os.Getenv("CLOUDAPI_TIMEOUT"); v != "" {
		cfg.CloudAPI.TimeoutSeconds = getEnvInt("CLOUDAPI_TIMEOUT", cfg.CloudAPI.TimeoutSeconds)
	}
	if v := os.Getenv("CLOUDAPI_RPS"); v != "" {
		cfg.CloudAPI.RateLimit = getEnvFloat("CLOUDAPI_RPS", cfg.CloudAPI.RateLimit)
	}
	if v := os.Getenv("CLOUDAPI_BURST"); v != "" {
		cfg.CloudAPI.Burst = getEnvInt("CLOUDAPI_BURST", cfg.CloudAPI.Burst)
	}

	// DNS — могут меняться в docker-compose.yml
	if v := os.Getenv("DNS_LISTEN_UDP"); v != "" {
		cfg.DNS.ListenUDP = v
	}
	if v := os.Getenv("DNS_LISTEN_TCP"); v != "" {
		cfg.DNS.ListenTCP = v
	}
	if v := os.Getenv("DNS_UPSTREAMS"); v != "" {
		cfg.DNS.Upstream = strings.Split(v, ",")
	}
	if v := os.Getenv("DNS_SINKHOLE_IPV4"); v != "" {
		cfg.DNS.SinkholeIPv4 = v
	}
	if v := os.Getenv("DNS_SINKHOLE_IPV6"); v != "" {
		cfg.DNS.SinkholeIPv6 = v
	}

	// Valkey — адрес меняется между окружениями
	if v := os.Getenv("VALKEY_ADDR"); v != "" {
		cfg.Valkey.Address = v
	}
	if v := os.Getenv("VALKEY_PASSWORD"); v != "" {
		cfg.Valkey.Password = v
	}

	// HTTP
	if v := os.Getenv("HTTP_LISTEN"); v != "" {
		cfg.HTTP.Listen = v
	}

	// Logging
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}

	return cfg, nil
}
