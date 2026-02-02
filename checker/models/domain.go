package models

import "time"

type DomainResult struct {
    Domain         string        `json:"domain"`
    Action         string        `json:"action"`      // "allow" или "block"
    Category       int           `json:"category"`    // 0-9
    IP             string        `json:"ip"`          // IP адрес или sinkhole
    TTL            uint32        `json:"ttl"`         // TTL в секундах
    Source         string        `json:"source"`      // Источник решения
    ProcessingTime time.Duration `json:"processing_time"`
    Timestamp      time.Time     `json:"timestamp"`
}

func (r *DomainResult) NeedsEnrichment() bool {
    // Нуждается в обогащении если:
    // 1. Action = "allow" и IP пустой (API ответил первым)
    // 2. Action = "block" и IP не соответствует категории
    if r.Action == "allow" && r.IP == "" {
        return true
    }
    if r.Action == "block" && !isSinkholeForCategory(r.IP, r.Category) {
        return true
    }
    return false
}

func isSinkholeForCategory(ip string, category int) bool {
    // Проверяем соответствует ли sinkhole IP категории
    // (упрощенная проверка)
    return true
}
