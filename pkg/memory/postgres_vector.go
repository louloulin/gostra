package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// PostgresVectorOptions contains configuration for PostgreSQL vector storage
type PostgresVectorOptions struct {
	// ConnectionString for the PostgreSQL database
	ConnectionString string

	// TableName for storing vectors (default: "vector_store")
	TableName string

	// VectorDimension defines the size of the vectors to be stored
	VectorDimension int

	// BatchSize for batch operations (default: 100)
	BatchSize int
}

// PostgresVectorStorage implements VectorStorage interface for PostgreSQL
type PostgresVectorStorage struct {
	db              *sql.DB
	tableName       string
	vectorDimension int
	batchSize       int
}

// NewPostgresVectorStorage creates a new PostgreSQL vector storage
func NewPostgresVectorStorage(opts PostgresVectorOptions) (*PostgresVectorStorage, error) {
	if opts.ConnectionString == "" {
		return nil, errors.New("connection string is required")
	}

	if opts.VectorDimension <= 0 {
		return nil, errors.New("vector dimension must be positive")
	}

	if opts.TableName == "" {
		opts.TableName = "vector_store"
	}

	if opts.BatchSize <= 0 {
		opts.BatchSize = 100
	}

	db, err := sql.Open("postgres", opts.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	store := &PostgresVectorStorage{
		db:              db,
		tableName:       opts.TableName,
		vectorDimension: opts.VectorDimension,
		batchSize:       opts.BatchSize,
	}

	// Initialize the database schema
	if err := store.initialize(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initialize creates the required PostgreSQL extension and table if they don't exist
func (p *PostgresVectorStorage) initialize() error {
	// Create pgvector extension if it doesn't exist
	_, err := p.db.Exec("CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("failed to create vector extension: %w", err)
	}

	// Create table if it doesn't exist
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			vector vector(%d) NOT NULL,
			metadata JSONB
		)
	`, p.tableName, p.vectorDimension)

	_, err = p.db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create index for vector similarity search
	createIndexSQL := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_vector_idx ON %s USING ivfflat (vector vector_cosine_ops)
		WITH (lists = 100)
	`, p.tableName, p.tableName)

	_, err = p.db.Exec(createIndexSQL)
	if err != nil {
		return fmt.Errorf("failed to create vector index: %w", err)
	}

	return nil
}

// Store stores vectors in the database
func (p *PostgresVectorStorage) Store(ctx context.Context, vectors []Vector) error {
	if len(vectors) == 0 {
		return nil
	}

	// Begin transaction
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare insert statement
	stmt, err := tx.PrepareContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (id, vector, metadata)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE
		SET vector = $2, metadata = $3
	`, p.tableName))
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Process items in batches
	for i := 0; i < len(vectors); i += p.batchSize {
		end := i + p.batchSize
		if end > len(vectors) {
			end = len(vectors)
		}

		batch := vectors[i:end]
		for _, vector := range batch {
			// Check vector dimension
			if len(vector.Values) != p.vectorDimension {
				return fmt.Errorf("vector dimension mismatch: expected %d, got %d", p.vectorDimension, len(vector.Values))
			}

			// Convert vector to PostgreSQL format
			vectorStr := fmt.Sprintf("[%s]", strings.Join(float32sToStrings(vector.Values), ","))

			// Convert metadata to JSON
			var metadataJSON []byte
			if vector.Metadata != nil {
				metadataJSON, err = json.Marshal(vector.Metadata)
				if err != nil {
					return fmt.Errorf("failed to marshal metadata: %w", err)
				}
			}

			// Execute insert
			_, err = stmt.ExecContext(ctx, vector.ID, vectorStr, metadataJSON)
			if err != nil {
				return fmt.Errorf("failed to insert item: %w", err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Search searches for similar vectors in the database
func (p *PostgresVectorStorage) Search(ctx context.Context, query Vector, opts VectorSearchOptions) ([]VectorSearchResult, error) {
	if len(query.Values) != p.vectorDimension {
		return nil, fmt.Errorf("query vector dimension mismatch: expected %d, got %d", p.vectorDimension, len(query.Values))
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}

	threshold := opts.Threshold

	// Convert query vector to PostgreSQL format
	vectorStr := fmt.Sprintf("[%s]", strings.Join(float32sToStrings(query.Values), ","))

	// Build query
	var queryBuilder strings.Builder
	var args []interface{}
	var argIndex int = 1

	queryBuilder.WriteString(fmt.Sprintf(`
		SELECT id, vector, metadata, 
		       1 - (vector <=> $%d) AS score
		FROM %s
	`, argIndex, p.tableName))
	args = append(args, vectorStr)
	argIndex++

	// Add metadata filter if provided
	if opts.Filter != nil && len(opts.Filter) > 0 {
		metadataFilter, filterArgs, err := buildMetadataFilter(opts.Filter, &argIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to build metadata filter: %w", err)
		}
		if metadataFilter != "" {
			queryBuilder.WriteString(" WHERE " + metadataFilter)
			args = append(args, filterArgs...)
		}
	}

	// Add threshold
	if threshold > 0 {
		if strings.Contains(queryBuilder.String(), "WHERE") {
			queryBuilder.WriteString(fmt.Sprintf(" AND 1 - (vector <=> $%d) >= $%d", 1, argIndex))
		} else {
			queryBuilder.WriteString(fmt.Sprintf(" WHERE 1 - (vector <=> $%d) >= $%d", 1, argIndex))
		}
		args = append(args, threshold)
		argIndex++
	}

	// Add order and limit
	queryBuilder.WriteString(fmt.Sprintf(` ORDER BY score DESC LIMIT $%d`, argIndex))
	args = append(args, limit)

	// Execute query
	rows, err := p.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	// Parse results
	var results []VectorSearchResult
	for rows.Next() {
		var id string
		var vectorBytes []byte
		var metadataBytes []byte
		var score float32

		err := rows.Scan(&id, &vectorBytes, &metadataBytes, &score)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Parse vector
		vectorValues, err := parseVector(string(vectorBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to parse vector: %w", err)
		}

		// Parse metadata
		var metadata map[string]interface{}
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		vector := Vector{
			ID:       id,
			Values:   vectorValues,
			Metadata: metadata,
		}

		results = append(results, VectorSearchResult{
			Vector:   vector,
			Score:    score,
			Distance: 1 - score,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

// Delete deletes vectors from the database
func (p *PostgresVectorStorage) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// Begin transaction
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Process IDs in batches
	for i := 0; i < len(ids); i += p.batchSize {
		end := i + p.batchSize
		if end > len(ids) {
			end = len(ids)
		}

		batch := ids[i:end]

		// Build placeholders for query
		placeholders := make([]string, len(batch))
		args := make([]interface{}, len(batch))
		for j, id := range batch {
			placeholders[j] = fmt.Sprintf("$%d", j+1)
			args[j] = id
		}

		// Execute delete
		deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", p.tableName, strings.Join(placeholders, ","))
		_, err = tx.ExecContext(ctx, deleteSQL, args...)
		if err != nil {
			return fmt.Errorf("failed to delete items: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Clear removes all vectors from the database
func (p *PostgresVectorStorage) Clear(ctx context.Context) error {
	_, err := p.db.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s", p.tableName))
	if err != nil {
		return fmt.Errorf("failed to clear vector store: %w", err)
	}
	return nil
}

// Close closes the database connection
func (p *PostgresVectorStorage) Close() error {
	return p.db.Close()
}

// Helper functions

// float32sToStrings converts a slice of float32 to a slice of strings
func float32sToStrings(floats []float32) []string {
	strings := make([]string, len(floats))
	for i, f := range floats {
		strings[i] = fmt.Sprintf("%f", f)
	}
	return strings
}

// parseVector parses a PostgreSQL vector string into a slice of float32
func parseVector(vectorStr string) ([]float32, error) {
	// Remove brackets and split by comma
	vectorStr = strings.Trim(vectorStr, "[]")
	components := strings.Split(vectorStr, ",")

	vector := make([]float32, len(components))
	for i, component := range components {
		var value float32
		_, err := fmt.Sscanf(component, "%f", &value)
		if err != nil {
			return nil, fmt.Errorf("failed to parse vector component: %w", err)
		}
		vector[i] = value
	}

	return vector, nil
}

// FilterOperator represents the supported filter operators
type FilterOperator string

const (
	OpEqual              FilterOperator = "$eq"
	OpNotEqual           FilterOperator = "$ne"
	OpGreaterThan        FilterOperator = "$gt"
	OpGreaterThanOrEqual FilterOperator = "$gte"
	OpLessThan           FilterOperator = "$lt"
	OpLessThanOrEqual    FilterOperator = "$lte"
	OpIn                 FilterOperator = "$in"
	OpRegex              FilterOperator = "$regex"
)

// buildMetadataFilter constructs a JSONB query for metadata filtering
func buildMetadataFilter(filter map[string]interface{}, argIndex *int) (string, []interface{}, error) {
	if len(filter) == 0 {
		return "", nil, nil
	}

	var conditions []string
	var args []interface{}

	for path, value := range filter {
		// Handle nested paths
		pathParts := strings.Split(path, ".")
		jsonPathExpr := "metadata"

		// Build JSON path expression for nested fields
		if len(pathParts) > 1 {
			// For nested paths like "nested.keywords"
			for i, part := range pathParts[:len(pathParts)-1] {
				if i == 0 {
					jsonPathExpr = fmt.Sprintf("%s->'%s'", jsonPathExpr, part)
				} else {
					jsonPathExpr = fmt.Sprintf("%s->'%s'", jsonPathExpr, part)
				}
			}
			jsonPathExpr = fmt.Sprintf("%s->>'%s'", jsonPathExpr, pathParts[len(pathParts)-1])
		} else {
			// Simple path like "text"
			jsonPathExpr = fmt.Sprintf("metadata->>'%s'", path)
		}

		switch val := value.(type) {
		case map[string]interface{}:
			// Handle operator conditions (e.g., {"field": {"$gt": 5}})
			for opStr, opVal := range val {
				op := FilterOperator(opStr)
				condition, opArgs, err := buildOperatorCondition(jsonPathExpr, op, opVal, *argIndex)
				if err != nil {
					return "", nil, err
				}
				conditions = append(conditions, condition)
				args = append(args, opArgs...)
				*argIndex += len(opArgs)
			}
		default:
			// Simple equality match
			conditions = append(conditions, fmt.Sprintf("%s = $%d", jsonPathExpr, *argIndex))
			args = append(args, fmt.Sprint(value))
			*argIndex++
		}
	}

	return strings.Join(conditions, " AND "), args, nil
}

// buildOperatorCondition builds condition for specific operator
func buildOperatorCondition(jsonPath string, operator FilterOperator, value interface{}, startArgIndex int) (string, []interface{}, error) {
	var condition string
	var args []interface{}

	switch operator {
	case OpEqual:
		condition = fmt.Sprintf("%s = $%d", jsonPath, startArgIndex)
		args = append(args, fmt.Sprint(value))
	case OpNotEqual:
		condition = fmt.Sprintf("%s <> $%d", jsonPath, startArgIndex)
		args = append(args, fmt.Sprint(value))
	case OpGreaterThan:
		condition = fmt.Sprintf("(CAST(%s AS numeric) > $%d)", jsonPath, startArgIndex)
		args = append(args, fmt.Sprint(value))
	case OpGreaterThanOrEqual:
		condition = fmt.Sprintf("(CAST(%s AS numeric) >= $%d)", jsonPath, startArgIndex)
		args = append(args, fmt.Sprint(value))
	case OpLessThan:
		condition = fmt.Sprintf("(CAST(%s AS numeric) < $%d)", jsonPath, startArgIndex)
		args = append(args, fmt.Sprint(value))
	case OpLessThanOrEqual:
		condition = fmt.Sprintf("(CAST(%s AS numeric) <= $%d)", jsonPath, startArgIndex)
		args = append(args, fmt.Sprint(value))
	case OpRegex:
		condition = fmt.Sprintf("%s ~ $%d", jsonPath, startArgIndex)
		args = append(args, fmt.Sprint(value))
	case OpIn:
		if values, ok := value.([]interface{}); ok {
			placeholders := make([]string, len(values))
			for i, v := range values {
				placeholders[i] = fmt.Sprintf("$%d", startArgIndex+i)
				args = append(args, fmt.Sprint(v))
			}
			condition = fmt.Sprintf("%s IN (%s)", jsonPath, strings.Join(placeholders, ", "))
		} else {
			return "", nil, fmt.Errorf("$in operator requires an array value")
		}
	default:
		return "", nil, fmt.Errorf("unsupported operator: %s", operator)
	}

	return condition, args, nil
}
