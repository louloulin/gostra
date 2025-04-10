package workflow

import (
	"context"
	"testing"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/louloulin/gostra/pkg/agent"
	"github.com/louloulin/gostra/pkg/models"
	"github.com/stretchr/testify/assert"
)

// TestModelProvider for testing
type TestModelProvider struct{}

func (m *TestModelProvider) Generate(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (string, error) {
	// Return a response based on the prompt
	for _, msg := range messages {
		if msg.Role == "user" {
			if contains(msg.Content, "analyze") || contains(msg.Content, "initial") {
				return "Analysis: The provided context contains key information about X and Y", nil
			} else if contains(msg.Content, "breakdown") || contains(msg.Content, "thinking") {
				return "Thought breakdown: The information about X relates to the query because...", nil
			} else if contains(msg.Content, "connect") || contains(msg.Content, "connecting") {
				return "Connection: X and Y are connected through Z, which implies...", nil
			} else if contains(msg.Content, "conclusion") {
				return "Conclusion: Based on the evidence, we can conclude that...", nil
			} else if contains(msg.Content, "final") || contains(msg.Content, "answer") {
				return "Final answer: After careful analysis, the answer to the query is...", nil
			}
		}
	}
	return "Generic response", nil
}

func (m *TestModelProvider) GenerateWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (*models.ResponseWithFunctionCalls, error) {
	text, err := m.Generate(ctx, messages, options)
	return &models.ResponseWithFunctionCalls{
		Text: text,
	}, err
}

func (m *TestModelProvider) GetID() string {
	return "test-model"
}

func (m *TestModelProvider) GetProvider() string {
	return "test"
}

func (m *TestModelProvider) Stream(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan string, error) {
	result, err := m.Generate(ctx, messages, options)
	ch := make(chan string, 1)
	if err != nil {
		close(ch)
		return ch, err
	}

	go func() {
		ch <- result
		close(ch)
	}()

	return ch, nil
}

func (m *TestModelProvider) StreamWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan *models.ResponseChunk, error) {
	result, err := m.Generate(ctx, messages, options)
	ch := make(chan *models.ResponseChunk, 1)
	if err != nil {
		close(ch)
		return ch, err
	}

	go func() {
		ch <- &models.ResponseChunk{
			Text:       result,
			IsFinished: true,
		}
		close(ch)
	}()

	return ch, nil
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return s != "" && s != " " && subset(s, substr)
}

// subset to avoid importing strings (which can cause conflicts)
func subset(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestChainOfThoughtWorkflow(t *testing.T) {
	// Create a test model provider
	model := &TestModelProvider{}

	// Create a simple workflow
	workflow := NewChainOfThoughtWorkflow("Test Workflow", "Testing chain of thought reasoning")

	// Add steps
	workflow.AddStep(&ChainOfThoughtStep{
		ID:            "step1",
		Name:          "Step 1",
		Description:   "First step",
		PromptFormat:  "Analyze: ${query}",
		OutputKey:     "step1_output",
		InputKeys:     []string{"query"},
		ModelProvider: model,
	})

	workflow.AddStep(&ChainOfThoughtStep{
		ID:            "step2",
		Name:          "Step 2",
		Description:   "Second step",
		PromptFormat:  "Process the output: ${step1_output}",
		OutputKey:     "step2_output",
		InputKeys:     []string{"step1_output"},
		ModelProvider: model,
	})

	// Run the workflow
	input := map[string]interface{}{
		"query": "What is the meaning of life?",
	}

	result, err := workflow.Run(context.Background(), input)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result["step1_output"], "Analysis")
	assert.Contains(t, result["step2_output"], "Thought breakdown")

	// Verify workflow status
	assert.Equal(t, StatusCompleted, workflow.Status)

	// Verify step results are stored
	step1Result := workflow.Workflow.GetStepResult("step1")
	assert.NotNil(t, step1Result)
	assert.Equal(t, StatusCompleted, step1Result.Status)

	step2Result := workflow.Workflow.GetStepResult("step2")
	assert.NotNil(t, step2Result)
	assert.Equal(t, StatusCompleted, step2Result.Status)

	// Also verify the string results in the ChainOfThoughtWorkflow's ResultsMap
	assert.Contains(t, workflow.ResultsMap["step1"], "Analysis")
	assert.Contains(t, workflow.ResultsMap["step2"], "Thought breakdown")
}

