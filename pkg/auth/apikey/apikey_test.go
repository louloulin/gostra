package apikey

import (
	"strings"
	"testing"
	"time"

	"github.com/louloulin/gostra/pkg/api"
)

func TestAPIKeyGeneration(t *testing.T) {
	// Create a repository
	repo := NewInMemoryAPIKeyRepository()
	service := NewAPIKeyService(repo)

	// Convert API roles and permissions to strings
	roles := []string{string(api.RoleUser)}
	permissions := []string{string(api.PermissionAgentRead)}

	// Create API key options
	opts := &APIKeyOptions{
		Name:        "Test API Key",
		OwnerID:     "user123",
		ExpiresIn:   24 * time.Hour,
		Scopes:      []string{"api:read", "api:write"},
		Roles:       roles,
		Permissions: permissions,
	}

	// Generate the API key
	apiKey, fullKey, err := service.GenerateAPIKey(opts)

	// Check results
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	if apiKey.Secret == "" {
		t.Error("API key secret is empty")
	}

	if apiKey.Name != opts.Name {
		t.Errorf("API key name mismatch, got %s, want %s", apiKey.Name, opts.Name)
	}

	if apiKey.OwnerID != opts.OwnerID {
		t.Errorf("API key owner ID mismatch, got %s, want %s", apiKey.OwnerID, opts.OwnerID)
	}

	if len(apiKey.Scopes) != len(opts.Scopes) {
		t.Errorf("API key scopes count mismatch, got %d, want %d", len(apiKey.Scopes), len(opts.Scopes))
	}

	// Verify the prefix format
	if len(apiKey.Prefix) == 0 {
		t.Errorf("API key prefix is empty")
	}

	// Verify the full key includes the prefix
	if !strings.HasPrefix(fullKey, apiKey.Prefix) {
		t.Errorf("Full API key doesn't start with the prefix")
	}
}

func TestAPIKeyValidation(t *testing.T) {
	// Create a repository
	repo := NewInMemoryAPIKeyRepository()
	service := NewAPIKeyService(repo)

	// Create an API key
	roles := []string{string(api.RoleUser)}
	permissions := []string{string(api.PermissionAgentRead)}

	opts := &APIKeyOptions{
		Name:        "Test Validation Key",
		OwnerID:     "user456",
		ExpiresIn:   24 * time.Hour,
		Scopes:      []string{"api:read"},
		Roles:       roles,
		Permissions: permissions,
	}

	// Generate the API key
	apiKey, fullKey, err := service.GenerateAPIKey(opts)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Validate the API key using its secret
	validatedKey, err := service.ValidateAPIKey(fullKey)
	if err != nil {
		t.Fatalf("Failed to validate API key: %v", err)
	}

	// Check the validated key
	if validatedKey.ID != apiKey.ID {
		t.Errorf("Validated key ID mismatch, got %s, want %s", validatedKey.ID, apiKey.ID)
	}

	if validatedKey.Name != apiKey.Name {
		t.Errorf("Validated key name mismatch, got %s, want %s", validatedKey.Name, apiKey.Name)
	}

	// Try with an invalid key
	_, err = service.ValidateAPIKey("invalid_key")
	if err == nil {
		t.Error("Expected error for invalid key, got nil")
	}

	// Validate with a prefix that doesn't exist
	_, err = service.ValidateAPIKey("nonexistent.secretpart")
	if err == nil {
		t.Error("Expected error for nonexistent prefix, got nil")
	}
}

func TestAPIKeyRevocation(t *testing.T) {
	// Create a repository
	repo := NewInMemoryAPIKeyRepository()
	service := NewAPIKeyService(repo)

	// Create an API key
	opts := &APIKeyOptions{
		Name:      "Revocable Key",
		OwnerID:   "user789",
		ExpiresIn: 24 * time.Hour,
		Scopes:    []string{"api:read"},
	}

	// Generate the API key
	apiKey, fullKey, err := service.GenerateAPIKey(opts)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Validate it works
	_, err = service.ValidateAPIKey(fullKey)
	if err != nil {
		t.Fatalf("Failed to validate API key before revocation: %v", err)
	}

	// Revoke the key
	err = service.RevokeAPIKey(apiKey.ID)
	if err != nil {
		t.Fatalf("Failed to revoke API key: %v", err)
	}

	// Try to validate it again - should fail
	_, err = service.ValidateAPIKey(fullKey)
	if err == nil {
		t.Error("Expected error for revoked key, got nil")
	}
	if err != ErrAPIKeyRevoked {
		t.Errorf("Expected ErrAPIKeyRevoked, got %v", err)
	}
}

