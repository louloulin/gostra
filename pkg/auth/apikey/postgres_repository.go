package apikey

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// PostgresAPIKeyRepository implements APIKeyRepository interface using PostgreSQL
type PostgresAPIKeyRepository struct {
	db *sql.DB
}

// NewPostgresAPIKeyRepository creates a new PostgreSQL-based API key repository
func NewPostgresAPIKeyRepository(db *sql.DB) (*PostgresAPIKeyRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	repo := &PostgresAPIKeyRepository{
		db: db,
	}

	// Create table if it doesn't exist
	err := repo.initializeTable()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize table: %w", err)
	}

	return repo, nil
}

// initializeTable creates the API keys table if it doesn't exist
func (r *PostgresAPIKeyRepository) initializeTable() error {
	// Create the apikeys table if it doesn't exist
	query := `
	CREATE TABLE IF NOT EXISTS apikeys (
		id VARCHAR(36) PRIMARY KEY,
		prefix VARCHAR(16) UNIQUE NOT NULL,
		secret VARCHAR(128) NOT NULL,
		name VARCHAR(255) NOT NULL,
		description TEXT,
		owner_id VARCHAR(255) NOT NULL,
		created_at TIMESTAMP NOT NULL,
		expires_at TIMESTAMP NULL,
		last_used_at TIMESTAMP NULL,
		is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
		revoked_at TIMESTAMP NULL,
		scopes TEXT[],
		roles TEXT[],
		permissions TEXT[]
	);
	CREATE INDEX IF NOT EXISTS idx_apikeys_prefix ON apikeys(prefix);
	CREATE INDEX IF NOT EXISTS idx_apikeys_owner_id ON apikeys(owner_id);
	`

	_, err := r.db.Exec(query)
	return err
}

// CreateAPIKey stores a new API key
func (r *PostgresAPIKeyRepository) CreateAPIKey(key *APIKey) error {
	if key == nil {
		return fmt.Errorf("api key cannot be nil")
	}

	// Check if key with same ID already exists
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM apikeys WHERE id = $1", key.ID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check if key exists: %w", err)
	}

	if count > 0 {
		return fmt.Errorf("api key with this ID already exists")
	}

	// Insert the new API key
	query := `
	INSERT INTO apikeys (
		id, prefix, secret, name, description, owner_id, 
		created_at, expires_at, last_used_at, is_revoked, revoked_at,
		scopes, roles, permissions
	) VALUES (
		$1, $2, $3, $4, $5, $6, 
		$7, $8, $9, $10, $11,
		$12, $13, $14
	)
	`

	_, err = r.db.Exec(
		query,
		key.ID, key.Prefix, key.Secret, key.Name, key.Description, key.OwnerID,
		key.CreatedAt, key.ExpiresAt, key.LastUsedAt, key.IsRevoked, key.RevokedAt,
		key.Scopes, key.Roles, key.Permissions,
	)

	if err != nil {
		return fmt.Errorf("failed to insert API key: %w", err)
	}

	return nil
}

// FindAPIKeyByID retrieves an API key by its ID
func (r *PostgresAPIKeyRepository) FindAPIKeyByID(id string) (*APIKey, error) {
	query := `
	SELECT 
		id, prefix, secret, name, description, owner_id, 
		created_at, expires_at, last_used_at, is_revoked, revoked_at,
		scopes, roles, permissions
	FROM apikeys
	WHERE id = $1
	`

	var key APIKey
	var expiresAt, lastUsedAt, revokedAt sql.NullTime

	err := r.db.QueryRow(query, id).Scan(
		&key.ID, &key.Prefix, &key.Secret, &key.Name, &key.Description, &key.OwnerID,
		&key.CreatedAt, &expiresAt, &lastUsedAt, &key.IsRevoked, &revokedAt,
		&key.Scopes, &key.Roles, &key.Permissions,
	)

	if err == sql.ErrNoRows {
		return nil, ErrAPIKeyNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find API key: %w", err)
	}

	// Handle nullable fields
	if expiresAt.Valid {
		key.ExpiresAt = expiresAt.Time
	}
	if lastUsedAt.Valid {
		key.LastUsedAt = lastUsedAt.Time
	}
	if revokedAt.Valid {
		key.RevokedAt = revokedAt.Time
	}

	return &key, nil
}

