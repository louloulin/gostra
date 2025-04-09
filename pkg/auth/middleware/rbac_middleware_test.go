package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourusername/gostra/pkg/api"
)

func TestRBACMiddleware(t *testing.T) {
	// Create test handler that returns success
	successHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Helper to set claims in context for testing
	setClaimsContext := func(next http.Handler, claims *api.Claims) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), api.ContextKeyUser, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// Test cases for roles
	roleTests := []struct {
		name           string
		userRoles      []api.Role
		requiredRoles  []api.Role
		anyRole        bool
		expectedStatus int
	}{
		{
			name:           "User with admin role",
			userRoles:      []api.Role{api.RoleAdmin},
			requiredRoles:  []api.Role{api.RoleAdmin},
			anyRole:        false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "User with user role, requires admin",
			userRoles:      []api.Role{api.RoleUser},
			requiredRoles:  []api.Role{api.RoleAdmin},
			anyRole:        false,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "User with multiple roles, requires any",
			userRoles:      []api.Role{api.RoleUser, api.RoleGuest},
			requiredRoles:  []api.Role{api.RoleAdmin, api.RoleUser},
			anyRole:        true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "User with no matching roles, requires any",
			userRoles:      []api.Role{api.RoleGuest},
			requiredRoles:  []api.Role{api.RoleAdmin, api.RoleUser},
			anyRole:        true,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "No roles",
			userRoles:      []api.Role{},
			requiredRoles:  []api.Role{api.RoleUser},
			anyRole:        false,
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tc := range roleTests {
		t.Run(tc.name, func(t *testing.T) {
			// Create claims with test roles
			claims := &api.Claims{
				UserID: "test-user",
				Roles:  tc.userRoles,
			}

			// Create middleware with test options
			rbacMiddleware := NewRBACMiddleware(&RBACOptions{
				RequiredRoles: tc.requiredRoles,
				AnyRole:       tc.anyRole,
			})

			// Create a handler with context and middleware
			handler := rbacMiddleware(successHandler)
			contextHandler := setClaimsContext(handler, claims)

			// Create request and response recorder
			req := httptest.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()

			// Serve the request
			contextHandler.ServeHTTP(rr, req)

			// Check the status code
			if rr.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, rr.Code)
			}
		})
	}

	// Test cases for permissions
	permissionTests := []struct {
		name                string
		userPermissions     []api.Permission
		requiredPermissions []api.Permission
		expectedStatus      int
	}{
		{
			name:                "User with all required permissions",
			userPermissions:     []api.Permission{api.PermissionAgentRead, api.PermissionAgentWrite},
			requiredPermissions: []api.Permission{api.PermissionAgentRead},
			expectedStatus:      http.StatusOK,
		},
		{
			name:                "User lacking required permission",
			userPermissions:     []api.Permission{api.PermissionAgentRead},
			requiredPermissions: []api.Permission{api.PermissionAgentWrite},
			expectedStatus:      http.StatusForbidden,
		},
		{
			name:                "Multiple required permissions - has all",
			userPermissions:     []api.Permission{api.PermissionAgentRead, api.PermissionAgentWrite, api.PermissionAgentExecute},
			requiredPermissions: []api.Permission{api.PermissionAgentRead, api.PermissionAgentWrite},
			expectedStatus:      http.StatusOK,
		},
		{
			name:                "Multiple required permissions - missing one",
			userPermissions:     []api.Permission{api.PermissionAgentRead},
			requiredPermissions: []api.Permission{api.PermissionAgentRead, api.PermissionAgentWrite},
			expectedStatus:      http.StatusForbidden,
		},
		{
			name:                "No permissions",
			userPermissions:     []api.Permission{},
			requiredPermissions: []api.Permission{api.PermissionAgentRead},
			expectedStatus:      http.StatusForbidden,
		},
	}

	for _, tc := range permissionTests {
		t.Run(tc.name, func(t *testing.T) {
			// Create claims with test permissions
			claims := &api.Claims{
				UserID:      "test-user",
				Roles:       []api.Role{api.RoleUser}, // Add a role to pass the role check
				Permissions: tc.userPermissions,
			}

			// Create middleware with test options
			rbacMiddleware := NewRBACMiddleware(&RBACOptions{
				RequiredPermissions: tc.requiredPermissions,
			})

			// Create a handler with context and middleware
			handler := rbacMiddleware(successHandler)
			contextHandler := setClaimsContext(handler, claims)

			// Create request and response recorder
			req := httptest.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()

			// Serve the request
			contextHandler.ServeHTTP(rr, req)

			// Check the status code
			if rr.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, rr.Code)
			}
		})
	}
}
