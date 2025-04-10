# Redis Workflow State Store Example

This example demonstrates how to use Redis for persistent workflow state storage in Gostra. The implementation is similar to Mastra's Upstash storage option but adapted for Go and the actor-based architecture of Gostra.

## Features

- Persistent storage of workflow states in Redis
- Suspend and resume workflows with state preservation
- Configurable key expiration for automatic cleanup
- Fast and efficient access to workflow states
- Complete support for all workflow state operations

## Setup Requirements

1. Running Redis server (local or remote)
2. Redis connection information as environment variables

## Running the Example

1. Make sure Redis is running (locally or remotely)
2. Set environment variables for Redis connection:

```sh
# Required: Redis address
export REDIS_ADDR="localhost:6379"

# Optional: Redis password if authentication is enabled
export REDIS_PASSWORD="your-password"
```

3. Run the example:

```sh
cd examples/redis_state_workflow
go run main.go
```

## How It Works

The example demonstrates a simple workflow with three steps:

1. **Initial Step**: Processes the input message and proceeds to the next step
2. **Process Step**: Suspends the workflow, waiting for an approval event
3. **Final Step**: Completes the workflow with results from previous steps

When the workflow suspends, its state is automatically persisted to Redis. This allows the workflow to be resumed at any time, even if the application has been restarted or deployed to a different server.

## Redis Storage Details

- Workflow states are stored as JSON-serialized objects in Redis
- Keys are prefixed to avoid collisions with other applications
- Keys can be configured with an expiration time for automatic cleanup
- Redis pipelining is used for efficient batch operations

## Implementation Details

The Redis state store implements the `WorkflowStateStore` interface:

```go
type WorkflowStateStore interface {
    SaveWorkflowState(state *WorkflowState) error
    LoadWorkflowState(workflowID string) (*WorkflowState, error)
    ListWorkflowStates() ([]*WorkflowState, error)
    DeleteWorkflowState(workflowID string) error
}
```

Configuration options for the Redis state store include:

```go
type RedisStateStoreOptions struct {
    // Address for the Redis server (host:port)
    Address string

    // Password for Redis authentication (optional)
    Password string

    // Database to select after connecting to Redis
    Database int

    // KeyPrefix for Redis keys to avoid collisions (default: "workflow:")
    KeyPrefix string

    // Expiration is the default expiration time for workflow states
    Expiration time.Duration
}
```

## Best Practices

- Use key prefixes to namespace your workflow states
- Consider setting appropriate expiration times based on your workflow lifecycle
- For production deployments, use Redis with persistence (AOF or RDB snapshots)
- Use Redis Sentinel or Redis Cluster for high availability
- Monitor Redis memory usage, especially when storing large workflow states 