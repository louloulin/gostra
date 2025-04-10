package apikey

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/louloulin/gostra/pkg/api"
)

func TestAPIKeyMiddleware(t *testing.T) {
	// Create repository and service
	repo := NewInMemoryAPIKeyRepository()
	service := NewAPIKeyService(repo)

	// Convert API roles and permissions to strings
	roles := []string{string(api.RoleUser)}
	permissions := []string{string(api.PermissionAgentRead)}

	// Generate a test API key
	opts := &APIKeyOptions{
		Name:        "Test Middleware Key",
		OwnerID:     "user555",
		ExpiresIn:   24 * time.Hour,
		Scopes:      []string{"api:read", "api:write"},
		Roles:       roles,
		Permissions: permissions,
	}

	_, fullKey, err := service.GenerateAPIKey(opts)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Create middleware
	middleware := NewAPIKeyMiddleware(&APIKeyMiddlewareOptions{
		APIKeyService: service,
		Extractors:    []APIKeyExtractor{DefaultAPIKeyExtractor, BearerExtractor},
		ExcludedPaths: []string{"/public", "/api/docs/*"},
	})

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if API key context is available
		key, ok := GetAPIKeyFromContext(r.Context())
		if !ok {
			t.Error("API key not found in context")
		} else if key.OwnerID != "user555" {
			t.Errorf("API key ownerID mismatch, got %s, want %s", key.OwnerID, "user555")
		}

		// Check if user claims context is available
		claims, ok := r.Context().Value(api.ContextKeyUser).(*api.Claims)
		if !ok {
			t.Error("Claims not found in context")
		}

		// Check if the claims are properly populated
		if claims.UserID != "user555" {
			t.Errorf("Claims UserID mismatch, got %s, expected user555", claims.UserID)
		}

		w.WriteHeader(http.StatusOK)
	})

	// Create wrapped handler
	handler := middleware(testHandler)

	// Test case 1: Request with valid API key
	req1 := httptest.NewRequest("GET", "/api/protected", nil)
	req1.Header.Set("X-API-Key", fullKey)
	w1 := httptest.NewRecorder()

	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Expected status 200 for valid key, got %d", w1.Code)
	}

	// Test case 2: Request with valid API key in Authorization header
	req2 := httptest.NewRequest("GET", "/api/protected", nil)
	req2.Header.Set("Authorization", "Bearer "+fullKey)
	w2 := httptest.NewRecorder()

	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 for valid key in Auth header, got %d", w2.Code)
	}

	// Test case 3: Request with invalid API key
	req3 := httptest.NewRequest("GET", "/api/protected", nil)
	req3.Header.Set("X-API-Key", "invalid-key")
	w3 := httptest.NewRecorder()

	handler.ServeHTTP(w3, req3)

	if w3.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for invalid key, got %d", w3.Code)
	}

	// Test case 4: Request without API key
	req4 := httptest.NewRequest("GET", "/api/protected", nil)
	w4 := httptest.NewRecorder()

	handler.ServeHTTP(w4, req4)

	if w4.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for missing key, got %d", w4.Code)
	}

	// Test case 5: Request to excluded path
	req5 := httptest.NewRequest("GET", "/public", nil)
	w5 := httptest.NewRecorder()

	handler.ServeHTTP(w5, req5)

	if w5.Code != http.StatusOK {
		t.Errorf("Expected status 200 for excluded path, got %d", w5.Code)
	}

	// Test case 6: Request to wildcard excluded path
	req6 := httptest.NewRequest("GET", "/api/docs/index", nil)
	w6 := httptest.NewRecorder()

	handler.ServeHTTP(w6, req6)

	if w6.Code != http.StatusOK {
		t.Errorf("Expected status 200 for wildcard excluded path, got %d", w6.Code)
	}
}

func TestScopeMiddleware(t *testing.T) {
	// Create repository and service
	repo := NewInMemoryAPIKeyRepository()
	service := NewAPIKeyService(repo)

	// Generate a test API key with specific scopes
	opts := &APIKeyOptions{
		Name:      "Scope Middleware Key",
		OwnerID:   "user666",
		ExpiresIn: 24 * time.Hour,
		Scopes:    []string{"api:read"},
	}

	_, fullKey, err := service.GenerateAPIKey(opts)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Create API key middleware
	apiKeyMiddleware := NewAPIKeyMiddleware(&APIKeyMiddlewareOptions{
		APIKeyService: service,
	})

	// Create scope middleware
	scopeMiddleware := RequireAPIKeyScope(service, "api:read")
	deniedScopeMiddleware := RequireAPIKeyScope(service, "api:write")

	// Create test handlers
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Chain middlewares
	handlerWithScope := apiKeyMiddleware(scopeMiddleware(okHandler))
	handlerWithDeniedScope := apiKeyMiddleware(deniedScopeMiddleware(okHandler))

	// Test case 1: Request with API key having proper scope
	req1 := httptest.NewRequest("GET", "/api/data", nil)
	req1.Header.Set("X-API-Key", fullKey)
	w1 := httptest.NewRecorder()

	handlerWithScope.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Expected status 200 for key with proper scope, got %d", w1.Code)
	}

	// Test case 2: Request with API key missing required scope
	req2 := httptest.NewRequest("GET", "/api/data", nil)
	req2.Header.Set("X-API-Key", fullKey)
	w2 := httptest.NewRecorder()

	handlerWithDeniedScope.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for key missing required scope, got %d", w2.Code)
	}
}

func TestPermissionMiddleware(t *testing.T) {
	// Create repository and service
	repo := NewInMemoryAPIKeyRepository()
	service := NewAPIKeyService(repo)

	// Generate a test API key with specific permissions
	permissions := []string{string(api.PermissionAgentRead)}

	opts := &APIKeyOptions{
		Name:        "Permission Middleware Key",
		OwnerID:     "user777",
		ExpiresIn:   24 * time.Hour,
		Permissions: permissions,
	}

	_, fullKey, err := service.GenerateAPIKey(opts)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Create API key middleware
	apiKeyMiddleware := NewAPIKeyMiddleware(&APIKeyMiddlewareOptions{
		APIKeyService: service,
	})

	// Create permission middleware
	permMiddleware := RequireAPIKeyPermission(service, api.PermissionAgentRead)
	deniedPermMiddleware := RequireAPIKeyPermission(service, api.PermissionAgentWrite)

	// Create test handlers
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Chain middlewares
	handlerWithPerm := apiKeyMiddleware(permMiddleware(okHandler))
	handlerWithDeniedPerm := apiKeyMiddleware(deniedPermMiddleware(okHandler))

	// Test case 1: Request with API key having proper permission
	req1 := httptest.NewRequest("GET", "/api/agents", nil)
	req1.Header.Set("X-API-Key", fullKey)
	w1 := httptest.NewRecorder()

	handlerWithPerm.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Expected status 200 for key with proper permission, got %d", w1.Code)
	}

	// Test case 2: Request with API key missing required permission
	req2 := httptest.NewRequest("GET", "/api/agents", nil)
	req2.Header.Set("X-API-Key", fullKey)
	w2 := httptest.NewRecorder()

	handlerWithDeniedPerm.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for key missing required permission, got %d", w2.Code)
	}
}
