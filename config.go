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

	Valkey struct {
		Address  string
		Password string
		DB       int
	}

	CloudAPI struct {
		URL     string
		Timeout int
		RPS     int
	}

	Workers int
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

	// Valkey
	cfg.Valkey.Address = getEnv("VALKEY_ADDR", "valkey:6379")
	cfg.Valkey.Password = os.Getenv("VALKEY_PASSWORD")
	cfg.Valkey.DB = getEnvInt("VALKEY_DB", 0)

	// CloudAPI
	cfg.CloudAPI.URL = getEnv("CLOUDAPI_URL", "")
	cfg.CloudAPI.Timeout = getEnvInt("CLOUDAPI_TIMEOUT_MS", 2000)
	cfg.CloudAPI.RPS = getEnvInt("CLOUDAPI_RPS", 500)

	cfg.Workers = getEnvInt("WORKERS", 100)

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
