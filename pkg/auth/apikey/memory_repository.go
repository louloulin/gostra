package apikey

import (
	"errors"
	"sync"
	"time"
)

// MemoryAPIKeyRepository is an in-memory implementation of APIKeyRepository
type MemoryAPIKeyRepository struct {
	keys map[string]*APIKey // map of ID to APIKey
	mu   sync.RWMutex
}

// NewInMemoryAPIKeyRepository creates a new in-memory API key repository
func NewInMemoryAPIKeyRepository() *MemoryAPIKeyRepository {
	return &MemoryAPIKeyRepository{
		keys: make(map[string]*APIKey),
	}
}

// CreateAPIKey stores a new API key
func (r *MemoryAPIKeyRepository) CreateAPIKey(key *APIKey) error {
	if key == nil {
		return errors.New("api key cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.keys[key.ID]; exists {
		return errors.New("api key with this ID already exists")
	}

	// Store a copy to prevent external modifications
	keyCopy := *key
	r.keys[key.ID] = &keyCopy

	return nil
}

// FindAPIKeyByID retrieves an API key by its ID
func (r *MemoryAPIKeyRepository) FindAPIKeyByID(id string) (*APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key, exists := r.keys[id]
	if !exists {
		return nil, ErrAPIKeyNotFound
	}

	// Return a copy to prevent external modifications
	keyCopy := *key
	return &keyCopy, nil
}

// FindAPIKeyByPrefix retrieves an API key by its prefix
func (r *MemoryAPIKeyRepository) FindAPIKeyByPrefix(prefix string) (*APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, key := range r.keys {
		if key.Prefix == prefix {
			// Return a copy to prevent external modifications
			keyCopy := *key
			return &keyCopy, nil
		}
	}

	return nil, ErrAPIKeyNotFound
}

// FindAPIKeysByOwnerID retrieves all API keys for a specific owner
func (r *MemoryAPIKeyRepository) FindAPIKeysByOwnerID(ownerID string) ([]*APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*APIKey

	for _, key := range r.keys {
		if key.OwnerID == ownerID {
			// Add a copy to prevent external modifications
			keyCopy := *key
			result = append(result, &keyCopy)
		}
	}

	return result, nil
}

// UpdateAPIKey updates an existing API key
func (r *MemoryAPIKeyRepository) UpdateAPIKey(key *APIKey) error {
	if key == nil {
		return errors.New("api key cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.keys[key.ID]; !exists {
		return ErrAPIKeyNotFound
	}

	// Store a copy to prevent external modifications
	keyCopy := *key
	r.keys[key.ID] = &keyCopy

	return nil
}

// DeleteAPIKey removes an API key
func (r *MemoryAPIKeyRepository) DeleteAPIKey(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.keys[id]; !exists {
		return ErrAPIKeyNotFound
	}

	delete(r.keys, id)

	return nil
}

// CleanExpiredKeys removes all expired API keys and returns count of removed keys
func (r *MemoryAPIKeyRepository) CleanExpiredKeys() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	count := 0

	for id, key := range r.keys {
		if !key.ExpiresAt.IsZero() && now.After(key.ExpiresAt) {
			delete(r.keys, id)
			count++
		}
	}

	return count
}
