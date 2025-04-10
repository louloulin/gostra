# PostgreSQL Workflow State Store Example

This example demonstrates how to use PostgreSQL for persistent workflow state storage in Gostra. The implementation is similar to Mastra's storage options but adapted for Go and the actor-based architecture.

## Features

- Persistent storage of workflow states in a PostgreSQL database
- Suspend and resume workflows with state preservation
- State survives application restarts and server crashes
- Complete support for all workflow state operations

## Setup Requirements

1. PostgreSQL database (version 10+)
2. The PostgreSQL connection string as an environment variable

## Running the Example

1. Install PostgreSQL or use a hosted service
2. Create a database for Gostra
3. Set the connection string as an environment variable:

```sh
export POSTGRES_CONN_STRING="postgres://username:password@localhost:5432/database_name?sslmode=disable"
```

4. Run the example:

```sh
cd examples/postgres_state_workflow
go run main.go
```

## How It Works

The example demonstrates a simple workflow with three steps:

1. **Initial Step**: Processes the input message and proceeds to the next step
2. **Process Step**: Suspends the workflow, waiting for an approval event
3. **Final Step**: Completes the workflow with results from previous steps

When the workflow suspends, its state is automatically persisted to PostgreSQL. This allows the workflow to be resumed at any time, even if the application has been restarted.

## PostgreSQL Schema

The state store creates a `workflow_states` table with the following schema:

```sql
CREATE TABLE workflow_states (
    workflow_id TEXT PRIMARY KEY,
    workflow_name TEXT NOT NULL,
    status TEXT NOT NULL,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    last_updated TIMESTAMP WITH TIME ZONE NOT NULL,
    current_step_id TEXT,
    results JSONB,
    suspended_at TIMESTAMP WITH TIME ZONE,
    resume_data JSONB
);
```

## Implementation Details

The PostgreSQL state store implements the `WorkflowStateStore` interface defined in the workflow package:

```go
type WorkflowStateStore interface {
    SaveWorkflowState(state *WorkflowState) error
    LoadWorkflowState(workflowID string) (*WorkflowState, error)
    ListWorkflowStates() ([]*WorkflowState, error)
    DeleteWorkflowState(workflowID string) error
}
```

Each method maps to a PostgreSQL operation:
- `SaveWorkflowState`: Inserts or updates a workflow state
- `LoadWorkflowState`: Retrieves a workflow state by ID
- `ListWorkflowStates`: Lists all workflow states
- `DeleteWorkflowState`: Removes a workflow state

## Best Practices

- Use connection pooling for production deployments
- Create indexes for frequently queried fields
- Implement transaction handling for critical operations
- Consider implementing a cleanup strategy for completed workflows 