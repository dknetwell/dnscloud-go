package main

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// CategoryConfig — настройки для одной категории угроз
type CategoryConfig struct {
	Name         string `yaml:"name"`
	Action       string `yaml:"action"`        // "block" или "allow"
	SinkholeIPv4 string `yaml:"sinkhole_ipv4"` // переопределяет глобальный
	SinkholeIPv6 string `yaml:"sinkhole_ipv6"`
}

type Config struct {

	Logging struct {
		Level  string `yaml:"level"`
		Syslog bool   `yaml:"syslog"`
	} `yaml:"logging"`

	DNS struct {
		ListenUDP     string   `yaml:"listen_udp"`
		ListenTCP     string   `yaml:"listen_tcp"`
		Upstream      []string `yaml:"upstream"`
		SinkholeIPv4  string   `yaml:"sinkhole_ipv4"`  // глобальный fallback
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

	// Словарь категорий: ключ — номер категории из CloudAPI
	// Изменять здесь или в config.yaml в секции categories:
	//
	//   categories:
	//     1:
	//       name: malware
	//       action: block
	//       sinkhole_ipv4: "0.0.0.0"
	//       sinkhole_ipv6: "::"
	//     6:
	//       name: grayware
	//       action: block
	//       sinkhole_ipv4: "192.168.1.100"  # страница-заглушка
	//       sinkhole_ipv6: "::1"
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

	// YAML optional
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			_ = yaml.Unmarshal(data, cfg)
		}
	}

	// Дефолтный словарь категорий если не задан в YAML
	// ─────────────────────────────────────────────────────────────────
	// Категории CloudAPI:
	//   0 - benign/unknown  → allow
	//   1 - malware         → block → 0.0.0.0 (hard drop)
	//   2 - command&control → block → 0.0.0.0 (hard drop)
	//   3 - phishing        → block → 0.0.0.0 (hard drop)
	//   4 - dynamicDNS      → block → 0.0.0.0
	//   5 - newly registered→ block → 0.0.0.0
	//   6 - grayware        → block → глобальный sinkhole
	//   7 - parked          → allow  (не блокируем)
	//   8 - proxy           → block → 0.0.0.0
	//   9 - allowlist       → allow
	// ─────────────────────────────────────────────────────────────────
	if cfg.Categories == nil {
		cfg.Categories = map[int]CategoryConfig{
			1: {Name: "malware",             Action: "block", SinkholeIPv4: "0.0.0.0",       SinkholeIPv6: "::"},
			2: {Name: "command_and_control", Action: "block", SinkholeIPv4: "0.0.0.0",       SinkholeIPv6: "::"},
			3: {Name: "phishing",            Action: "block", SinkholeIPv4: "0.0.0.0",       SinkholeIPv6: "::"},
			4: {Name: "dynamic_dns",         Action: "block", SinkholeIPv4: "0.0.0.0",       SinkholeIPv6: "::"},
			5: {Name: "newly_registered",    Action: "block", SinkholeIPv4: "0.0.0.0",       SinkholeIPv6: "::"},
			6: {Name: "grayware",            Action: "block", SinkholeIPv4: "",               SinkholeIPv6: ""},   // "" → fallback на глобальный dns.sinkhole_ipv4
			7: {Name: "parked",              Action: "allow", SinkholeIPv4: "",               SinkholeIPv6: ""},
			8: {Name: "proxy",               Action: "block", SinkholeIPv4: "0.0.0.0",       SinkholeIPv6: "::"},
			9: {Name: "allowlist",           Action: "allow", SinkholeIPv4: "",               SinkholeIPv6: ""},
		}
	}

	// ===== DNS =====
	cfg.DNS.ListenUDP = getEnv("DNS_LISTEN_UDP", defaultStr(cfg.DNS.ListenUDP, ":53"))
	cfg.DNS.ListenTCP = getEnv("DNS_LISTEN_TCP", defaultStr(cfg.DNS.ListenTCP, ":53"))
	cfg.DNS.MaxPacketSize = getEnvInt("DNS_MAX_PACKET", defaultInt(cfg.DNS.MaxPacketSize, 1232))
	cfg.DNS.SinkholeIPv4 = getEnv("DNS_SINKHOLE_IPV4", defaultStr(cfg.DNS.SinkholeIPv4, "0.0.0.0"))
	cfg.DNS.SinkholeIPv6 = getEnv("DNS_SINKHOLE_IPV6", defaultStr(cfg.DNS.SinkholeIPv6, "::"))

	if env := os.Getenv("DNS_UPSTREAMS"); env != "" {
		cfg.DNS.Upstream = strings.Split(env, ",")
	}
	if len(cfg.DNS.Upstream) == 0 {
		cfg.DNS.Upstream = []string{"8.8.8.8:53", "1.1.1.1:53"}
	}

	// ===== HTTP =====
	cfg.HTTP.Listen = getEnv("HTTP_LISTEN", defaultStr(cfg.HTTP.Listen, ":8080"))

	// ===== Valkey =====
	cfg.Valkey.Address = getEnv("VALKEY_ADDR", defaultStr(cfg.Valkey.Address, "valkey:6379"))
	cfg.Valkey.Password = getEnv("VALKEY_PASSWORD", cfg.Valkey.Password)
	cfg.Valkey.DB = getEnvInt("VALKEY_DB", cfg.Valkey.DB)

	// ===== Engine =====
	cfg.Engine.WorkerCount = getEnvInt("ENGINE_WORKERS", defaultInt(cfg.Engine.WorkerCount, 100))
	cfg.Engine.WorkerQueueSize = getEnvInt("ENGINE_QUEUE", defaultInt(cfg.Engine.WorkerQueueSize, 1000))

	// ===== CloudAPI =====
	cfg.CloudAPI.Endpoint = getEnv("CLOUDAPI_ENDPOINT", cfg.CloudAPI.Endpoint)
	cfg.CloudAPI.APIKey = getEnv("CLOUDAPI_APIKEY", cfg.CloudAPI.APIKey)
	cfg.CloudAPI.TimeoutSeconds = getEnvInt("CLOUDAPI_TIMEOUT", defaultInt(cfg.CloudAPI.TimeoutSeconds, 2))
	cfg.CloudAPI.RateLimit = getEnvFloat("CLOUDAPI_RPS", cfg.CloudAPI.RateLimit)
	cfg.CloudAPI.Burst = getEnvInt("CLOUDAPI_BURST", defaultInt(cfg.CloudAPI.Burst, 100))
	cfg.CloudAPI.InsecureSkipVerify = getEnvBool("CLOUDAPI_INSECURE", cfg.CloudAPI.InsecureSkipVerify)

	// ===== TTL =====
	cfg.TTL.Default = getEnvInt("TTL_DEFAULT", defaultInt(cfg.TTL.Default, 300))
	cfg.TTL.Min = getEnvInt("TTL_MIN", defaultInt(cfg.TTL.Min, 60))
	cfg.TTL.Max = getEnvInt("TTL_MAX", defaultInt(cfg.TTL.Max, 86400))

	// ===== Cache =====
	cfg.Cache.MaxCost = getEnvInt64("CACHE_MAX_COST", defaultInt64(cfg.Cache.MaxCost, 1<<30))

	return cfg, nil
}
