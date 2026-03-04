package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// =============================================================================
// Data Types
// =============================================================================

// LogEntry represents a single log line with all fields
type LogEntry struct {
	Timestamp    string `json:"timestamp"`
	WorkerID     string `json:"worker_id"`
	RequestID    string `json:"request_id"`
	Component    string `json:"component"`
	ClientIP     string `json:"client_ip"`
	HTTPMethod   string `json:"http_method"`
	Endpoint     string `json:"endpoint"`
	StatusCode   int    `json:"status_code"`
	ResponseTime int    `json:"response_time_ms"`
	Latency      int    `json:"latency_ms"`
	ResponseSize int    `json:"response_size"`
	Message      string `json:"message"`
	LogLevel     string `json:"log_level"` // INFO, WARN, ERROR derived from status code
}

// MapRequest is sent from master to worker to request a batch of mapped logs
type MapRequest struct {
	BatchSize int `json:"batch_size"`
}

// MapResponse is returned from worker after executing Map function
// Contains intermediate key-value pairs from the Map phase
type MapResponse struct {
	LogLevelCounts   map[string]int   `json:"log_level_counts"`   // Count of INFO/WARN/ERROR
	EndpointCounts   map[string]int   `json:"endpoint_counts"`    // Count per endpoint
	StatusCodeCounts map[int]int      `json:"status_code_counts"` // Count per HTTP status
	LatencyBuckets   map[string][]int `json:"latency_buckets"`    // Raw latencies grouped by bucket
	WorkerCounts     map[string]int   `json:"worker_counts"`      // Logs per worker
	ProcessedCount   int              `json:"processed_count"`    // Total logs processed
	LogLines         []LogEntry       `json:"log_lines"`          // Raw logs for display
}

// AggregatedMetrics is the final output after Reduce phase
// Contains both window stats (last 2s) and cumulative stats (since start)
type AggregatedMetrics struct {
	// Cumulative (since start)
	LogLevelCounts   map[string]int  `json:"log_level_counts"`
	EndpointCounts   map[string]int  `json:"endpoint_counts"`
	StatusCodeCounts map[int]int     `json:"status_code_counts"`
	TotalRequests    int             `json:"total_requests"`
	AverageLatency   float64         `json:"average_latency"`
	ErrorRate        float64         `json:"error_rate"`
	TopEndpoints     []EndpointCount `json:"top_endpoints"`

	// For display
	LatencyHistory []LatencyDataPoint `json:"latency_history"`
	RecentLogs     []LogEntry         `json:"recent_logs"`

	// Window-specific (last 2 seconds)
	WindowRequests   int     `json:"window_requests"`
	WindowAvgLatency float64 `json:"window_avg_latency"`
	WindowErrorRate  float64 `json:"window_error_rate"`

	Timestamp time.Time `json:"timestamp"`
}

type EndpointCount struct {
	Endpoint string `json:"endpoint"`
	Count    int    `json:"count"`
}

type LatencyDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Latency   float64   `json:"latency"`
}

// =============================================================================
// Log Generation
// =============================================================================

// Sample data for realistic log generation
var (
	components = []string{"auth", "api", "gateway", "payment", "user", "notification", "search"}
	endpoints  = []string{"/login", "/logout", "/register", "/profile", "/dashboard", "/api/users", "/api/products", "/api/orders", "/api/search", "/webhook"}
	methods    = []string{"GET", "POST", "PUT", "DELETE"}
	messages   = []string{
		"Request processed successfully",
		"User login success",
		"User login failed",
		"User logged out",
		"Profile updated",
		"Data fetched from database",
		"Cache hit",
		"Cache miss",
		"Rate limit exceeded",
		"Invalid request payload",
		"Resource not found",
		"Internal server error",
		"Database connection timeout",
		"External API call failed",
		"Authentication token expired",
	}
)

// GenerateLogEntry creates a realistic mock log entry
// Status codes are weighted to simulate real traffic (mostly 200s, some errors)
func GenerateLogEntry(workerID string) LogEntry {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Weighted status codes: mostly success (200), some redirects (3xx), some errors (4xx, 5xx)
	statusCodes := []int{200, 200, 200, 200, 201, 204, 301, 400, 401, 403, 404, 500, 503}
	statusCode := statusCodes[r.Intn(len(statusCodes))]

	// Derive log level from status code
	logLevel := "INFO"
	if statusCode >= 400 && statusCode < 500 {
		logLevel = "WARN"
	} else if statusCode >= 500 {
		logLevel = "ERROR"
	} else if r.Float32() < 0.05 { // 5% chance of WARN for successful requests
		logLevel = "WARN"
	}

	// Random but realistic values
	responseTime := r.Intn(500) + 10    // 10-510ms
	latency := r.Intn(responseTime)     // Latency < response time
	responseSize := r.Intn(10000) + 100 // 100-10100 bytes

	return LogEntry{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		WorkerID:     workerID,
		RequestID:    generateRequestID(r),
		Component:    components[r.Intn(len(components))],
		ClientIP:     generateClientIP(r),
		HTTPMethod:   methods[r.Intn(len(methods))],
		Endpoint:     endpoints[r.Intn(len(endpoints))],
		StatusCode:   statusCode,
		ResponseTime: responseTime,
		Latency:      latency,
		ResponseSize: responseSize,
		Message:      messages[r.Intn(len(messages))],
		LogLevel:     logLevel,
	}
}

