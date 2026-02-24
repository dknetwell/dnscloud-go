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

func LogDebug(component, msg string) {
	if shouldLog("debug") {
		log.Println("[DEBUG]", "["+component+"]", msg)
	}
}

func LogInfo(component, msg string) {
	if shouldLog("info") {
		log.Println("[INFO]", "["+component+"]", msg)
	}
}

func LogWarn(component, msg string) {
	if shouldLog("warn") {
		log.Println("[WARN]", "["+component+"]", msg)
	}
}

func LogError(component, msg string, err error) {
	if shouldLog("error") {
		if err != nil {
			log.Println("[ERROR]", "["+component+"]", msg, err)
		} else {
			log.Println("[ERROR]", "["+component+"]", msg)
		}
	}
}

func LogFatal(component, msg string, err error) {
	log.Println("[FATAL]", "["+component+"]", msg, err)
	os.Exit(1)
}
