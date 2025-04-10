package apikey

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/louloulin/gostra/pkg/api"
)

const (
	// DefaultSecretLength is the default length for API key secrets
	DefaultSecretLength = 32

	// DefaultPrefixLength is the default length for API key prefixes
	DefaultPrefixLength = 8

	// DefaultKeySeparator is the character used to separate prefix and secret
	DefaultKeySeparator = "."
)

var (
	// ErrAPIKeyNotFound is returned when an API key is not found
	ErrAPIKeyNotFound = errors.New("api key not found")

	// ErrAPIKeyRevoked is returned when an API key has been revoked
	ErrAPIKeyRevoked = errors.New("api key has been revoked")

	// ErrAPIKeyExpired is returned when an API key has expired
	ErrAPIKeyExpired = errors.New("api key has expired")

	// ErrAPIKeyInvalid is returned when an API key is invalid
	ErrAPIKeyInvalid = errors.New("invalid api key")

	// ErrMissingScopePermission is returned when an API key lacks a required scope
	ErrMissingScopePermission = errors.New("api key missing required scope or permission")
)

// APIKey represents an API key with its metadata
type APIKey struct {
	// ID is the unique identifier for the API key
	ID string `json:"id"`

	// Secret is the secret part of the API key (never stored in plain text)
	Secret string `json:"-"`

	// Prefix is the public prefix of the API key
	Prefix string `json:"prefix"`

	// Name is a human-readable name for the API key
	Name string `json:"name"`

	// Description provides details about the API key's purpose
	Description string `json:"description,omitempty"`

	// OwnerID identifies the user who owns this API key
	OwnerID string `json:"owner_id"`

	// CreatedAt is when the API key was created
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when the API key will expire (zero time means no expiration)
	ExpiresAt time.Time `json:"expires_at,omitempty"`

	// LastUsedAt tracks the last time the API key was used
	LastUsedAt time.Time `json:"last_used_at,omitempty"`

	// IsRevoked indicates if the API key has been revoked
	IsRevoked bool `json:"is_revoked"`

	// RevokedAt is when the API key was revoked (if applicable)
	RevokedAt time.Time `json:"revoked_at,omitempty"`

	// Scopes are the allowed scopes for this API key
	Scopes []string `json:"scopes,omitempty"`

	// Roles are the roles assigned to this API key
	Roles []string `json:"roles,omitempty"`

	// Permissions are the specific permissions granted to this API key
	Permissions []string `json:"permissions,omitempty"`
}

// APIKeyOptions defines options for creating a new API key
type APIKeyOptions struct {
	// Name is a human-readable name for the API key
	Name string

	// Description provides details about the API key's purpose
	Description string

	// OwnerID identifies the user who owns this API key
	OwnerID string

	// ExpiresIn sets the duration until the API key expires
	ExpiresIn time.Duration

	// Scopes are the allowed scopes for this API key
	Scopes []string

	// Roles are the roles assigned to this API key
	Roles []string

	// Permissions are the specific permissions granted to this API key
	Permissions []string

	// SecretLength overrides the default secret length
	SecretLength int

	// PrefixLength overrides the default prefix length
	PrefixLength int
}

// APIKeyService provides methods for API key management
type APIKeyService struct {
	repository APIKeyRepository
}

// NewAPIKeyService creates a new API key service
func NewAPIKeyService(repository APIKeyRepository) *APIKeyService {
	return &APIKeyService{
		repository: repository,
	}
}

