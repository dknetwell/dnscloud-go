package main

import (
    "fmt"
    "os"
    "time"
    
    "github.com/joho/godotenv"
    "gopkg.in/yaml.v3"
)

// Config - основная конфигурация
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
}

type TimeoutConfig struct {
    Total       time.Duration `yaml:"total"`
    CloudAPI    time.Duration `yaml:"cloud_api"`
    CacheRead   time.Duration `yaml:"cache_read"`
    CacheWrite  time.Duration `yaml:"cache_write"`
}

type CloudAPIConfig struct {
    URL       string        `yaml:"url"`
    Key       string        `yaml:"key"`
    Timeout   time.Duration `yaml:"timeout"`
    RateLimit int           `yaml:"rate_limit"`
}

type CacheConfig struct {
    Strategy      string        `yaml:"strategy"`
    ValkeyAddr    string        `yaml:"valkey_address"`
    ValkeyPass    string        `yaml:"valkey_password"`
    MemoryMaxSize int           `yaml:"memory_max_size_mb"`
}

type SinkholeConfig struct {
    Categories map[int]string `yaml:"categories"`
    Default    string         `yaml:"default"`
}

type TTLConfig struct {
    ByCategory map[int]int `yaml:"by_category"`
    Fallback   int         `yaml:"fallback"`
}

var config *Config

func loadConfig() error {
    // Загружаем .env если есть
    godotenv.Load()
    
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
    cfg.Cache.ValkeyPass = os.ExpandEnv(cfg.Cache.ValkeyPass)
    
    // Если LOG_LEVEL установлен в env, используем его
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
