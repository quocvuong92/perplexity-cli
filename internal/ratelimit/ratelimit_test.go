package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestNewLimiter(t *testing.T) {
	l := NewLimiter(60)
	if l == nil {
		t.Error("Expected non-nil limiter")
	}
	if l.interval != time.Second {
		t.Errorf("Expected 1s interval for 60 rpm, got %v", l.interval)
	}
}

func TestNewLimiterZero(t *testing.T) {
	l := NewLimiter(0)
	if l != nil {
		t.Error("Expected nil limiter for 0 rpm")
	}
}

func TestNewLimiterNegative(t *testing.T) {
	l := NewLimiter(-1)
	if l != nil {
		t.Error("Expected nil limiter for negative rpm")
	}
}

func TestWaitNilLimiter(t *testing.T) {
	var l *Limiter
	err := l.Wait(context.Background())
	if err != nil {
		t.Errorf("Expected no error for nil limiter, got %v", err)
	}
}

func TestWaitFirstRequest(t *testing.T) {
	l := NewLimiter(6000)
	start := time.Now()
	err := l.Wait(context.Background())
	elapsed := time.Since(start)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("First request should be immediate, took %v", elapsed)
	}
}

func TestWaitContextCancelled(t *testing.T) {
	l := NewLimiter(1)
	_ = l.Wait(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := l.Wait(ctx)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}
