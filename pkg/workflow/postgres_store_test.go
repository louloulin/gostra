package workflow

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// setupPostgresTest sets up a test database connection
// Requires a running PostgreSQL instance
// Set the POSTGRES_TEST_DSN environment variable to run these tests
func setupPostgresTest(t *testing.T) (*PostgresStateStore, func()) {
	t.Helper()

	// Skip if no DSN is provided
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("Skipping PostgreSQL tests: POSTGRES_TEST_DSN not set")
	}

	// Create test table name with unique suffix to avoid conflicts
	testTableName := "workflow_states_test_" + time.Now().Format("20060102150405")

	// Create and initialize repository
	store, err := NewPostgresStateStore(PostgresStateStoreOptions{
		ConnectionString: dsn,
		TableName:        testTableName,
	})
	if err != nil {
		t.Fatalf("Failed to create PostgreSQL state store: %v", err)
	}

	// Create cleanup function
	cleanup := func() {
		// Drop test table
		_, err := store.db.Exec("DROP TABLE IF EXISTS " + testTableName)
		if err != nil {
			t.Logf("Failed to drop test table: %v", err)
		}
		store.Close()
	}

	return store, cleanup
}

func TestPostgresStateStore(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PostgreSQL integration test in short mode")
	}

	store, cleanup := setupPostgresTest(t)
	defer cleanup()

	// Create test workflow state
	state := &WorkflowState{
		WorkflowID:    "test-workflow-1",
		WorkflowName:  "Test Workflow",
		Status:        EventStatusPending,
		StartTime:     time.Now().UTC().Truncate(time.Microsecond), // Truncate to match PostgreSQL precision
		LastUpdated:   time.Now().UTC().Truncate(time.Microsecond),
		CurrentStepID: "step-1",
		Results: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
			"nested": map[string]interface{}{
				"nestedKey": "nestedValue",
			},
		},
	}

	// Test SaveWorkflowState
	t.Run("SaveWorkflowState", func(t *testing.T) {
		err := store.SaveWorkflowState(state)
		assert.NoError(t, err)
	})

	// Test LoadWorkflowState
	t.Run("LoadWorkflowState", func(t *testing.T) {
		loadedState, err := store.LoadWorkflowState(state.WorkflowID)
		assert.NoError(t, err)
		assert.Equal(t, state.WorkflowID, loadedState.WorkflowID)
		assert.Equal(t, state.WorkflowName, loadedState.WorkflowName)
		assert.Equal(t, state.Status, loadedState.Status)
		assert.Equal(t, state.StartTime.Format(time.RFC3339), loadedState.StartTime.Format(time.RFC3339))
		assert.Equal(t, state.CurrentStepID, loadedState.CurrentStepID)

		// Check results
		assert.Equal(t, "value1", loadedState.Results["key1"])
		assert.Equal(t, float64(123), loadedState.Results["key2"]) // JSON numbers convert to float64

		// Check nested values
		nested, ok := loadedState.Results["nested"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "nestedValue", nested["nestedKey"])
	})

	// Test updating an existing state
	t.Run("UpdateWorkflowState", func(t *testing.T) {
		// Update workflow state
		state.Status = EventStatusRunning
		state.CurrentStepID = "step-2"
		state.Results["key3"] = "new-value"

		// Add resume data
		state.SuspendedAt = time.Now().UTC().Truncate(time.Microsecond)
		state.ResumeData = map[string]interface{}{
			"resumeKey": "resumeValue",
		}

		err := store.SaveWorkflowState(state)
		assert.NoError(t, err)

		// Verify update
		loadedState, err := store.LoadWorkflowState(state.WorkflowID)
		assert.NoError(t, err)
		assert.Equal(t, EventStatusRunning, loadedState.Status)
		assert.Equal(t, "step-2", loadedState.CurrentStepID)
		assert.Equal(t, "new-value", loadedState.Results["key3"])

		// Check resume data
		resumeData, ok := loadedState.ResumeData.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "resumeValue", resumeData["resumeKey"])

		// Check suspended time
		assert.Equal(t, state.SuspendedAt.Format(time.RFC3339), loadedState.SuspendedAt.Format(time.RFC3339))
	})

	// Test non-existent workflow state
	t.Run("LoadNonExistentState", func(t *testing.T) {
		_, err := store.LoadWorkflowState("non-existent-id")
		assert.Error(t, err)
	})

	// Create another state for listing
	t.Run("CreateSecondState", func(t *testing.T) {
		state2 := &WorkflowState{
			WorkflowID:    "test-workflow-2",
			WorkflowName:  "Test Workflow 2",
			Status:        EventStatusCompleted,
			StartTime:     time.Now().UTC().Truncate(time.Microsecond),
			LastUpdated:   time.Now().UTC().Truncate(time.Microsecond),
			CurrentStepID: "step-final",
			Results: map[string]interface{}{
				"result": "success",
			},
		}

		err := store.SaveWorkflowState(state2)
		assert.NoError(t, err)
	})

	// Test ListWorkflowStates
	t.Run("ListWorkflowStates", func(t *testing.T) {
		states, err := store.ListWorkflowStates()
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(states), 2)

		// Find our test workflow states
		var foundFirst, foundSecond bool
		for _, s := range states {
			if s.WorkflowID == "test-workflow-1" {
				foundFirst = true
				assert.Equal(t, EventStatusRunning, s.Status)
			}
			if s.WorkflowID == "test-workflow-2" {
				foundSecond = true
				assert.Equal(t, EventStatusCompleted, s.Status)
			}
		}

		assert.True(t, foundFirst, "First test workflow not found in list")
		assert.True(t, foundSecond, "Second test workflow not found in list")
	})

	// Test DeleteWorkflowState
	t.Run("DeleteWorkflowState", func(t *testing.T) {
		err := store.DeleteWorkflowState(state.WorkflowID)
		assert.NoError(t, err)

		// Verify deletion
		_, err = store.LoadWorkflowState(state.WorkflowID)
		assert.Error(t, err)
	})

	// Test deleting non-existent workflow
	t.Run("DeleteNonExistentState", func(t *testing.T) {
		err := store.DeleteWorkflowState("non-existent-id")
		assert.Error(t, err)
	})
}

