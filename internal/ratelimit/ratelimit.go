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
	now := time.Now()
	elapsed := now.Sub(l.lastRequest)
	var waitTime time.Duration

	if elapsed < l.interval {
		waitTime = l.interval - elapsed
	}

	// Update lastRequest before releasing lock to reserve our slot
	l.lastRequest = now.Add(waitTime)
	l.mu.Unlock()

	// Sleep outside the lock
	if waitTime > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
		}
	}

	return nil
}
