package ratelimit

import (
	"context"
	"sync"
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

func TestWaitConcurrent(t *testing.T) {
	// Create a limiter with 60 requests per minute (1 per second)
	l := NewLimiter(60)

	// First request to set lastRequest
	_ = l.Wait(context.Background())

	// Test that lock is not held during sleep by verifying a second goroutine
	// can acquire the lock and be cancelled while the first is sleeping.
	// With the old implementation (lock held during sleep), the second goroutine
	// would be blocked waiting for the lock and couldn't respond to cancellation.
	var wg sync.WaitGroup
	wg.Add(2)

	start := time.Now()
	cancelledAt := make(chan time.Duration, 1)
	completedAt := make(chan time.Duration, 1)

	// Goroutine 1: waits normally
	go func() {
		defer wg.Done()
		_ = l.Wait(context.Background())
		completedAt <- time.Since(start)
	}()

	// Give goroutine 1 a moment to acquire lock and start sleeping
	time.Sleep(50 * time.Millisecond)

	// Goroutine 2: has a cancelled context, should return immediately
	// if lock is not held during sleep
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Pre-cancel the context
		err := l.Wait(ctx)
		if err == context.Canceled {
			cancelledAt <- time.Since(start)
		}
	}()

	wg.Wait()
	close(cancelledAt)
	close(completedAt)

	// The cancelled goroutine should return quickly (not blocked waiting for lock)
	cancelDuration := <-cancelledAt
	if cancelDuration > 200*time.Millisecond {
		t.Errorf("Cancelled goroutine took too long: %v (expected < 200ms)", cancelDuration)
	}

	// The first goroutine should complete at ~1s
	completeDuration := <-completedAt
	if completeDuration < 900*time.Millisecond || completeDuration > 1200*time.Millisecond {
		t.Errorf("First goroutine completed at unexpected time: %v (expected ~1s)", completeDuration)
	}
}
