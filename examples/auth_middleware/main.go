package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/louloulin/gostra/pkg/api"
	"github.com/louloulin/gostra/pkg/auth/middleware"
)

func main() {
	// Create a user repository
	userRepo := api.NewInMemoryUserRepository()

	// Create a test user
	testUser := &api.User{
		ID:           "test-user-id",
		Username:     "testuser",
		Email:        "test@example.com",
		Roles:        []api.Role{api.RoleAdmin},
		Permissions:  []api.Permission{api.PermissionAgentRead, api.PermissionAgentWrite},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		PasswordHash: "hashed-password", // In a real app, use a proper password hashing function
	}

	// Add the user to the repository
	if err := userRepo.Create(testUser); err != nil {
		log.Fatalf("Failed to create test user: %v", err)
	}

	// Create the auth service
	authConfig := &api.AuthConfig{
		JWTSecret:       "your-jwt-secret", // Use a secure secret in production
		TokenExpiration: 24 * time.Hour,
		EnableAuth:      true,
		AllowPublicAPIs: true,
	}
	authService := api.NewAuthService(authConfig, userRepo)

	// Create the API server
	serverOptions := api.DefaultServerOptions()
	serverOptions.Port = 8080

	// Create a simple placeholder Gostra instance (would be your real instance in production)
	// For a complete example, you would initialize Gostra with your agents, models, etc.
	gostra := &struct{}{}

	server := api.NewServer(gostra, serverOptions)

	// Create auth middleware
	authMiddleware := middleware.NewAuthMiddleware(&middleware.AuthMiddlewareOptions{
		AuthService: authService,
		ExcludedPaths: []string{
			"/health",
			"/api/v1/auth/login",
			"/api/v1/auth/register",
		},
	})

	// Create RBAC middleware for admin-only endpoints
	adminMiddleware := middleware.NewRBACMiddleware(&middleware.RBACOptions{
		RequiredRoles: []api.Role{api.RoleAdmin},
	})

	// Create RBAC middleware for specific permissions
	agentReadMiddleware := middleware.NewRBACMiddleware(&middleware.RBACOptions{
		RequiredPermissions: []api.Permission{api.PermissionAgentRead},
	})

	// Add the middleware to the server
	// For all protected routes
	server.AddMiddleware(authMiddleware)

	// Register admin-only routes
	http.HandleFunc("/admin", middleware.Protected(authMiddleware, adminMiddleware)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Admin area - you have admin privileges")
	})).ServeHTTP)

	// Register agent read routes
	http.HandleFunc("/agents/read", middleware.Protected(authMiddleware, agentReadMiddleware)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Agent read - you have permission to read agents")
	})).ServeHTTP)

	// Add a login handler to get a token
	http.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// In a real app, get credentials from request body
		username := r.FormValue("username")
		password := r.FormValue("password")

		// For demo purposes, accept any valid username
		user, err := userRepo.FindByUsername(username)
		if err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// In a real app, validate password properly
		// This is just for demo purposes
		if password != "password" {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// Generate token
		token, err := authService.GenerateToken(user)
		if err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		// Return token
		fmt.Fprintf(w, "Token: %s", token)
	})

	// Add a public endpoint
	http.HandleFunc("/public", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Public endpoint - no auth required")
	})

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting server on port %d", serverOptions.Port)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// Wait for interrupt signal
	<-stop

	// Create a deadline for the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shut down the server
	log.Println("Shutting down server...")
	if err := server.Stop(ctx); err != nil {
		log.Fatalf("Error during shutdown: %v", err)
	}

	log.Println("Server stopped")
}