func TestRAGWorkflow(t *testing.T) {
	// Create a test model provider
	model := &TestModelProvider{}

	// Create actor system for network
	system := actor.NewActorSystem()

	// Create network
	networkOpts := &agent.AgentNetworkOptions{
		ID:          "test-network",
		Name:        "Test Network",
		ActorSystem: system,
	}

	network, err := agent.NewAgentNetwork(networkOpts, model)
	assert.NoError(t, err)

	// Create RAG workflow
	workflow := BuildRAGWorkflow("Test RAG Workflow", model, "rag_agent", network)

	// Register a test agent to handle RAG queries
	testAgent := &TestAgent{
		id: "rag_agent",
	}

	err = network.RegisterAgent("rag_agent", testAgent)
	assert.NoError(t, err)

	// Run the workflow
	input := map[string]interface{}{
		"query": "What are the key points in the document?",
	}

	// There's no real agent, so this will use the model provider directly
	result, err := workflow.Run(context.Background(), input)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check for expected outputs from each step
	assert.Contains(t, result["initialAnalysis"], "Analysis")
	assert.Contains(t, result["breakdown"], "Thought breakdown")
	assert.Contains(t, result["connections"], "Connection")
	assert.Contains(t, result["conclusions"], "Conclusion")
	assert.Contains(t, result["answer"], "Final answer")

	// Verify workflow status
	assert.Equal(t, StatusCompleted, workflow.Status)
}

// TestAgent implements actor.Actor for testing
type TestAgent struct {
	id string
}

func (a *TestAgent) Receive(context actor.Context) {
	switch msg := context.Message().(type) {
	case *agent.NetworkMessage:
		// Send back a response
		context.Respond(&agent.NetworkMessage{
			From:    a.id,
			To:      msg.From,
			Content: "Response from test agent: " + msg.Content,
		})
	case *agent.TransmitRequest:
		// Send back a response
		context.Respond(&agent.TransmitResponse{
			Results: []agent.AgentResponse{
				{
					Agent:   a.id,
					Content: "Response from test agent for: " + msg.Message,
				},
			},
		})
	}
}

func TestDynamicCOTWorkflow(t *testing.T) {
	// Create a test model provider
	model := &TestModelProvider{}

	// Create actor system for network
	system := actor.NewActorSystem()

	// Create network
	networkOpts := &agent.AgentNetworkOptions{
		ID:          "test-dynamic-network",
		Name:        "Test Dynamic Network",
		ActorSystem: system,
	}

	network, err := agent.NewAgentNetwork(networkOpts, model)
	assert.NoError(t, err)

	// Register a test agent to handle RAG queries
	testAgent := &TestAgent{
		id: "rag_agent",
	}

	err = network.RegisterAgent("rag_agent", testAgent)
	assert.NoError(t, err)

	// Create custom steps
	customSteps := []map[string]interface{}{
		{
			"id":          "customInit",
			"name":        "Initial Analysis",
			"description": "Custom initial analysis step",
			"prompt":      "Analyze the following query: ${query}",
			"output_key":  "initialResult",
			"input_keys":  []interface{}{"query"},
		},
		{
			"id":          "customFinal",
			"name":        "Final Response",
			"description": "Custom final response step",
			"prompt":      "Based on your analysis (${initialResult}), provide a final answer to: ${query}",
			"output_key":  "finalResult",
			"input_keys":  []interface{}{"query", "initialResult"},
		},
	}

	// Create workflow options with custom steps
	options := &WorkflowOptions{
		Name:        "Custom Dynamic COT",
		Description: "Test workflow with custom COT steps",
		Metadata: map[string]interface{}{
			"steps": customSteps,
		},
	}

	// Build the dynamic workflow
	workflow := BuildDynamicCOTWorkflow(options, model, "rag_agent", network)

	// Run the workflow
	input := map[string]interface{}{
		"query": "Test query for custom workflow",
	}

	result, err := workflow.Run(context.Background(), input)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check expected keys based on custom step output_key values
	assert.Contains(t, result, "initialResult")
	assert.Contains(t, result, "finalResult")

	// Verify only the custom steps were added
	assert.Equal(t, 2, len(workflow.Steps))
	assert.Equal(t, "customInit", workflow.Steps[0].ID)
	assert.Equal(t, "customFinal", workflow.Steps[1].ID)

	// Verify workflow status
	assert.Equal(t, StatusCompleted, workflow.Status)
}

