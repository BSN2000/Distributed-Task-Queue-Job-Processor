package main

import (
	"flag"
	"job-queue/internal/handler"
	"job-queue/internal/metrics"
	"job-queue/internal/repository"
	"job-queue/internal/service"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	dbPath := flag.String("db", "jobs.db", "path to SQLite database")
	port := flag.String("port", "8080", "HTTP server port")
	flag.Parse()

	// Initialize repository
	repo, err := repository.NewSQLiteRepository(*dbPath)
	if err != nil {
		log.Fatalf("failed to initialize repository: %v", err)
	}
	defer repo.Close()

	// Initialize metrics
	metricsInstance := metrics.NewMetrics()

	// Initialize rate limiter
	rateLimiter := service.NewRateLimiter(5, 10) // 5 concurrent, 10 per minute

	// Initialize services
	jobService := service.NewJobService(repo, rateLimiter, metricsInstance)

	// Initialize handlers
	jobHandler := handler.NewJobHandler(jobService, metricsInstance)

	// CORS middleware - sets headers for all responses
	corsMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers first
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			
			// Handle preflight OPTIONS request
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			
			// Call the actual handler
			next(w, r)
		}
	}

	// Setup routes with CORS
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			jobHandler.CreateJob(w, r)
		} else if r.Method == http.MethodGet {
			jobHandler.ListJobs(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	mux.HandleFunc("/jobs/", corsMiddleware(jobHandler.GetJob))
	mux.HandleFunc("/metrics", corsMiddleware(jobHandler.GetMetrics))
	mux.HandleFunc("/dlq", corsMiddleware(jobHandler.GetDeadLetterQueue))

	// Start server
	server := &http.Server{
		Addr:    ":" + *port,
		Handler: mux,
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("API server starting on port %s", *port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-sigChan
	log.Println("shutting down server...")
	if err := server.Close(); err != nil {
		log.Printf("error closing server: %v", err)
	}
	log.Println("server stopped")
}
