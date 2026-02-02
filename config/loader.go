package config

import (
    "os"
    "path/filepath"
    "strings"
    "time"
    
    "github.com/joho/godotenv"
    "gopkg.in/yaml.v3"
)

func Load() error {
    // Загружаем .env файл
    godotenv.Load()
    
    // Читаем конфиг
    data, err := os.ReadFile("config/config.yaml")
    if err != nil {
        return err
    }
    
    // Подставляем переменные окружения
    expanded := os.ExpandEnv(string(data))
    
    // Парсим YAML
    cfg = &Config{}
    if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
        return err
    }
    
    // Конвертируем миллисекунды в Duration
    cfg.CloudAPI.Timeout = cfg.Timeouts.CloudAPI
    cfg.CloudDNS.Timeout = cfg.Timeouts.CloudDNS
    
    // Устанавливаем таймауты как Duration
    readTimeout, _ := time.ParseDuration(cfg.Cache.Valkey.ReadTimeout.String())
    writeTimeout, _ := time.ParseDuration(cfg.Cache.Valkey.WriteTimeout.String())
    cfg.Cache.Valkey.ReadTimeout = readTimeout
    cfg.Cache.Valkey.WriteTimeout = writeTimeout
    
    return nil
}

func GetSinkholeIP(category int) string {
    if ip, ok := cfg.Sinkholes.Categories[category]; ok {
        return ip
    }
    return cfg.Sinkholes.Default
}

func GetTTLByCategory(category int) uint32 {
    if ttl, ok := cfg.TTL.ByCategory[category]; ok {
        return uint32(ttl)
    }
    return uint32(cfg.TTL.Fallback)
}

func GetMaxTTL() uint32 {
    return uint32(cfg.TTL.Max)
}

func GetMinTTL() uint32 {
    return uint32(cfg.TTL.Min)
}
