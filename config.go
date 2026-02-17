package main

import (
	"os"

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

func LoadConfig(path string) (*Config, error) {

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
