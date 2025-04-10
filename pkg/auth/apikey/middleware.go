package apikey

import (
	"context"
	"net/http"
	"strings"

	"github.com/louloulin/gostra/pkg/api"
)

// ContextKey for storing API key information
type ContextKey string

const (
	// ContextKeyAPIKey is the context key for API key
	ContextKeyAPIKey ContextKey = "apikey"
)

// APIKeyExtractor defines how to extract an API key from an HTTP request
type APIKeyExtractor func(r *http.Request) string

// HeaderExtractor extracts an API key from a specific header
func HeaderExtractor(headerName string) APIKeyExtractor {
	return func(r *http.Request) string {
		return r.Header.Get(headerName)
	}
}

// QueryExtractor extracts an API key from a query parameter
func QueryExtractor(paramName string) APIKeyExtractor {
	return func(r *http.Request) string {
		return r.URL.Query().Get(paramName)
	}
}

// DefaultAPIKeyExtractor extracts an API key from the X-API-Key header
func DefaultAPIKeyExtractor(r *http.Request) string {
	return r.Header.Get("X-API-Key")
}

// BearerExtractor extracts an API key from the Authorization header with Bearer prefix
func BearerExtractor(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}

	return parts[1]
}

// APIKeyMiddlewareOptions configures the API key middleware
type APIKeyMiddlewareOptions struct {
	APIKeyService  *APIKeyService
	Extractors     []APIKeyExtractor
	ExcludedPaths  []string
	RequiredScopes []string
}

// NewAPIKeyMiddleware creates a middleware function that authenticates requests using API keys
func NewAPIKeyMiddleware(options *APIKeyMiddlewareOptions) func(http.Handler) http.Handler {
	if options == nil {
		panic("APIKeyMiddlewareOptions cannot be nil")
	}

	if options.APIKeyService == nil {
		panic("APIKeyService cannot be nil")
	}

	// Set default extractors if none provided
	if len(options.Extractors) == 0 {
		options.Extractors = []APIKeyExtractor{DefaultAPIKeyExtractor, BearerExtractor}
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

			// Extract API key using provided extractors
			var apiKeyString string
			for _, extractor := range options.Extractors {
				apiKeyString = extractor(r)
				if apiKeyString != "" {
					break
				}
			}

			// No API key found
			if apiKeyString == "" {
				http.Error(w, "Unauthorized: Missing API key", http.StatusUnauthorized)
				return
			}

			// Validate API key
			apiKey, err := options.APIKeyService.ValidateAPIKey(apiKeyString)
			if err != nil {
				http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}

			// Check required scopes
			for _, scope := range options.RequiredScopes {
				if !options.APIKeyService.HasScope(apiKey, scope) {
					http.Error(w, "Forbidden: Missing required scope: "+scope, http.StatusForbidden)
					return
				}
			}

			// Create a Claims object from the API key for compatibility with existing auth
			var roles []api.Role
			for _, role := range apiKey.Roles {
				roles = append(roles, api.Role(role))
			}

			var permissions []api.Permission
			for _, perm := range apiKey.Permissions {
				permissions = append(permissions, api.Permission(perm))
			}

			claims := &api.Claims{
				UserID:      apiKey.OwnerID,
				Username:    "apikey:" + apiKey.Name,
				Roles:       roles,
				Permissions: permissions,
			}

			// Add API key and claims to context
			ctx := context.WithValue(r.Context(), ContextKeyAPIKey, apiKey)
			ctx = context.WithValue(ctx, api.ContextKeyUser, claims)

			// Call the next handler with enhanced context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetAPIKeyFromContext retrieves API key from the request context
func GetAPIKeyFromContext(ctx context.Context) (*APIKey, bool) {
	apiKey, ok := ctx.Value(ContextKeyAPIKey).(*APIKey)
	return apiKey, ok
}

// RequireAPIKeyScope creates a middleware that checks if the API key has the required scope
func RequireAPIKeyScope(service *APIKeyService, scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey, ok := GetAPIKeyFromContext(r.Context())
			if !ok {
				http.Error(w, "Unauthorized: API key required", http.StatusUnauthorized)
				return
			}

			if !service.HasScope(apiKey, scope) {
				http.Error(w, "Forbidden: Missing required scope: "+scope, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAPIKeyPermission creates a middleware that checks if the API key has the required permission
func RequireAPIKeyPermission(service *APIKeyService, permission api.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey, ok := GetAPIKeyFromContext(r.Context())
			if !ok {
				http.Error(w, "Unauthorized: API key required", http.StatusUnauthorized)
				return
			}

			if !service.HasPermission(apiKey, permission) {
				http.Error(w, "Forbidden: Missing required permission", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
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
