package service

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_CheckSubmissionRate_WithinLimit(t *testing.T) {
	rl := NewRateLimiter(5, 10)

	err := rl.CheckSubmissionRate(context.Background(), "tenant-1")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestRateLimiter_CheckSubmissionRate_ExceedsLimit(t *testing.T) {
	rl := NewRateLimiter(5, 2) // Max 2 per minute

	// Submit 2 jobs - should succeed
	for i := 0; i < 2; i++ {
		err := rl.CheckSubmissionRate(context.Background(), "tenant-1")
		if err != nil {
			t.Errorf("expected no error for submission %d, got %v", i+1, err)
		}
	}

	// Third submission should fail
	err := rl.CheckSubmissionRate(context.Background(), "tenant-1")
	if err != ErrRateLimitExceeded {
		t.Errorf("expected rate limit error, got %v", err)
	}
}

func TestRateLimiter_CheckSubmissionRate_WindowExpiry(t *testing.T) {
	rl := NewRateLimiter(5, 2)

	// Exhaust limit
	rl.CheckSubmissionRate(context.Background(), "tenant-1")
	rl.CheckSubmissionRate(context.Background(), "tenant-1")

	// Should be rate limited
	err := rl.CheckSubmissionRate(context.Background(), "tenant-1")
	if err != ErrRateLimitExceeded {
		t.Errorf("expected rate limit error, got %v", err)
	}

	// Manually expire window
	rl.mu.Lock()
	if window, exists := rl.submissionWindows["tenant-1"]; exists {
		window.windowEnd = time.Now().Add(-1 * time.Minute)
	}
	rl.mu.Unlock()

	// Should succeed after window expiry
	err = rl.CheckSubmissionRate(context.Background(), "tenant-1")
	if err != nil {
		t.Errorf("expected no error after window expiry, got %v", err)
	}
}

func TestRateLimiter_CheckConcurrentLimit_WithinLimit(t *testing.T) {
	rl := NewRateLimiter(5, 10)

	err := rl.CheckConcurrentLimit(context.Background(), "tenant-1", 3)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestRateLimiter_CheckConcurrentLimit_AtLimit(t *testing.T) {
	rl := NewRateLimiter(5, 10)

	err := rl.CheckConcurrentLimit(context.Background(), "tenant-1", 5)
	if err != ErrRateLimitExceeded {
		t.Errorf("expected rate limit error, got %v", err)
	}
}

func TestRateLimiter_CheckConcurrentLimit_ExceedsLimit(t *testing.T) {
	rl := NewRateLimiter(5, 10)

	err := rl.CheckConcurrentLimit(context.Background(), "tenant-1", 6)
	if err != ErrRateLimitExceeded {
		t.Errorf("expected rate limit error, got %v", err)
	}
}

func TestRateLimiter_MultipleTenants(t *testing.T) {
	rl := NewRateLimiter(5, 2)

	// Tenant 1 exhausts limit
	rl.CheckSubmissionRate(context.Background(), "tenant-1")
	rl.CheckSubmissionRate(context.Background(), "tenant-1")

	// Tenant 2 should still be able to submit
	err := rl.CheckSubmissionRate(context.Background(), "tenant-2")
	if err != nil {
		t.Errorf("expected no error for tenant-2, got %v", err)
	}

	// Tenant 1 should be rate limited
	err = rl.CheckSubmissionRate(context.Background(), "tenant-1")
	if err != ErrRateLimitExceeded {
		t.Errorf("expected rate limit error for tenant-1, got %v", err)
	}
}
