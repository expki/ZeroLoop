package middleware

import (
	"net/http"
	"time"
)

// NewRateLimiter creates a SEM limiter with a queue
func NewRateLimiter(handler http.Handler) http.Handler {
	handler = http.TimeoutHandler(handler, 45*time.Second, "Server Busy")
	sem := make(chan struct{}, 20)
	return http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			handler.ServeHTTP(w, r)
		case <-time.After(45 * time.Second):
			http.Error(w, "Server Busy", http.StatusServiceUnavailable)
		}
	}))
}
