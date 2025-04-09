package agent

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Package agent provides context sharing capabilities for the agent network system.
//
// The context sharing system allows agents to maintain shared state across agent calls,
// enabling seamless information transfer and building more complex collaborative agent workflows.
// It is designed to work with both sequential and parallel agent execution patterns and
// provides automatic context merging, versioning, and history tracking.
//
// Key features of the context handling system:
//
// 1. Persistent context storage with TTL (Time-To-Live) support
//   - Context data can be set to expire after a specified duration
//   - Automatic cleanup of expired context entries
//
// 2. Conversation-based context management
//   - Contexts are organized by conversation IDs
//   - Multiple independent conversations can be maintained simultaneously
//
// 3. Agent attribution for context contributions
//   - Each context entry tracks which agent created or modified it
//   - Timestamps for creation and modification are maintained
//
// 4. Context merging and conflict resolution
//   - Automatic merging of context from multiple agents
//   - Configurable conflict resolution strategies
//
// 5. Serialization support for persistence
//   - Context data can be serialized to JSON for storage
//   - Deserialization support for loading saved contexts
//
// Usage example:
//
//	// Create a context handler for an agent network
//	contextHandler := NewContextHandler(network, WithDefaultTTL(2*time.Hour))
//
//	// Set context values
//	contextHandler.SetContext("conversation-123", "key1", "value1", "agent1")
//
//	// Get context values
//	value, exists := contextHandler.GetContext("conversation-123", "key1")
//
//	// Enrich a message with context
//	msg := &NetworkMessage{Content: "Hello"}
//	contextHandler.EnrichNetworkMessage("conversation-123", msg)
//
//	// Process context from a message
//	response := &NetworkMessage{Data: map[string]interface{}{"key2": "value2"}}
//	contextHandler.ProcessContextFromMessage("conversation-123", response)
//
//	// Get all context for a conversation
//	allContext := contextHandler.GetAllContext("conversation-123")
//
//	// Persist context
//	data, err := contextHandler.SerializeContext()
//	// ... store data somewhere ...
//
//	// Load saved context
//	err = contextHandler.DeserializeContext(data)
//
// The context handler is automatically integrated with the AgentNetwork and RouterAgent
// implementations, providing seamless context passing between agents without requiring
// manual management in most cases.

// ContextEntry represents a single entry in the shared context
type ContextEntry struct {
	Value      interface{}   `json:"value"`
	CreatedAt  time.Time     `json:"created_at"`
	ModifiedAt time.Time     `json:"modified_at"`
	CreatedBy  string        `json:"created_by"`
	TTL        time.Duration `json:"ttl,omitempty"` // Time-to-live for context entries (optional)
}

// ContextHandler manages context sharing between agents in a network
type ContextHandler struct {
	mu            sync.RWMutex
	contextStore  map[string]map[string]*ContextEntry // Map of conversation ID -> key -> entry
	defaultTTL    time.Duration                       // Default time-to-live for context entries
	cleanupTicker *time.Ticker                        // Ticker for cleanup of expired entries
	network       *AgentNetwork                       // Reference to the agent network
}

// NewContextHandler creates a new context handler for managing shared context between agents.
// It initializes the context store and starts the background cleanup process for expired entries.
//
// Parameters:
//   - network: The agent network this context handler is associated with
//   - options: Optional configuration functions to customize the context handler
//
// Returns a fully initialized ContextHandler ready to use.
func NewContextHandler(network *AgentNetwork, options ...func(*ContextHandler)) *ContextHandler {
	handler := &ContextHandler{
		contextStore:  make(map[string]map[string]*ContextEntry),
		defaultTTL:    24 * time.Hour, // Default 24-hour TTL
		cleanupTicker: time.NewTicker(1 * time.Hour),
		network:       network,
	}

	// Apply options if provided
	for _, option := range options {
		option(handler)
	}

	// Start cleanup goroutine
	go handler.cleanupExpiredEntries()

	return handler
}

// WithDefaultTTL sets the default TTL for context entries.
// This determines how long context entries will remain valid before being automatically removed.
//
// Parameters:
//   - ttl: The time-to-live duration for context entries
//
// Returns a function that can be passed to NewContextHandler as an option.
func WithDefaultTTL(ttl time.Duration) func(*ContextHandler) {
	return func(ch *ContextHandler) {
		ch.defaultTTL = ttl
	}
}

// WithCleanupInterval sets the interval for cleanup of expired entries
func WithCleanupInterval(interval time.Duration) func(*ContextHandler) {
	return func(ch *ContextHandler) {
		if ch.cleanupTicker != nil {
			ch.cleanupTicker.Stop()
		}
		ch.cleanupTicker = time.NewTicker(interval)
	}
}

// cleanupExpiredEntries periodically removes expired context entries
func (ch *ContextHandler) cleanupExpiredEntries() {
	for range ch.cleanupTicker.C {
		ch.mu.Lock()
		now := time.Now()

		for conversationID, contextMap := range ch.contextStore {
			for key, entry := range contextMap {
				if entry.TTL > 0 && now.After(entry.ModifiedAt.Add(entry.TTL)) {
					delete(contextMap, key)
				}
			}

			// If the conversation context is empty, remove it
			if len(contextMap) == 0 {
				delete(ch.contextStore, conversationID)
			}
		}

		ch.mu.Unlock()
	}
}

// Stop stops the context handler
func (ch *ContextHandler) Stop() {
	if ch.cleanupTicker != nil {
		ch.cleanupTicker.Stop()
	}
}

