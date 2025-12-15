package logger

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

type Interface interface {
	Error(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
}

var (
	loggerInstance Interface
	loggerOnce     sync.Once
)

func NewLogger(logLevel string, logType string, logFile string) Interface {
	loggerOnce.Do(func() {
		loggerInstance = setupLogger(logLevel, logType, logFile)
	})
	return loggerInstance
}

func setupLogger(logLevel string, logType string, logFile string) Interface {
	logLevelEnum := parseLogLevel(logLevel)
	output, err := getLogOutput(logFile)

	baseLogger := createLogger(logType, output, logLevelEnum)
	wrappedLogger := NewSlogWrapper(baseLogger)

	if err != nil {
		wrappedLogger.Error("Error initializing log file", "error", err, "logFile", logFile)
	}

	wrappedLogger.Info("Logger initialized", "level", logLevel, "format", logType, "file", logFile)

	return wrappedLogger
}

func parseLogLevel(logLevel string) slog.Level {
	switch logLevel {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelDebug
	}
}

func getLogOutput(logFile string) (*os.File, error) {
	if logFile != "" {
		dir := filepath.Dir(logFile)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		output, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		return output, nil
	}
	return os.Stdout, nil
}

func createLogger(logType string, output *os.File, logLevel slog.Level) *slog.Logger {
	options := &slog.HandlerOptions{Level: logLevel}
	switch logType {
	case "json":
		return slog.New(slog.NewJSONHandler(output, options))
	case "text":
		return slog.New(slog.NewTextHandler(output, options))
	default:
		return slog.New(slog.NewTextHandler(output, options))
	}
}
