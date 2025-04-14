# Callback System in Gostra

## Overview
The Gostra framework provides a powerful callback system that allows developers to hook into key moments during agent execution. This enables monitoring, logging, custom business logic implementation, and integration with external systems.

## Types of Callbacks
Gostra currently supports two primary callback types:

1. **OnStepFinish**: Invoked when each individual step of agent execution completes
2. **OnFinish**: Invoked when the entire agent execution completes

## Callback Data Structures

### StepFinishData
When a step finishes, this structure is passed to the `OnStepFinish` callback:

```go
type StepFinishData struct {
    Text        string                 // The raw text response from the model
    ToolCalls   []models.ToolCall      // Any tool calls made during this step
    ToolResults map[string]interface{} // Results for each tool call (keyed by call ID)
    StepIndex   int                    // Current step index
    TotalSteps  int                    // Total steps expected
    Error       error                  // Any error that occurred during the step
}
```

### RunResult
When the entire execution finishes, this structure is passed to the `OnFinish` callback:

```go
type RunResult struct {
    Response      string    // Final response text
    NumberOfSteps int       // Number of steps executed
    Steps         []Step    // Step details
    Conversation  []Message // Conversation history
    Error         error     // Any error that occurred
}
```

## Usage Examples

### Basic Callback Implementation

```go
func main() {
    // Create Gostra instance
    g := gostra.NewGostra(&gostra.Options{})
    
    // Register agent
    agent, _ := g.RegisterAgent("assistant", &agent.Options{
        // Agent configuration
    })
    
    // Create context
    ctx := context.Background()
    
    // Define callbacks
    onStepFinish := func(ctx context.Context, data *agent.StepFinishData) error {
        fmt.Printf("Step completed with text: %s\n", data.Text)
        if data.Error != nil {
            fmt.Printf("Step error: %s\n", data.Error)
        }
        
        for _, toolCall := range data.ToolCalls {
            fmt.Printf("Tool call: %s\n", toolCall.Function.Name)
            if result, ok := data.ToolResults[toolCall.ID]; ok {
                fmt.Printf("Tool result: %v\n", result)
            }
        }
        
        return nil // Return nil if callback succeeds
    }
    
    onFinish := func(ctx context.Context, data *agent.RunResult) error {
        fmt.Printf("Agent execution completed\n")
        fmt.Printf("Final response: %s\n", data.Response)
        fmt.Printf("Message history length: %d\n", len(data.Conversation))
        if data.Error != nil {
            fmt.Printf("Execution error: %s\n", data.Error)
        }
        
        return nil // Return nil if callback succeeds
    }
    
    // Run agent with callbacks
    response, err := agent.RunWithCallbacks(ctx, &agent.RunOptions{
        ThreadID: "thread-123",
        Input: "Hello, can you help me?",
        OnStepFinish: onStepFinish,
        OnFinish: onFinish,
    })
    
    // Process response and error
}
```

## Integration with Actor Model

Gostra's callback system works seamlessly with the Actor model architecture. Here's how to leverage callbacks in distributed agent scenarios:

### Actor-Based Callback Handling