// Test for invalid inputs
func TestPostgresStateStoreInvalid(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PostgreSQL integration test in short mode")
	}

	// Skip if no DSN is provided
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("Skipping PostgreSQL tests: POSTGRES_TEST_DSN not set")
	}

	t.Run("InvalidConnectionString", func(t *testing.T) {
		_, err := NewPostgresStateStore(PostgresStateStoreOptions{
			ConnectionString: "invalid-connection-string",
		})
		assert.Error(t, err)
	})

	t.Run("EmptyConnectionString", func(t *testing.T) {
		_, err := NewPostgresStateStore(PostgresStateStoreOptions{
			ConnectionString: "",
		})
		assert.Error(t, err)
	})

	store, cleanup := setupPostgresTest(t)
	defer cleanup()

	t.Run("NilState", func(t *testing.T) {
		err := store.SaveWorkflowState(nil)
		assert.Error(t, err)
	})

	t.Run("EmptyWorkflowID", func(t *testing.T) {
		err := store.SaveWorkflowState(&WorkflowState{
			WorkflowName: "Test",
			Status:       EventStatusPending,
			StartTime:    time.Now(),
			LastUpdated:  time.Now(),
		})
		assert.Error(t, err)
	})

	t.Run("EmptyWorkflowIDLoad", func(t *testing.T) {
		_, err := store.LoadWorkflowState("")
		assert.Error(t, err)
	})

	t.Run("EmptyWorkflowIDDelete", func(t *testing.T) {
		err := store.DeleteWorkflowState("")
		assert.Error(t, err)
	})
}
