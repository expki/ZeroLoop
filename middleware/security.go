package middleware

import (
	"net/http"
	"strings"
)

// SecurityConfig holds configuration for security headers
type SecurityConfig struct {
	IsDevelopment bool
	TLSEnabled    bool
}

// SecurityHeaders returns a middleware that adds comprehensive security headers to ALL responses
// This includes both security headers and CORS handling in one place for consistency
func (cfg SecurityConfig) SecurityHeaders() func(http.Handler) http.Handler {
	csp := buildCSP(cfg.IsDevelopment)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Core security headers
			w.Header().Set("Vary", "Accept-Encoding")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
			w.Header().Set("Cross-Origin-Embedder-Policy", "unsafe-none") // Required for Stripe iframes

			if cfg.TLSEnabled {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}

			w.Header().Set("Content-Security-Policy", csp)

			// CORS: echo back origin for API endpoints (allows any domain that accesses us)
			if needsCORS(r.URL.Path) {
				if origin := r.Header.Get("Origin"); origin != "" {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, HEAD, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusOK)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func needsCORS(path string) bool {
	return path == "/graphql" || strings.HasPrefix(path, "/api/")
}

func buildCSP(isDevelopment bool) string {
	if isDevelopment {
		// Relaxed CSP for development with hot reloading support
		return strings.Join([]string{
			"default-src 'self' 'unsafe-inline' 'unsafe-eval'",
			"script-src 'self' 'unsafe-inline' 'unsafe-eval' https://www.googletagmanager.com https://m.stripe.network https://js.stripe.com https://staticcdn.co.nz https://www.clarity.ms https://scripts.clarity.ms https://static.cloudflareinsights.com data: blob:",
			"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
			"font-src 'self' https://fonts.gstatic.com",
			"img-src 'self' data: blob: https:",
			"connect-src 'self' http://localhost:9090 https://www.googletagmanager.com https://www.google-analytics.com https://api.stripe.com https://m.stripe.network https://*.clarity.ms https://cloudflareinsights.com https://*.cloudflareinsights.com ws: wss:",
			"frame-src 'self' https://js.stripe.com https://hooks.stripe.com https://m.stripe.network https://staticcdn.co.nz",
			"object-src 'none'",
			"base-uri 'self'",
			"frame-ancestors 'none'",
		}, "; ")
	}

	// Production CSP
	// Note: 'unsafe-inline' for styles is required because React/Radix apply dynamic inline styles at runtime
	return strings.Join([]string{
		"default-src 'self'",
		"script-src 'self' 'unsafe-inline' https://www.googletagmanager.com https://m.stripe.network https://js.stripe.com https://staticcdn.co.nz https://www.clarity.ms https://scripts.clarity.ms https://static.cloudflareinsights.com",
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
		"font-src 'self' https://fonts.gstatic.com",
		"img-src 'self' data: blob: https:",
		"connect-src 'self' https://www.googletagmanager.com https://www.google-analytics.com https://api.stripe.com https://m.stripe.network https://*.clarity.ms https://cloudflareinsights.com https://*.cloudflareinsights.com",
		"frame-src 'self' https://js.stripe.com https://hooks.stripe.com https://m.stripe.network https://staticcdn.co.nz",
		"object-src 'none'",
		"base-uri 'self'",
		"frame-ancestors 'none'",
	}, "; ")
}
