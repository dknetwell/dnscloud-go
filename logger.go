package main

import (
    "fmt"
    "time"
)

type logLevel int

const (
    debugLevel logLevel = iota
    infoLevel
    warnLevel
    errorLevel
)

var (
    currentLevel logLevel = infoLevel
)

func initLogger(level string) {
    switch level {
    case "debug":
        currentLevel = debugLevel
    case "info":
        currentLevel = infoLevel
    case "warn":
        currentLevel = warnLevel
    case "error":
        currentLevel = errorLevel
    default:
        currentLevel = infoLevel
    }
}

func logDebug(msg string, args ...interface{}) {
    if currentLevel <= debugLevel {
        log("DEBUG", msg, args...)
    }
}

func logInfo(msg string, args ...interface{}) {
    if currentLevel <= infoLevel {
        log("INFO", msg, args...)
    }
}

func logWarn(msg string, args ...interface{}) {
    if currentLevel <= warnLevel {
        log("WARN", msg, args...)
    }
}

func logError(msg string, err error, args ...interface{}) {
    if currentLevel <= errorLevel {
        allArgs := append([]interface{}{"error", err.Error()}, args...)
        log("ERROR", msg, allArgs...)
    }
}

func log(level, msg string, args ...interface{}) {
    timestamp := time.Now().Format("2006-01-02 15:04:05")
    fmt.Printf("%s [%s] %s", timestamp, level, msg)

    if len(args) > 0 {
        for i := 0; i < len(args); i += 2 {
            if i+1 < len(args) {
                fmt.Printf(" %v=%v", args[i], args[i+1])
            }
        }
    }
    fmt.Println()
}
