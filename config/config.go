package config

import (
    "time"
    
    "gopkg.in/yaml.v3"
)

type Config struct {
    Server      ServerConfig      `yaml:"server"`
    Timeouts    TimeoutConfig     `yaml:"timeouts"`
    CloudAPI    CloudAPIConfig    `yaml:"cloud_api"`
    CloudDNS    CloudDNSConfig    `yaml:"cloud_dns"`
    FallbackDNS FallbackDNSConfig `yaml:"fallback_dns"`
    Sinkholes   SinkholeConfig    `yaml:"sinkholes"`
    TTL         TTLConfig         `yaml:"ttl"`
    Cache       CacheConfig       `yaml:"cache"`
    Logging     LoggingConfig     `yaml:"logging"`
    Monitoring  MonitoringConfig  `yaml:"monitoring"`
    Blocklists  BlocklistConfig   `yaml:"blocklists"`
}

type ServerConfig struct {
    Listen         string `yaml:"listen"`
    UDPBufferSize  int    `yaml:"udp_buffer_size"`
    PrometheusPort int    `yaml:"prometheus_port"`
    HealthPort     int    `yaml:"health_port"`
}

type TimeoutConfig struct {
    Total       int `yaml:"total"`
    CloudAPI    int `yaml:"cloud_api"`
    CloudDNS    int `yaml:"cloud_dns"`
    FallbackDNS int `yaml:"fallback_dns"`
    Enrichment  int `yaml:"enrichment"`
}

type CloudAPIConfig struct {
    URL       string `yaml:"url"`
    Key       string `yaml:"key"`
    RateLimit int    `yaml:"rate_limit"`
    Burst     int    `yaml:"burst"`
    Timeout   int    `yaml:"timeout"`
}

type CloudDNSConfig struct {
    Server  string `yaml:"server"`
    Timeout int    `yaml:"timeout"`
}

type FallbackDNSConfig struct {
    Servers []string `yaml:"servers"`
    Timeout int      `yaml:"timeout"`
}

type SinkholeConfig struct {
    Categories map[int]string `yaml:"categories"`
    Default    string         `yaml:"default"`
    IPv6       string         `yaml:"ipv6"`
}

type TTLConfig struct {
    ByCategory       map[int]int `yaml:"by_category"`
    Fallback         int         `yaml:"fallback"`
    Min              int         `yaml:"min"`
    Max              int         `yaml:"max"`
    EnrichedMultiplier float64   `yaml:"enriched_multiplier"`
}

type CacheConfig struct {
    Memory     MemoryCacheConfig `yaml:"memory"`
    Valkey     ValkeyConfig      `yaml:"valkey"`
    Strategy   string            `yaml:"strategy"`
    PreloadOnStart bool          `yaml:"preload_on_start"`
}

type MemoryCacheConfig struct {
    MaxSizeMB    int  `yaml:"max_size_mb"`
    NumCounters  int64 `yaml:"num_counters"`
    BufferItems  int64 `yaml:"buffer_items"`
    CostEstimation bool `yaml:"cost_estimation"`
}

type ValkeyConfig struct {
    Address     string        `yaml:"address"`
    Password    string        `yaml:"password"`
    PoolSize    int           `yaml:"pool_size"`
    ReadTimeout time.Duration `yaml:"read_timeout"`
    WriteTimeout time.Duration `yaml:"write_timeout"`
}

type LoggingConfig struct {
    Level       string `yaml:"level"`
    Format      string `yaml:"format"`
    Output      string `yaml:"output"`
    FilePath    string `yaml:"file_path"`
    MaxSizeMB   int    `yaml:"max_size_mb"`
    MaxBackups  int    `yaml:"max_backups"`
    MaxAgeDays  int    `yaml:"max_age_days"`
}

type MonitoringConfig struct {
    PrometheusEnabled bool          `yaml:"prometheus_enabled"`
    StatsdEnabled     bool          `yaml:"statsd_enabled"`
    HealthCheckInterval time.Duration `yaml:"health_check_interval"`
}

type BlocklistConfig struct {
    PreloadFiles  []string      `yaml:"preload_files"`
    UpdateInterval time.Duration `yaml:"update_interval"`
}

var cfg *Config

func Get() *Config {
    return cfg
}
