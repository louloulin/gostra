package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/yourusername/gostra/pkg/api"
)

// TokenValidator is the interface needed by the auth middleware
type TokenValidator interface {
	ValidateToken(tokenString string) (*api.Claims, error)
}

// TokenExtractor defines how to extract a token from an HTTP request
type TokenExtractor func(r *http.Request) string

// DefaultHeaderExtractor extracts token from the Authorization header
func DefaultHeaderExtractor(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	// Check for Bearer token format
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}

	return parts[1]
}

// DefaultQueryExtractor extracts token from the query parameter
func DefaultQueryExtractor(paramName string) TokenExtractor {
	return func(r *http.Request) string {
		return r.URL.Query().Get(paramName)
	}
}

// AuthMiddlewareOptions configures the authentication middleware
type AuthMiddlewareOptions struct {
	AuthService     TokenValidator
	TokenExtractors []TokenExtractor
	ExcludedPaths   []string
}

// NewAuthMiddleware creates a middleware function that authenticates requests
func NewAuthMiddleware(options *AuthMiddlewareOptions) func(http.Handler) http.Handler {
	if options == nil {
		panic("AuthMiddlewareOptions cannot be nil")
	}

	if options.AuthService == nil {
		panic("AuthService cannot be nil")
	}

	// Set default token extractors if none specified
	if len(options.TokenExtractors) == 0 {
		options.TokenExtractors = []TokenExtractor{DefaultHeaderExtractor}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path is excluded from authentication
			for _, path := range options.ExcludedPaths {
				if r.URL.Path == path || matchWildcardPath(path, r.URL.Path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Extract token using provided extractors
			var token string
			for _, extractor := range options.TokenExtractors {
				token = extractor(r)
				if token != "" {
					break
				}
			}

			// No token found
			if token == "" {
				http.Error(w, "Unauthorized: Missing token", http.StatusUnauthorized)
				return
			}

			// Validate token
			claims, err := options.AuthService.ValidateToken(token)
			if err != nil {
				http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}

			// Add claims to request context
			ctx := context.WithValue(r.Context(), api.ContextKeyUser, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// matchWildcardPath checks if a path matches a wildcard pattern
// e.g., "/api/*" matches "/api/users", "/api/auth", etc.
func matchWildcardPath(pattern, path string) bool {
	if strings.HasSuffix(pattern, "/*") {
		base := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, base)
	}
	return false
}
