package models

import (
    "time"
)

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
