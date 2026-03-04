package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// Worker generates realistic log entries and processes them via Map function
// Each worker runs as a separate process/port and communicates via HTTP
type Worker struct {
	ID           string
	Port         string
	LogQueue     chan LogEntry // Buffered channel - acts as the log buffer
	QueueSize    int           // Capacity of the log queue
	Server       *http.Server  // HTTP server for this worker
	mu           sync.Mutex
	processedCnt int // Total logs processed (for stats)
}

// NewWorker creates a new worker with a buffered channel queue
// queueSize determines how many logs can be buffered before dropping
func NewWorker(id string, port string, queueSize int) *Worker {
	return &Worker{
		ID:        id,
		Port:      port,
		LogQueue:  make(chan LogEntry, queueSize),
		QueueSize: queueSize,
	}
}

// Start launches the worker's log generator and HTTP server
func (w *Worker) Start() error {
	// Start the continuous log generator
	w.startLogGenerator()

	// Set up HTTP handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/map", w.handleMap)       // Map function endpoint
	mux.HandleFunc("/health", w.handleHealth) // Health check

	w.Server = &http.Server{
		Addr:    ":" + w.Port,
		Handler: mux,
	}

	log.Printf("Worker %s starting on port %s", w.ID, w.Port)
	return w.Server.ListenAndServe()
}

// startLogGenerator runs continuously in a goroutine
// Generates a new log entry every 100ms and pushes to the queue
// Uses non-blocking send (select with default) to prevent blocking if queue is full
func (w *Worker) startLogGenerator() {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			entry := GenerateLogEntry(w.ID)
			select {
			case w.LogQueue <- entry:
				// Successfully queued
			default:
				// Queue full - drop this log (backpressure handling)
			}
		}
	}()
}

// handleMap is the Map function endpoint
// Receives a batch size, dequeues that many logs, runs Map, returns results
// This is the "Map" phase of MapReduce - executed on each worker
func (w *Worker) handleMap(wr http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(wr, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var mapReq MapRequest
	if err := json.NewDecoder(req.Body).Decode(&mapReq); err != nil {
		http.Error(wr, err.Error(), http.StatusBadRequest)
		return
	}

	// Default batch size if not specified
	batchSize := mapReq.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	// Dequeue logs from queue (non-blocking)
	logEntries := w.dequeueBatch(batchSize)

	// Return empty response if no logs available
	if len(logEntries) == 0 {
		resp := MapResponse{
			LogLevelCounts:   make(map[string]int),
			EndpointCounts:   make(map[string]int),
			StatusCodeCounts: make(map[int]int),
			LatencyBuckets:   make(map[string][]int),
			WorkerCounts:     make(map[string]int),
			ProcessedCount:   0,
			LogLines:         []LogEntry{},
		}
		wr.Header().Set("Content-Type", "application/json")
		json.NewEncoder(wr).Encode(resp)
		return
	}

	// Execute Map function on the batch of log entries
	result := MapEntries(logEntries)

	// Track processed count (thread-safe)
	w.mu.Lock()
	w.processedCnt += result.ProcessedCount
	w.mu.Unlock()

	wr.Header().Set("Content-Type", "application/json")
	json.NewEncoder(wr).Encode(result)
}

// handleHealth returns worker status for monitoring
func (w *Worker) handleHealth(wr http.ResponseWriter, req *http.Request) {
	wr.Header().Set("Content-Type", "application/json")
	json.NewEncoder(wr).Encode(map[string]interface{}{
		"worker_id":      w.ID,
		"port":           w.Port,
		"queue_size":     len(w.LogQueue),
		"queue_capacity": w.QueueSize,
		"processed":      w.processedCnt,
	})
}

// dequeueBatch retrieves up to batchSize logs from the queue
// Uses non-blocking receives (select with default) for fast response
func (w *Worker) dequeueBatch(batchSize int) []LogEntry {
	logEntries := make([]LogEntry, 0, batchSize)

	for i := 0; i < batchSize; i++ {
		select {
		case entry := <-w.LogQueue:
			logEntries = append(logEntries, entry)
		default:
			// No more logs available - exit early
			break
		}
	}

	return logEntries
}
