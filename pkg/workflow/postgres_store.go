package workflow

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// PostgresStateStore implements WorkflowStateStore interface for PostgreSQL
type PostgresStateStore struct {
	db        *sql.DB
	tableName string
}

// PostgresStateStoreOptions contains configuration for PostgreSQL state storage
type PostgresStateStoreOptions struct {
	// ConnectionString for the PostgreSQL database
	ConnectionString string

	// TableName for storing workflow states (default: "workflow_states")
	TableName string
}

// NewPostgresStateStore creates a new PostgreSQL workflow state store
func NewPostgresStateStore(opts PostgresStateStoreOptions) (*PostgresStateStore, error) {
	if opts.ConnectionString == "" {
		return nil, errors.New("connection string is required")
	}

	if opts.TableName == "" {
		opts.TableName = "workflow_states"
	}

	db, err := sql.Open("postgres", opts.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL database: %w", err)
	}

	store := &PostgresStateStore{
		db:        db,
		tableName: opts.TableName,
	}

	// Initialize the database schema
	if err := store.initialize(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initialize creates the required PostgreSQL table if it doesn't exist
func (p *PostgresStateStore) initialize() error {
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			workflow_id TEXT PRIMARY KEY,
			workflow_name TEXT NOT NULL,
			status TEXT NOT NULL,
			start_time TIMESTAMP WITH TIME ZONE NOT NULL,
			last_updated TIMESTAMP WITH TIME ZONE NOT NULL,
			current_step_id TEXT,
			results JSONB,
			suspended_at TIMESTAMP WITH TIME ZONE,
			resume_data JSONB
		)
	`, p.tableName)

	_, err := p.db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create workflow states table: %w", err)
	}

	return nil
}

// SaveWorkflowState saves a workflow state to the PostgreSQL database
func (p *PostgresStateStore) SaveWorkflowState(state *WorkflowState) error {
	if state == nil {
		return errors.New("state cannot be nil")
	}

	if state.WorkflowID == "" {
		return errors.New("workflow ID is required")
	}

	// Marshal results to JSON
	var resultsJSON []byte
	if state.Results != nil {
		var err error
		resultsJSON, err = json.Marshal(state.Results)
		if err != nil {
			return fmt.Errorf("failed to marshal results to JSON: %w", err)
		}
	}

	// Marshal resume data to JSON
	var resumeDataJSON []byte
	if state.ResumeData != nil {
		var err error
		resumeDataJSON, err = json.Marshal(state.ResumeData)
		if err != nil {
			return fmt.Errorf("failed to marshal resume data to JSON: %w", err)
		}
	}

	// Update or insert workflow state
	query := fmt.Sprintf(`
		INSERT INTO %s (
			workflow_id, workflow_name, status, start_time, last_updated, 
			current_step_id, results, suspended_at, resume_data
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (workflow_id) DO UPDATE SET
			workflow_name = $2,
			status = $3,
			start_time = $4,
			last_updated = $5,
			current_step_id = $6,
			results = $7,
			suspended_at = $8,
			resume_data = $9
	`, p.tableName)

	_, err := p.db.Exec(
		query,
		state.WorkflowID,
		state.WorkflowName,
		state.Status,
		state.StartTime,
		time.Now(), // Always update last_updated time
		state.CurrentStepID,
		resultsJSON,
		state.SuspendedAt,
		resumeDataJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to save workflow state: %w", err)
	}

	return nil
}

// LoadWorkflowState loads a workflow state from the PostgreSQL database
func (p *PostgresStateStore) LoadWorkflowState(workflowID string) (*WorkflowState, error) {
	if workflowID == "" {
		return nil, errors.New("workflow ID is required")
	}

	query := fmt.Sprintf(`
		SELECT 
			workflow_id, workflow_name, status, start_time, last_updated, 
			current_step_id, results, suspended_at, resume_data
		FROM %s
		WHERE workflow_id = $1
	`, p.tableName)

	var (
		state                   WorkflowState
		currentStepID           sql.NullString
		resultsJSON, resumeJSON []byte
		suspendedAt             sql.NullTime
	)

	err := p.db.QueryRow(query, workflowID).Scan(
		&state.WorkflowID,
		&state.WorkflowName,
		&state.Status,
		&state.StartTime,
		&state.LastUpdated,
		&currentStepID,
		&resultsJSON,
		&suspendedAt,
		&resumeJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("workflow state not found for ID: %s", workflowID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load workflow state: %w", err)
	}

	// Set nullable fields
	if currentStepID.Valid {
		state.CurrentStepID = currentStepID.String
	}

	if suspendedAt.Valid {
		state.SuspendedAt = suspendedAt.Time
	}

	// Unmarshal results if present
	if len(resultsJSON) > 0 {
		results := make(map[string]interface{})
		if err := json.Unmarshal(resultsJSON, &results); err != nil {
			return nil, fmt.Errorf("failed to unmarshal workflow results: %w", err)
		}
		state.Results = results
	} else {
		state.Results = make(map[string]interface{})
	}

	// Unmarshal resume data if present
	if len(resumeJSON) > 0 {
		var resumeData interface{}
		if err := json.Unmarshal(resumeJSON, &resumeData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal resume data: %w", err)
		}
		state.ResumeData = resumeData
	}

	return &state, nil
}

// ListWorkflowStates lists all workflow states from the PostgreSQL database
func (p *PostgresStateStore) ListWorkflowStates() ([]*WorkflowState, error) {
	query := fmt.Sprintf(`
		SELECT 
			workflow_id, workflow_name, status, start_time, last_updated, 
			current_step_id, results, suspended_at, resume_data
		FROM %s
		ORDER BY last_updated DESC
	`, p.tableName)

	rows, err := p.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow states: %w", err)
	}
	defer rows.Close()

	var states []*WorkflowState

	for rows.Next() {
		var (
			state                   WorkflowState
			currentStepID           sql.NullString
			resultsJSON, resumeJSON []byte
			suspendedAt             sql.NullTime
		)

		err := rows.Scan(
			&state.WorkflowID,
			&state.WorkflowName,
			&state.Status,
			&state.StartTime,
			&state.LastUpdated,
			&currentStepID,
			&resultsJSON,
			&suspendedAt,
			&resumeJSON,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan workflow state: %w", err)
		}

		// Set nullable fields
		if currentStepID.Valid {
			state.CurrentStepID = currentStepID.String
		}

		if suspendedAt.Valid {
			state.SuspendedAt = suspendedAt.Time
		}

		// Unmarshal results if present
		if len(resultsJSON) > 0 {
			results := make(map[string]interface{})
			if err := json.Unmarshal(resultsJSON, &results); err != nil {
				return nil, fmt.Errorf("failed to unmarshal workflow results: %w", err)
			}
			state.Results = results
		} else {
			state.Results = make(map[string]interface{})
		}

		// Unmarshal resume data if present
		if len(resumeJSON) > 0 {
			var resumeData interface{}
			if err := json.Unmarshal(resumeJSON, &resumeData); err != nil {
				return nil, fmt.Errorf("failed to unmarshal resume data: %w", err)
			}
			state.ResumeData = resumeData
		}

		states = append(states, &state)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over workflow states: %w", err)
	}

	return states, nil
}

// DeleteWorkflowState deletes a workflow state from the PostgreSQL database
func (p *PostgresStateStore) DeleteWorkflowState(workflowID string) error {
	if workflowID == "" {
		return errors.New("workflow ID is required")
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE workflow_id = $1", p.tableName)

	result, err := p.db.Exec(query, workflowID)
	if err != nil {
		return fmt.Errorf("failed to delete workflow state: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("workflow state not found for ID: %s", workflowID)
	}

	return nil
}

// Close closes the database connection
func (p *PostgresStateStore) Close() error {
	return p.db.Close()
}
