package config

import (
    "fmt"
    "os"
    "time"
    
    "github.com/joho/godotenv"
    "gopkg.in/yaml.v3"
)

type Config struct {
    Server    ServerConfig    `yaml:"server"`
    Timeouts  TimeoutConfig   `yaml:"timeouts"`
    CloudAPI  CloudAPIConfig  `yaml:"cloud_api"`
    Sinkholes SinkholeConfig  `yaml:"sinkholes"`
    TTL       TTLConfig       `yaml:"ttl"`
    Cache     CacheConfig     `yaml:"cache"`
    Logging   LoggingConfig   `yaml:"logging"`
    Monitoring MonitoringConfig `yaml:"monitoring"`
}

type ServerConfig struct {
    DNSListen     string        `yaml:"dns_listen"`
    HTTPListen    string        `yaml:"http_listen"`
    UDPMaxSize    int           `yaml:"udp_buffer_size"`
    ReadTimeout   time.Duration `yaml:"read_timeout"`
    WriteTimeout  time.Duration `yaml:"write_timeout"`
    IdleTimeout   time.Duration `yaml:"idle_timeout"`
}

type TimeoutConfig struct {
    Total       time.Duration `yaml:"total"`
    CloudAPI    time.Duration `yaml:"cloud_api"`
    CacheRead   time.Duration `yaml:"cache_read"`
    CacheWrite  time.Duration `yaml:"cache_write"`
    DNSResponse time.Duration `yaml:"dns_response"`
}

type CloudAPIConfig struct {
    URL       string        `yaml:"url"`
    Key       string        `yaml:"key"`
    RateLimit int           `yaml:"rate_limit"`
    Burst     int           `yaml:"burst"`
    Timeout   time.Duration `yaml:"timeout"`
}

type SinkholeConfig struct {
    Categories map[int]string `yaml:"categories"`
    Default    string         `yaml:"default"`
    IPv6       string         `yaml:"ipv6"`
}

type TTLConfig struct {
    ByCategory map[int]int `yaml:"by_category"`
    Fallback   int         `yaml:"fallback"`
    Min        int         `yaml:"min"`
    Max        int         `yaml:"max"`
}

type CacheConfig struct {
    Memory  MemoryCacheConfig  `yaml:"memory"`
    Valkey  ValkeyCacheConfig  `yaml:"valkey"`
    Strategy string            `yaml:"strategy"`
}

type MemoryCacheConfig struct {
    MaxSizeMB        int           `yaml:"max_size_mb"`
    DefaultExpiration time.Duration `yaml:"default_expiration"`
    CleanupInterval  time.Duration `yaml:"cleanup_interval"`
}

type ValkeyCacheConfig struct {
    Address     string        `yaml:"address"`
    Password    string        `yaml:"password"`
    PoolSize    int           `yaml:"pool_size"`
    ReadTimeout time.Duration `yaml:"read_timeout"`
    WriteTimeout time.Duration `yaml:"write_timeout"`
}

type LoggingConfig struct {
    Level     string `yaml:"level"`
    Format    string `yaml:"format"`
    Output    string `yaml:"output"`
    FilePath  string `yaml:"file_path"`
    MaxSizeMB int    `yaml:"max_size_mb"`
    MaxBackups int   `yaml:"max_backups"`
    MaxAgeDays int   `yaml:"max_age_days"`
}

type MonitoringConfig struct {
    PrometheusEnabled bool   `yaml:"prometheus_enabled"`
    MetricsPrefix     string `yaml:"metrics_prefix"`
    CollectInterval   time.Duration `yaml:"collect_interval"`
}

var instance *Config

func Load() error {
    // Загружаем .env
    if err := godotenv.Load(); err != nil {
        fmt.Printf("Warning: .env file not found: %v\n", err)
    }
    
    // Читаем конфигурацию YAML
    data, err := os.ReadFile("config/config.yaml")
    if err != nil {
        return fmt.Errorf("failed to read config file: %w", err)
    }
    
    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return fmt.Errorf("failed to parse config: %w", err)
    }
    
    // Заменяем переменные окружения
    cfg.CloudAPI.Key = os.ExpandEnv(cfg.CloudAPI.Key)
    cfg.CloudAPI.URL = os.ExpandEnv(cfg.CloudAPI.URL)
    cfg.Cache.Valkey.Password = os.ExpandEnv(cfg.Cache.Valkey.Password)
    
    instance = &cfg
    return nil
}

func Get() *Config {
    if instance == nil {
        panic("config not loaded")
    }
    return instance
}

func GetTTLByCategory(category int) int {
    cfg := Get()
    if ttl, ok := cfg.TTL.ByCategory[category]; ok {
        return ttl
    }
    return cfg.TTL.Fallback
}

func GetSinkholeIP(category int) string {
    cfg := Get()
    if ip, ok := cfg.Sinkholes.Categories[category]; ok {
        return ip
    }
    return cfg.Sinkholes.Default
}
