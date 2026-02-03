package service

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements per-tenant rate limiting
type RateLimiter struct {
	mu sync.RWMutex

	// Per-tenant concurrent running jobs limit
	maxConcurrentRunning int

	// Per-tenant submission rate limit
	maxSubmissionsPerMinute int
	submissionWindows       map[string]*submissionWindow
}

type submissionWindow struct {
	count     int
	windowEnd time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxConcurrentRunning, maxSubmissionsPerMinute int) *RateLimiter {
	return &RateLimiter{
		maxConcurrentRunning:    maxConcurrentRunning,
		maxSubmissionsPerMinute: maxSubmissionsPerMinute,
		submissionWindows:       make(map[string]*submissionWindow),
	}
}

// CheckConcurrentLimit checks if a tenant can run more concurrent jobs
func (rl *RateLimiter) CheckConcurrentLimit(ctx context.Context, tenantID string, currentRunning int) error {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if currentRunning >= rl.maxConcurrentRunning {
		return ErrRateLimitExceeded
	}

	return nil
}

// CheckSubmissionRate checks if a tenant can submit more jobs
func (rl *RateLimiter) CheckSubmissionRate(ctx context.Context, tenantID string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	window, exists := rl.submissionWindows[tenantID]

	if !exists || now.After(window.windowEnd) {
		// New window or expired window
		rl.submissionWindows[tenantID] = &submissionWindow{
			count:     1,
			windowEnd: now.Add(1 * time.Minute),
		}
		return nil
	}

	if window.count >= rl.maxSubmissionsPerMinute {
		return ErrRateLimitExceeded
	}

	window.count++
	return nil
}
