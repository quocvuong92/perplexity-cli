package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
)

var (
	logger     *slog.Logger
	loggerOnce sync.Once
	getOnce    sync.Once
)

// Level represents the logging level
type Level = slog.Level

// Log levels
const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Config holds logging configuration
type Config struct {
	Level   Level
	Output  io.Writer
	Verbose bool
}

// DefaultConfig returns the default logging configuration
func DefaultConfig() Config {
	return Config{
		Level:   LevelInfo,
		Output:  os.Stderr,
		Verbose: false,
	}
}

// Init initializes the global logger with the given configuration
func Init(cfg Config) {
	loggerOnce.Do(func() {
		level := cfg.Level
		if cfg.Verbose {
			level = LevelDebug
		}

		output := cfg.Output
		if output == nil {
			output = io.Discard
		}

		opts := &slog.HandlerOptions{
			Level: level,
		}

		handler := slog.NewTextHandler(output, opts)
		logger = slog.New(handler)
	})
}

// InitDiscardLogger initializes a logger that discards all output
func InitDiscardLogger() {
	loggerOnce.Do(func() {
		handler := slog.NewTextHandler(io.Discard, nil)
		logger = slog.New(handler)
	})
}

// Get returns the global logger, initializing with defaults if needed
func Get() *slog.Logger {
	getOnce.Do(func() {
		if logger == nil {
			Init(DefaultConfig())
		}
	})
	return logger
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	Get().Debug(msg, args...)
}

// DebugContext logs a debug message with context
func DebugContext(ctx context.Context, msg string, args ...any) {
	Get().DebugContext(ctx, msg, args...)
}

// Info logs an info message
func Info(msg string, args ...any) {
	Get().Info(msg, args...)
}

// InfoContext logs an info message with context
func InfoContext(ctx context.Context, msg string, args ...any) {
	Get().InfoContext(ctx, msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	Get().Warn(msg, args...)
}

// WarnContext logs a warning message with context
func WarnContext(ctx context.Context, msg string, args ...any) {
	Get().WarnContext(ctx, msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	Get().Error(msg, args...)
}

// ErrorContext logs an error message with context
func ErrorContext(ctx context.Context, msg string, args ...any) {
	Get().ErrorContext(ctx, msg, args...)
}

// With returns a logger with additional attributes
func With(args ...any) *slog.Logger {
	return Get().With(args...)
}

// WithGroup returns a logger with a group name
func WithGroup(name string) *slog.Logger {
	return Get().WithGroup(name)
}

// Attr creates a slog.Attr for structured logging
func Attr(key string, value any) slog.Attr {
	return slog.Any(key, value)
}

// String creates a string attribute
func String(key, value string) slog.Attr {
	return slog.String(key, value)
}

// Int creates an int attribute
func Int(key string, value int) slog.Attr {
	return slog.Int(key, value)
}

// Bool creates a bool attribute
func Bool(key string, value bool) slog.Attr {
	return slog.Bool(key, value)
}

// Err creates an error attribute
func Err(err error) slog.Attr {
	return slog.Any("error", err)
}

// Duration creates a duration attribute
func Duration(key string, value any) slog.Attr {
	return slog.Any(key, value)
}