```go
// Define a supervisor actor that will receive callback messages
type SupervisorActor struct {
    context actor.Context
    // Other fields as needed
}

func (s *SupervisorActor) Receive(context actor.Context) {
    switch msg := context.Message().(type) {
    case *StepCompleteMessage:
        // Handle step completion
        fmt.Printf("Agent %s completed step with response: %s\n", msg.AgentID, msg.StepData.Text)
        
    case *ExecutionCompleteMessage:
        // Handle execution completion
        fmt.Printf("Agent %s execution completed\n", msg.AgentID)
        
    // Other message types
    }
}

// Create message types for actor communication
type StepCompleteMessage struct {
    AgentID  string
    StepData *agent.StepFinishData
}

type ExecutionCompleteMessage struct {
    AgentID string
    Data    *agent.RunResult
}

// In main code
func main() {
    // Create actor system
    system := actor.NewActorSystem()
    
    // Start supervisor actor
    supervisorProps := actor.PropsFromProducer(func() actor.Actor {
        return &SupervisorActor{}
    })
    supervisorPID, _ := system.Root.SpawnNamed(supervisorProps, "supervisor")
    
    // Create Gostra instance
    g := gostra.NewGostra(&gostra.Options{})
    
    // Register agent
    agent, _ := g.RegisterAgent("assistant", &agent.Options{
        // Agent configuration
    })
    
    // Create context
    ctx := context.Background()
    
    // Define callbacks that send messages to the supervisor
    onStepFinish := func(ctx context.Context, data *agent.StepFinishData) error {
        system.Root.Send(supervisorPID, &StepCompleteMessage{
            AgentID:  "assistant",
            StepData: data,
        })
        return nil
    }
    
    onFinish := func(ctx context.Context, data *agent.RunResult) error {
        system.Root.Send(supervisorPID, &ExecutionCompleteMessage{
            AgentID: "assistant",
            Data:    data,
        })
        return nil
    }
    
    // Run agent with callbacks
    agent.RunWithCallbacks(ctx, &agent.RunOptions{
        ThreadID: "thread-123",
        Input: "Hello, can you help me?",
        OnStepFinish: onStepFinish,
        OnFinish: onFinish,
    })
}
```

## Best Practices

1. **Keep callbacks lightweight**: Since callbacks are executed in the same execution flow as the agent processing, heavy operations should be delegated to separate goroutines or actors.

2. **Error handling**: While callbacks return errors, these are logged but don't affect the agent execution flow. Use error returns for situations where you want to notify about callback-specific issues.

3. **State management**: If callbacks need to maintain state between calls, use closures or dedicated state management structures.

4. **Timeouts**: For integrations with external systems, implement appropriate timeouts to prevent blocking the agent execution.

## Advanced Usage

### Callbacks for Distributed Tracing

```go
func createTracingCallbacks(traceID string) (agent.OnStepFinishFunc, agent.OnFinishFunc) {
    onStepFinish := func(ctx context.Context, data *agent.StepFinishData) error {
        span := tracer.StartSpan("agent_step", opentracing.ChildOf(parentSpanContext))
        defer span.Finish()
        
        span.SetTag("trace_id", traceID)
        span.SetTag("has_error", data.Error != nil)
        span.SetTag("tool_call_count", len(data.ToolCalls))
        
        if data.Error != nil {
            span.SetTag("error", true)
            span.LogFields(log.Error(data.Error))
        }
        
        return nil
    }
    
    onFinish := func(ctx context.Context, data *agent.RunResult) error {
        span := tracer.StartSpan("agent_execution_complete", opentracing.ChildOf(parentSpanContext))
        defer span.Finish()
        
        span.SetTag("trace_id", traceID)
        span.SetTag("has_error", data.Error != nil)
        span.SetTag("message_count", len(data.Conversation))
        
        if data.Error != nil {
            span.SetTag("error", true)
            span.LogFields(log.Error(data.Error))
        }
        
        return nil
    }
    
    return onStepFinish, onFinish
}
```

### Conditional Callback Execution

```go
func createConditionalCallbacks(condition func() bool) (agent.OnStepFinishFunc, agent.OnFinishFunc) {
    onStepFinish := func(ctx context.Context, data *agent.StepFinishData) error {
        if !condition() {
            return nil
        }
        // Proceed with callback logic
        return nil
    }
    
    onFinish := func(ctx context.Context, data *agent.RunResult) error {
        if !condition() {
            return nil
        }
        // Proceed with callback logic
        return nil
    }
    
    return onStepFinish, onFinish
}
```

## Conclusion

Gostra's callback system provides a flexible and powerful way to hook into agent execution flow. By leveraging callbacks, developers can implement custom business logic, monitoring, logging, and integration with external systems. When combined with Gostra's actor model architecture, callbacks enable sophisticated distributed agent systems with robust communication patterns. 