package api

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryUserRepository implements the UserRepository interface
// storing users in memory
type InMemoryUserRepository struct {
	users      map[string]*User  // Map of ID to user
	byUsername map[string]string // Map of username to ID
	mu         sync.RWMutex
}

// NewInMemoryUserRepository creates a new in-memory user repository
func NewInMemoryUserRepository() *InMemoryUserRepository {
	return &InMemoryUserRepository{
		users:      make(map[string]*User),
		byUsername: make(map[string]string),
	}
}

// FindByID finds a user by ID
func (r *InMemoryUserRepository) FindByID(id string) (*User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[id]
	if !exists {
		return nil, errors.New("user not found")
	}

	// Return a copy of the user to prevent modification of the stored user
	return cloneUser(user), nil
}

// FindByUsername finds a user by username
func (r *InMemoryUserRepository) FindByUsername(username string) (*User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, exists := r.byUsername[username]
	if !exists {
		return nil, errors.New("user not found")
	}

	user, exists := r.users[id]
	if !exists {
		// This shouldn't happen if the maps are properly synchronized
		return nil, errors.New("user not found")
	}

	// Return a copy of the user to prevent modification of the stored user
	return cloneUser(user), nil
}

// Create creates a new user
func (r *InMemoryUserRepository) Create(user *User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if username already exists
	if _, exists := r.byUsername[user.Username]; exists {
		return errors.New("username already exists")
	}

	// Generate new ID if not provided
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	// Set created and updated timestamps
	now := time.Now()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now

	// Store the user
	r.users[user.ID] = cloneUser(user)
	r.byUsername[user.Username] = user.ID

	return nil
}

// Update updates an existing user
func (r *InMemoryUserRepository) Update(user *User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if the user exists
	existingUser, exists := r.users[user.ID]
	if !exists {
		return errors.New("user not found")
	}

	// Check if username is being changed and if the new username is available
	if user.Username != existingUser.Username {
		if _, taken := r.byUsername[user.Username]; taken {
			return errors.New("username already exists")
		}

		// Remove old username mapping
		delete(r.byUsername, existingUser.Username)

		// Add new username mapping
		r.byUsername[user.Username] = user.ID
	}

	// Update timestamps
	user.CreatedAt = existingUser.CreatedAt
	user.UpdatedAt = time.Now()

	// Update the user
	r.users[user.ID] = cloneUser(user)

	return nil
}

// Delete deletes a user by ID
func (r *InMemoryUserRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if the user exists
	user, exists := r.users[id]
	if !exists {
		return errors.New("user not found")
	}

	// Remove the user
	delete(r.users, id)
	delete(r.byUsername, user.Username)

	return nil
}

// List returns all users
func (r *InMemoryUserRepository) List() ([]*User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a slice of users
	users := make([]*User, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, cloneUser(user))
	}

	return users, nil
}

// Helper functions

// cloneUser creates a deep copy of a user
func cloneUser(u *User) *User {
	if u == nil {
		return nil
	}

	// Create a new user
	clone := &User{
		ID:           u.ID,
		Username:     u.Username,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}

	// Clone roles
	if u.Roles != nil {
		clone.Roles = make([]Role, len(u.Roles))
		copy(clone.Roles, u.Roles)
	}

	// Clone permissions
	if u.Permissions != nil {
		clone.Permissions = make([]Permission, len(u.Permissions))
		copy(clone.Permissions, u.Permissions)
	}

	return clone
}
