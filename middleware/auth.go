package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/expki/ZeroLoop.git/config"
	"github.com/expki/ZeroLoop.git/logger"
)

type contextKey string

const (
	UserContextKey           contextKey = "user"
	ResponseWriterContextKey contextKey = "responseWriter"
	RequestContextKey        contextKey = "request"
	PlatformContextKey       contextKey = "platform"
	ClientIPContextKey       contextKey = "clientIP"
)

// AuthMiddleware extracts JWT from Authorization header or cookie and adds user to context
// TODO: create auth middleware
func AuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add response writer and request to context so resolvers can set/read cookies
			ctx := context.WithValue(r.Context(), ResponseWriterContextKey, w)
			ctx = context.WithValue(ctx, RequestContextKey, r)

			// Extract real client IP from Cloudflare/proxy headers
			// Priority: CF-Connecting-IP > X-Real-IP > X-Forwarded-For > RemoteAddr
			ctx = context.WithValue(ctx, ClientIPContextKey, extractClientIP(r))

			var tokenString string

			// Try Authorization header first
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				tokenString = strings.TrimPrefix(authHeader, "Bearer ")
				if tokenString == authHeader {
					// No Bearer prefix - invalid header format
					tokenString = ""
				}
			}

			// Fall back to cookie if no valid Authorization header
			if tokenString == "" {
				if cookie, err := r.Cookie("zeroloop_access_token"); err == nil && cookie.Value != "" {
					tokenString = cookie.Value
				}
			}

			// No token found
			if tokenString == "" {
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// TODO: get user from token
			user := "todo"

			// Add user to context
			ctx = context.WithValue(ctx, UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetResponseWriter retrieves the response writer from context
func GetResponseWriter(ctx context.Context) http.ResponseWriter {
	w, ok := ctx.Value(ResponseWriterContextKey).(http.ResponseWriter)
	if !ok {
		return nil
	}
	return w
}

// GetRefreshTokenFromContext retrieves the refresh token from the request cookie
func GetRefreshTokenFromContext(ctx context.Context) string {
	r, ok := ctx.Value(RequestContextKey).(*http.Request)
	if !ok || r == nil {
		return ""
	}
	cookie, err := r.Cookie("zeroloop_refresh_token")
	if err != nil || cookie.Value == "" {
		return ""
	}
	return cookie.Value
}

// SetAuthCookies sets HttpOnly auth cookies on the response
func SetAuthCookies(ctx context.Context, accessToken, refreshToken string, accessExpiresIn int) {
	w := GetResponseWriter(ctx)
	if w == nil {
		logger.Log.Warnw("auth: cannot set cookies - no response writer in context")
		return
	}

	// SameSite: Strict in production, Lax in development (for proxy compatibility)
	sameSite := http.SameSiteLaxMode
	if config.Load().IsProduction() {
		sameSite = http.SameSiteStrictMode
	}

	// Access token cookie - session cookie (no MaxAge = cleared when browser closes)
	accessCookie := &http.Cookie{
		Name:     "zeroloop_access_token",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   config.Load().IsProduction(),
		SameSite: sameSite,
	}
	http.SetCookie(w, accessCookie)

	// Refresh token cookie - longer lived but still session-based for security
	refreshCookie := &http.Cookie{
		Name:     "zeroloop_refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   config.Load().IsProduction(),
		SameSite: sameSite,
	}
	http.SetCookie(w, refreshCookie)
}

// ClearAuthCookies removes auth cookies from the response
func ClearAuthCookies(ctx context.Context) {
	w := GetResponseWriter(ctx)
	if w == nil {
		logger.Log.Warnw("auth: cannot clear cookies - no response writer in context")
		return
	}

	// SameSite: Strict in production, Lax in development (for proxy compatibility)
	sameSite := http.SameSiteLaxMode
	if config.Load().IsProduction() {
		sameSite = http.SameSiteStrictMode
	}

	// Clear access token
	http.SetCookie(w, &http.Cookie{
		Name:     "zeroloop_access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   config.Load().IsProduction(),
		SameSite: sameSite,
	})

	// Clear refresh token
	http.SetCookie(w, &http.Cookie{
		Name:     "zeroloop_refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   config.Load().IsProduction(),
		SameSite: sameSite,
	})
}

// Common errors
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}

var (
	ErrUnauthorized = &AuthError{Message: "unauthorized"}
	ErrForbidden    = &AuthError{Message: "forbidden"}
)

// extractClientIP returns the real client IP from Cloudflare/proxy headers.
// Priority: CF-Connecting-IP > X-Real-IP > X-Forwarded-For (first) > RemoteAddr
func extractClientIP(r *http.Request) string {
	// Cloudflare sets this to the true client IP
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	// nginx sets X-Real-IP from CF-Connecting-IP
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	// X-Forwarded-For can contain a chain: client, proxy1, proxy2
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(ip)
		}
		return strings.TrimSpace(xff)
	}
	// Fallback to direct connection (will be the proxy IP behind nginx)
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// GetClientIP retrieves the real client IP from context
func GetClientIP(ctx context.Context) string {
	ip, _ := ctx.Value(ClientIPContextKey).(string)
	return ip
}