func TestChainOfThoughtWorkflowWithSchema(t *testing.T) {
	// Create a test model provider
	model := &TestModelProvider{}

	// Create a workflow with schema validation
	workflow := NewChainOfThoughtWorkflow("Schema Test Workflow", "Testing chain of thought with schema validation")

	// Add custom schemas
	inputSchema := NewSchema(TypeObject, "Custom input schema")
	nameSchema := NewSimpleSchema(TypeString, "Name field")
	nameSchema.MinLength = 3
	inputSchema.AddProperty("name", nameSchema, true)

	promptSchema := NewSimpleSchema(TypeString, "Prompt field")
	promptSchema.MinLength = 5
	inputSchema.AddProperty("query", promptSchema, true)

	// Set custom input schema
	workflow.SetInputSchema(inputSchema)

	// Create actor system for network
	system := actor.NewActorSystem()

	// Create network
	networkOpts := &agent.AgentNetworkOptions{
		ID:          "test-schema-network",
		Name:        "Test Schema Network",
		ActorSystem: system,
	}

	network, err := agent.NewAgentNetwork(networkOpts, model)
	assert.NoError(t, err)

	// Add steps
	workflow.AddStep(&ChainOfThoughtStep{
		ID:            "step1",
		Name:          "Step 1",
		Description:   "First step",
		PromptFormat:  "Analyze name: ${name}, query: ${query}",
		OutputKey:     "step1_output",
		InputKeys:     []string{"name", "query"},
		ModelProvider: model,
		Network:       network,
		AgentID:       "", // No agent ID, use model directly
	})

	// Valid input
	validInput := map[string]interface{}{
		"name":  "John Doe",
		"query": "What is the meaning of life?",
	}

	// Should succeed
	result, err := workflow.Run(context.Background(), validInput)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result["step1_output"], "Analysis")

	// Invalid input (short name)
	invalidInput1 := map[string]interface{}{
		"name":  "Jo", // Too short
		"query": "What is the meaning of life?",
	}

	// Should fail validation
	_, err = workflow.Run(context.Background(), invalidInput1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
	assert.Contains(t, err.Error(), "length")

	// Invalid input (missing query)
	invalidInput2 := map[string]interface{}{
		"name": "John Doe",
		// Missing query
	}

	// Should fail validation
	_, err = workflow.Run(context.Background(), invalidInput2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
	assert.Contains(t, err.Error(), "required")

	// Set output schema
	outputSchema := NewSchema(TypeObject, "Custom output schema")
	outputField := NewSimpleSchema(TypeString, "Output field")
	outputField.MinLength = 1000 // Deliberately too long to trigger validation warnings
	outputSchema.AddProperty("step1_output", outputField, true)
	workflow.SetOutputSchema(outputSchema)

	// Should run but log validation warnings about output
	result, err = workflow.Run(context.Background(), validInput)
	assert.NoError(t, err) // Validation warnings don't cause errors
	assert.NotNil(t, result)
}
