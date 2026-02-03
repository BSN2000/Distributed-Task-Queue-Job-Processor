package handler

import (
	"encoding/json"
	"job-queue/internal/metrics"
	"job-queue/internal/models"
	"job-queue/internal/service"
	"log"
	"net/http"
	"strings"
)

// JobHandler handles HTTP requests for jobs
type JobHandler struct {
	jobService   *service.JobService
	metrics      *metrics.Metrics
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
		if err == service.ErrRateLimitExceeded {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		log.Printf("error creating job: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
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
		http.Error(w, "internal server error", http.StatusInternalServerError)
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
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
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
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(dlqJobs); err != nil {
		log.Printf("error encoding response: %v", err)
	}
}
