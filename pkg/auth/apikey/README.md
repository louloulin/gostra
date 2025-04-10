# API Key Management for Gostra

The Gostra API Key Management system provides a complete solution for adding API key authentication to your Gostra-based applications. It includes:

- API key generation and management
- Multiple storage backends (in-memory and PostgreSQL)
- Middleware for HTTP request authentication
- Scope and permission-based authorization
- Automatic expiration handling

## Key Features

- **Full Lifecycle Management**: Generate, validate, revoke, and delete API keys
- **Flexible Storage Options**: Store API keys in memory for development or PostgreSQL for production
- **Authentication Middleware**: Easy integration with HTTP handlers
- **Scope and Permission Checking**: Fine-grained access control
- **Expiration and Revocation**: Built-in handling of key lifecycle events
- **Session Integration**: API keys are converted to user claims for compatibility with existing auth

## Usage

### Creating API Keys

```go
// Create a repository (in-memory or PostgreSQL)
repo := apikey.NewInMemoryAPIKeyRepository()
// OR
repo, err := apikey.NewPostgresAPIKeyRepository(db)

// Create the service
service := apikey.NewAPIKeyService(repo)

// Generate an API key
opts := &apikey.APIKeyOptions{
    Name:        "My API Key",
    OwnerID:     "user-123",
    ExpiresIn:   30 * 24 * time.Hour, // 30 days
    Scopes:      []string{"api:read", "api:write"},
    Roles:       []string{"user"},
    Permissions: []string{"api:read"},
}

apiKey, fullKey, err := service.GenerateAPIKey(opts)
if err != nil {
    // Handle error
}

// fullKey is the complete API key string to provide to the user
// apiKey is the internal representation with metadata
```

### Using the Middleware

```go
// Create middleware
middleware := apikey.NewAPIKeyMiddleware(&apikey.APIKeyMiddlewareOptions{
    APIKeyService: service,
    Extractors:    []apikey.APIKeyExtractor{apikey.DefaultAPIKeyExtractor},
    ExcludedPaths: []string{"/api/public", "/api/docs/*"},
    RequiredScopes: []string{"api:read"}, // Optional: require these scopes for all routes
})

// Apply to your handlers
handler := middleware(myHandler)

// Use with specific scope requirements
scopedHandler := middleware(apikey.RequireAPIKeyScope(service, "api:write")(myHandler))

// Use with specific permission requirements
permHandler := middleware(apikey.RequireAPIKeyPermission(service, api.PermissionAgentExecute)(myHandler))
```

### Accessing API Keys in Handlers

```go
func myHandler(w http.ResponseWriter, r *http.Request) {
    // Get API key from request context
    key, ok := apikey.GetAPIKeyFromContext(r.Context())
    if !ok {
        http.Error(w, "API key not found in context", http.StatusInternalServerError)
        return
    }

    // Access key properties
    fmt.Printf("API Key Name: %s\n", key.Name)
    fmt.Printf("Owner: %s\n", key.OwnerID)
    
    // Get user claims - converted from API key
    claims, ok := r.Context().Value(api.ContextKeyUser).(*api.Claims)
    if !ok {
        http.Error(w, "Claims not found", http.StatusInternalServerError)
        return
    }
    
    // Use claims
    fmt.Printf("User ID: %s\n", claims.UserID)
}
```

### Validating Keys

```go
// Validate a key string
apiKey, err := service.ValidateAPIKey("prefix.secret")
if err != nil {
    // Handle validation errors:
    // - ErrAPIKeyNotFound
    // - ErrAPIKeyRevoked
    // - ErrAPIKeyExpired
    // - ErrAPIKeyInvalid
}
```

### Revoking Keys

```go
// Revoke a key by ID
err := service.RevokeAPIKey("key-id")
if err != nil {
    // Handle error
}
```

### Cleaning Expired Keys

```go
// Clean expired keys and get count of keys removed
count := service.CleanExpiredKeys()
fmt.Printf("Cleaned %d expired keys\n", count)
```

## Storage Options

### In-Memory Repository

The `MemoryAPIKeyRepository` provides an in-memory storage solution that's perfect for development, testing, and simple applications. Data is lost when the application restarts.

```go
repo := apikey.NewInMemoryAPIKeyRepository()
service := apikey.NewAPIKeyService(repo)
```

### PostgreSQL Repository

The `PostgresAPIKeyRepository` provides a persistent storage solution using PostgreSQL. It automatically creates the necessary table if it doesn't exist.

```go
db, err := sql.Open("postgres", "postgresql://user:password@localhost/dbname")
if err != nil {
    log.Fatalf("Failed to connect to database: %v", err)
}

repo, err := apikey.NewPostgresAPIKeyRepository(db)
if err != nil {
    log.Fatalf("Failed to create repository: %v", err)
}

service := apikey.NewAPIKeyService(repo)
```

## Examples

See the example applications in the `examples` directory:

- `api_key_management`: Basic example with in-memory storage
- `api_key_with_postgres`: Advanced example with PostgreSQL storage

## Running the Examples

### In-Memory Example

```bash
cd examples/api_key_management
go run main.go
```

### PostgreSQL Example

```bash
cd examples/api_key_with_postgres
export POSTGRES_DSN="postgresql://user:password@localhost/dbname"
go run main.go
```

## Implementation Details

The API key format is `prefix.secret` where:
- `prefix` is a public identifier for the key (default 8 characters)
- `secret` is the secret part of the key (default 32 characters)

The system is designed with security best practices:
- API key secrets are never exposed after initial generation
- Keys can be scoped to specific operations
- Built-in expiration and revocation
- Permission-based access control
- Rate limiting can be added with additional middleware 