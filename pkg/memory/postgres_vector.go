package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

// PostgresVectorStore implements VectorStorage interface using PostgreSQL with pgvector
type PostgresVectorStore struct {
	db        *sql.DB
	tableName string
	dimension int
	batchSize int
}

// PostgresVectorOptions contains options for PostgreSQL vector store
type PostgresVectorOptions struct {
	ConnectionString string
	TableName        string
	Dimension        int
	BatchSize        int
}

// NewPostgresVectorStore creates a new PostgreSQL vector store
func NewPostgresVectorStore(opts PostgresVectorOptions) (*PostgresVectorStore, error) {
	if opts.TableName == "" {
		opts.TableName = "vector_store"
	}
	if opts.Dimension <= 0 {
		return nil, fmt.Errorf("dimension must be positive")
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 1000
	}

	db, err := sql.Open("postgres", opts.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	store := &PostgresVectorStore{
		db:        db,
		tableName: opts.TableName,
		dimension: opts.Dimension,
		batchSize: opts.BatchSize,
	}

	if err := store.initialize(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initialize creates the necessary tables and indexes
func (s *PostgresVectorStore) initialize() error {
	// Create extension if not exists
	_, err := s.db.Exec("CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("failed to create vector extension: %w", err)
	}

	// Create table if not exists
	_, err = s.db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			vector vector(%d),
			metadata JSONB
		)
	`, s.tableName, s.dimension))
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create index if not exists
	_, err = s.db.Exec(fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_vector_idx ON %s USING ivfflat (vector vector_cosine_ops)
	`, s.tableName, s.tableName))
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}

// Store implements VectorStorage.Store
func (s *PostgresVectorStore) Store(ctx context.Context, vectors []Vector) error {
	if len(vectors) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statement for batch insert
	stmt, err := tx.PrepareContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (id, vector, metadata)
		VALUES ($1, $2::float4[], $3)
		ON CONFLICT (id) DO UPDATE
		SET vector = EXCLUDED.vector,
			metadata = EXCLUDED.metadata
	`, s.tableName))
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert vectors in batches
	for i := 0; i < len(vectors); i += s.batchSize {
		end := i + s.batchSize
		if end > len(vectors) {
			end = len(vectors)
		}

		for _, v := range vectors[i:end] {
			if len(v.Values) != s.dimension {
				return fmt.Errorf("vector dimension mismatch: expected %d, got %d", s.dimension, len(v.Values))
			}

			metadata, err := json.Marshal(v.Metadata)
			if err != nil {
				return fmt.Errorf("failed to marshal metadata: %w", err)
			}

			_, err = stmt.ExecContext(ctx, v.ID, v.Values, metadata)
			if err != nil {
				return fmt.Errorf("failed to insert vector: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Search implements VectorStorage.Search
func (s *PostgresVectorStore) Search(ctx context.Context, query Vector, opts VectorSearchOptions) ([]VectorSearchResult, error) {
	if len(query.Values) != s.dimension {
		return nil, fmt.Errorf("query vector dimension mismatch: expected %d, got %d", s.dimension, len(query.Values))
	}

	if opts.Limit <= 0 {
		opts.Limit = 10
	}

	// Build query with metadata filter if provided
	filterCondition := ""
	filterValues := []interface{}{query.Values, opts.Limit}
	if opts.Filter != nil {
		conditions := make([]string, 0, len(opts.Filter))
		i := 3
		for k, v := range opts.Filter {
			conditions = append(conditions, fmt.Sprintf("metadata->>'%s' = $%d", k, i))
			filterValues = append(filterValues, v)
			i++
		}
		if len(conditions) > 0 {
			filterCondition = "AND " + strings.Join(conditions, " AND ")
		}
	}

	// Execute search query
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, vector, metadata,
			   1 - (vector <=> $1::vector) as similarity
		FROM %s
		WHERE 1 - (vector <=> $1::vector) >= $2
		%s
		ORDER BY similarity DESC
		LIMIT $3
	`, s.tableName, filterCondition), filterValues...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	var results []VectorSearchResult
	for rows.Next() {
		var (
			v        Vector
			metadata []byte
			score    float32
		)

		if err := rows.Scan(&v.ID, &v.Values, &metadata, &score); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal(metadata, &v.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		results = append(results, VectorSearchResult{
			Vector:   v,
			Score:    score,
			Distance: 1 - score,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

// Delete implements VectorStorage.Delete
func (s *PostgresVectorStore) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// Create placeholders for the query
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	// Execute delete query
	_, err := s.db.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM %s
		WHERE id IN (%s)
	`, s.tableName, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return fmt.Errorf("failed to delete vectors: %w", err)
	}

	return nil
}

// Clear implements VectorStorage.Clear
func (s *PostgresVectorStore) Clear(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s", s.tableName))
	if err != nil {
		return fmt.Errorf("failed to clear table: %w", err)
	}
	return nil
}

// Close closes the database connection
func (s *PostgresVectorStore) Close() error {
	return s.db.Close()
}
