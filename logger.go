package main

import (
	"bytes"
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

// LogEntry — все поля без omitempty на числах чтобы category=0 и latency_ms=0.0 попадали в JSON
type LogEntry struct {
	Timestamp string `json:"ts"`
	Level     string `json:"level"`
	Component string `json:"component"`
	Msg       string `json:"msg"`

	// omitempty только на строках — пустая строка не несёт смысла
	Domain   string `json:"domain,omitempty"`
	ClientIP string `json:"client_ip,omitempty"`
	Action   string `json:"action,omitempty"`
	Source   string `json:"source,omitempty"`
	Qtype    string `json:"qtype,omitempty"`
	Error    string `json:"error,omitempty"`

	// Числа и bool — всегда пишем если поле задано через *ptr или явно
	Category  *int     `json:"category,omitempty"`  // указатель: nil = не задано, 0 = benign
	LatencyMs *float64 `json:"latency_ms,omitempty"`
	TTL       *int     `json:"ttl,omitempty"`
	Blocked   *bool    `json:"blocked,omitempty"`
	CacheHit  *bool    `json:"cache_hit,omitempty"`

	Fields map[string]interface{} `json:"fields,omitempty"`
}

func writeLog(entry LogEntry) {
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false) // не экранировать <, >, & в строках логов
	_ = enc.Encode(entry)    // Encode добавляет \n автоматически
	os.Stdout.Write(buf.Bytes())
}

func ptrInt(v int) *int         { return &v }
func ptrFloat(v float64) *float64 { return &v }
func ptrBool(v bool) *bool      { return &v }

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

func LogDNSRequest(domain, clientIP, qtype, action, source string, latencyMs float64, ttl int, blocked bool, category int) {
	if !shouldLog("info") {
		return
	}
	writeLog(LogEntry{
		Level:     "info",
		Component: "dns",
		Msg:       "dns_request",
		Domain:    domain,
		ClientIP:  clientIP,
		Qtype:     qtype,
		Action:    action,
		Source:    source,
		LatencyMs: ptrFloat(latencyMs),
		TTL:       ptrInt(ttl),
		Blocked:   ptrBool(blocked),
		Category:  ptrInt(category),
	})
}

func LogEnrichResult(component, domain string, latencyMs float64, category int, action, source string, blocked bool, err error) {
	if err != nil {
		if !shouldLog("warn") {
			return
		}
		writeLog(LogEntry{
			Level:     "warn",
			Component: component,
			Msg:       "enrich_error",
			Domain:    domain,
			LatencyMs: ptrFloat(latencyMs),
			Error:     err.Error(),
		})
		return
	}
	if !shouldLog("info") {
		return
	}
	writeLog(LogEntry{
		Level:     "info",
		Component: component,
		Msg:       "enrich_ok",
		Domain:    domain,
		LatencyMs: ptrFloat(latencyMs),
		Category:  ptrInt(category),
		Action:    action,
		Source:    source,
		Blocked:   ptrBool(blocked),
	})
}

func LogInfoFields(component, msg string, fields map[string]interface{}) {
	if shouldLog("info") {
		writeLog(LogEntry{Level: "info", Component: component, Msg: msg, Fields: fields})
	}
}
