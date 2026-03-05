package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Master coordinates the distributed log analysis
// It polls workers for log data, performs reduce aggregation,
// and serves metrics via SSE to connected clients
type Master struct {
	Port      string
	Workers   []string
	Metrics   *AggregatedMetrics
	MetricsMu sync.RWMutex

	// Cumulative stats - accumulate over time since start
	CumulativeTotalRequests    int
	CumulativeLogLevelCounts   map[string]int
	CumulativeEndpointCounts   map[string]int
	CumulativeStatusCodeCounts map[int]int
	CumulativeTotalLatency     int // Sum of all latencies for calculating average
	CumulativeErrorCount       int // Sum of all errors for calculating error rate

	// Window stats - track history for charts
	LatencyHistory []LatencyDataPoint // Keep last 60 data points (~2 min of data)
	RecentLogs     []LogEntry         // Keep last 100 log entries for display

	Ticker   *time.Ticker // Triggers collectAndReduce every 2 seconds
	StopChan chan struct{}
}

func NewMaster(port string, workers []string) *Master {
	return &Master{
		Port:                       port,
		Workers:                    workers,
		Metrics:                    &AggregatedMetrics{},
		CumulativeLogLevelCounts:   make(map[string]int),
		CumulativeEndpointCounts:   make(map[string]int),
		CumulativeStatusCodeCounts: make(map[int]int),
		StopChan:                   make(chan struct{}),
	}
}

// StartCoordinator begins the polling loop
// Runs collectAndReduce() every 2 seconds in a goroutine
func (m *Master) StartCoordinator() error {
	m.Ticker = time.NewTicker(2 * time.Second)
	go func() {
		for {
			select {
			case <-m.Ticker.C:
				m.collectAndReduce()
			case <-m.StopChan:
				return
			}
		}
	}()
	return nil
}

func (m *Master) Stop() {
	close(m.StopChan)
}

// collectAndReduce is the main MapReduce orchestration function
// 1. Polls each worker for their batch of logs (Map phase via RPC)
// 2. Aggregates window metrics (Reduce phase)
// 3. Updates cumulative stats
// 4. Builds response with both window and cumulative data
func (m *Master) collectAndReduce() {
	log.Printf("[INFO] [master] polling %d workers: %v", len(m.Workers), m.Workers)

	results := make([]MapResponse, 0, len(m.Workers))

	// Step 1: Poll all workers to collect their map results
	// Each worker returns: log counts, endpoint counts, status codes, latency buckets
	for _, workerAddr := range m.Workers {
		result := m.callWorkerMap(workerAddr)
		if result != nil {
			results = append(results, *result)
			log.Printf("[INFO] [master] successfully polled worker worker=%s count=%d", workerAddr, result.ProcessedCount)
		} else {
			log.Printf("[ERROR] [master] failed to poll worker worker=%s", workerAddr)
		}
	}

	if len(results) == 0 {
		return
	}

	// Step 2: Reduce - aggregate window-level metrics from all workers
	// This gives us stats for just the last 2 second polling interval
	windowMetrics := Reduce(results)

	// Step 3: Update latency history for the line chart
	// Keep last 60 points (~2 minutes of data at 2s intervals)
	m.LatencyHistory = append(m.LatencyHistory, LatencyDataPoint{
		Timestamp: time.Now(),
		Latency:   windowMetrics.AverageLatency,
	})
	if len(m.LatencyHistory) > 60 {
		m.LatencyHistory = m.LatencyHistory[len(m.LatencyHistory)-60:]
	}

	// Step 4: Collect recent logs from this cycle for display
	// Keep last 100 logs total
	var allLogs []LogEntry
	for _, r := range results {
		allLogs = append(allLogs, r.LogLines...)
	}
	m.RecentLogs = append(m.RecentLogs, allLogs...)
	if len(m.RecentLogs) > 100 {
		m.RecentLogs = m.RecentLogs[len(m.RecentLogs)-100:]
	}

	// Step 5: Update cumulative stats (totals since system start)
	m.CumulativeTotalRequests += windowMetrics.TotalRequests
	// Store latency as sum (not average) to recalculate cumulative average later
	m.CumulativeTotalLatency += windowMetrics.TotalRequests * int(windowMetrics.AverageLatency)
	// Convert error rate back to count, then add to cumulative
	m.CumulativeErrorCount += int(float64(windowMetrics.TotalRequests) * windowMetrics.ErrorRate / 100)

	// Accumulate counts from this window into cumulative totals
	for k, v := range windowMetrics.LogLevelCounts {
		m.CumulativeLogLevelCounts[k] += v
	}
	for k, v := range windowMetrics.EndpointCounts {
		m.CumulativeEndpointCounts[k] += v
	}
	for k, v := range windowMetrics.StatusCodeCounts {
		m.CumulativeStatusCodeCounts[k] += v
	}

	// Step 6: Calculate cumulative averages
	cumulativeAvgLatency := float64(m.CumulativeTotalLatency) / float64(m.CumulativeTotalRequests)
	cumulativeErrorRate := float64(m.CumulativeErrorCount) / float64(m.CumulativeTotalRequests) * 100

	// Step 7: Calculate top 5 endpoints by count (bubble sort)
	var topEndpoints []EndpointCount
	for endpoint, count := range m.CumulativeEndpointCounts {
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

	// Step 8: Build final metrics response
	// Include both cumulative totals AND window-specific stats
	windowRequests := windowMetrics.TotalRequests
	windowAvgLatency := windowMetrics.AverageLatency
	windowErrorRate := windowMetrics.ErrorRate

	windowMetrics.LatencyHistory = m.LatencyHistory
	windowMetrics.RecentLogs = m.RecentLogs
	windowMetrics.TotalRequests = m.CumulativeTotalRequests
	windowMetrics.LogLevelCounts = m.CumulativeLogLevelCounts
	windowMetrics.EndpointCounts = m.CumulativeEndpointCounts
	windowMetrics.StatusCodeCounts = m.CumulativeStatusCodeCounts
	windowMetrics.AverageLatency = cumulativeAvgLatency
	windowMetrics.ErrorRate = cumulativeErrorRate
	windowMetrics.TopEndpoints = topEndpoints

	// Window-specific fields for 2-second window display
	windowMetrics.WindowRequests = windowRequests
	windowMetrics.WindowAvgLatency = windowAvgLatency
	windowMetrics.WindowErrorRate = windowErrorRate

	m.MetricsMu.Lock()
	m.Metrics = &windowMetrics
	m.MetricsMu.Unlock()
}

// callWorkerMap makes an RPC call to a worker to get their map results
// This is the "Map" phase communication - master pulls from workers
func (m *Master) callWorkerMap(workerAddr string) *MapResponse {
	url := fmt.Sprintf("http://%s/map", workerAddr)

	// Request batch of 50 logs from worker
	reqBody, _ := json.Marshal(MapRequest{BatchSize: 50})
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("Failed to create request for worker %s: %v", workerAddr, err)
		return nil
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to call worker %s: %v", workerAddr, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Worker %s returned status %d", workerAddr, resp.StatusCode)
		return nil
	}

	var result MapResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Failed to decode response from worker %s: %v", workerAddr, err)
		return nil
	}

	return &result
}

// GetMetrics returns the current aggregated metrics (thread-safe)
func (m *Master) GetMetrics() *AggregatedMetrics {
	m.MetricsMu.RLock()
	defer m.MetricsMu.RUnlock()
	return m.Metrics
}