// generateRequestID creates a random 8-character hex ID
func generateRequestID(r *rand.Rand) string {
	const charset = "abcdef0123456789"
	id := make([]byte, 8)
	for i := range id {
		id[i] = charset[r.Intn(len(charset))]
	}
	return string(id)
}

// generateClientIP creates a mock 192.168.x.x IP address
func generateClientIP(r *rand.Rand) string {
	return fmt.Sprintf("192.168.%d.%d", r.Intn(256), r.Intn(256))
}

// =============================================================================
// MAP PHASE - Executed on each Worker
// =============================================================================

// MapEntries is the Map function in MapReduce
// Input: batch of log entries from the worker's queue
// Output: intermediate key-value pairs (counts, buckets, raw logs)
//
// This function extracts meaningful metrics from raw logs:
// - Counts by log level (INFO/WARN/ERROR)
// - Counts by endpoint
// - Counts by HTTP status code
// - Latency buckets for distribution analysis
// - Raw log entries for live display
func MapEntries(logEntries []LogEntry) MapResponse {
	result := MapResponse{
		LogLevelCounts:   make(map[string]int),
		EndpointCounts:   make(map[string]int),
		StatusCodeCounts: make(map[int]int),
		LatencyBuckets:   make(map[string][]int),
		WorkerCounts:     make(map[string]int),
		ProcessedCount:   len(logEntries),
		LogLines:         logEntries,
	}

	// Extract metrics from each log entry
	for _, entry := range logEntries {
		result.LogLevelCounts[entry.LogLevel]++
		result.EndpointCounts[entry.Endpoint]++
		result.StatusCodeCounts[entry.StatusCode]++
		result.WorkerCounts[entry.WorkerID]++

		// Bucket latency for distribution analysis
		bucket := getLatencyBucket(entry.ResponseTime)
		result.LatencyBuckets[bucket] = append(result.LatencyBuckets[bucket], entry.ResponseTime)
	}

	return result
}

// getLatencyBucket groups latency values into buckets for analysis
func getLatencyBucket(latency int) string {
	switch {
	case latency < 50:
		return "0-50ms"
	case latency < 100:
		return "50-100ms"
	case latency < 200:
		return "100-200ms"
	case latency < 500:
		return "200-500ms"
	default:
		return "500ms+"
	}
}

// =============================================================================
// REDUCE PHASE - Executed on Master
// =============================================================================

// Reduce aggregates intermediate results from all workers
// Input: array of MapResponse from each worker
// Output: AggregatedMetrics with combined totals for this polling window
//
// This is the "Reduce" in MapReduce - it combines results from all workers:
// - Sums up all counts
// - Calculates window-level averages
// - Identifies top endpoints
func Reduce(results []MapResponse) AggregatedMetrics {
	metrics := AggregatedMetrics{
		LogLevelCounts:   make(map[string]int),
		EndpointCounts:   make(map[string]int),
		StatusCodeCounts: make(map[int]int),
		TotalRequests:    0,
		Timestamp:        time.Now(),
	}

	var totalLatency int
	var errorCount int

	// Aggregate all results from workers
	for _, result := range results {
		// Sum counts
		for k, v := range result.LogLevelCounts {
			metrics.LogLevelCounts[k] += v
		}
		for k, v := range result.EndpointCounts {
			metrics.EndpointCounts[k] += v
		}
		for k, v := range result.StatusCodeCounts {
			metrics.StatusCodeCounts[k] += v
		}

		metrics.TotalRequests += result.ProcessedCount

		// Sum latencies for average calculation
		for _, latencies := range result.LatencyBuckets {
			for _, lat := range latencies {
				totalLatency += lat
			}
		}

		// Count 5xx errors (500, 503)
		errorCount += result.StatusCodeCounts[500] + result.StatusCodeCounts[503]
	}

	// Calculate window-level averages
	if metrics.TotalRequests > 0 {
		metrics.AverageLatency = float64(totalLatency) / float64(metrics.TotalRequests)
		metrics.ErrorRate = float64(errorCount) / float64(metrics.TotalRequests) * 100
	}

	// Find top 5 endpoints by request count
	var topEndpoints []EndpointCount
	for endpoint, count := range metrics.EndpointCounts {
		topEndpoints = append(topEndpoints, EndpointCount{Endpoint: endpoint, Count: count})
	}
	// Simple bubble sort for small dataset
	for i := 0; i < len(topEndpoints)-1; i++ {
		for j := i + 1; j < len(topEndpoints); j++ {
			if topEndpoints[j].Count > topEndpoints[i].Count {
				topEndpoints[i], topEndpoints[j] = topEndpoints[j], topEndpoints[i]
			}
		}
	}
	if len(topEndpoints) > 5 {
		topEndpoints = topEndpoints[:5]
	}
	metrics.TopEndpoints = topEndpoints

	return metrics
}

// ToJSON serializes metrics to JSON for SSE response
func (m *AggregatedMetrics) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}
