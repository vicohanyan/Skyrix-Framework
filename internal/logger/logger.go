package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"skyrix/internal/config"
	"strings"
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

func NewLogger(cfg config.Logger) Interface {
	loggerOnce.Do(func() {
		loggerInstance = setupLogger(cfg)
	})

	return loggerInstance
}

func setupLogger(cfg config.Logger) Interface {
	logLevelEnum := parseLogLevel(cfg.LogLevel)

	output, err := getLogOutput(cfg.LogOutput, cfg.LogDir, cfg.LogFile)

	baseLogger := createLogger(cfg.LogType, output, logLevelEnum).With()

	wrappedLogger := NewSlogWrapper(baseLogger)

	if err != nil {
		wrappedLogger.Warn("logger output fallback", "error", err)
	}

	wrappedLogger.Info(
		"logger initialized",
		"level", cfg.LogLevel,
		"format", cfg.LogType,
		"output", cfg.LogOutput,
		"dir", cfg.LogDir,
		"file", cfg.LogFile,
	)

	return wrappedLogger
}

func parseLogLevel(logLevel string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(logLevel)) {
	case "debug":
		return slog.LevelDebug
	case "info", "":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func getLogOutput(logOutput string, logDir string, logFile string) (io.Writer, error) {
	switch strings.ToLower(strings.TrimSpace(logOutput)) {
	case "", "stdout", "std":
		return os.Stdout, nil

	case "stderr":
		return os.Stderr, nil

	case "file":
		logFile = strings.TrimSpace(logFile)
		logDir = strings.TrimSpace(logDir)

		if logFile == "" {
			return os.Stdout, fmt.Errorf("LOG_OUTPUT=file requires LOG_FILE, fallback to stdout")
		}

		path := logFile
		if !filepath.IsAbs(logFile) && logDir != "" {
			path = filepath.Join(logDir, logFile)
		}

		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return os.Stdout, fmt.Errorf("failed to create log directory: %w", err)
		}

		output, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return os.Stdout, fmt.Errorf("failed to open log file: %w", err)
		}

		return output, nil

	default:
		return os.Stdout, fmt.Errorf("unknown LOG_OUTPUT=%q, fallback to stdout", logOutput)
	}
}

func createLogger(logType string, output io.Writer, logLevel slog.Level) *slog.Logger {
	options := &slog.HandlerOptions{Level: logLevel}

	switch strings.ToLower(strings.TrimSpace(logType)) {
	case "json", "":
		return slog.New(slog.NewJSONHandler(output, options))
	case "text":
		return slog.New(slog.NewTextHandler(output, options))
	default:
		return slog.New(slog.NewJSONHandler(output, options))
	}
}
