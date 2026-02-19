package main

import (
	"log"
	"os"
	"strings"
)

var logLevel = "info"

func InitLogger(cfg *Config) {
	logLevel = strings.ToLower(cfg.Logging.Level)
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func shouldLog(level string) bool {
	order := map[string]int{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
		"fatal": 4,
	}
	return order[level] >= order[logLevel]
}

func LogDebug(msg string) {
	if shouldLog("debug") {
		log.Println("[DEBUG]", msg)
	}
}

func LogInfo(msg string) {
	if shouldLog("info") {
		log.Println("[INFO]", msg)
	}
}

func LogWarn(msg string) {
	if shouldLog("warn") {
		log.Println("[WARN]", msg)
	}
}

func LogError(msg string, err error) {
	if shouldLog("error") {
		if err != nil {
			log.Println("[ERROR]", msg, err)
		} else {
			log.Println("[ERROR]", msg)
		}
	}
}

func LogFatal(msg string, err error) {
	log.Println("[FATAL]", msg, err)
	os.Exit(1)
}
