package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DNS struct {
		Upstream     []string `yaml:"upstream"`
		SinkholeIPv4 string   `yaml:"sinkhole_ipv4"`
		SinkholeIPv6 string   `yaml:"sinkhole_ipv6"`
	} `yaml:"dns"`

	CloudAPI struct {
		Endpoint           string  `yaml:"endpoint"`
		APIKey             string  `yaml:"api_key"`
		InsecureSkipVerify bool    `yaml:"insecure_skip_verify"`
		RateLimit          float64 `yaml:"rate_limit"`
		Burst              int     `yaml:"burst"`
	} `yaml:"cloud_api"`

	TTL struct {
		Default int `yaml:"default"`
		Min     int `yaml:"min"`
		Max     int `yaml:"max"`
	} `yaml:"ttl"`

	Cache struct {
		MaxCost int64 `yaml:"max_cost"`
	} `yaml:"cache"`

	Engine struct {
		WorkerCount     int `yaml:"worker_count"`
		WorkerQueueSize int `yaml:"worker_queue_size"`
	} `yaml:"engine"`
}

func LoadConfig(path string) (*Config, error) {

	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(file, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
