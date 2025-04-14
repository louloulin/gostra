package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/louloulin/gostra/pkg"
	"github.com/louloulin/gostra/pkg/api"
)

func main() {
	// Create new Gostra instance
	gostra := pkg.NewGostra(pkg.DefaultOptions())

	// Create user repository
	userRepo := api.NewInMemoryUserRepository()

	// Add some initial users
	addInitialUsers(userRepo)

	// Create auth config with auth enabled
	authConfig := &api.AuthConfig{
		JWTSecret:       "gostra-secure-example-secret-key",
		TokenExpiration: 24 * time.Hour,
		EnableAuth:      true,
		AllowPublicAPIs: true,
	}

	// Create auth service
	authService := api.NewAuthService(authConfig, userRepo)

	// Create API server with default options
	apiOptions := api.DefaultServerOptions()
	apiOptions.Port = 8085 // Use a different port
	server := api.NewServer(gostra, apiOptions)

	// Register auth middleware with API server
	server.AddMiddleware(authService.AuthMiddleware)

	// Create auth plugin
	authPlugin := &api.Plugin{
		Name:        "auth",
		Description: "Authentication and user management API",
		Version:     "1.0.0",

		Initialize: func() error {
			log.Println("Initializing auth plugin...")
			return nil
		},

		RegisterRoutes: func(r *mux.Router) {
			// Public routes (don't require authentication)
			r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
				handleLogin(w, r, authService)
			}).Methods("POST")

			r.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
				handleRegister(w, r, userRepo)
			}).Methods("POST")

			// Protected routes (require authentication)
			usersRouter := r.PathPrefix("/users").Subrouter()
			usersRouter.Use(authService.RequireRole(api.RoleAdmin))

			usersRouter.HandleFunc("", func(w http.ResponseWriter, r *http.Request) {
				handleListUsers(w, r, userRepo)
			}).Methods("GET")

			usersRouter.HandleFunc("/{id}", func(w http.ResponseWriter, r *http.Request) {
				handleGetUser(w, r, userRepo)
			}).Methods("GET")
		},

		Shutdown: func() error {
			log.Println("Shutting down auth plugin...")
			return nil
		},
	}

	// Register the auth plugin
	if err := server.RegisterPlugin(authPlugin); err != nil {
		log.Fatalf("Failed to register auth plugin: %v", err)
	}

	// Create admin dashboard plugin
	adminPlugin := &api.Plugin{
		Name:        "admin",
		Description: "Administrative dashboard API",
		Version:     "1.0.0",

		Initialize: func() error {
			log.Println("Initializing admin plugin...")
			return nil
		},

		RegisterRoutes: func(r *mux.Router) {
			// All admin routes require admin role
			r.Use(authService.RequireRole(api.RoleAdmin))

			r.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
				handleDashboard(w, r)
			}).Methods("GET")

			r.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
				handleStats(w, r)
			}).Methods("GET")
		},

		Shutdown: func() error {
			log.Println("Shutting down admin plugin...")
			return nil
		},
	}

	// Register the admin plugin
	if err := server.RegisterPlugin(adminPlugin); err != nil {
		log.Fatalf("Failed to register admin plugin: %v", err)
	}

	// Create user features plugin
	userPlugin := &api.Plugin{
		Name:        "user-features",
		Description: "User features API",
		Version:     "1.0.0",

		Initialize: func() error {
			log.Println("Initializing user features plugin...")
			return nil
		},

		RegisterRoutes: func(r *mux.Router) {
			// Routes that require authenticated user
			r.Use(authService.RequireRole(api.RoleUser))

			r.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
				handleProfile(w, r)
			}).Methods("GET")

			// Route that requires a specific permission
			agentsRouter := r.PathPrefix("/agents").Subrouter()
			agentsRouter.Use(authService.RequirePermission(api.PermissionAgentExecute))

			agentsRouter.HandleFunc("/run", func(w http.ResponseWriter, r *http.Request) {
				handleRunAgent(w, r)
			}).Methods("POST")
		},

		Shutdown: func() error {
			log.Println("Shutting down user features plugin...")
			return nil
		},
	}

	// Register the user features plugin
	if err := server.RegisterPlugin(userPlugin); err != nil {
		log.Fatalf("Failed to register user features plugin: %v", err)
	}

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting secure API server on port %d...", apiOptions.Port)
		if err := server.Start(); err != nil {
			log.Fatalf("Error starting server: %v", err)
		}
	}()

	log.Println("Server started. Use the following endpoints for testing:")
	log.Println("- Login (POST): http://localhost:8085/plugins/auth/login")
	log.Println("- Register (POST): http://localhost:8085/plugins/auth/register")
	log.Println("- List Users (Admin only, GET): http://localhost:8085/plugins/auth/users")
	log.Println("- Admin Dashboard (Admin only, GET): http://localhost:8085/plugins/admin/dashboard")
	log.Println("- User Profile (Any authenticated user, GET): http://localhost:8085/plugins/user-features/profile")
	log.Println("- Run Agent (Requires agent:execute permission, POST): http://localhost:8085/plugins/user-features/agents/run")

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Create a deadline to wait for
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline
	if err := server.Stop(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}

