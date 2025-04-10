package apikey

// APIKeyRepository defines the interface for storing and retrieving API keys
type APIKeyRepository interface {
	// CreateAPIKey stores a new API key
	CreateAPIKey(key *APIKey) error

	// FindAPIKeyByID retrieves an API key by its ID
	FindAPIKeyByID(id string) (*APIKey, error)

	// FindAPIKeyByPrefix retrieves an API key by its prefix
	FindAPIKeyByPrefix(prefix string) (*APIKey, error)

	// FindAPIKeysByOwnerID retrieves all API keys for a specific owner
	FindAPIKeysByOwnerID(ownerID string) ([]*APIKey, error)

	// UpdateAPIKey updates an existing API key
	UpdateAPIKey(key *APIKey) error

	// DeleteAPIKey removes an API key
	DeleteAPIKey(id string) error

	// CleanExpiredKeys removes all expired API keys and returns count of removed keys
	CleanExpiredKeys() int
}
