package metrics

import (
	"sync"
	"testing"
)

func TestMetrics_IncrementTotalJobs(t *testing.T) {
	m := NewMetrics()
	m.IncrementTotalJobs()

	snapshot := m.GetSnapshot()
	if snapshot["total_jobs"] != 1 {
		t.Errorf("expected total_jobs 1, got %d", snapshot["total_jobs"])
	}
}

func TestMetrics_IncrementCompletedJobs(t *testing.T) {
	m := NewMetrics()
	m.IncrementCompletedJobs()

	snapshot := m.GetSnapshot()
	if snapshot["completed_jobs"] != 1 {
		t.Errorf("expected completed_jobs 1, got %d", snapshot["completed_jobs"])
	}
}

func TestMetrics_IncrementFailedJobs(t *testing.T) {
	m := NewMetrics()
	m.IncrementFailedJobs()

	snapshot := m.GetSnapshot()
	if snapshot["failed_jobs"] != 1 {
		t.Errorf("expected failed_jobs 1, got %d", snapshot["failed_jobs"])
	}
}

func TestMetrics_IncrementRetriedJobs(t *testing.T) {
	m := NewMetrics()
	m.IncrementRetriedJobs()

	snapshot := m.GetSnapshot()
	if snapshot["retried_jobs"] != 1 {
		t.Errorf("expected retried_jobs 1, got %d", snapshot["retried_jobs"])
	}
}

func TestMetrics_ConcurrentAccess(t *testing.T) {
	m := NewMetrics()
	var wg sync.WaitGroup

	// Concurrent increments
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.IncrementTotalJobs()
			m.IncrementCompletedJobs()
			m.IncrementFailedJobs()
			m.IncrementRetriedJobs()
		}()
	}

	wg.Wait()

	snapshot := m.GetSnapshot()
	if snapshot["total_jobs"] != 100 {
		t.Errorf("expected total_jobs 100, got %d", snapshot["total_jobs"])
	}
	if snapshot["completed_jobs"] != 100 {
		t.Errorf("expected completed_jobs 100, got %d", snapshot["completed_jobs"])
	}
}

func TestMetrics_GetSnapshot(t *testing.T) {
	m := NewMetrics()
	m.IncrementTotalJobs()
	m.IncrementTotalJobs()
	m.IncrementCompletedJobs()
	m.IncrementFailedJobs()
	m.IncrementRetriedJobs()

	snapshot := m.GetSnapshot()

	expected := map[string]int64{
		"total_jobs":     2,
		"completed_jobs": 1,
		"failed_jobs":    1,
		"retried_jobs":   1,
	}

	for key, expectedValue := range expected {
		if snapshot[key] != expectedValue {
			t.Errorf("expected %s %d, got %d", key, expectedValue, snapshot[key])
		}
	}
}
