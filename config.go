package main

import (
    "fmt"
    "os"
    "strconv"
    "strings"
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
    Metrics    MetricsConfig `yaml:"metrics"`
}

// ... остальные структуры остаются такими же ...

func loadConfig() error {
    // Загружаем .env если есть
    godotenv.Load()
    
    configPath := "config/config.yaml"
    if _, err := os.Stat(configPath); os.IsNotExist(err) {
        return fmt.Errorf("config file not found: %s", configPath)
    }
    
    // Читаем конфигурацию YAML
    data, err := os.ReadFile(configPath)
    if err != nil {
        return fmt.Errorf("failed to read config file: %w", err)
    }
    
    // Заменяем переменные окружения в YAML
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
    
    if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
        cfg.LogLevel = envLevel
    }
    
    // Устанавливаем значения по умолчанию если не установлены
    if cfg.CloudAPI.RateLimit == 0 {
        cfg.CloudAPI.RateLimit = 5
    }
    
    config = &cfg
    return nil
}

// ... остальные функции без изменений ...
