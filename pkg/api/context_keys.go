package api

// ContextKey is a type for context keys to avoid collisions
type ContextKey string

// Context keys
const (
	// ContextKeyUser is the key for user claims in the request context
	ContextKeyUser ContextKey = "user"
)
