package retry

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"net"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// Config holds retry configuration
type Config struct {
	MaxRetries     int           // Maximum number of retry attempts
	InitialBackoff time.Duration // Initial backoff duration
	MaxBackoff     time.Duration // Maximum backoff duration
	Multiplier     float64       // Backoff multiplier (e.g., 2.0 for exponential)
	Jitter         float64       // Jitter factor (0.0-1.0) to add randomness
}

// DefaultConfig returns the default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxRetries:     3,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     30 * time.Second,
		Multiplier:     2.0,
		Jitter:         0.2,
	}
}

// IsRetryableError checks if an error is a transient network error that can be retried
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context cancellation - don't retry these
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		// Timeout errors are retryable
		if netErr.Timeout() {
			return true
		}
	}

	// Check for DNS errors (always retry DNS lookup failures)
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	// Check for connection refused, reset, etc.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// Check for URL errors (connection issues)
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		// Unwrap and check the underlying error
		return IsRetryableError(urlErr.Err)
	}

	// Check for specific syscall errors
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.EHOSTUNREACH) {
		return true
	}

	// Check error message for common network error patterns
	errMsg := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"connection timed out",
		"no such host",
		"network is unreachable",
		"host is unreachable",
		"temporary failure",
		"try again",
		"i/o timeout",
		"eof",
		"broken pipe",
		"connection closed",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// CalculateBackoff calculates the backoff duration for a given attempt
func (c Config) CalculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return c.InitialBackoff
	}

	// Calculate exponential backoff
	backoff := float64(c.InitialBackoff) * math.Pow(c.Multiplier, float64(attempt))

	// Apply maximum backoff cap
	if backoff > float64(c.MaxBackoff) {
		backoff = float64(c.MaxBackoff)
	}

	// Apply jitter
	if c.Jitter > 0 {
		jitterRange := backoff * c.Jitter
		backoff = backoff - jitterRange + (rand.Float64() * 2 * jitterRange)
	}

	return time.Duration(backoff)
}

// Wait waits for the calculated backoff duration, respecting context cancellation
func (c Config) Wait(ctx context.Context, attempt int) error {
	backoff := c.CalculateBackoff(attempt)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(backoff):
		return nil
	}
}

// RetryInfo contains information about a retry attempt for logging/callbacks
type RetryInfo struct {
	Attempt     int           // Current attempt number (0-indexed)
	MaxRetries  int           // Maximum retries configured
	Error       error         // The error that triggered the retry
	NextBackoff time.Duration // The backoff duration before next attempt
}

// OnRetryFunc is a callback function called before each retry attempt
type OnRetryFunc func(info RetryInfo)

// Do executes a function with retry logic
func Do(ctx context.Context, cfg Config, fn func() error, onRetry OnRetryFunc) error {
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Check context before attempting
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryableError(err) {
			return err
		}

		// Don't wait after the last attempt
		if attempt == cfg.MaxRetries {
			break
		}

		// Calculate next backoff
		nextBackoff := cfg.CalculateBackoff(attempt)

		// Call retry callback if provided
		if onRetry != nil {
			onRetry(RetryInfo{
				Attempt:     attempt,
				MaxRetries:  cfg.MaxRetries,
				Error:       err,
				NextBackoff: nextBackoff,
			})
		}

		// Wait before next attempt
		if err := cfg.Wait(ctx, attempt); err != nil {
			return err
		}
	}

	return lastErr
}
