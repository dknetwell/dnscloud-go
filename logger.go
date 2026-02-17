package main

import (
	"encoding/json"
	"log"
	"log/syslog"
	"os"
	"time"
)

var logger *log.Logger

func InitLogger(cfg *Config) {

	if cfg.Logging.Syslog {
		writer, err := syslog.New(syslog.LOG_INFO|syslog.LOG_LOCAL0, "dns-proxy")
		if err == nil {
			logger = log.New(writer, "", 0)
			return
		}
	}

	logger = log.New(os.Stdout, "", 0)
}

func logJSON(level, component, message string, fields map[string]interface{}) {

	entry := map[string]interface{}{
		"time":      time.Now().UTC(),
		"level":     level,
		"component": component,
		"message":   message,
	}

	for k, v := range fields {
		entry[k] = v
	}

	data, _ := json.Marshal(entry)
	logger.Println(string(data))
}

func LogInfo(component, message string) {
	logJSON("info", component, message, nil)
}

func LogError(component, message string, err error) {
	logJSON("error", component, message, map[string]interface{}{
		"error": err.Error(),
	})
}
