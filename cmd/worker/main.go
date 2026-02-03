package main

import (
	"context"
	"flag"
	"job-queue/internal/metrics"
	"job-queue/internal/repository"
	"job-queue/internal/service"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	dbPath := flag.String("db", "jobs.db", "path to SQLite database")
	flag.Parse()

	// Initialize repository
	repo, err := repository.NewSQLiteRepository(*dbPath)
	if err != nil {
		log.Fatalf("failed to initialize repository: %v", err)
	}
	defer repo.Close()

	// Initialize metrics
	metricsInstance := metrics.NewMetrics()

	// Initialize worker service
	workerService := service.NewWorkerService(repo, metricsInstance)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("shutting down worker...")
		cancel()
	}()

	// Start processing jobs
	leaseDuration := 30 * time.Second
	log.Println("worker started, polling for jobs...")
	
	if err := workerService.ProcessJobs(ctx, leaseDuration); err != nil && err != context.Canceled {
		log.Fatalf("worker error: %v", err)
	}

	log.Println("worker stopped")
}
