# Configurable Timeouts in Agent Networks

The Gostra AgentNetwork now supports configurable timeouts for different types of agent operations. This feature allows developers to fine-tune the timeout values based on the specific requirements of their applications.

## Router Timeout Options

The `RouterOptions` struct provides the following configuration options:

```go
type RouterOptions struct {
    DefaultTimeout    time.Duration // Default timeout for agent communication
    RoutingTimeout    time.Duration // Timeout specifically for LLM routing decisions
    ParallelTimeout   time.Duration // Timeout for parallel agent calls
    SequentialTimeout time.Duration // Timeout for sequential agent calls
}
```

## Default Values

If not explicitly configured, the system uses the following default values:

```go
func DefaultRouterOptions() *RouterOptions {
    return &RouterOptions{
        DefaultTimeout:    30 * time.Second,
        RoutingTimeout:    15 * time.Second,
        ParallelTimeout:   45 * time.Second,
        SequentialTimeout: 30 * time.Second,
    }
}
```

## Usage in Agent Network

To use custom timeout values, provide a `RouterOptions` instance when creating an `AgentNetwork`:

```go
// Create custom router options
routerOptions := &agent.RouterOptions{
    DefaultTimeout:    30 * time.Second,
    RoutingTimeout:    15 * time.Second,
    ParallelTimeout:   60 * time.Second, // Longer timeout for parallel operations
    SequentialTimeout: 30 * time.Second,
}

// Create agent network with custom timeouts
networkOptions := &agent.AgentNetworkOptions{
    ID:            "my-network",
    Name:          "My Network",
    ActorSystem:   actorSystem,
    RouterOptions: routerOptions,
}

network, err := agent.NewAgentNetwork(networkOptions, modelProvider)
```

## When to Customize Timeouts

You may want to adjust timeouts in the following scenarios:

1. **Increase ParallelTimeout**: When running computationally intensive operations in parallel
2. **Increase SequentialTimeout**: For operations that involve complex chains of reasoning
3. **Decrease RoutingTimeout**: If routing decisions are simple and you want faster error handling
4. **Increase DefaultTimeout**: For general communication in high-latency environments

## Timeout Best Practices

When configuring timeouts, consider the following best practices:

- **Parallel Timeout**: Should typically be longer than sequential timeout since parallel operations wait for the slowest operation to complete
- **Routing Timeout**: Keep this relatively short to avoid blocking on routing decisions
- **Default Timeout**: Set this based on the average expected response time of your agents
- **Sequential Timeout**: Should be sufficient for your most complex single-agent operation

## Error Handling

When a timeout occurs, the system will propagate appropriate error messages. Make sure to implement proper error handling in your application to deal with timeouts gracefully.

## Example

See a complete example in `examples/agent_network/configurable_timeout.go`. 