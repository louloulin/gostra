package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourusername/gostra/pkg/api"
)

// MockAuthService implements the minimum required interface for the auth middleware
type MockAuthService struct {
	validateTokenFunc func(token string) (*api.Claims, error)
}

// ValidateToken mocks the token validation
func (m *MockAuthService) ValidateToken(tokenString string) (*api.Claims, error) {
	return m.validateTokenFunc(tokenString)
}

// We only need to implement ValidateToken for our tests
// These methods are just stubs to satisfy the interface
func (m *MockAuthService) AuthenticateUser(username, password string) (string, error) {
	return "", nil
}

func (m *MockAuthService) GenerateToken(user *api.User) (string, error) {
	return "", nil
}

func (m *MockAuthService) AuthMiddleware(next http.Handler) http.Handler {
	return next
}

func (m *MockAuthService) RequirePermission(permission api.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return next
	}
}

func (m *MockAuthService) RequireRole(role api.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return next
	}
}

func TestAuthMiddleware(t *testing.T) {
	// Create a handler to use as "next" which will verify that the middleware correctly
	// passes a user claim object to the context
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if claims are in context
		claims, ok := r.Context().Value(api.ContextKeyUser).(*api.Claims)
		if !ok {
			t.Error("Claims not found in request context")
		}
		if claims.UserID != "test-user" {
			t.Errorf("Expected user ID 'test-user', got '%s'", claims.UserID)
		}
		w.WriteHeader(http.StatusOK)
	})

	// Create mock AuthService
	mockService := &MockAuthService{
		validateTokenFunc: func(token string) (*api.Claims, error) {
			if token == "valid-token" {
				return &api.Claims{
					UserID: "test-user",
					Roles:  []api.Role{api.RoleUser},
				}, nil
			}
			return nil, api.ErrInvalidToken
		},
	}

	// Create the middleware
	middleware := NewAuthMiddleware(&AuthMiddlewareOptions{
		AuthService: mockService,
		TokenExtractors: []TokenExtractor{
			DefaultHeaderExtractor,
		},
		ExcludedPaths: []string{"/public", "/api/public/*"},
	})

	// Create a handler with our middleware
	handler := middleware(nextHandler)

	// Test cases
	tests := []struct {
		name           string
		url            string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "Valid token",
			url:            "/api/protected",
			authHeader:     "Bearer valid-token",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid token",
			url:            "/api/protected",
			authHeader:     "Bearer invalid-token",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Missing token",
			url:            "/api/protected",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Excluded path",
			url:            "/public",
			authHeader:     "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Excluded wildcard path",
			url:            "/api/public/resource",
			authHeader:     "",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a request with the auth header
			req := httptest.NewRequest("GET", tc.url, nil)
			if tc.authHeader != "" {
				req.Header.Add("Authorization", tc.authHeader)
			}

			// Create a ResponseRecorder to record the response
			rr := httptest.NewRecorder()

			// Serve the request
			handler.ServeHTTP(rr, req)

			// Check the status code
			if rr.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, rr.Code)
			}
		})
	}
}

func TestDefaultHeaderExtractor(t *testing.T) {
	tests := []struct {
		name          string
		headerValue   string
		expectedToken string
	}{
		{
			name:          "Valid Bearer token",
			headerValue:   "Bearer token123",
			expectedToken: "token123",
		},
		{
			name:          "Empty header",
			headerValue:   "",
			expectedToken: "",
		},
		{
			name:          "Invalid format - no space",
			headerValue:   "Bearertoken123",
			expectedToken: "",
		},
		{
			name:          "Invalid format - wrong prefix",
			headerValue:   "Basic token123",
			expectedToken: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tc.headerValue != "" {
				req.Header.Add("Authorization", tc.headerValue)
			}

			token := DefaultHeaderExtractor(req)
			if token != tc.expectedToken {
				t.Errorf("Expected token '%s', got '%s'", tc.expectedToken, token)
			}
		})
	}
}

func TestQueryExtractor(t *testing.T) {
	extractor := DefaultQueryExtractor("token")

	tests := []struct {
		name          string
		url           string
		expectedToken string
	}{
		{
			name:          "Token in query string",
			url:           "/?token=query-token",
			expectedToken: "query-token",
		},
		{
			name:          "No token in query string",
			url:           "/?other=value",
			expectedToken: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.url, nil)
			token := extractor(req)
			if token != tc.expectedToken {
				t.Errorf("Expected token '%s', got '%s'", tc.expectedToken, token)
			}
		})
	}
}
