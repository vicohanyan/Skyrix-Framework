package logger

import "log/slog"

type SlogWrapper struct {
	logger *slog.Logger
}

func NewSlogWrapper(logger *slog.Logger) *SlogWrapper {
	return &SlogWrapper{logger: logger}
}

func (l *SlogWrapper) Error(msg string, keysAndValues ...interface{}) {
	l.logger.Error(msg, keysAndValues...)
}

func (l *SlogWrapper) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Info(msg, keysAndValues...)
}

func (l *SlogWrapper) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.Debug(msg, keysAndValues...)
}

func (l *SlogWrapper) Warn(msg string, keysAndValues ...interface{}) {
	l.logger.Warn(msg, keysAndValues...)
}
