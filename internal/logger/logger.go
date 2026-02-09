package logger

import (
    "os"
    
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
    "gopkg.in/natefinch/lumberjack.v2"
    
    "github.com/dknetwell/dnscloud-go/config"
)

var (
    log   *zap.Logger
    sugar *zap.SugaredLogger
)

type Config struct {
    Level      string
    Format     string
    Output     string
    FilePath   string
    MaxSizeMB  int
    MaxBackups int
    MaxAgeDays int
}

func Init(cfg config.LoggingConfig) {
    // Настройка кодировщика
    encoderConfig := zapcore.EncoderConfig{
        TimeKey:        "timestamp",
        LevelKey:       "level",
        NameKey:        "logger",
        CallerKey:      "caller",
        FunctionKey:    zapcore.OmitKey,
        MessageKey:     "msg",
        StacktraceKey:  "stacktrace",
        LineEnding:     zapcore.DefaultLineEnding,
        EncodeLevel:    zapcore.LowercaseLevelEncoder,
        EncodeTime:     zapcore.ISO8601TimeEncoder,
        EncodeDuration: zapcore.SecondsDurationEncoder,
        EncodeCaller:   zapcore.ShortCallerEncoder,
    }
    
    // Уровень логирования
    var level zapcore.Level
    switch cfg.Level {
    case "debug":
        level = zapcore.DebugLevel
    case "info":
        level = zapcore.InfoLevel
    case "warn":
        level = zapcore.WarnLevel
    case "error":
        level = zapcore.ErrorLevel
    default:
        level = zapcore.InfoLevel
    }
    
    // Настройка выходов
    var cores []zapcore.Core
    
    // Console output
    consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
    consoleCore := zapcore.NewCore(
        consoleEncoder,
        zapcore.AddSync(os.Stdout),
        level,
    )
    cores = append(cores, consoleCore)
    
    // File output если указан
    if cfg.FilePath != "" {
        fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
        fileWriter := zapcore.AddSync(&lumberjack.Logger{
            Filename:   cfg.FilePath,
            MaxSize:    100, // MB
            MaxBackups: 3,
            MaxAge:     30, // days
            Compress:   true,
        })
        fileCore := zapcore.NewCore(
            fileEncoder,
            fileWriter,
            level,
        )
        cores = append(cores, fileCore)
    }
    
    // Создаем логгер
    core := zapcore.NewTee(cores...)
    log = zap.New(core, 
        zap.AddCaller(), 
        zap.AddStacktrace(zapcore.ErrorLevel),
        zap.Fields(zap.String("service", "dns-proxy")),
    )
    sugar = log.Sugar()
    
    log.Info("Logger initialized",
        zap.String("level", cfg.Level),
        zap.String("output", cfg.Output))
}

// Методы логирования
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

// Sugar методы
func Debugf(template string, args ...interface{}) {
    sugar.Debugf(template, args...)
}

func Infof(template string, args ...interface{}) {
    sugar.Infof(template, args...)
}

func Warnf(template string, args ...interface{}) {
    sugar.Warnf(template, args...)
}

func Errorf(template string, args ...interface{}) {
    sugar.Errorf(template, args...)
}

func Fatalf(template string, args ...interface{}) {
    sugar.Fatalf(template, args...)
}
