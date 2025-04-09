package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// MockUserRepository is a mock implementation of UserRepository for testing
type MockUserRepository struct {
	users map[string]*User // Map of username to user
}

// NewMockUserRepository creates a new mock user repository with some test users
func NewMockUserRepository() *MockUserRepository {
	repo := &MockUserRepository{
		users: make(map[string]*User),
	}

	// Add some test users
	repo.users["admin"] = &User{
		ID:           "user-1",
		Username:     "admin",
		Email:        "admin@example.com",
		Roles:        []Role{RoleAdmin},
		Permissions:  []Permission{PermissionSystemAdmin},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		PasswordHash: "hashed_adminpass",
	}

	repo.users["user"] = &User{
		ID:           "user-2",
		Username:     "user",
		Email:        "user@example.com",
		Roles:        []Role{RoleUser},
		Permissions:  []Permission{PermissionAgentRead, PermissionToolRead, PermissionThreadRead},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		PasswordHash: "hashed_userpass",
	}

	return repo
}

// FindByID implements UserRepository.FindByID
func (r *MockUserRepository) FindByID(id string) (*User, error) {
	for _, user := range r.users {
		if user.ID == id {
			return user, nil
		}
	}
	return nil, ErrInvalidCredentials
}

// FindByUsername implements UserRepository.FindByUsername
func (r *MockUserRepository) FindByUsername(username string) (*User, error) {
	user, exists := r.users[username]
	if !exists {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

// Create implements UserRepository.Create
func (r *MockUserRepository) Create(user *User) error {
	if _, exists := r.users[user.Username]; exists {
		return errors.New("user already exists")
	}
	r.users[user.Username] = user
	return nil
}

// Update implements UserRepository.Update
func (r *MockUserRepository) Update(user *User) error {
	if _, exists := r.users[user.Username]; !exists {
		return ErrInvalidCredentials
	}
	r.users[user.Username] = user
	return nil
}

// Delete implements UserRepository.Delete
func (r *MockUserRepository) Delete(id string) error {
	for username, user := range r.users {
		if user.ID == id {
			delete(r.users, username)
			return nil
		}
	}
	return ErrInvalidCredentials
}

// List implements UserRepository.List
func (r *MockUserRepository) List() ([]*User, error) {
	users := make([]*User, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, user)
	}
	return users, nil
}

// TestAuthenticateUser tests the AuthenticateUser function
func TestAuthenticateUser(t *testing.T) {
	repo := NewMockUserRepository()
	config := &AuthConfig{
		JWTSecret:       "test-secret",
		TokenExpiration: 1 * time.Hour,
		EnableAuth:      true,
		AllowPublicAPIs: true,
	}
	authService := NewAuthService(config, repo)

	tests := []struct {
		name          string
		username      string
		password      string
		expectSuccess bool
	}{
		{
			name:          "Valid admin credentials",
			username:      "admin",
			password:      "adminpass",
			expectSuccess: true,
		},
		{
			name:          "Valid user credentials",
			username:      "user",
			password:      "userpass",
			expectSuccess: true,
		},
		{
			name:          "Invalid username",
			username:      "nonexistent",
			password:      "password",
			expectSuccess: false,
		},
		{
			name:          "Invalid password",
			username:      "admin",
			password:      "wrongpassword",
			expectSuccess: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token, err := authService.AuthenticateUser(tc.username, tc.password)

			if tc.expectSuccess {
				if err != nil {
					t.Errorf("Expected success, got error: %v", err)
				}
				if token == "" {
					t.Error("Expected token, got empty string")
				}
			} else {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				if token != "" {
					t.Errorf("Expected empty token, got: %s", token)
				}
			}
		})
	}
}

// TestAuthMiddleware tests the authentication middleware
func TestAuthMiddleware(t *testing.T) {
	repo := NewMockUserRepository()
	config := &AuthConfig{
		JWTSecret:       "test-secret",
		TokenExpiration: 1 * time.Hour,
		EnableAuth:      true,
		AllowPublicAPIs: true,
	}
	authService := NewAuthService(config, repo)

	// Create a test handler that checks if claims exist in context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetUserFromContext(r.Context())
		if !ok {
			http.Error(w, "No claims in context", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(claims.Username))
	})

	// Apply auth middleware
	handler := authService.AuthMiddleware(testHandler)

	tests := []struct {
		name             string
		path             string
		token            string
		expectStatusCode int
	}{
		{
			name:             "Public path without token",
			path:             "/health",
			token:            "",
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "Protected path without token",
			path:             "/api/v1/agents",
			token:            "",
			expectStatusCode: http.StatusUnauthorized,
		},
		{
			name:             "Protected path with valid token",
			path:             "/api/v1/agents",
			token:            "valid-token", // Will be replaced with actual token in the test
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "Protected path with invalid token",
			path:             "/api/v1/agents",
			token:            "invalid-token",
			expectStatusCode: http.StatusUnauthorized,
		},
	}

	// Get a valid token for admin user
	adminUser, _ := repo.FindByUsername("admin")
	validToken, _ := authService.GenerateToken(adminUser)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			req, err := http.NewRequest("GET", tc.path, nil)
			if err != nil {
				t.Fatalf("Error creating request: %v", err)
			}

			// Add token if specified
			if tc.token != "" {
				if tc.token == "valid-token" {
					tc.token = validToken
				}
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Serve request
			handler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tc.expectStatusCode {
				t.Errorf("Expected status %d, got %d", tc.expectStatusCode, rr.Code)
			}
		})
	}
}

