package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/louloulin/gostra/pkg/api"
	"github.com/louloulin/gostra/pkg/auth/apikey"
)

// Define API endpoints
func setupRoutes(mux *http.ServeMux, apiKeyService *apikey.APIKeyService) {
	// Public endpoints don't require authentication
	mux.HandleFunc("/api/public", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "This is a public endpoint, no API key required")
	})

	// Create API key middleware
	apiKeyMiddleware := apikey.NewAPIKeyMiddleware(&apikey.APIKeyMiddlewareOptions{
		APIKeyService: apiKeyService,
		ExcludedPaths: []string{"/api/public", "/api/docs/*"},
	})

	// User endpoint requires API key with user:read scope
	userHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get API key from context
		key, ok := apikey.GetAPIKeyFromContext(r.Context())
		if !ok {
			http.Error(w, "API key not found", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "API Key '%s' used to access user data\n", key.Name)
		fmt.Fprintf(w, "Owner ID: %s\n", key.OwnerID)
		fmt.Fprintf(w, "Expires: %s\n", key.ExpiresAt.Format(time.RFC3339))
	})

	// Admin endpoint requires API key with admin role
	adminHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(api.ContextKeyUser).(*api.Claims)
		if !ok {
			http.Error(w, "Claims not found", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "Admin action performed by: %s\n", claims.Username)
	})

	// Apply middleware and register routes
	mux.Handle("/api/users", apiKeyMiddleware(
		apikey.RequireAPIKeyScope(apiKeyService, "user:read")(userHandler),
	))

	mux.Handle("/api/admin", apiKeyMiddleware(
		apikey.RequireAPIKeyPermission(apiKeyService, api.PermissionSystemAdmin)(adminHandler),
	))

	// API key management endpoints
	apiKeyManagementHandler := newAPIKeyManagementHandler(apiKeyService)
	mux.Handle("/api/keys/create", apiKeyMiddleware(http.HandlerFunc(apiKeyManagementHandler.createAPIKey)))
	mux.Handle("/api/keys/list", apiKeyMiddleware(http.HandlerFunc(apiKeyManagementHandler.listAPIKeys)))
	mux.Handle("/api/keys/revoke", apiKeyMiddleware(http.HandlerFunc(apiKeyManagementHandler.revokeAPIKey)))
}

// APIKeyManagementHandler provides endpoints for managing API keys
type APIKeyManagementHandler struct {
	service *apikey.APIKeyService
}

func newAPIKeyManagementHandler(service *apikey.APIKeyService) *APIKeyManagementHandler {
	return &APIKeyManagementHandler{
		service: service,
	}
}