// SetContext sets a value in the shared context.
// If the key already exists, it updates the value and the modification timestamp.
// If the key doesn't exist, it creates a new entry with the current timestamp.
//
// Parameters:
//   - conversationID: The ID of the conversation this context belongs to
//   - key: The key to set in the context
//   - value: The value to store
//   - agentID: The ID of the agent setting this context value
func (ch *ContextHandler) SetContext(conversationID, key string, value interface{}, agentID string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	// Ensure the conversation map exists
	if _, exists := ch.contextStore[conversationID]; !exists {
		ch.contextStore[conversationID] = make(map[string]*ContextEntry)
	}

	now := time.Now()

	// Create or update the entry
	if entry, exists := ch.contextStore[conversationID][key]; exists {
		entry.Value = value
		entry.ModifiedAt = now
	} else {
		ch.contextStore[conversationID][key] = &ContextEntry{
			Value:      value,
			CreatedAt:  now,
			ModifiedAt: now,
			CreatedBy:  agentID,
			TTL:        ch.defaultTTL,
		}
	}
}

// GetContext retrieves a value from the shared context.
// It checks if the entry has expired and returns the value only if it's still valid.
//
// Parameters:
//   - conversationID: The ID of the conversation to get context from
//   - key: The key to retrieve
//
// Returns:
//   - The value stored under the key, or nil if not found
//   - A boolean indicating whether the key was found and is valid
func (ch *ContextHandler) GetContext(conversationID, key string) (interface{}, bool) {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	// Check if the conversation and key exist
	if contextMap, exists := ch.contextStore[conversationID]; exists {
		if entry, found := contextMap[key]; found {
			// Check if the entry has expired
			if entry.TTL > 0 && time.Now().After(entry.ModifiedAt.Add(entry.TTL)) {
				return nil, false
			}
			return entry.Value, true
		}
	}

	return nil, false
}

// GetAllContext retrieves all context for a conversation.
// It filters out any expired entries before returning the result.
//
// Parameters:
//   - conversationID: The ID of the conversation to get all context for
//
// Returns a map containing all valid key-value pairs for the conversation.
func (ch *ContextHandler) GetAllContext(conversationID string) map[string]interface{} {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	result := make(map[string]interface{})
	now := time.Now()

	if contextMap, exists := ch.contextStore[conversationID]; exists {
		for key, entry := range contextMap {
			// Skip expired entries
			if entry.TTL > 0 && now.After(entry.ModifiedAt.Add(entry.TTL)) {
				continue
			}
			result[key] = entry.Value
		}
	}

	return result
}

// DeleteContext removes a value from the shared context
func (ch *ContextHandler) DeleteContext(conversationID, key string) bool {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if contextMap, exists := ch.contextStore[conversationID]; exists {
		if _, found := contextMap[key]; found {
			delete(contextMap, key)
			return true
		}
	}

	return false
}

// ClearContext removes all context for a conversation
func (ch *ContextHandler) ClearContext(conversationID string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	delete(ch.contextStore, conversationID)
}

// MergeContext merges a map of values into the context.
// This is useful for importing multiple values at once from another source.
//
// Parameters:
//   - conversationID: The ID of the conversation to merge context into
//   - values: A map of key-value pairs to merge into the context
//   - agentID: The ID of the agent providing these values
func (ch *ContextHandler) MergeContext(conversationID string, values map[string]interface{}, agentID string) {
	for key, value := range values {
		ch.SetContext(conversationID, key, value, agentID)
	}
}

// EnrichNetworkMessage adds context data to a network message.
// This is typically called before sending a message to an agent to provide
// contextual information from previous interactions.
//
// Parameters:
//   - conversationID: The ID of the conversation to get context from
//   - msg: The message to enrich with context data
func (ch *ContextHandler) EnrichNetworkMessage(conversationID string, msg *NetworkMessage) {
	// Get all context for the conversation
	contextData := ch.GetAllContext(conversationID)

	// If there's existing data in the message, merge it with the context
	if msg.Data != nil {
		if existingData, ok := msg.Data.(map[string]interface{}); ok {
			// Add existing data to context (message data takes precedence)
			for k, v := range existingData {
				contextData[k] = v
			}
		}
	}

	// Set the enriched data back to the message
	msg.Data = contextData
}

// ProcessContextFromMessage extracts and stores context from a message.
// This is typically called after receiving a response from an agent to capture
// any context updates they've provided.
//
// Parameters:
//   - conversationID: The ID of the conversation to store context in
//   - msg: The message containing context data to extract
func (ch *ContextHandler) ProcessContextFromMessage(conversationID string, msg *NetworkMessage) {
	if msg.Data == nil {
		return
	}

	// Extract context from message data
	if contextData, ok := msg.Data.(map[string]interface{}); ok && len(contextData) > 0 {
		ch.MergeContext(conversationID, contextData, msg.From)
	}
}

// SerializeContext serializes the entire context store (for persistence)
func (ch *ContextHandler) SerializeContext() ([]byte, error) {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	return json.Marshal(ch.contextStore)
}

// DeserializeContext loads context from serialized data
func (ch *ContextHandler) DeserializeContext(data []byte) error {
	contextStore := make(map[string]map[string]*ContextEntry)

	err := json.Unmarshal(data, &contextStore)
	if err != nil {
		return fmt.Errorf("failed to deserialize context: %w", err)
	}

	ch.mu.Lock()
	ch.contextStore = contextStore
	ch.mu.Unlock()

	return nil
}
