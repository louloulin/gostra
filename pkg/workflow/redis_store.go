package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisStateStore implements WorkflowStateStore interface for Redis
type RedisStateStore struct {
	client     *redis.Client
	keyPrefix  string
	expiration time.Duration
}

// RedisStateStoreOptions contains configuration for Redis state storage
type RedisStateStoreOptions struct {
	// Address for the Redis server (host:port)
	Address string

	// Password for Redis authentication (optional)
	Password string

	// Database to select after connecting to Redis
	Database int

	// KeyPrefix for Redis keys to avoid collisions (default: "workflow:")
	KeyPrefix string

	// Expiration is the default expiration time for workflow states (default: 0 - no expiration)
	Expiration time.Duration
}

// NewRedisStateStore creates a new Redis workflow state store
func NewRedisStateStore(ctx context.Context, opts RedisStateStoreOptions) (*RedisStateStore, error) {
	if opts.Address == "" {
		return nil, errors.New("Redis address is required")
	}

	if opts.KeyPrefix == "" {
		opts.KeyPrefix = "workflow:"
	}

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     opts.Address,
		Password: opts.Password,
		DB:       opts.Database,
	})

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStateStore{
		client:     client,
		keyPrefix:  opts.KeyPrefix,
		expiration: opts.Expiration,
	}, nil
}

// formatKey creates a Redis key with the configured prefix
func (r *RedisStateStore) formatKey(workflowID string) string {
	return r.keyPrefix + workflowID
}

// SaveWorkflowState saves a workflow state to Redis
func (r *RedisStateStore) SaveWorkflowState(state *WorkflowState) error {
	if state == nil {
		return errors.New("state cannot be nil")
	}

	if state.WorkflowID == "" {
		return errors.New("workflow ID is required")
	}

	// Update last updated timestamp
	state.LastUpdated = time.Now()

	// Serialize state to JSON
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow state: %w", err)
	}

	// Save to Redis
	ctx := context.Background()
	key := r.formatKey(state.WorkflowID)
	if err := r.client.Set(ctx, key, data, r.expiration).Err(); err != nil {
		return fmt.Errorf("failed to save workflow state to Redis: %w", err)
	}

	return nil
}

// LoadWorkflowState loads a workflow state from Redis
func (r *RedisStateStore) LoadWorkflowState(workflowID string) (*WorkflowState, error) {
	if workflowID == "" {
		return nil, errors.New("workflow ID is required")
	}

	// Get from Redis
	ctx := context.Background()
	key := r.formatKey(workflowID)
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("workflow state not found for ID: %s", workflowID)
		}
		return nil, fmt.Errorf("failed to load workflow state from Redis: %w", err)
	}

	// Deserialize from JSON
	var state WorkflowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow state: %w", err)
	}

	return &state, nil
}

// ListWorkflowStates lists all workflow states from Redis
func (r *RedisStateStore) ListWorkflowStates() ([]*WorkflowState, error) {
	ctx := context.Background()

	// Get all keys with the workflow prefix
	pattern := r.keyPrefix + "*"
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow keys from Redis: %w", err)
	}

	if len(keys) == 0 {
		return []*WorkflowState{}, nil
	}

	// Use pipeline to get all values efficiently
	pipe := r.client.Pipeline()
	cmds := make(map[string]*redis.StringCmd, len(keys))

	for _, key := range keys {
		cmds[key] = pipe.Get(ctx, key)
	}

	_, err = pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to execute pipeline: %w", err)
	}

	// Process results
	states := make([]*WorkflowState, 0, len(keys))
	for _, cmd := range cmds {
		data, err := cmd.Bytes()
		if err != nil {
			if err == redis.Nil {
				continue // Key was deleted between KEYS and GET
			}
			return nil, fmt.Errorf("failed to get workflow state data: %w", err)
		}

		var state WorkflowState
		if err := json.Unmarshal(data, &state); err != nil {
			continue // Skip invalid states
		}

		states = append(states, &state)
	}

	return states, nil
}

// DeleteWorkflowState deletes a workflow state from Redis
func (r *RedisStateStore) DeleteWorkflowState(workflowID string) error {
	if workflowID == "" {
		return errors.New("workflow ID is required")
	}

	ctx := context.Background()
	key := r.formatKey(workflowID)

	// Check if the key exists first
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to check if workflow state exists: %w", err)
	}

	if exists == 0 {
		return fmt.Errorf("workflow state not found for ID: %s", workflowID)
	}

	// Delete the key
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete workflow state: %w", err)
	}

	return nil
}

// Close closes the Redis connection
func (r *RedisStateStore) Close() error {
	return r.client.Close()
}
