package main

import (
	"encoding/json"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"rate-limiters/limiters"
)

// Any struct with the same function definition will satisfy this interface.
type Limiter interface {
	Allow(key string, now time.Time) Decision
}

// Decision represents the internal response for each Allow call.
type Decision = limiters.Decision

// AlwaysAllowLimiter is a simple struct that allows all requests.
type AlwaysAllowLimiter struct{}

// Allow implements the Limiter interface for AlwaysAllowLimiter.
// Go does not have an "implements" keyword.
// Go interfaces are satisfied structurally, not nominally.
func (AlwaysAllowLimiter) Allow(key string, now time.Time) Decision {
	return Decision{
		Allowed:    true,
		Remaining:  -1,
		RetryAfter: 0,
		Reason:     "always_allow",
	}
}

// AlwaysDenyLimiter is a simple struct that denies all requests.
type AlwaysDenyLimiter struct{}

func (AlwaysDenyLimiter) Allow(key string, now time.Time) Decision {
	return Decision{
		Allowed:    false,
		Remaining:  0,
		RetryAfter: 10 * time.Second,
		Reason:     "always_deny",
	}
}

// Handler handles requests to the /limited endpoint.
type Handler struct {
	limiter Limiter
	logger  *slog.Logger
}

// NewHandler creates a new Handler with the provided limiter and logger.
func NewHandler(limiter Limiter, logger *slog.Logger) *Handler {
	return &Handler{
		limiter: limiter,
		logger:  logger,
	}
}

// responseBody represents the JSON response body structure.
type responseBody struct {
	Allowed           bool   `json:"allowed"`
	Remaining         int    `json:"remaining"`
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
	Reason            string `json:"reason"`
	Message           string `json:"message"`
}

// ServeHTTP handles HTTP requests and implements the http.Handler interface.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key, parsed := clientKey(r)

	if !parsed {
		h.logger.Warn("failed to parse client IP, using fallback", "key", key)
	}

	h.logger.Debug("incoming request",
		"key", key,
		"method", r.Method,
		"path", r.URL.Path,
	)

	// Call the limiter's Allow method to check if the request is permitted.
	decision := h.limiter.Allow(key, time.Now())

	if !decision.Allowed {
		h.logger.Warn("rate limit exceeded",
			"key", key,
			"reason", decision.Reason,
			"retry_after", decision.RetryAfter,
		)
	} else {
		h.logger.Info("request allowed",
			"key", key,
			"remaining", decision.Remaining,
			"reason", decision.Reason,
		)
	}

	w.Header().Set("Content-Type", "application/json")

	if !decision.Allowed {
		retryAfterSeconds := durationSecondsCeil(decision.RetryAfter)

		// Set the Retry-After header when retry is required.
		if retryAfterSeconds > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
		}

		w.WriteHeader(http.StatusTooManyRequests)

		if err := json.NewEncoder(w).Encode(responseBody{
			Allowed:           decision.Allowed,
			Remaining:         decision.Remaining,
			RetryAfterSeconds: retryAfterSeconds,
			Reason:            decision.Reason,
			Message:           "rate limit exceeded",
		}); err != nil {
			h.logger.Error("failed to encode response", "error", err)
		}

		return
	}

	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(responseBody{
		Allowed:   decision.Allowed,
		Remaining: decision.Remaining,
		Reason:    decision.Reason,
		Message:   "request allowed",
	}); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}

// clientKey extracts the client's IP address from the request's RemoteAddr.
// If parsing fails, it returns the entire RemoteAddr string as a fallback.
func clientKey(r *http.Request) (string, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host, true
	}

	return r.RemoteAddr, false
}

// durationSecondsCeil returns the ceiling of a duration in seconds.
// For example, 1.3 seconds becomes 2.
func durationSecondsCeil(d time.Duration) int {
	if d <= 0 {
		return 0
	}

	return int(math.Ceil(d.Seconds()))
}

func main() {
	// Initialize the logger with human-readable text output.
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	limiter := limiters.NewFixedWindowCounter()
	handler := NewHandler(limiter, logger)

	mux := http.NewServeMux()
	mux.Handle("/api/limited", handler)

	logger.Info("starting server",
		"address", ":8080",
		"algorithm", "fixed_window_counter",
		"rate_limit_requests", limiters.FixedWindowCounterLimit,
		"rate_limit_window_seconds", int(limiters.FixedWindowCounterWindow/time.Second),
	)

	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		logger.Error("server failed to start", "error", err)
		os.Exit(1)
	}
}
