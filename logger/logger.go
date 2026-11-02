package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Config configures the framework logger.
type Config struct {
	LogOutput string `yaml:"LOG_OUTPUT" env:"LOG_OUTPUT" env-default:"stdout"`
	LogType   string `yaml:"LOG_TYPE" env:"LOG_TYPE" env-default:"json"`
	LogLevel  string `yaml:"LOG_LEVEL" env:"LOG_LEVEL" env-default:"info"`
	LogDir    string `yaml:"LOG_DIR" env:"LOG_DIR" env-default:""`
	LogFile   string `yaml:"LOG_FILE" env:"LOG_FILE" env-default:""`
}

// Interface is the backend-independent structured logging contract.
// With returns a derived logger without changing the receiver.
type Interface interface {
	Error(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	With(keysAndValues ...interface{}) Interface
}

// NewLogger creates an independent logger. The owner must call cleanup after
// all consumers have stopped; cleanup closes only a file opened by this call.
// Output failures retain the stdout fallback policy and emit a warning.
func NewLogger(cfg Config) (Interface, func()) {
	logLevelEnum := parseLogLevel(cfg.LogLevel)

	output, cleanup, err := getLogOutput(cfg.LogOutput, cfg.LogDir, cfg.LogFile)

	baseLogger := createLogger(cfg.LogType, output, logLevelEnum)

	if err != nil {
		baseLogger.Warn("logger output fallback", "error", err)
	}

	baseLogger.Info(
		"logger initialized",
		"level", cfg.LogLevel,
		"format", cfg.LogType,
		"output", cfg.LogOutput,
		"dir", cfg.LogDir,
		"file", cfg.LogFile,
	)

	return NewSlogWrapper(baseLogger), cleanup
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

func getLogOutput(logOutput string, logDir string, logFile string) (io.Writer, func(), error) {
	noop := func() {}
	switch strings.ToLower(strings.TrimSpace(logOutput)) {
	case "", "stdout", "std":
		return os.Stdout, noop, nil

	case "stderr":
		return os.Stderr, noop, nil

	case "file":
		logFile = strings.TrimSpace(logFile)
		logDir = strings.TrimSpace(logDir)

		if logFile == "" {
			return os.Stdout, noop, fmt.Errorf("LOG_OUTPUT=file requires LOG_FILE, fallback to stdout")
		}

		path := logFile
		if !filepath.IsAbs(logFile) && logDir != "" {
			path = filepath.Join(logDir, logFile)
		}

		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return os.Stdout, noop, fmt.Errorf("failed to create log directory: %w", err)
		}

		output, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return os.Stdout, noop, fmt.Errorf("failed to open log file: %w", err)
		}

		return output, func() { _ = output.Close() }, nil

	default:
		return os.Stdout, noop, fmt.Errorf("unknown LOG_OUTPUT=%q, fallback to stdout", logOutput)
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
