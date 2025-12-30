package retry

import (
	"context"
	"errors"
	"net"
	"net/url"
	"syscall"
	"testing"
	"time"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "connection refused",
			err:      syscall.ECONNREFUSED,
			expected: true,
		},
		{
			name:     "connection reset",
			err:      syscall.ECONNRESET,
			expected: true,
		},
		{
			name:     "network unreachable",
			err:      syscall.ENETUNREACH,
			expected: true,
		},
		{
			name:     "host unreachable",
			err:      syscall.EHOSTUNREACH,
			expected: true,
		},
		{
			name:     "connection timed out",
			err:      syscall.ETIMEDOUT,
			expected: true,
		},
		{
			name:     "generic error",
			err:      errors.New("some random error"),
			expected: false,
		},
		{
			name:     "error with connection refused in message",
			err:      errors.New("dial tcp: connection refused"),
			expected: true,
		},
		{
			name:     "error with timeout in message",
			err:      errors.New("i/o timeout"),
			expected: true,
		},
		{
			name:     "error with EOF",
			err:      errors.New("unexpected eof"),
			expected: true,
		},
		{
			name:     "url error wrapping connection refused",
			err:      &url.Error{Op: "Post", URL: "https://example.com", Err: syscall.ECONNREFUSED},
			expected: true,
		},
		{
			name:     "net op error",
			err:      &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetryableError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	cfg := Config{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
		Jitter:         0, // No jitter for predictable tests
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{10, 10 * time.Second}, // Should be capped at MaxBackoff
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := cfg.CalculateBackoff(tt.attempt)
			if result != tt.expected {
				t.Errorf("CalculateBackoff(%d) = %v, expected %v", tt.attempt, result, tt.expected)
			}
		})
	}
}

func TestCalculateBackoffWithJitter(t *testing.T) {
	cfg := Config{
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		Multiplier:     2.0,
		Jitter:         0.2,
	}

	// With 20% jitter on 1 second, result should be between 0.8s and 1.2s
	for i := 0; i < 10; i++ {
		result := cfg.CalculateBackoff(0)
		if result < 800*time.Millisecond || result > 1200*time.Millisecond {
			t.Errorf("CalculateBackoff with jitter = %v, expected between 800ms and 1200ms", result)
		}
	}
}

func TestWaitContextCancellation(t *testing.T) {
	cfg := Config{
		InitialBackoff: 10 * time.Second,
		MaxBackoff:     30 * time.Second,
		Multiplier:     2.0,
		Jitter:         0,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	start := time.Now()
	err := cfg.Wait(ctx, 0)
	elapsed := time.Since(start)

	if err != context.Canceled {
		t.Errorf("Wait() returned %v, expected context.Canceled", err)
	}

	if elapsed > 100*time.Millisecond {
		t.Errorf("Wait() took %v, expected immediate return on cancellation", elapsed)
	}
}

func TestDoSuccess(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxRetries = 3

	attempts := 0
	err := Do(context.Background(), cfg, func() error {
		attempts++
		return nil
	}, nil)

	if err != nil {
		t.Errorf("Do() returned error: %v", err)
	}

	if attempts != 1 {
		t.Errorf("Do() made %d attempts, expected 1", attempts)
	}
}

func TestDoRetryableError(t *testing.T) {
	cfg := Config{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
		Jitter:         0,
	}

	attempts := 0
	err := Do(context.Background(), cfg, func() error {
		attempts++
		if attempts < 3 {
			return syscall.ECONNREFUSED
		}
		return nil
	}, nil)

	if err != nil {
		t.Errorf("Do() returned error: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Do() made %d attempts, expected 3", attempts)
	}
}

func TestDoNonRetryableError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InitialBackoff = 1 * time.Millisecond

	expectedErr := errors.New("non-retryable error")
	attempts := 0

	err := Do(context.Background(), cfg, func() error {
		attempts++
		return expectedErr
	}, nil)

	if err != expectedErr {
		t.Errorf("Do() returned %v, expected %v", err, expectedErr)
	}

	if attempts != 1 {
		t.Errorf("Do() made %d attempts, expected 1 for non-retryable error", attempts)
	}
}

func TestDoMaxRetriesExceeded(t *testing.T) {
	cfg := Config{
		MaxRetries:     2,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
		Jitter:         0,
	}

	attempts := 0
	err := Do(context.Background(), cfg, func() error {
		attempts++
		return syscall.ECONNREFUSED
	}, nil)

	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Errorf("Do() returned %v, expected ECONNREFUSED", err)
	}

	// Initial attempt + 2 retries = 3 attempts
	if attempts != 3 {
		t.Errorf("Do() made %d attempts, expected 3", attempts)
	}
}

func TestDoContextCancellation(t *testing.T) {
	cfg := Config{
		MaxRetries:     10,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
		Jitter:         0,
	}

	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := Do(ctx, cfg, func() error {
		attempts++
		return syscall.ECONNREFUSED
	}, nil)

	if err != context.Canceled {
		t.Errorf("Do() returned %v, expected context.Canceled", err)
	}
}

func TestDoOnRetryCallback(t *testing.T) {
	cfg := Config{
		MaxRetries:     2,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
		Jitter:         0,
	}

	retryInfos := []RetryInfo{}
	attempts := 0

	_ = Do(context.Background(), cfg, func() error {
		attempts++
		return syscall.ECONNREFUSED
	}, func(info RetryInfo) {
		retryInfos = append(retryInfos, info)
	})

	// Should have 2 retry callbacks (before retry 1 and retry 2)
	if len(retryInfos) != 2 {
		t.Errorf("Got %d retry callbacks, expected 2", len(retryInfos))
	}

	if len(retryInfos) >= 1 {
		if retryInfos[0].Attempt != 0 {
			t.Errorf("First retry attempt = %d, expected 0", retryInfos[0].Attempt)
		}
		if retryInfos[0].MaxRetries != 2 {
			t.Errorf("MaxRetries = %d, expected 2", retryInfos[0].MaxRetries)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("DefaultConfig MaxRetries = %d, expected 3", cfg.MaxRetries)
	}

	if cfg.InitialBackoff != 500*time.Millisecond {
		t.Errorf("DefaultConfig InitialBackoff = %v, expected 500ms", cfg.InitialBackoff)
	}

	if cfg.MaxBackoff != 30*time.Second {
		t.Errorf("DefaultConfig MaxBackoff = %v, expected 30s", cfg.MaxBackoff)
	}

	if cfg.Multiplier != 2.0 {
		t.Errorf("DefaultConfig Multiplier = %f, expected 2.0", cfg.Multiplier)
	}

	if cfg.Jitter != 0.2 {
		t.Errorf("DefaultConfig Jitter = %f, expected 0.2", cfg.Jitter)
	}
}