func TestAPIKeyExpiration(t *testing.T) {
	// Create a repository
	repo := NewInMemoryAPIKeyRepository()
	service := NewAPIKeyService(repo)

	// Create an API key with a very short expiration
	opts := &APIKeyOptions{
		Name:      "Expiring Key",
		OwnerID:   "user101",
		ExpiresIn: 1 * time.Millisecond, // Very short to test expiration
		Scopes:    []string{"api:read"},
	}

	// Generate the API key
	_, fullKey, err := service.GenerateAPIKey(opts)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	// Try to validate it - should fail
	_, err = service.ValidateAPIKey(fullKey)
	if err == nil {
		t.Error("Expected error for expired key, got nil")
	}
	if err != ErrAPIKeyExpired {
		t.Errorf("Expected ErrAPIKeyExpired, got %v", err)
	}

	// Test the cleanup function
	count := service.CleanExpiredKeys()
	if count != 1 {
		t.Errorf("Expected 1 expired key to be cleaned up, got %d", count)
	}
}

func TestAPIKeyPermissionChecks(t *testing.T) {
	// Create a repository
	repo := NewInMemoryAPIKeyRepository()
	service := NewAPIKeyService(repo)

	// Create an API key with specific permissions
	permissions := []string{
		string(api.PermissionAgentRead),
		string(api.PermissionThreadRead),
	}

	roles := []string{string(api.RoleUser)}

	opts := &APIKeyOptions{
		Name:        "Permission Test Key",
		OwnerID:     "user202",
		ExpiresIn:   24 * time.Hour,
		Roles:       roles,
		Permissions: permissions,
	}

	// Generate the API key
	apiKey, _, err := service.GenerateAPIKey(opts)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Check permissions that should exist
	if !service.HasPermission(apiKey, api.PermissionAgentRead) {
		t.Error("Expected key to have PermissionAgentRead")
	}

	if !service.HasPermission(apiKey, api.PermissionThreadRead) {
		t.Error("Expected key to have PermissionThreadRead")
	}

	// Check permission that shouldn't exist
	if service.HasPermission(apiKey, api.PermissionAgentWrite) {
		t.Error("Key should not have PermissionAgentWrite")
	}

	// Create a key with admin role
	adminRoles := []string{string(api.RoleAdmin)}

	adminOpts := &APIKeyOptions{
		Name:      "Admin Key",
		OwnerID:   "admin101",
		ExpiresIn: 24 * time.Hour,
		Roles:     adminRoles,
	}

	adminKey, _, err := service.GenerateAPIKey(adminOpts)
	if err != nil {
		t.Fatalf("Failed to generate admin API key: %v", err)
	}

	// Admin should have all permissions
	if !service.HasPermission(adminKey, api.PermissionAgentWrite) {
		t.Error("Admin key should have all permissions")
	}
}

func TestAPIKeyScopeChecks(t *testing.T) {
	// Create a repository
	repo := NewInMemoryAPIKeyRepository()
	service := NewAPIKeyService(repo)

	// Create an API key with specific scopes
	opts := &APIKeyOptions{
		Name:      "Scope Test Key",
		OwnerID:   "user303",
		ExpiresIn: 24 * time.Hour,
		Scopes:    []string{"api:read", "user:profile"},
	}

	// Generate the API key
	apiKey, _, err := service.GenerateAPIKey(opts)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	// Check scopes that should exist
	if !service.HasScope(apiKey, "api:read") {
		t.Error("Expected key to have api:read scope")
	}

	if !service.HasScope(apiKey, "user:profile") {
		t.Error("Expected key to have user:profile scope")
	}

	// Check scope that shouldn't exist
	if service.HasScope(apiKey, "api:write") {
		t.Error("Key should not have api:write scope")
	}

	// Create a key with wildcard scope
	wildcardOpts := &APIKeyOptions{
		Name:      "Wildcard Key",
		OwnerID:   "user404",
		ExpiresIn: 24 * time.Hour,
		Scopes:    []string{"*"},
	}

	wildcardKey, _, err := service.GenerateAPIKey(wildcardOpts)
	if err != nil {
		t.Fatalf("Failed to generate wildcard API key: %v", err)
	}

	// Wildcard should match any scope
	if !service.HasScope(wildcardKey, "any:arbitrary:scope") {
		t.Error("Wildcard key should have all scopes")
	}
}