// createAPIKey handles API key creation requests
func (h *APIKeyManagementHandler) createAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// In a real application, would parse request body for API key options
	// For this example, we'll use hardcoded values
	expiresIn := 30 * 24 * time.Hour // 30 days

	// Convert role types
	roles := []string{}
	for _, role := range []api.Role{api.RoleUser} {
		roles = append(roles, string(role))
	}

	opts := &apikey.APIKeyOptions{
		Name:      "Sample Key",
		OwnerID:   "current-user-id", // Would come from authenticated user
		ExpiresIn: expiresIn,         // 30 days
		Scopes:    []string{"user:read", "user:write"},
		Roles:     roles,
	}

	// Create the API key
	key, fullKey, err := h.service.GenerateAPIKey(opts)
	if err != nil {
		http.Error(w, "Failed to create API key: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// In a real application, would return JSON
	fmt.Fprintf(w, "API Key created successfully\n")
	fmt.Fprintf(w, "Key ID: %s\n", key.ID)
	fmt.Fprintf(w, "Key Name: %s\n", key.Name)
	fmt.Fprintf(w, "Secret (only shown once): %s\n", fullKey)
	fmt.Fprintf(w, "Expires: %s\n", key.ExpiresAt.Format(time.RFC3339))
}

// listAPIKeys handles requests to list all API keys for the current user
func (h *APIKeyManagementHandler) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from claims
	claims, ok := r.Context().Value(api.ContextKeyUser).(*api.Claims)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// List API keys for the user
	keys, err := h.service.ListAPIKeys(claims.UserID)
	if err != nil {
		http.Error(w, "Failed to list API keys: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// In a real application, would return JSON
	fmt.Fprintf(w, "API Keys for user %s:\n\n", claims.UserID)

	if len(keys) == 0 {
		fmt.Fprintf(w, "No API keys found\n")
		return
	}

	for i, key := range keys {
		fmt.Fprintf(w, "Key %d:\n", i+1)
		fmt.Fprintf(w, "  ID: %s\n", key.ID)
		fmt.Fprintf(w, "  Name: %s\n", key.Name)
		fmt.Fprintf(w, "  Prefix: %s\n", key.Prefix)
		fmt.Fprintf(w, "  Expires: %s\n", key.ExpiresAt.Format(time.RFC3339))
		fmt.Fprintf(w, "  Revoked: %t\n\n", key.IsRevoked)
	}
}

// revokeAPIKey handles requests to revoke an API key
func (h *APIKeyManagementHandler) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// In a real application, would parse request body
	keyID := r.URL.Query().Get("id")
	if keyID == "" {
		http.Error(w, "API key ID required", http.StatusBadRequest)
		return
	}

	// Revoke the key
	err := h.service.RevokeAPIKey(keyID)
	if err != nil {
		http.Error(w, "Failed to revoke API key: "+err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "API Key %s revoked successfully\n", keyID)
}

func main() {
	// Create API key repository
	repo := apikey.NewInMemoryAPIKeyRepository()

	// Create API key service
	service := apikey.NewAPIKeyService(repo)

	// Create admin API key for testing
	adminRoles := []string{}
	for _, role := range []api.Role{api.RoleAdmin} {
		adminRoles = append(adminRoles, string(role))
	}

	adminPermissions := []string{}
	for _, perm := range []api.Permission{api.PermissionSystemAdmin} {
		adminPermissions = append(adminPermissions, string(perm))
	}

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

	// Store the reference key ID for demos
	adminKeyID := adminKey.ID

	// Create user API key for testing
	userRoles := []string{}
	for _, role := range []api.Role{api.RoleUser} {
		userRoles = append(userRoles, string(role))
	}

	userPermissions := []string{}
	for _, perm := range []api.Permission{api.PermissionAgentRead, api.PermissionThreadRead} {
		userPermissions = append(userPermissions, string(perm))
	}

	userKeyOpts := &apikey.APIKeyOptions{
		Name:        "User API Key",
		OwnerID:     "regular-user",
		ExpiresIn:   24 * time.Hour,
		Roles:       userRoles,
		Permissions: userPermissions,
		Scopes:      []string{"user:read"},
	}

	userKey, userFullKey, err := service.GenerateAPIKey(userKeyOpts)
	if err != nil {
		log.Fatalf("Failed to create user API key: %v", err)
	}

	// Store the reference key ID for demos
	userKeyID := userKey.ID

	// Set up HTTP server
	mux := http.NewServeMux()
	setupRoutes(mux, service)

	// Start the server
	fmt.Println("Starting API key example server on http://localhost:8080")
	fmt.Println("\nTest with the following curl commands:")
	fmt.Printf("  Public endpoint: curl http://localhost:8080/api/public\n")
	fmt.Printf("  User endpoint with user key: curl -H \"X-API-Key: %s\" http://localhost:8080/api/users\n", userFullKey)
	fmt.Printf("  Admin endpoint with admin key: curl -H \"X-API-Key: %s\" http://localhost:8080/api/admin\n", adminFullKey)
	fmt.Printf("  Admin endpoint with user key (should fail): curl -H \"X-API-Key: %s\" http://localhost:8080/api/admin\n", userFullKey)

	fmt.Println("\nAPI Key Management:")
	fmt.Printf("  List keys: curl -H \"X-API-Key: %s\" http://localhost:8080/api/keys/list\n", adminFullKey)
	fmt.Printf("  Create key: curl -X POST -H \"X-API-Key: %s\" http://localhost:8080/api/keys/create\n", adminFullKey)
	fmt.Printf("  Revoke key: curl -X POST -H \"X-API-Key: %s\" \"http://localhost:8080/api/keys/revoke?id=%s\"\n", adminFullKey, adminKeyID)
	fmt.Printf("  Revoke user key: curl -X POST -H \"X-API-Key: %s\" \"http://localhost:8080/api/keys/revoke?id=%s\"\n", adminFullKey, userKeyID)

	log.Fatal(http.ListenAndServe(":8080", mux))
}
