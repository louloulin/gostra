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

## Troubleshooting Timeout Issues

If you're experiencing timeout errors in your agent network, try the following:

1. **API Handler and Router Timeout Mismatch**: Ensure that the API handler timeout and router timeouts are aligned. If your API handler waits 30 seconds but your router times out after 10 seconds, you'll see timeout errors.

2. **Monitor Agent Response Times**: Use logging to track how long agents take to respond, then adjust timeout values accordingly.

3. **Gradual Increments**: If you're unsure what timeout values to use, start with the defaults and gradually increase them if needed, rather than setting extremely high values.

4. **Different Timeouts for Different Tasks**: Consider using different timeout configurations for different types of agent networks, based on their specific workloads.

5. **Timeout Cascade**: Be aware that timeouts can cascade through your system - if one agent times out, it could cause other agents waiting for its response to time out as well.

## Error Handling

When a timeout occurs, the system will propagate appropriate error messages. Make sure to implement proper error handling in your application to deal with timeouts gracefully:

```go
if err := network.Transmit(ctx, msg); err != nil {
    if strings.Contains(err.Error(), "timeout") {
        // Handle timeout specifically
        log.Printf("Operation timed out: %v. Consider increasing the timeout.", err)
        // Implement recovery strategy
    } else {
        // Handle other errors
        log.Printf("Error: %v", err)
    }
}
```

## Example

See a complete example in `examples/agent_network/configurable_timeout.go`. 