// addInitialUsers adds some initial users to the repository
func addInitialUsers(repo api.UserRepository) {
	// Admin user
	adminUser := &api.User{
		Username: "admin",
		Email:    "admin@example.com",
		Roles:    []api.Role{api.RoleAdmin},
		Permissions: []api.Permission{
			api.PermissionSystemAdmin,
			api.PermissionAgentRead,
			api.PermissionAgentWrite,
			api.PermissionAgentExecute,
			api.PermissionToolRead,
			api.PermissionToolExecute,
		},
		PasswordHash: "hashed_adminpass", // In a real app, this would be bcrypt hashed
	}
	if err := repo.Create(adminUser); err != nil {
		log.Printf("Error creating admin user: %v", err)
	}

	// Regular user
	regularUser := &api.User{
		Username: "user",
		Email:    "user@example.com",
		Roles:    []api.Role{api.RoleUser},
		Permissions: []api.Permission{
			api.PermissionAgentRead,
			api.PermissionToolRead,
			api.PermissionThreadRead,
			api.PermissionThreadWrite,
		},
		PasswordHash: "hashed_userpass", // In a real app, this would be bcrypt hashed
	}
	if err := repo.Create(regularUser); err != nil {
		log.Printf("Error creating regular user: %v", err)
	}

	// Power user
	powerUser := &api.User{
		Username: "power",
		Email:    "power@example.com",
		Roles:    []api.Role{api.RoleUser},
		Permissions: []api.Permission{
			api.PermissionAgentRead,
			api.PermissionAgentExecute,
			api.PermissionToolRead,
			api.PermissionToolExecute,
			api.PermissionThreadRead,
			api.PermissionThreadWrite,
		},
		PasswordHash: "hashed_powerpass", // In a real app, this would be bcrypt hashed
	}
	if err := repo.Create(powerUser); err != nil {
		log.Printf("Error creating power user: %v", err)
	}
}

// Handler functions for the auth plugin

