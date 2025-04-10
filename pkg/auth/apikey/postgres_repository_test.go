package apikey

import (
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// setupPostgresTest sets up a test database connection
// Note: This requires a running PostgreSQL instance with the appropriate configuration
// Set the POSTGRES_TEST_DSN environment variable to run these tests
func setupPostgresTest(t *testing.T) (*sql.DB, *PostgresAPIKeyRepository, func()) {
	t.Helper()

	// Skip if no DSN is provided
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("Skipping PostgreSQL tests: POSTGRES_TEST_DSN not set")
	}

	// Connect to database
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Create and initialize repository
	repo, err := NewPostgresAPIKeyRepository(db)
	if err != nil {
		db.Close()
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Create cleanup function
	cleanup := func() {
		// Drop test table
		_, err := db.Exec("DROP TABLE IF EXISTS apikeys")
		if err != nil {
			t.Logf("Failed to drop test table: %v", err)
		}
		db.Close()
	}

	return db, repo, cleanup
}

func TestPostgresAPIKeyRepository_CRUD(t *testing.T) {
	_, repo, cleanup := setupPostgresTest(t)
	defer cleanup()

	// Create a test API key
	key := &APIKey{
		ID:          "test-id-1",
		Prefix:      "test-prefix-1",
		Secret:      "test-secret-1",
		Name:        "Test Key 1",
		Description: "Test key for PostgreSQL repository tests",
		OwnerID:     "test-owner-1",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		Scopes:      []string{"api:read", "api:write"},
		Roles:       []string{"user"},
		Permissions: []string{"api:read", "api:write"},
	}

	// Test CreateAPIKey
	err := repo.CreateAPIKey(key)
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Test FindAPIKeyByID
	foundKey, err := repo.FindAPIKeyByID(key.ID)
	if err != nil {
		t.Fatalf("Failed to find API key by ID: %v", err)
	}

	if foundKey.ID != key.ID || foundKey.Name != key.Name {
		t.Errorf("Found key doesn't match original. Got ID=%s, Name=%s, want ID=%s, Name=%s",
			foundKey.ID, foundKey.Name, key.ID, key.Name)
	}

	// Test FindAPIKeyByPrefix
	foundByPrefix, err := repo.FindAPIKeyByPrefix(key.Prefix)
	if err != nil {
		t.Fatalf("Failed to find API key by prefix: %v", err)
	}

	if foundByPrefix.ID != key.ID {
		t.Errorf("Key found by prefix doesn't match. Got ID=%s, want ID=%s",
			foundByPrefix.ID, key.ID)
	}

	// Test UpdateAPIKey
	key.Name = "Updated Test Key"
	key.Description = "Updated description"
	key.LastUsedAt = time.Now()

	err = repo.UpdateAPIKey(key)
	if err != nil {
		t.Fatalf("Failed to update API key: %v", err)
	}

	// Verify update worked
	updatedKey, err := repo.FindAPIKeyByID(key.ID)
	if err != nil {
		t.Fatalf("Failed to find updated API key: %v", err)
	}

	if updatedKey.Name != "Updated Test Key" || updatedKey.Description != "Updated description" {
		t.Errorf("Update didn't apply correctly. Got Name=%s, Description=%s",
			updatedKey.Name, updatedKey.Description)
	}

	// Create a second key for the same owner
	key2 := &APIKey{
		ID:          "test-id-2",
		Prefix:      "test-prefix-2",
		Secret:      "test-secret-2",
		Name:        "Test Key 2",
		Description: "Second test key",
		OwnerID:     "test-owner-1", // Same owner
		CreatedAt:   time.Now(),
		Scopes:      []string{"api:read"},
	}

	err = repo.CreateAPIKey(key2)
	if err != nil {
		t.Fatalf("Failed to create second API key: %v", err)
	}

	// Test FindAPIKeysByOwnerID
	ownerKeys, err := repo.FindAPIKeysByOwnerID("test-owner-1")
	if err != nil {
		t.Fatalf("Failed to find API keys by owner: %v", err)
	}

	if len(ownerKeys) != 2 {
		t.Errorf("Expected to find 2 keys for owner, got %d", len(ownerKeys))
	}

	// Test DeleteAPIKey
	err = repo.DeleteAPIKey(key.ID)
	if err != nil {
		t.Fatalf("Failed to delete API key: %v", err)
	}

	// Verify deletion
	_, err = repo.FindAPIKeyByID(key.ID)
	if err != ErrAPIKeyNotFound {
		t.Errorf("Expected ErrAPIKeyNotFound after deletion, got %v", err)
	}

	// Should still find the second key
	remainingKeys, err := repo.FindAPIKeysByOwnerID("test-owner-1")
	if err != nil {
		t.Fatalf("Failed to find remaining keys: %v", err)
	}

	if len(remainingKeys) != 1 || remainingKeys[0].ID != key2.ID {
		t.Errorf("Expected to find only key2 remaining, got %d keys", len(remainingKeys))
	}
}

func TestPostgresAPIKeyRepository_ExpirationAndRevocation(t *testing.T) {
	_, repo, cleanup := setupPostgresTest(t)
	defer cleanup()

	// Create an expired key
	expiredKey := &APIKey{
		ID:        "expired-key",
		Prefix:    "expired-prefix",
		Secret:    "expired-secret",
		Name:      "Expired Key",
		OwnerID:   "test-owner",
		CreatedAt: time.Now().Add(-48 * time.Hour),
		ExpiresAt: time.Now().Add(-24 * time.Hour), // Expired 24 hours ago
	}

	err := repo.CreateAPIKey(expiredKey)
	if err != nil {
		t.Fatalf("Failed to create expired key: %v", err)
	}

	// Create a valid key
	validKey := &APIKey{
		ID:        "valid-key",
		Prefix:    "valid-prefix",
		Secret:    "valid-secret",
		Name:      "Valid Key",
		OwnerID:   "test-owner",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour), // Valid for 24 hours
	}

	err = repo.CreateAPIKey(validKey)
	if err != nil {
		t.Fatalf("Failed to create valid key: %v", err)
	}

	// Test CleanExpiredKeys
	cleaned := repo.CleanExpiredKeys()
	if cleaned != 1 {
		t.Errorf("Expected to clean 1 expired key, cleaned %d", cleaned)
	}

	// Verify expired key is gone
	_, err = repo.FindAPIKeyByID(expiredKey.ID)
	if err != ErrAPIKeyNotFound {
		t.Errorf("Expected expired key to be deleted, got %v", err)
	}

	// Verify valid key still exists
	stillValid, err := repo.FindAPIKeyByID(validKey.ID)
	if err != nil {
		t.Fatalf("Failed to find valid key: %v", err)
	}

	if stillValid.ID != validKey.ID {
		t.Errorf("Expected to find valid key, got different key")
	}

	// Test key revocation
	validKey.IsRevoked = true
	validKey.RevokedAt = time.Now()

	err = repo.UpdateAPIKey(validKey)
	if err != nil {
		t.Fatalf("Failed to revoke key: %v", err)
	}

	// Verify revocation was saved
	revokedKey, err := repo.FindAPIKeyByID(validKey.ID)
	if err != nil {
		t.Fatalf("Failed to find revoked key: %v", err)
	}

	if !revokedKey.IsRevoked {
		t.Error("Expected key to be revoked, but IsRevoked is false")
	}

	if revokedKey.RevokedAt.IsZero() {
		t.Error("Expected RevokedAt to be set, but it's zero")
	}
}
