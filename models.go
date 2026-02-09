package main

import "time"

// DomainResult - результат проверки домена
type DomainResult struct {
    Domain         string        `json:"domain"`
    Category       int           `json:"category"`
    Action         string        `json:"action"`
    IP             string        `json:"ip,omitempty"`
    TTL            int           `json:"ttl"`
    Source         string        `json:"source"`
    Timestamp      time.Time     `json:"timestamp"`
    ProcessingTime time.Duration `json:"processing_time,omitempty"`
}

// CheckRequest - запрос на проверку
type CheckRequest struct {
    Domain string `json:"domain"`
    Client string `json:"client,omitempty"`
}

// APIResponse - ответ Cloud API
type APIResponse struct {
    Domain   string `json:"domain"`
    Category int    `json:"category"`
    TTL      int    `json:"ttl"`
}

// Stats - статистика работы
type Stats struct {
    TotalRequests    int64         `json:"total_requests"`
    CacheHits        int64         `json:"cache_hits"`
    CacheMisses      int64         `json:"cache_misses"`
    APICalls         int64         `json:"api_calls"`
    SLAViolations    int64         `json:"sla_violations"`
    AvgResponseTime  time.Duration `json:"avg_response_time"`
    ActiveConnections int          `json:"active_connections"`
}
