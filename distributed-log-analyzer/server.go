package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Server handles HTTP requests and SSE (Server-Sent Events) for real-time updates
type Server struct {
	Port         string
	Master       *Master
	SSEClients   map[chan []byte]struct{} // Connected SSE clients
	SSEClientsMu sync.Mutex               // Thread-safe access to clients
	Ticker       *time.Ticker             // Triggers broadcasts
	StopChan     chan struct{}
}

func NewServer(port string, master *Master) *Server {
	return &Server{
		Port:       port,
		Master:     master,
		SSEClients: make(map[chan []byte]struct{}),
		StopChan:   make(chan struct{}),
	}
}

// Start begins the HTTP server and SSE broadcaster
func (s *Server) Start() error {
	// Start broadcasting metrics to connected clients every 2 seconds
	s.StartSSEBroadcaster()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleDashboard)      // Serve HTML dashboard
	mux.HandleFunc("/events", s.handleEvents)   // SSE endpoint
	mux.HandleFunc("/metrics", s.handleMetrics) // JSON metrics API
	mux.HandleFunc("/health", s.handleHealth)   // Health check

	log.Printf("[INFO] [server] HTTP server started on port %s", s.Port)
	return http.ListenAndServe(":"+s.Port, mux)
}

func (s *Server) Stop() {
	close(s.StopChan)
}

// StartSSEBroadcaster periodically sends metrics to all connected clients
// Uses a separate 2-second ticker (synced with Master's polling)
func (s *Server) StartSSEBroadcaster() {
	s.Ticker = time.NewTicker(2 * time.Second)
	go func() {
		for {
			select {
			case <-s.Ticker.C:
				s.broadcastMetrics()
			case <-s.StopChan:
				return
			}
		}
	}()
}

// broadcastMetrics gets current metrics from Master and sends to all SSE clients
// Uses non-blocking send (select with default) to prevent slow clients blocking
func (s *Server) broadcastMetrics() {
	metrics := s.Master.GetMetrics()
	data, err := metrics.ToJSON()
	if err != nil {
		log.Printf("[ERROR] [server] failed to marshal metrics error=%v", err)
		return
	}

	s.SSEClientsMu.Lock()
	defer s.SSEClientsMu.Unlock()

	for clientChan := range s.SSEClients {
		select {
		case clientChan <- data:
			// Successfully sent
		default:
			// Client channel full - skip this client
		}
	}
}

func (s *Server) handleDashboard(wr http.ResponseWriter, req *http.Request) {
	http.ServeFile(wr, req, "dashboard.html")
}

// handleEvents implements Server-Sent Events (SSE)
// SSE allows server to push updates to client without polling
// Format: "data: <json>\n\n"
func (s *Server) handleEvents(wr http.ResponseWriter, req *http.Request) {
	// Set SSE-specific headers
	wr.Header().Set("Content-Type", "text/event-stream") // Tells client this is an event stream
	wr.Header().Set("Cache-Control", "no-cache")         // Prevent caching
	wr.Header().Set("Connection", "keep-alive")          // Keep connection open
	wr.Header().Set("Access-Control-Allow-Origin", "*")  // Allow cross-origin

	// Create a channel for this specific client
	clientChan := make(chan []byte, 10) // Buffer up to 10 messages

	// Register this client
	s.SSEClientsMu.Lock()
	s.SSEClients[clientChan] = struct{}{}
	s.SSEClientsMu.Unlock()

	// Clean up when client disconnects
	defer func() {
		s.SSEClientsMu.Lock()
		delete(s.SSEClients, clientChan)
		s.SSEClientsMu.Unlock()
		close(clientChan)
	}()

	// Get flusher for sending data
	flusher, ok := wr.(http.Flusher)
	if !ok {
		return // SSE not supported
	}

	// Listen for client disconnect via context
	notify := req.Context().Done()

	// Main loop: wait for data to send or client to disconnect
	for {
		select {
		case <-notify:
			return // Client disconnected
		case data := <-clientChan:
			// Send data in SSE format
			fmt.Fprintf(wr, "data: %s\n\n", data)
			flusher.Flush() // Force send immediately
		}
	}
}

func (s *Server) handleMetrics(wr http.ResponseWriter, req *http.Request) {
	metrics := s.Master.GetMetrics()
	wr.Header().Set("Content-Type", "application/json")
	json.NewEncoder(wr).Encode(metrics)
}

func (s *Server) handleHealth(wr http.ResponseWriter, req *http.Request) {
	wr.Header().Set("Content-Type", "application/json")
	json.NewEncoder(wr).Encode(map[string]string{
		"status": "ok",
		"server": s.Port,
	})
}
