package main

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

var logLevel = "info"

var levelOrder = map[string]int{
	"debug": 0,
	"info":  1,
	"warn":  2,
	"error": 3,
	"fatal": 4,
}

func InitLogger(cfg *Config) {
	logLevel = strings.ToLower(cfg.Logging.Level)
}

func shouldLog(level string) bool {
	return levelOrder[level] >= levelOrder[logLevel]
}

// LogEntry — структура одной строки лога
type LogEntry struct {
	Timestamp string `json:"ts"`
	Level     string `json:"level"`
	Component string `json:"component"`
	Msg       string `json:"msg"`

	// DNS-специфичные поля (опциональные)
	Domain    string  `json:"domain,omitempty"`
	ClientIP  string  `json:"client_ip,omitempty"`
	Category  int     `json:"category,omitempty"`
	Action    string  `json:"action,omitempty"`
	Source    string  `json:"source,omitempty"`
	Blocked   *bool   `json:"blocked,omitempty"`
	CacheHit  *bool   `json:"cache_hit,omitempty"`
	LatencyMs float64 `json:"latency_ms,omitempty"`
	TTL       int     `json:"ttl,omitempty"`
	Qtype     string  `json:"qtype,omitempty"`

	// Ошибка
	Error string `json:"error,omitempty"`

	// Произвольные доп. поля
	Fields map[string]interface{} `json:"fields,omitempty"`
}

func writeLog(entry LogEntry) {
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	data, _ := json.Marshal(entry)
	os.Stdout.Write(append(data, '\n'))
}

// --- Базовые методы ---

func LogDebug(component, msg string) {
	if shouldLog("debug") {
		writeLog(LogEntry{Level: "debug", Component: component, Msg: msg})
	}
}

func LogInfo(component, msg string) {
	if shouldLog("info") {
		writeLog(LogEntry{Level: "info", Component: component, Msg: msg})
	}
}

func LogWarn(component, msg string) {
	if shouldLog("warn") {
		writeLog(LogEntry{Level: "warn", Component: component, Msg: msg})
	}
}

func LogError(component, msg string, err error) {
	if shouldLog("error") {
		e := LogEntry{Level: "error", Component: component, Msg: msg}
		if err != nil {
			e.Error = err.Error()
		}
		writeLog(e)
	}
}

func LogFatal(component, msg string, err error) {
	e := LogEntry{Level: "fatal", Component: component, Msg: msg}
	if err != nil {
		e.Error = err.Error()
	}
	writeLog(e)
	os.Exit(1)
}

// --- DNS request лог (основной) ---

func LogDNSRequest(entry LogEntry) {
	if shouldLog("info") {
		entry.Level = "info"
		entry.Component = "dns"
		entry.Msg = "dns_request"
		writeLog(entry)
	}
}

// --- Enricher лог ---

func LogEnrich(component, domain string, latencyMs float64, err error) {
	if !shouldLog("debug") {
		return
	}
	e := LogEntry{
		Level:     "debug",
		Component: component,
		Msg:       "enrich",
		Domain:    domain,
		LatencyMs: latencyMs,
	}
	if err != nil {
		e.Error = err.Error()
		e.Level = "warn"
	}
	writeLog(e)
}

// --- WithFields для произвольного контекста ---

func LogInfoFields(component, msg string, fields map[string]interface{}) {
	if shouldLog("info") {
		writeLog(LogEntry{Level: "info", Component: component, Msg: msg, Fields: fields})
	}
}
