package ratelimit

import (
	"context"
	"sync"
	"time"
)

type Limiter struct {
	mu          sync.Mutex
	rate        float64
	interval    time.Duration
	lastRequest time.Time
}

func NewLimiter(requestsPerMinute float64) *Limiter {
	if requestsPerMinute <= 0 {
		return nil
	}
	return &Limiter{
		rate:     requestsPerMinute,
		interval: time.Duration(float64(time.Minute) / requestsPerMinute),
	}
}

func (l *Limiter) Wait(ctx context.Context) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.lastRequest)

	if elapsed < l.interval {
		waitTime := l.interval - elapsed
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
		}
	}

	l.lastRequest = time.Now()
	return nil
}
