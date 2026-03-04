package middleware

import (
	"net/http"
	"regexp"
	"strings"
)

// fingerprintedAssetPattern matches Vite fingerprinted assets: name-hash.ext
// Examples: AdminLayout-WK1BYKWg.js, index-DfG3k2.css
var fingerprintedAssetPattern = regexp.MustCompile(`-[a-zA-Z0-9]{8}\.(js|css)$`)

// CacheMiddleware adds appropriate Cache-Control headers based on path
func CacheMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cc := getCacheControl(r.URL.Path); cc != "" {
				w.Header().Set("Cache-Control", cc)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func getCacheControl(path string) string {
	// API endpoints - never cache
	if path == "/graphql" || path == "/health" || strings.HasPrefix(path, "/api/") {
		return "private, no-store"
	}

	// Ephemeral API is handled by its own handler
	if strings.HasPrefix(path, "/api/ephemeral/") {
		return ""
	}

	// index.html and SPA root - always revalidate
	if path == "/" || path == "/index.html" {
		return "no-cache, must-revalidate"
	}

	// Fingerprinted assets - immutable, long cache
	if strings.HasPrefix(path, "/assets/") && fingerprintedAssetPattern.MatchString(path) {
		return "public, max-age=31536000, immutable"
	}

	// Fonts - long cache (rarely change)
	if strings.HasPrefix(path, "/fonts/") {
		return "public, max-age=31536000, immutable"
	}

	// Static files (images, icons, etc.) - moderate cache
	if isStaticFile(path) {
		return "public, max-age=86400"
	}

	// SPA routes (no extension) - same as index.html
	if !strings.Contains(path, ".") {
		return "no-cache, must-revalidate"
	}

	return ""
}

func isStaticFile(path string) bool {
	staticExtensions := []string{
		".svg", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico",
		".woff", ".woff2", ".ttf",
		".txt", ".xml", ".json",
	}
	for _, ext := range staticExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}
