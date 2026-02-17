package main

import (
    "fmt"
    "os"
    "time"

    "github.com/joho/godotenv"
    "gopkg.in/yaml.v3"
)

// Структуры конфигурации
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

type CacheConfig struct {
    Strategy string            `yaml:"strategy"`
    Memory   MemoryCacheConfig `yaml:"memory"`
    Valkey   ValkeyCacheConfig `yaml:"valkey"`
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

type MetricsConfig struct {
    PrometheusEnabled bool          `yaml:"prometheus_enabled"`
    Prefix           string         `yaml:"prefix"`
    CollectInterval  time.Duration `yaml:"collect_interval"`
}

type Config struct {
    DNSListen  string        `yaml:"dns_listen"`
    HTTPListen string        `yaml:"http_listen"`
    LogLevel   string        `yaml:"log_level"`
    LogFormat  string        `yaml:"log_format"`
    Timeouts   TimeoutConfig `yaml:"timeouts"`
    CloudAPI   CloudAPIConfig `yaml:"cloud_api"`
    Cache      CacheConfig   `yaml:"cache"`
    Sinkholes  SinkholeConfig `yaml:"sinkholes"`
    TTL        TTLConfig     `yaml:"ttl"`
    Metrics    MetricsConfig `yaml:"metrics"`
}

var config *Config

func loadConfig() error {
    godotenv.Load()

    configPath := "config/config.yaml"
    if _, err := os.Stat(configPath); os.IsNotExist(err) {
        return fmt.Errorf("config file not found: %s", configPath)
    }

    data, err := os.ReadFile(configPath)
    if err != nil {
        return fmt.Errorf("failed to read config file: %w", err)
    }

    // Заменяем переменные окружения
    yamlContent := string(data)
    yamlContent = os.ExpandEnv(yamlContent)

    var cfg Config
    if err := yaml.Unmarshal([]byte(yamlContent), &cfg); err != nil {
        return fmt.Errorf("failed to parse config: %w", err)
    }

    // Дополнительные замены
    if cfg.CloudAPI.Key == "" {
        cfg.CloudAPI.Key = os.Getenv("CLOUD_API_KEY")
    }

    if cfg.Cache.Valkey.Password == "" {
        cfg.Cache.Valkey.Password = os.Getenv("VALKEY_PASSWORD")
    }

    // Исправленная строка 114 - убрана обратная косая черта перед $
    if cfg.CloudAPI.URL == "" || cfg.CloudAPI.URL == "${CLOUD_API_URL:-https://172.16.10.33/api/}" {
        if envURL := os.Getenv("CLOUD_API_URL"); envURL != "" {
            cfg.CloudAPI.URL = envURL
        } else {
            cfg.CloudAPI.URL = "https://172.16.10.33/api/"
        }
    }

    if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
        cfg.LogLevel = envLevel
    }

    config = &cfg
    return nil
}

func getConfig() *Config {
    if config == nil {
        panic("Config not loaded. Call loadConfig() first.")
    }
    return config
}

func getTTLByCategory(category int) int {
    cfg := getConfig()
    if ttl, ok := cfg.TTL.ByCategory[category]; ok {
        return ttl
    }
    return cfg.TTL.Fallback
}

func getSinkholeIP(category int) string {
    cfg := getConfig()
    if ip, ok := cfg.Sinkholes.Categories[category]; ok {
        return ip
    }
    return cfg.Sinkholes.Default
}