// handleLogin handles user login
func handleLogin(w http.ResponseWriter, r *http.Request, authService *api.AuthService) {
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	// Decode request body
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Authenticate user
	token, err := authService.AuthenticateUser(credentials.Username, credentials.Password)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Send response with token
	response := map[string]string{
		"token": token,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleRegister handles user registration
func handleRegister(w http.ResponseWriter, r *http.Request, userRepo api.UserRepository) {
	var userData struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	// Decode request body
	if err := json.NewDecoder(r.Body).Decode(&userData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if userData.Username == "" || userData.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// Create user
	user := &api.User{
		Username:     userData.Username,
		Email:        userData.Email,
		Roles:        []api.Role{api.RoleUser},
		Permissions:  []api.Permission{api.PermissionAgentRead, api.PermissionToolRead},
		PasswordHash: "hashed_" + userData.Password, // In a real app, this would be bcrypt hashed
	}

	if err := userRepo.Create(user); err != nil {
		http.Error(w, "Could not create user", http.StatusInternalServerError)
		return
	}

	// Send success response
	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "User created successfully"})
}

// handleListUsers lists all users (admin only)
func handleListUsers(w http.ResponseWriter, r *http.Request, userRepo api.UserRepository) {
	users, err := userRepo.List()
	if err != nil {
		http.Error(w, "Could not list users", http.StatusInternalServerError)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// handleGetUser gets a user by ID (admin only)
func handleGetUser(w http.ResponseWriter, r *http.Request, userRepo api.UserRepository) {
	vars := mux.Vars(r)
	id := vars["id"]

	user, err := userRepo.FindByID(id)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// Handler functions for the admin plugin

// handleDashboard handles the admin dashboard
func handleDashboard(w http.ResponseWriter, r *http.Request) {
	claims, ok := api.GetUserFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get admin dashboard data
	dashboard := map[string]interface{}{
		"admin": claims.Username,
		"system_stats": map[string]interface{}{
			"users":        15,
			"agents":       4,
			"tools":        8,
			"total_runs":   237,
			"success_rate": "98.3%",
		},
		"recent_activity": []map[string]interface{}{
			{
				"type":      "agent_creation",
				"timestamp": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
				"user":      "power",
				"details":   "Created new search agent",
			},
			{
				"type":      "tool_execution",
				"timestamp": time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
				"user":      "user",
				"details":   "Executed document search tool",
			},
		},
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dashboard)
}

// handleStats handles the system stats
func handleStats(w http.ResponseWriter, r *http.Request) {
	// Get system stats
	stats := map[string]interface{}{
		"memory_usage": map[string]string{
			"total":  "512MB",
			"used":   "324MB",
			"free":   "188MB",
			"cached": "156MB",
		},
		"cpu_usage": "23%",
		"disk_usage": map[string]string{
			"total": "10GB",
			"used":  "4.2GB",
			"free":  "5.8GB",
		},
		"network": map[string]string{
			"incoming": "1.2MB/s",
			"outgoing": "0.8MB/s",
		},
		"uptime": "3d 12h 45m",
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// Handler functions for the user features plugin

// handleProfile handles user profile
func handleProfile(w http.ResponseWriter, r *http.Request) {
	claims, ok := api.GetUserFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get user profile
	profile := map[string]interface{}{
		"username":    claims.Username,
		"roles":       claims.Roles,
		"permissions": claims.Permissions,
		"settings": map[string]interface{}{
			"theme":          "dark",
			"notifications":  true,
			"language":       "en",
			"timezone":       "UTC",
			"date_format":    "YYYY-MM-DD",
			"default_agent":  "assistant",
			"api_quota":      1000,
			"api_usage":      153,
			"premium_status": false,
		},
		"recent_activity": []map[string]interface{}{
			{
				"type":      "agent_run",
				"timestamp": time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
				"agent":     "search",
				"status":    "success",
			},
			{
				"type":      "tool_use",
				"timestamp": time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
				"tool":      "document_processor",
				"status":    "success",
			},
		},
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}

// handleRunAgent handles running an agent
func handleRunAgent(w http.ResponseWriter, r *http.Request) {
	claims, ok := api.GetUserFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var agentRequest struct {
		AgentName string                 `json:"agent_name"`
		Input     string                 `json:"input"`
		Options   map[string]interface{} `json:"options,omitempty"`
	}

	// Decode request body
	if err := json.NewDecoder(r.Body).Decode(&agentRequest); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Simulate agent run
	result := map[string]interface{}{
		"user":       claims.Username,
		"agent_name": agentRequest.AgentName,
		"input":      agentRequest.Input,
		"output":     fmt.Sprintf("Processed input: %s", agentRequest.Input),
		"timestamp":  time.Now().Format(time.RFC3339),
		"status":     "success",
		"duration":   "1.2s",
		"tools_used": []string{"search", "calculator"},
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
