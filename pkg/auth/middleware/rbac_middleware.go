package middleware

import (
	"context"
	"net/http"

	"github.com/yourusername/gostra/pkg/api"
)

// RBACOptions configures the role-based access control middleware
type RBACOptions struct {
	GetRoles            func(ctx context.Context) []api.Role
	RequiredRoles       []api.Role
	RequiredPermissions []api.Permission
	AnyRole             bool // If true, having any of the required roles is sufficient
}

// NewRBACMiddleware creates a middleware function that authorizes requests based on roles and permissions
func NewRBACMiddleware(options *RBACOptions) func(http.Handler) http.Handler {
	if options == nil {
		panic("RBACOptions cannot be nil")
	}

	if options.GetRoles == nil {
		options.GetRoles = func(ctx context.Context) []api.Role {
			// Get user claims from context
			claims, ok := ctx.Value(api.ContextKeyUser).(*api.Claims)
			if !ok {
				return nil
			}
			return claims.Roles
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get roles from context
			roles := options.GetRoles(r.Context())
			if len(roles) == 0 {
				http.Error(w, "Forbidden: Insufficient privileges", http.StatusForbidden)
				return
			}

			// Check if user has required roles
			if len(options.RequiredRoles) > 0 {
				hasRequiredRole := false

				for _, userRole := range roles {
					for _, requiredRole := range options.RequiredRoles {
						if userRole == requiredRole {
							hasRequiredRole = true
							if options.AnyRole {
								break
							}
						}
					}
					if options.AnyRole && hasRequiredRole {
						break
					}
				}

				if !hasRequiredRole {
					http.Error(w, "Forbidden: Required role not found", http.StatusForbidden)
					return
				}
			}

			// Get permissions from context
			var permissions []api.Permission
			claims, ok := r.Context().Value(api.ContextKeyUser).(*api.Claims)
			if ok {
				permissions = claims.Permissions
			}

			// Check if user has required permissions
			if len(options.RequiredPermissions) > 0 {
				hasAllPermissions := true

				for _, requiredPerm := range options.RequiredPermissions {
					hasPermission := false
					for _, userPerm := range permissions {
						if userPerm == requiredPerm {
							hasPermission = true
							break
						}
					}

					if !hasPermission {
						hasAllPermissions = false
						break
					}
				}

				if !hasAllPermissions {
					http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
					return
				}
			}

			// User has the required roles and permissions
			next.ServeHTTP(w, r)
		})
	}
}
