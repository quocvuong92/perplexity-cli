package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Level != LevelInfo {
		t.Errorf("DefaultConfig Level = %v, want %v", cfg.Level, LevelInfo)
	}

	if cfg.Verbose {
		t.Error("DefaultConfig Verbose should be false")
	}
}

func TestGet(t *testing.T) {
	// Get should return a logger (initializes with defaults if needed)
	l := Get()
	if l == nil {
		t.Error("Get() should return a non-nil logger")
	}
}

func TestAttributeHelpers(t *testing.T) {
	// Test String
	strAttr := String("key", "value")
	if strAttr.Key != "key" {
		t.Errorf("String key = %s, want 'key'", strAttr.Key)
	}

	// Test Int
	intAttr := Int("count", 42)
	if intAttr.Key != "count" {
		t.Errorf("Int key = %s, want 'count'", intAttr.Key)
	}

	// Test Bool
	boolAttr := Bool("enabled", true)
	if boolAttr.Key != "enabled" {
		t.Errorf("Bool key = %s, want 'enabled'", boolAttr.Key)
	}

	// Test Err
	errAttr := Err(nil)
	if errAttr.Key != "error" {
		t.Errorf("Err key = %s, want 'error'", errAttr.Key)
	}

	// Test Attr
	anyAttr := Attr("data", map[string]int{"a": 1})
	if anyAttr.Key != "data" {
		t.Errorf("Attr key = %s, want 'data'", anyAttr.Key)
	}
}

func TestLoggingOutput(t *testing.T) {
	var buf bytes.Buffer

	// Create a custom logger for testing
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewTextHandler(&buf, opts)
	testLogger := slog.New(handler)

	// Test logging
	testLogger.Debug("debug message")
	if !strings.Contains(buf.String(), "debug message") {
		t.Error("Debug message should be in output")
	}

	buf.Reset()
	testLogger.Info("info message")
	if !strings.Contains(buf.String(), "info message") {
		t.Error("Info message should be in output")
	}

	buf.Reset()
	testLogger.Warn("warn message")
	if !strings.Contains(buf.String(), "warn message") {
		t.Error("Warn message should be in output")
	}

	buf.Reset()
	testLogger.Error("error message")
	if !strings.Contains(buf.String(), "error message") {
		t.Error("Error message should be in output")
	}
}

func TestLoggingWithAttributes(t *testing.T) {
	var buf bytes.Buffer

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewTextHandler(&buf, opts)
	testLogger := slog.New(handler)

	testLogger.Info("test message",
		slog.String("key", "value"),
		slog.Int("count", 42),
		slog.Bool("enabled", true),
	)

	output := buf.String()
	if !strings.Contains(output, "key=value") {
		t.Error("String attribute should be in output")
	}
	if !strings.Contains(output, "count=42") {
		t.Error("Int attribute should be in output")
	}
	if !strings.Contains(output, "enabled=true") {
		t.Error("Bool attribute should be in output")
	}
}

func TestLoggerWith(t *testing.T) {
	var buf bytes.Buffer

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewTextHandler(&buf, opts)
	testLogger := slog.New(handler)

	childLogger := testLogger.With(slog.String("component", "test"))
	childLogger.Info("child message")

	output := buf.String()
	if !strings.Contains(output, "component=test") {
		t.Error("With() attribute should be in output")
	}
}

func TestLoggerWithGroup(t *testing.T) {
	var buf bytes.Buffer

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewTextHandler(&buf, opts)
	testLogger := slog.New(handler)

	groupLogger := testLogger.WithGroup("mygroup")
	groupLogger.Info("grouped message", slog.String("key", "value"))

	output := buf.String()
	if !strings.Contains(output, "mygroup") {
		t.Error("WithGroup() should add group to output")
	}
}
