package main

import (
	"net"
	"time"
)

// DomainResult — результат проверки домена
type DomainResult struct {
	Domain         string        `json:"domain"`
	Category       int           `json:"category"`
	Action         string        `json:"action"`
	IP             string        `json:"ip,omitempty"` // для совместимости со старой логикой
	TTL            int           `json:"ttl"`
	Source         string        `json:"source"`
	Timestamp      time.Time     `json:"timestamp"`
	ProcessingTime time.Duration `json:"processing_time,omitempty"`

	// 🔥 Новые поля для полноценной DNS-работы
	Blocked  bool     `json:"blocked"`
	RealIP   net.IP   `json:"-"`
	RealIPv6 net.IP   `json:"-"`
	CNAME    string   `json:"cname,omitempty"`
	MX       []MXRecord `json:"mx,omitempty"`
	TXT      []string `json:"txt,omitempty"`

	// negative caching
	Negative bool `json:"-"`
}

// MXRecord — MX запись
type MXRecord struct {
	Host     string `json:"host"`
	Priority uint16 `json:"priority"`
}

// APIResponse — ответ Cloud API
type APIResponse struct {
	Domain   string `json:"domain"`
	Category int    `json:"category"`
	TTL      int    `json:"ttl"`
	Action   string `json:"action"`
}

// Stats — статистика работы
type Stats struct {
	TotalRequests     int64         `json:"total_requests"`
	CacheHits         int64         `json:"cache_hits"`
	CacheMisses       int64         `json:"cache_misses"`
	APICalls          int64         `json:"api_calls"`
	SLAViolations     int64         `json:"sla_violations"`
	AvgResponseTime   time.Duration `json:"avg_response_time"`
	ActiveConnections int           `json:"active_connections"`
}
