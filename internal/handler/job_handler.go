package handler

import (
	"encoding/json"
	"errors"
	"job-queue/internal/metrics"
	"job-queue/internal/models"
	"job-queue/internal/repository"
	"job-queue/internal/service"
	"log"
	"net/http"
	"strings"
)

// JobHandler handles HTTP requests for jobs
type JobHandler struct {
	jobService *service.JobService
	metrics    *metrics.Metrics
}

// NewJobHandler creates a new job handler
func NewJobHandler(jobService *service.JobService, metrics *metrics.Metrics) *JobHandler {
	return &JobHandler{
		jobService: jobService,
		metrics:    metrics,
	}
}

// CreateJob handles POST /jobs
func (h *JobHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.TenantID == "" {
		http.Error(w, "tenant_id is required", http.StatusBadRequest)
		return
	}

	if req.Payload == "" {
		http.Error(w, "payload is required", http.StatusBadRequest)
		return
	}

	job, err := h.jobService.CreateJob(r.Context(), &req)
	if err != nil {
		// Log full error for debugging
		log.Printf("error creating job: %v (type: %T)", err, err)

		// Check for specific error types first
		if err == service.ErrRateLimitExceeded {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Check for repository duplicate error type (unwrapped)
		var dupErr *repository.ErrDuplicateIdempotencyKey
		if errors.As(err, &dupErr) {
			http.Error(w, "job creation failed: duplicate idempotency key", http.StatusConflict)
			return
		}

		// Unwrap error to check inner errors
		unwrappedErr := err
		for unwrappedErr != nil {
			if _, ok := unwrappedErr.(*repository.ErrDuplicateIdempotencyKey); ok {
				http.Error(w, "job creation failed: duplicate idempotency key", http.StatusConflict)
				return
			}
			unwrappedErr = errors.Unwrap(unwrappedErr)
		}

		// Provide more descriptive error messages based on error content
		errMsg := err.Error()
		if strings.Contains(errMsg, "UNIQUE constraint") ||
			strings.Contains(errMsg, "unique constraint") ||
			strings.Contains(errMsg, "duplicate idempotency key") {
			http.Error(w, "job creation failed: duplicate idempotency key", http.StatusConflict)
		} else if strings.Contains(errMsg, "failed to create job") {
			if strings.Contains(errMsg, "database") || strings.Contains(errMsg, "connection") {
				http.Error(w, "job creation failed: database error", http.StatusInternalServerError)
			} else {
				// Return the actual error message for better debugging
				http.Error(w, "job creation failed: "+errMsg, http.StatusInternalServerError)
			}
		} else if strings.Contains(errMsg, "failed to check idempotency") {
			http.Error(w, "job creation failed: idempotency check error", http.StatusInternalServerError)
		} else if strings.Contains(errMsg, "failed to get running jobs count") {
			http.Error(w, "job creation failed: rate limit check error", http.StatusInternalServerError)
		} else {
			// Return the actual error message
			http.Error(w, "job creation failed: "+errMsg, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(job); err != nil {
		log.Printf("error encoding response: %v", err)
	}
}

// GetJob handles GET /jobs/{id}
func (h *JobHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/jobs/")
	if path == "" || path == r.URL.Path {
		http.Error(w, "job id is required", http.StatusBadRequest)
		return
	}

	job, err := h.jobService.GetJob(r.Context(), path)
	if err != nil {
		if err == service.ErrJobNotFound {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}
		log.Printf("error getting job: %v", err)

		// Provide more descriptive error messages
		errMsg := err.Error()
		if strings.Contains(errMsg, "database") || strings.Contains(errMsg, "connection") {
			http.Error(w, "failed to retrieve job: database error", http.StatusInternalServerError)
		} else {
			http.Error(w, "failed to retrieve job: "+errMsg, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(job); err != nil {
		log.Printf("error encoding response: %v", err)
	}
}

// ListJobs handles GET /jobs?status=
func (h *JobHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("method not allowed"))
		return
	}

	statusStr := r.URL.Query().Get("status")
	if statusStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("status query parameter is required"))
		return
	}

	status := models.JobStatus(statusStr)
	if status != models.StatusPending && status != models.StatusRunning &&
		status != models.StatusDone && status != models.StatusFailed {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid status"))
		return
	}

	jobs, err := h.jobService.ListJobsByStatus(r.Context(), status)
	if err != nil {
		log.Printf("error listing jobs: %v", err)

		// Provide more descriptive error messages
		errMsg := err.Error()
		if strings.Contains(errMsg, "database") || strings.Contains(errMsg, "connection") {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("failed to list jobs: database error"))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("failed to list jobs: " + errMsg))
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(jobs); err != nil {
		log.Printf("error encoding response: %v", err)
	}
}

// GetMetrics handles GET /metrics
func (h *JobHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot := h.metrics.GetSnapshot()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		log.Printf("error encoding response: %v", err)
	}
}

// GetDeadLetterQueue handles GET /dlq
func (h *JobHandler) GetDeadLetterQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("method not allowed"))
		return
	}

	dlqJobs, err := h.jobService.ListDeadLetterJobs(r.Context())
	if err != nil {
		log.Printf("error listing dead letter jobs: %v", err)

		// Provide more descriptive error messages
		errMsg := err.Error()
		if strings.Contains(errMsg, "database") || strings.Contains(errMsg, "connection") {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("failed to retrieve dead letter queue: database error"))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("failed to retrieve dead letter queue: " + errMsg))
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(dlqJobs); err != nil {
		log.Printf("error encoding response: %v", err)
	}
}
