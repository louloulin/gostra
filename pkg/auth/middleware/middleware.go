package middleware

import (
	"net/http"
)

// Chain combines multiple middleware functions into a single middleware
func Chain(middleware ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		for i := len(middleware) - 1; i >= 0; i-- {
			next = middleware[i](next)
		}
		return next
	}
}

// Protected creates a middleware chain that applies authentication and (optionally) RBAC
// It's a convenience function for the common pattern of auth + rbac
func Protected(authMiddleware func(http.Handler) http.Handler, rbacMiddleware func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	if rbacMiddleware == nil {
		return authMiddleware
	}
	return Chain(authMiddleware, rbacMiddleware)
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This is a simple example, could be expanded with more detailed logging
		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware adds Cross-Origin Resource Sharing headers
func CORSMiddleware(origins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Default to all origins if none specified
			origin := "*"
			if len(origins) > 0 {
				// In a real implementation, we would check against the list of allowed origins
				origin = origins[0]
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