// GenerateAPIKey creates a new API key
func (s *APIKeyService) GenerateAPIKey(opts *APIKeyOptions) (*APIKey, string, error) {
	secretLen := DefaultSecretLength
	if opts.SecretLength > 0 {
		secretLen = opts.SecretLength
	}

	prefixLen := DefaultPrefixLength
	if opts.PrefixLength > 0 {
		prefixLen = opts.PrefixLength
	}

	// Generate random bytes for secret
	secretBytes := make([]byte, secretLen)
	_, err := rand.Read(secretBytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	secret := base64.URLEncoding.EncodeToString(secretBytes)[:secretLen]

	// Generate prefix
	prefixBytes := make([]byte, prefixLen)
	_, err = rand.Read(prefixBytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate random bytes for prefix: %w", err)
	}
	prefix := base64.URLEncoding.EncodeToString(prefixBytes)[:prefixLen]

	// Create API key
	apiKey := &APIKey{
		ID:          uuid.New().String(),
		Secret:      secret,
		Prefix:      prefix,
		Name:        opts.Name,
		Description: opts.Description,
		OwnerID:     opts.OwnerID,
		CreatedAt:   time.Now(),
		Scopes:      opts.Scopes,
		Roles:       opts.Roles,
		Permissions: opts.Permissions,
	}

	// Set expiration if provided
	if opts.ExpiresIn > 0 {
		apiKey.ExpiresAt = time.Now().Add(opts.ExpiresIn)
	}

	// Combine prefix and secret for the full API key
	fullAPIKey := fmt.Sprintf("%s%s%s", prefix, DefaultKeySeparator, secret)

	// Store in repository
	err = s.repository.CreateAPIKey(apiKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to store API key: %w", err)
	}

	return apiKey, fullAPIKey, nil
}

// ValidateAPIKey validates an API key and returns the associated API key
func (s *APIKeyService) ValidateAPIKey(apiKeyString string) (*APIKey, error) {
	parts := strings.SplitN(apiKeyString, DefaultKeySeparator, 2)
	if len(parts) != 2 {
		return nil, ErrAPIKeyInvalid
	}

	prefix := parts[0]
	secret := parts[1]

	// Find API key by prefix
	apiKey, err := s.repository.FindAPIKeyByPrefix(prefix)
	if err != nil {
		return nil, ErrAPIKeyNotFound
	}

	// Check if API key is revoked
	if apiKey.IsRevoked {
		return nil, ErrAPIKeyRevoked
	}

	// Check if API key is expired
	if !apiKey.ExpiresAt.IsZero() && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, ErrAPIKeyExpired
	}

	// Validate secret
	if apiKey.Secret != secret {
		return nil, ErrAPIKeyInvalid
	}

	// Update last used timestamp
	apiKey.LastUsedAt = time.Now()
	err = s.repository.UpdateAPIKey(apiKey)
	if err != nil {
		// Non-critical error, just log it
		fmt.Printf("Failed to update last used timestamp: %v\n", err)
	}

	return apiKey, nil
}

// RevokeAPIKey revokes an API key
func (s *APIKeyService) RevokeAPIKey(id string) error {
	apiKey, err := s.repository.FindAPIKeyByID(id)
	if err != nil {
		return ErrAPIKeyNotFound
	}

	apiKey.IsRevoked = true
	apiKey.RevokedAt = time.Now()

	return s.repository.UpdateAPIKey(apiKey)
}

// DeleteAPIKey permanently deletes an API key
func (s *APIKeyService) DeleteAPIKey(id string) error {
	return s.repository.DeleteAPIKey(id)
}

// GetAPIKey retrieves an API key by ID
func (s *APIKeyService) GetAPIKey(id string) (*APIKey, error) {
	return s.repository.FindAPIKeyByID(id)
}

// ListAPIKeys retrieves all API keys for a user
func (s *APIKeyService) ListAPIKeys(ownerID string) ([]*APIKey, error) {
	return s.repository.FindAPIKeysByOwnerID(ownerID)
}

// HasScope checks if an API key has a required scope
func (s *APIKeyService) HasScope(apiKey *APIKey, requiredScope string) bool {
	for _, scope := range apiKey.Scopes {
		if scope == "*" || scope == requiredScope {
			return true
		}
	}
	return false
}

// HasPermission checks if an API key has a required permission
func (s *APIKeyService) HasPermission(apiKey *APIKey, requiredPermission api.Permission) bool {
	permStr := string(requiredPermission)
	for _, permission := range apiKey.Permissions {
		if permission == "*" || permission == permStr {
			return true
		}
	}
	return false
}

// CleanExpiredKeys removes expired API keys
func (s *APIKeyService) CleanExpiredKeys() int {
	return s.repository.CleanExpiredKeys()
}