// FindAPIKeyByPrefix retrieves an API key by its prefix
func (r *PostgresAPIKeyRepository) FindAPIKeyByPrefix(prefix string) (*APIKey, error) {
	query := `
	SELECT 
		id, prefix, secret, name, description, owner_id, 
		created_at, expires_at, last_used_at, is_revoked, revoked_at,
		scopes, roles, permissions
	FROM apikeys
	WHERE prefix = $1
	`

	var key APIKey
	var expiresAt, lastUsedAt, revokedAt sql.NullTime

	err := r.db.QueryRow(query, prefix).Scan(
		&key.ID, &key.Prefix, &key.Secret, &key.Name, &key.Description, &key.OwnerID,
		&key.CreatedAt, &expiresAt, &lastUsedAt, &key.IsRevoked, &revokedAt,
		&key.Scopes, &key.Roles, &key.Permissions,
	)

	if err == sql.ErrNoRows {
		return nil, ErrAPIKeyNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to find API key: %w", err)
	}

	// Handle nullable fields
	if expiresAt.Valid {
		key.ExpiresAt = expiresAt.Time
	}
	if lastUsedAt.Valid {
		key.LastUsedAt = lastUsedAt.Time
	}
	if revokedAt.Valid {
		key.RevokedAt = revokedAt.Time
	}

	return &key, nil
}

// FindAPIKeysByOwnerID retrieves all API keys for a specific owner
func (r *PostgresAPIKeyRepository) FindAPIKeysByOwnerID(ownerID string) ([]*APIKey, error) {
	query := `
	SELECT 
		id, prefix, secret, name, description, owner_id, 
		created_at, expires_at, last_used_at, is_revoked, revoked_at,
		scopes, roles, permissions
	FROM apikeys
	WHERE owner_id = $1
	ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	defer rows.Close()

	var keys []*APIKey

	for rows.Next() {
		var key APIKey
		var expiresAt, lastUsedAt, revokedAt sql.NullTime

		err := rows.Scan(
			&key.ID, &key.Prefix, &key.Secret, &key.Name, &key.Description, &key.OwnerID,
			&key.CreatedAt, &expiresAt, &lastUsedAt, &key.IsRevoked, &revokedAt,
			&key.Scopes, &key.Roles, &key.Permissions,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}

		// Handle nullable fields
		if expiresAt.Valid {
			key.ExpiresAt = expiresAt.Time
		}
		if lastUsedAt.Valid {
			key.LastUsedAt = lastUsedAt.Time
		}
		if revokedAt.Valid {
			key.RevokedAt = revokedAt.Time
		}

		keys = append(keys, &key)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return keys, nil
}

// UpdateAPIKey updates an existing API key
func (r *PostgresAPIKeyRepository) UpdateAPIKey(key *APIKey) error {
	if key == nil {
		return fmt.Errorf("api key cannot be nil")
	}

	query := `
	UPDATE apikeys SET
		secret = $1,
		name = $2,
		description = $3,
		expires_at = $4,
		last_used_at = $5,
		is_revoked = $6,
		revoked_at = $7,
		scopes = $8,
		roles = $9,
		permissions = $10
	WHERE id = $11
	`

	result, err := r.db.Exec(
		query,
		key.Secret, key.Name, key.Description,
		key.ExpiresAt, key.LastUsedAt, key.IsRevoked, key.RevokedAt,
		key.Scopes, key.Roles, key.Permissions,
		key.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update API key: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrAPIKeyNotFound
	}

	return nil
}

// DeleteAPIKey removes an API key
func (r *PostgresAPIKeyRepository) DeleteAPIKey(id string) error {
	result, err := r.db.Exec("DELETE FROM apikeys WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrAPIKeyNotFound
	}

	return nil
}

// CleanExpiredKeys removes all expired API keys and returns count of removed keys
func (r *PostgresAPIKeyRepository) CleanExpiredKeys() int {
	result, err := r.db.Exec("DELETE FROM apikeys WHERE expires_at < NOW()")
	if err != nil {
		fmt.Printf("Failed to clean expired API keys: %v\n", err)
		return 0
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		fmt.Printf("Failed to get rows affected: %v\n", err)
		return 0
	}

	return int(rowsAffected)
}
