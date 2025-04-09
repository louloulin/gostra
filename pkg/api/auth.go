package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// Security related errors
var (
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccessDenied       = errors.New("access denied")
	ErrMissingToken       = errors.New("missing token")
)

// Role represents user roles in the system
type Role string

// Predefined roles
const (
	RoleAdmin  Role = "admin"
	RoleUser   Role = "user"
	RoleGuest  Role = "guest"
	RoleSystem Role = "system"
)

// Permission represents a specific capability in the system
type Permission string

// Predefined permissions
const (
	PermissionAgentRead        Permission = "agent:read"
	PermissionAgentWrite       Permission = "agent:write"
	PermissionAgentExecute     Permission = "agent:execute"
	PermissionToolRead         Permission = "tool:read"
	PermissionToolExecute      Permission = "tool:execute"
	PermissionThreadRead       Permission = "thread:read"
	PermissionThreadWrite      Permission = "thread:write"
	PermissionSystemAdmin      Permission = "system:admin"
	PermissionApiPluginExecute Permission = "api:plugin:execute"
)

// User represents a user in the system
type User struct {
	ID          string       `json:"id"`
	Username    string       `json:"username"`
	Email       string       `json:"email,omitempty"`
	Roles       []Role       `json:"roles"`
	Permissions []Permission `json:"permissions,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`

	// Fields not included in responses
	PasswordHash string `json:"-"`
}

// Claims represents JWT claims
type Claims struct {
	UserID      string       `json:"uid"`
	Username    string       `json:"username"`
	Roles       []Role       `json:"roles"`
	Permissions []Permission `json:"permissions"`
	jwt.RegisteredClaims
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	JWTSecret       string        // Secret key for JWT signing
	TokenExpiration time.Duration // Token expiration time
	EnableAuth      bool          // Whether authentication is enabled
	AllowPublicAPIs bool          // Whether some APIs are accessible without authentication
}

// DefaultAuthConfig returns default auth configuration
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		JWTSecret:       "gostra-jwt-secret-change-in-production",
		TokenExpiration: 24 * time.Hour,
		EnableAuth:      false,
		AllowPublicAPIs: true,
	}
}

// AuthService provides authentication and authorization services
type AuthService struct {
	config   *AuthConfig
	userRepo UserRepository
}

// UserRepository defines the interface for user storage
type UserRepository interface {
	FindByID(id string) (*User, error)
	FindByUsername(username string) (*User, error)
	Create(user *User) error
	Update(user *User) error
	Delete(id string) error
	List() ([]*User, error)
}

// NewAuthService creates a new authentication service
func NewAuthService(config *AuthConfig, repo UserRepository) *AuthService {
	if config == nil {
		config = DefaultAuthConfig()
	}

	return &AuthService{
		config:   config,
		userRepo: repo,
	}
}

// AuthenticateUser authenticates a user with username and password
func (s *AuthService) AuthenticateUser(username, password string) (string, error) {
	// Find user by username
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return "", ErrInvalidCredentials
	}

	// Check password (in a real implementation, this would use bcrypt or similar)
	if !checkPassword(password, user.PasswordHash) {
		return "", ErrInvalidCredentials
	}

	// Generate token
	token, err := s.GenerateToken(user)
	if err != nil {
		return "", err
	}

	return token, nil
}

// GenerateToken generates a JWT token for a user
func (s *AuthService) GenerateToken(user *User) (string, error) {
	// Create claims with user information
	claims := Claims{
		UserID:      user.ID,
		Username:    user.Username,
		Roles:       user.Roles,
		Permissions: user.Permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.config.TokenExpiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "gostra-api",
		},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token with secret key
	signedToken, err := token.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

// ValidateToken validates a JWT token and returns the claims
func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	// Check if token is expired
	if claims.ExpiresAt.Time.Before(time.Now()) {
		return nil, ErrTokenExpired
	}

	return claims, nil
}

// AuthMiddleware creates a middleware for authentication
func (s *AuthService) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth if disabled or request to public APIs
		if !s.config.EnableAuth || (s.config.AllowPublicAPIs && isPublicAPI(r.URL.Path)) {
			next.ServeHTTP(w, r)
			return
		}

		// Get token from Authorization header
		tokenString := extractTokenFromHeader(r)
		if tokenString == "" {
			sendError(w, http.StatusUnauthorized, ErrMissingToken.Error())
			return
		}

		// Validate token
		claims, err := s.ValidateToken(tokenString)
		if err != nil {
			sendError(w, http.StatusUnauthorized, err.Error())
			return
		}

		// Create a context with user claims
		ctx := context.WithValue(r.Context(), "claims", claims)

		// Call the next handler with the updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePermission creates a middleware that checks if the user has a specific permission
func (s *AuthService) RequirePermission(permission Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if disabled
			if !s.config.EnableAuth {
				next.ServeHTTP(w, r)
				return
			}

			// Get claims from context
			claims, ok := r.Context().Value("claims").(*Claims)
			if !ok {
				sendError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			// Check if the user has admin role
			for _, role := range claims.Roles {
				if role == RoleAdmin {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check if the user has the required permission
			hasPermission := false
			for _, p := range claims.Permissions {
				if p == permission {
					hasPermission = true
					break
				}
			}

			if !hasPermission {
				sendError(w, http.StatusForbidden, ErrAccessDenied.Error())
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole creates a middleware that checks if the user has a specific role
func (s *AuthService) RequireRole(role Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if disabled
			if !s.config.EnableAuth {
				next.ServeHTTP(w, r)
				return
			}

			// Get claims from context
			claims, ok := r.Context().Value("claims").(*Claims)
			if !ok {
				sendError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			// Check if the user has the required role
			hasRole := false
			for _, r := range claims.Roles {
				if r == role {
					hasRole = true
					break
				}

				// Admin role has access to everything
				if r == RoleAdmin {
					hasRole = true
					break
				}
			}

			if !hasRole {
				sendError(w, http.StatusForbidden, ErrAccessDenied.Error())
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetUserFromContext extracts user claims from context
func GetUserFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value("claims").(*Claims)
	return claims, ok
}

// Helper functions

// extractTokenFromHeader extracts JWT token from Authorization header
func extractTokenFromHeader(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	// Check if the header starts with "Bearer "
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}

	return strings.TrimPrefix(auth, prefix)
}

// isPublicAPI checks if an API endpoint is public
func isPublicAPI(path string) bool {
	publicPaths := []string{
		"/health",
		"/api/v1/auth/login",
		"/api/v1/auth/register",
	}

	for _, p := range publicPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}

	return false
}

// Mock implementation of checking passwords - in a real app, use bcrypt
func checkPassword(plain, hashed string) bool {
	// For demonstration purposes only!
	// In a real implementation, use bcrypt.CompareHashAndPassword
	return hashed == "hashed_"+plain
}
