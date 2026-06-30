// Package sloglog provides a structured logger based on log/slog.
// It replaces ad-hoc log.Printf/fmt.Printf usage with leveled, structured logging.
package sloglog

import (
	"log/slog"
	"os"
)

var logger *slog.Logger

func init() {
	// Default: JSON logger to stderr at Info level.
	// Set LOG_LEVEL=debug or LOG_FORMAT=text to override.
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	var handler slog.Handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	if os.Getenv("LOG_FORMAT") == "text" {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	}
	logger = slog.New(handler)
}

// SetLogger replaces the default logger. Call once at startup if custom
// configuration is needed (e.g., file output, different format).
func SetLogger(l *slog.Logger) {
	logger = l
}

func Debug(msg string, args ...any) { logger.Debug(msg, args...) }
func Info(msg string, args ...any)  { logger.Info(msg, args...) }
func Warn(msg string, args ...any)  { logger.Warn(msg, args...) }
func Error(msg string, args ...any) { logger.Error(msg, args...) }
