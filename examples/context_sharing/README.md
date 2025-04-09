# Context Sharing in Agent Networks

This example demonstrates how to use Gostra's context sharing feature to enable collaborative agent workflows.

## Overview

Agent context sharing is the ability for agents in a network to maintain shared state, passing information through a conversation as they work together on tasks. This capability is critical for building complex agent workflows where multiple specialized agents need to contribute different skills while maintaining awareness of the overall conversation.

The example includes:

1. A `ResearchAgent` that simulates finding information on a given topic
2. A `SummaryAgent` that takes research results and generates a summary
3. A demonstration of passing context between these agents through the network

## Key Features

- **Automatic Context Merging**: Context from different agents is automatically merged and tracked
- **Conversation Tracing**: A detailed trace of message flow is maintained
- **Agent Contributions**: Individual agent contributions are recorded with timestamps
- **Stateful Workflows**: The system maintains state across multiple request-response cycles
- **Context Persistence**: Context can be stored and retrieved across sessions

## How Context Sharing Works

The context sharing system in Gostra works through these main components:

1. **ContextHandler**: Manages storage, retrieval, and expiration of context data
2. **NetworkMessage**: Carries context data in its `Data` field 
3. **AgentNetwork**: Uses the context handler to enrich messages and process responses
4. **RouterAgent**: Handles passing context between agents in both sequential and parallel call patterns

## Running the Example

To run this example:

```bash
cd gostra
go run examples/context_sharing/main.go
```

## Example Output

The example will show:

1. Context being passed from the user to the research agent
2. Research results being added to the context
3. Context being passed from the research agent to the summary agent
4. The final summary being generated with all accumulated context
5. A detailed conversation trace showing the message flow

## Implementation Details

The context sharing in this example is implemented through several mechanisms:

### Context Handling Interface

The core functionality is built around the `ContextHandler` which provides:

```go
// Key methods
func (ch *ContextHandler) SetContext(conversationID, key string, value interface{}, agentID string)
func (ch *ContextHandler) GetContext(conversationID, key string) (interface{}, bool)
func (ch *ContextHandler) GetAllContext(conversationID string) map[string]interface{}
func (ch *ContextHandler) MergeContext(conversationID string, values map[string]interface{}, agentID string)
func (ch *ContextHandler) EnrichNetworkMessage(conversationID string, msg *NetworkMessage)
func (ch *ContextHandler) ProcessContextFromMessage(conversationID string, msg *NetworkMessage)
```

### Automatic Context Management

Most of the context handling is done automatically by the router when processing agent requests:

1. Each agent's response data is merged into the context
2. Conversation traces are maintained automatically 
3. Agent contributions are tracked with timestamps
4. Context keys are preserved across agent calls

### Building Your Own Context-Aware Agents

When building agents that can effectively participate in context sharing:

1. Agents should always check their incoming message's `Data` field for context
2. When sending a response, include relevant information in the message's `Data` field
3. Structure your agent's context contributions to avoid key conflicts
4. Consider using agent-specific prefixes for context keys

## Example Agent Implementation

Here's a simplified version of how agents work with context:

```go
func (a *MyAgent) Receive(ctx actor.Context) {
    switch msg := ctx.Message().(type) {
    case *agent.NetworkMessage:
        // Extract context from the incoming message
        inputContext := make(map[string]interface{})
        if msg.Data != nil {
            if existingData, ok := msg.Data.(map[string]interface{}); ok {
                for k, v := range existingData {
                    inputContext[k] = v
                }
            }
        }
        
        // Process the message using the context
        result := processWithContext(msg.Content, inputContext)
        
        // Add new information to the context
        inputContext["my_agent.result"] = result
        
        // Send response with updated context
        response := &agent.NetworkMessage{
            From:    "my_agent",
            To:      msg.From,
            Content: "Processed result",
            Data:    inputContext,
        }
        
        ctx.Respond(response)
    }
}
```

## Best Practices

When using context sharing:

1. Use consistent key naming conventions to avoid conflicts
2. Structure complex data as nested maps for better organization
3. Consider using JSON-serializable values for cross-system compatibility
4. Use conversation IDs to isolate different user sessions
5. Consider TTL settings for context that should expire after a certain time

For more details, see the API documentation in the `agent` package.

## Future Improvements

The current context sharing implementation offers a solid foundation, but could be extended with the following features:

1. **Database-Backed Persistence**
   - Support for storing context in PostgreSQL or other databases
   - Integration with vector stores for semantic context lookup
   - Automatic archiving of old contexts

2. **Context Expiration Policies**
   - More granular TTL controls at the key level
   - Policy-based expiration (e.g., expire all keys with a certain prefix)
   - Support for conditional expiration based on access patterns

3. **Enhanced Security**
   - Encryption of sensitive context data
   - Access control for context keys based on agent permissions
   - Audit logging for context access and modifications

4. **Conflict Resolution Strategies**
   - More sophisticated conflict resolution for parallel agent execution
   - Support for custom merge functions per key or key pattern
   - Locking mechanisms for critical context updates

5. **Improved Testing and Monitoring**
   - Performance benchmarks for context operations
   - Monitoring of context size and access patterns
   - Stress testing with large numbers of agents and context entries

6. **UI and Debugging Tools**
   - Web UI for viewing and editing context data
   - Visualization of context flow between agents
   - Context diff tools for debugging

These improvements would enhance the robustness and utility of the context sharing system for more complex agent workflows. 