// TestRequirePermission tests the permission middleware
func TestRequirePermission(t *testing.T) {
	repo := NewMockUserRepository()
	config := &AuthConfig{
		JWTSecret:       "test-secret",
		TokenExpiration: 1 * time.Hour,
		EnableAuth:      true,
		AllowPublicAPIs: false, // All APIs require auth for this test
	}
	authService := NewAuthService(config, repo)

	// Create a simple success handler
	successHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Get tokens for different users
	adminUser, _ := repo.FindByUsername("admin")
	adminToken, _ := authService.GenerateToken(adminUser)

	regularUser, _ := repo.FindByUsername("user")
	userToken, _ := authService.GenerateToken(regularUser)

	tests := []struct {
		name             string
		permission       Permission
		token            string
		expectStatusCode int
	}{
		{
			name:             "Admin has system:admin permission",
			permission:       PermissionSystemAdmin,
			token:            adminToken,
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "Regular user does not have system:admin permission",
			permission:       PermissionSystemAdmin,
			token:            userToken,
			expectStatusCode: http.StatusForbidden,
		},
		{
			name:             "Regular user has agent:read permission",
			permission:       PermissionAgentRead,
			token:            userToken,
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "Regular user does not have agent:write permission",
			permission:       PermissionAgentWrite,
			token:            userToken,
			expectStatusCode: http.StatusForbidden,
		},
		{
			name:             "No token provided",
			permission:       PermissionAgentRead,
			token:            "",
			expectStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create handler with permission middleware
			handler := authService.RequirePermission(tc.permission)(successHandler)

			// Apply auth middleware first
			handler = authService.AuthMiddleware(handler)

			// Create request
			req, _ := http.NewRequest("GET", "/test", nil)

			// Add token if specified
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Serve request
			handler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tc.expectStatusCode {
				t.Errorf("Expected status %d, got %d", tc.expectStatusCode, rr.Code)
			}
		})
	}
}

// TestRequireRole tests the role middleware
func TestRequireRole(t *testing.T) {
	repo := NewMockUserRepository()
	config := &AuthConfig{
		JWTSecret:       "test-secret",
		TokenExpiration: 1 * time.Hour,
		EnableAuth:      true,
		AllowPublicAPIs: false, // All APIs require auth for this test
	}
	authService := NewAuthService(config, repo)

	// Create a simple success handler
	successHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Get tokens for different users
	adminUser, _ := repo.FindByUsername("admin")
	adminToken, _ := authService.GenerateToken(adminUser)

	regularUser, _ := repo.FindByUsername("user")
	userToken, _ := authService.GenerateToken(regularUser)

	tests := []struct {
		name             string
		role             Role
		token            string
		expectStatusCode int
	}{
		{
			name:             "Admin has admin role",
			role:             RoleAdmin,
			token:            adminToken,
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "Regular user does not have admin role",
			role:             RoleAdmin,
			token:            userToken,
			expectStatusCode: http.StatusForbidden,
		},
		{
			name:             "Regular user has user role",
			role:             RoleUser,
			token:            userToken,
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "Admin can access user role endpoints",
			role:             RoleUser,
			token:            adminToken,
			expectStatusCode: http.StatusOK,
		},
		{
			name:             "No token provided",
			role:             RoleUser,
			token:            "",
			expectStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create handler with role middleware
			handler := authService.RequireRole(tc.role)(successHandler)

			// Apply auth middleware first
			handler = authService.AuthMiddleware(handler)

			// Create request
			req, _ := http.NewRequest("GET", "/test", nil)

			// Add token if specified
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Serve request
			handler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tc.expectStatusCode {
				t.Errorf("Expected status %d, got %d", tc.expectStatusCode, rr.Code)
			}
		})
	}
}
