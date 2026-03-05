package middleware

import (
	"net/http"
	"strings"
	"time"
)

// NewRateLimiter creates a SEM limiter with a queue.
// WebSocket connections bypass the timeout handler because http.TimeoutHandler
// wraps the response writer in a way that doesn't support http.Hijacker.
func NewRateLimiter(handler http.Handler) http.Handler {
	timeoutHandler := http.TimeoutHandler(handler, 45*time.Second, "Server Busy")
	sem := make(chan struct{}, 20)
	return http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip timeout handler for WebSocket upgrades (needs http.Hijacker)
		isWebSocket := strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
		h := timeoutHandler
		if isWebSocket {
			h = handler
		}

		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			h.ServeHTTP(w, r)
		case <-time.After(45 * time.Second):
			http.Error(w, "Server Busy", http.StatusServiceUnavailable)
		}
	}))
}
