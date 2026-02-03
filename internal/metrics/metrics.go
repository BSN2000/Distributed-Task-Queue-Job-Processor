package metrics

import (
	"sync"
)

// Metrics tracks system metrics
type Metrics struct {
	mu sync.RWMutex

	totalJobs     int64
	completedJobs int64
	failedJobs    int64
	retriedJobs   int64
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{}
}

// IncrementTotalJobs increments the total jobs counter
func (m *Metrics) IncrementTotalJobs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalJobs++
}

// IncrementCompletedJobs increments the completed jobs counter
func (m *Metrics) IncrementCompletedJobs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completedJobs++
}

// IncrementFailedJobs increments the failed jobs counter
func (m *Metrics) IncrementFailedJobs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failedJobs++
}

// IncrementRetriedJobs increments the retried jobs counter
func (m *Metrics) IncrementRetriedJobs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retriedJobs++
}

// GetSnapshot returns a snapshot of all metrics
func (m *Metrics) GetSnapshot() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]int64{
		"total_jobs":     m.totalJobs,
		"completed_jobs": m.completedJobs,
		"failed_jobs":    m.failedJobs,
		"retried_jobs":   m.retriedJobs,
	}
}
