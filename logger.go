package logger

import (
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

var log *zap.Logger

type Config struct {
    Level  string
    Format string
    Output string
}

func Init(config Config) {
    var err error
    
    cfg := zap.NewProductionConfig()
    
    // Устанавливаем уровень логирования
    switch config.Level {
    case "debug":
        cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
    case "info":
        cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
    case "warn":
        cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
    case "error":
        cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
    default:
        cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
    }
    
    // Формат вывода
    if config.Format == "console" {
        cfg.Encoding = "console"
        cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
    } else {
        cfg.Encoding = "json"
    }
    
    // Выход
    if config.Output == "file" || config.Output == "both" {
        cfg.OutputPaths = []string{config.FilePath}
        cfg.ErrorOutputPaths = []string{config.FilePath}
    }
    if config.Output == "stdout" || config.Output == "both" {
        cfg.OutputPaths = append(cfg.OutputPaths, "stdout")
        cfg.ErrorOutputPaths = append(cfg.ErrorOutputPaths, "stdout")
    }
    
    log, err = cfg.Build()
    if err != nil {
        panic(err)
    }
}

func Debug(msg string, fields ...zap.Field) {
    log.Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
    log.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
    log.Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
    log.Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
    log.Fatal(msg, fields...)
}
