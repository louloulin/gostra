package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/louloulin/gostra/pkg/api"
	"github.com/louloulin/gostra/pkg/auth/apikey"
)

func main() {
	// Get database connection string from environment variable
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		log.Fatal("POSTGRES_DSN environment variable must be set")
	}

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Check connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Println("Successfully connected to PostgreSQL database")

	// Create PostgreSQL repository
	repo, err := apikey.NewPostgresAPIKeyRepository(db)
	if err != nil {
		log.Fatalf("Failed to create repository: %v", err)
	}

	// Create API key service
	service := apikey.NewAPIKeyService(repo)

	// Set up HTTP server
	mux := http.NewServeMux()

	// Public endpoint
	mux.HandleFunc("/api/public", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "This is a public endpoint, no API key required")
	})

	// Create API key middleware
	apiKeyMiddleware := apikey.NewAPIKeyMiddleware(&apikey.APIKeyMiddlewareOptions{
		APIKeyService: service,
		ExcludedPaths: []string{"/api/public", "/api/docs/*"},
	})

	// Protected endpoints
	userHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get API key from context
		key, ok := apikey.GetAPIKeyFromContext(r.Context())
		if !ok {
			http.Error(w, "API key not found", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "User endpoint accessed with API key: %s\n", key.Name)
		fmt.Fprintf(w, "Owner ID: %s\n", key.OwnerID)
		fmt.Fprintf(w, "Expires: %s\n", key.ExpiresAt.Format(time.RFC3339))
	})

	adminHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Admin endpoint accessed\n")
	})

	// Register protected routes
	mux.Handle("/api/users", apiKeyMiddleware(
		apikey.RequireAPIKeyScope(service, "user:read")(userHandler),
	))

	mux.Handle("/api/admin", apiKeyMiddleware(
		apikey.RequireAPIKeyPermission(service, api.PermissionSystemAdmin)(adminHandler),
	))

	// API Key management endpoints
	mux.Handle("/api/keys/create", apiKeyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Convert roles and permissions to strings
		roles := []string{string(api.RoleUser)}
		permissions := []string{string(api.PermissionAgentRead)}

		// Create API key options
		opts := &apikey.APIKeyOptions{
			Name:        "Generated API Key",
			OwnerID:     "user-123",
			ExpiresIn:   30 * 24 * time.Hour, // 30 days
			Scopes:      []string{"user:read", "user:write"},
			Roles:       roles,
			Permissions: permissions,
		}

		// Generate the API key
		key, fullKey, err := service.GenerateAPIKey(opts)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create API key: %v", err), http.StatusInternalServerError)
			return
		}

		// Display API key info
		fmt.Fprintf(w, "API Key created successfully\n")
		fmt.Fprintf(w, "Key ID: %s\n", key.ID)
		fmt.Fprintf(w, "Key Name: %s\n", key.Name)
		fmt.Fprintf(w, "Secret (only shown once): %s\n", fullKey)
		fmt.Fprintf(w, "Expires: %s\n", key.ExpiresAt.Format(time.RFC3339))
	})))

	mux.Handle("/api/keys/list", apiKeyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get user ID from context
		claims, ok := r.Context().Value(api.ContextKeyUser).(*api.Claims)
		if !ok {
			http.Error(w, "User not authenticated", http.StatusUnauthorized)
			return
		}

		// List API keys for user
		keys, err := service.ListAPIKeys(claims.UserID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to list API keys: %v", err), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "API Keys for user %s:\n\n", claims.UserID)

		if len(keys) == 0 {
			fmt.Fprintf(w, "No API keys found")
			return
		}

		for i, key := range keys {
			fmt.Fprintf(w, "Key %d:\n", i+1)
			fmt.Fprintf(w, "  ID: %s\n", key.ID)
			fmt.Fprintf(w, "  Name: %s\n", key.Name)
			fmt.Fprintf(w, "  Created: %s\n", key.CreatedAt.Format(time.RFC3339))
			fmt.Fprintf(w, "  Expires: %s\n", key.ExpiresAt.Format(time.RFC3339))
			fmt.Fprintf(w, "  Revoked: %t\n", key.IsRevoked)
			fmt.Fprintf(w, "\n")
		}
	})))

	mux.Handle("/api/keys/revoke", apiKeyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		keyID := r.URL.Query().Get("id")
		if keyID == "" {
			http.Error(w, "Missing key ID", http.StatusBadRequest)
			return
		}

		// Revoke the key
		err := service.RevokeAPIKey(keyID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to revoke API key: %v", err), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "API Key %s successfully revoked", keyID)
	})))

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create an admin key for testing
	adminRoles := []string{string(api.RoleAdmin)}
	adminPermissions := []string{string(api.PermissionSystemAdmin)}

	adminKeyOpts := &apikey.APIKeyOptions{
		Name:        "Admin API Key",
		OwnerID:     "admin-user",
		ExpiresIn:   24 * time.Hour,
		Roles:       adminRoles,
		Permissions: adminPermissions,
		Scopes:      []string{"user:read", "user:write", "admin:*"},
	}

	adminKey, adminFullKey, err := service.GenerateAPIKey(adminKeyOpts)
	if err != nil {
		log.Fatalf("Failed to create admin API key: %v", err)
	}

	// Print connection info and test keys
	fmt.Printf("Server running on http://localhost:%s\n", port)
	fmt.Println("\nTest with the following curl commands:")
	fmt.Printf("  Public endpoint: curl http://localhost:%s/api/public\n", port)
	fmt.Printf("  Admin endpoint: curl -H \"X-API-Key: %s\" http://localhost:%s/api/admin\n", adminFullKey, port)
	fmt.Printf("  Create new key: curl -X POST -H \"X-API-Key: %s\" http://localhost:%s/api/keys/create\n", adminFullKey, port)
	fmt.Printf("  List keys: curl -H \"X-API-Key: %s\" http://localhost:%s/api/keys/list\n", adminFullKey, port)
	fmt.Printf("  Revoke admin key: curl -X POST -H \"X-API-Key: %s\" \"http://localhost:%s/api/keys/revoke?id=%s\"\n", adminFullKey, port, adminKey.ID)

	// Schedule periodic cleanup of expired keys
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			count := service.CleanExpiredKeys()
			if count > 0 {
				log.Printf("Cleaned up %d expired API keys", count)
			}
		}
	}()

	// Start HTTP server
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
