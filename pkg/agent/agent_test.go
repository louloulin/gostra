package agent

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/google/uuid"
	"github.com/louloulin/gostra/pkg/memory"
	"github.com/louloulin/gostra/pkg/models"
	"github.com/louloulin/gostra/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Mock ModelProvider ---

type MockModelProvider struct {
	mock.Mock
}

func (m *MockModelProvider) Generate(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (string, error) {
	args := m.Called(ctx, messages, options)
	return args.String(0), args.Error(1)
}

func (m *MockModelProvider) Stream(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan string, error) {
	args := m.Called(ctx, messages, options)
	return args.Get(0).(<-chan string), args.Error(1)
}

func (m *MockModelProvider) GenerateWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (*models.ResponseWithFunctionCalls, error) {
	args := m.Called(ctx, messages, options)
	res := args.Get(0)
	if res == nil {
		return nil, args.Error(1)
	}
	return res.(*models.ResponseWithFunctionCalls), args.Error(1)
}

func (m *MockModelProvider) StreamWithFunctionCalls(ctx context.Context, messages []models.Message, options *models.GenerateOptions) (<-chan *models.ResponseChunk, error) {
	args := m.Called(ctx, messages, options)
	res := args.Get(0)
	if res == nil {
		return nil, args.Error(1)
	}
	return res.(<-chan *models.ResponseChunk), args.Error(1)
}

func (m *MockModelProvider) GetID() string {
	args := m.Called()
	if args.Get(0) != nil {
		return args.String(0)
	}
	return "mock-model"
}

func (m *MockModelProvider) GetProvider() string {
	args := m.Called()
	if args.Get(0) != nil {
		return args.String(0)
	}
	return "mock"
}

// --- Mock MemoryProvider ---

type MockMemoryProvider struct {
	mock.Mock
	mu       sync.Mutex
	threads  map[string]*memory.Thread    // Added for thread operations
	messages map[string][]*memory.Message // Changed to slice of pointers
}

func NewMockMemoryProvider() *MockMemoryProvider {
	return &MockMemoryProvider{
		threads:  make(map[string]*memory.Thread), // Initialize threads map
		messages: make(map[string][]*memory.Message),
	}
}

// Implement missing MemoryProvider methods
func (m *MockMemoryProvider) CreateThread(ctx context.Context, metadata map[string]interface{}) (*memory.Thread, error) {
	args := m.Called(ctx, metadata)
	res := args.Get(0)
	if res == nil {
		// Default mock behavior if no return specified
		m.mu.Lock()
		defer m.mu.Unlock()
		threadID := uuid.NewString()
		thread := &memory.Thread{ID: threadID, Metadata: metadata, CreatedAt: time.Now()}
		m.threads[threadID] = thread
		m.messages[threadID] = []*memory.Message{}
		return thread, args.Error(1)
	}
	return res.(*memory.Thread), args.Error(1)
}

func (m *MockMemoryProvider) GetThread(ctx context.Context, threadID string) (*memory.Thread, error) {
	args := m.Called(ctx, threadID)
	res := args.Get(0)
	if res == nil {
		// Default mock behavior
		m.mu.Lock()
		defer m.mu.Unlock()
		thread, ok := m.threads[threadID]
		if !ok {
			return nil, errors.New("thread not found")
		}
		return thread, args.Error(1)
	}
	return res.(*memory.Thread), args.Error(1)
}

func (m *MockMemoryProvider) UpdateThread(ctx context.Context, threadID string, metadata map[string]interface{}) (*memory.Thread, error) {
	args := m.Called(ctx, threadID, metadata)
	res := args.Get(0)
	if res == nil {
		// Default mock behavior
		m.mu.Lock()
		defer m.mu.Unlock()
		thread, ok := m.threads[threadID]
		if !ok {
			return nil, errors.New("thread not found")
		}
		thread.Metadata = metadata
		return thread, args.Error(1)
	}
	return res.(*memory.Thread), args.Error(1)
}

func (m *MockMemoryProvider) ListThreads(ctx context.Context, limit int, offset int) ([]*memory.Thread, error) {
	args := m.Called(ctx, limit, offset)
	res := args.Get(0)
	if res == nil {
		// Default mock behavior
		m.mu.Lock()
		defer m.mu.Unlock()
		threads := make([]*memory.Thread, 0, len(m.threads))
		for _, thread := range m.threads {
			threads = append(threads, thread)
		}
		// Simple pagination for mock
		start := offset
		if start >= len(threads) {
			return []*memory.Thread{}, args.Error(1)
		}
		end := start + limit
		if end > len(threads) {
			end = len(threads)
		}
		return threads[start:end], args.Error(1)
	}
	return res.([]*memory.Thread), args.Error(1)
}

func (m *MockMemoryProvider) DeleteMessages(ctx context.Context, threadID string, messageIDs []string) error {
	args := m.Called(ctx, threadID, messageIDs)
	// Default mock behavior
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.messages[threadID]; !ok {
		return errors.New("thread not found")
	}
	idMap := make(map[string]bool)
	for _, id := range messageIDs {
		idMap[id] = true
	}
	filtered := make([]*memory.Message, 0)
	for _, msg := range m.messages[threadID] {
		if !idMap[msg.ID] {
			filtered = append(filtered, msg)
		}
	}
	m.messages[threadID] = filtered
	return args.Error(0)
}

// Add missing DeleteThread method
func (m *MockMemoryProvider) DeleteThread(ctx context.Context, threadID string) error {
	args := m.Called(ctx, threadID)
	// Default mock behavior
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.threads[threadID]; ok {
		delete(m.threads, threadID)
		delete(m.messages, threadID)
		return args.Error(0)
	}
	return errors.New("thread not found")
}

// Correct AddMessage signature and implementation
func (m *MockMemoryProvider) AddMessage(ctx context.Context, threadID string, role string, content string, metadata map[string]interface{}) (*memory.Message, error) {
	args := m.Called(ctx, threadID, role, content, metadata)
	res := args.Get(0)
	if res == nil {
		// Default mock behavior
		m.mu.Lock()
		defer m.mu.Unlock()
		if _, ok := m.threads[threadID]; !ok {
			// Optionally create thread if it doesn't exist for testing convenience
			thread := &memory.Thread{ID: threadID, CreatedAt: time.Now()}
			m.threads[threadID] = thread
			m.messages[threadID] = []*memory.Message{}
		}
		msg := &memory.Message{
			ID:        uuid.NewString(),
			Role:      role,
			Content:   content,
			Metadata:  metadata,
			CreatedAt: time.Now(),
		}
		m.messages[threadID] = append(m.messages[threadID], msg)
		return msg, args.Error(1)
	}
	return res.(*memory.Message), args.Error(1)
}

// Correct GetMessages signature and implementation
func (m *MockMemoryProvider) GetMessages(ctx context.Context, threadID string, limit, offset int) ([]*memory.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	args := m.Called(ctx, threadID, limit, offset)
	res := args.Get(0)   // Get the first return argument defined by On()
	err := args.Error(1) // Get the second return argument (error)

	if res != nil {
		// If a specific return value was set in the test, use it
		return res.([]*memory.Message), err
	}

	// Default mock behavior if no specific return value was set
	msgs, exists := m.messages[threadID]
	if !exists {
		return []*memory.Message{}, err // Return potentially configured error or nil
	}
	// Simple pagination for mock
	start := offset
	if start >= len(msgs) {
		return []*memory.Message{}, err
	}
	end := start + limit
	if end > len(msgs) {
		end = len(msgs)
	}
	return msgs[start:end], err
}

// --- Mock Tool ---

type MockTool struct {
	mock.Mock
}

func (m *MockTool) GetID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockTool) GetDescription() string {
	args := m.Called()
	return args.String(0)
}

// mockSchema implements tools.Schema
type mockSchema struct{}

// Implement JSONSchema method
func (s mockSchema) JSONSchema() (map[string]interface{}, error) {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{}, // Empty properties for mock
	}, nil
}

func (s mockSchema) Validate(input map[string]interface{}) error { // Changed input type
	return nil
}

func (m *MockTool) GetInputSchema() tools.Schema {
	m.Called()
	return mockSchema{}
}

func (m *MockTool) Execute(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error) {
	args := m.Called(params, options)
	res := args.Get(0)
	err := args.Error(1)
	// Check if res is nil before trying to assert type
	if res == nil {
		return nil, err
	}
	return res, err
}

// --- Test Suite Setup ---

func setupTestAgent(t *testing.T) (*Agent, *MockModelProvider, *MockMemoryProvider, *MockTool) {
	mockModel := new(MockModelProvider)
	mockMemory := NewMockMemoryProvider() // Use constructor
	mockTool := new(MockTool)

	opts := &Options{
		ID:             "test-agent",
		Name:           "Test Agent",
		SystemPrompt:   "You are a test agent.",
		ModelProvider:  mockModel,
		MemoryProvider: mockMemory,
		Tools:          []tools.Tool{mockTool},
		MaxTokens:      100,
	}

	agent, err := NewAgent(opts)
	assert.NoError(t, err)
	assert.NotNil(t, agent)

	// Setup default mock tool behavior
	mockTool.On("GetID").Return("mock_tool")
	mockTool.On("GetDescription").Return("A mock tool")
	mockTool.On("GetInputSchema").Return(mockSchema{})

	return agent, mockModel, mockMemory, mockTool
}

// --- Test Cases ---

func TestAgent_Run_NoTools(t *testing.T) {
	agent, mockModel, mockMemory, _ := setupTestAgent(t)
	ctx := context.Background()
	threadID := "thread-no-tools"
	userInput := "Hello"

	// Mock MemoryProvider interactions
	mockMemory.On("AddMessage", ctx, threadID, "user", userInput, mock.Anything).Return("msg1", nil)
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]memory.Message{
		{Role: "user", Content: userInput},
	}, nil).Once() // Expect GetMessages before model call
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "Response: Hello", mock.Anything).Return("msg2", nil) // Expect saving assistant message

	// Mock ModelProvider interaction (no tool calls expected)
	modelResponse := &models.ResponseWithFunctionCalls{
		Text:         "Response: Hello",
		FinishReason: "stop",
	}
	mockModel.On("GenerateWithFunctionCalls", ctx, mock.AnythingOfType("[]models.Message"), mock.AnythingOfType("*models.GenerateOptions")).Return(modelResponse, nil).Once()

	runOpts := &RunOptions{
		ThreadID: threadID,
		Input:    userInput,
	}

	finalResponse, err := agent.Run(ctx, runOpts)

	assert.NoError(t, err)
	assert.Equal(t, "Response: Hello", finalResponse)

	// Verify mocks
	mockModel.AssertExpectations(t)
	mockMemory.AssertExpectations(t)
}

func TestAgent_Run_SingleToolCall(t *testing.T) {
	agent, mockModel, mockMemory, mockTool := setupTestAgent(t)
	ctx := context.Background()
	threadID := "thread-single-tool"
	userInput := "Use the tool"

	// Mock MemoryProvider interactions
	// Use *memory.Message for return values where applicable
	mockMemory.On("AddMessage", ctx, threadID, "user", userInput, mock.Anything).Return(&memory.Message{ID: "msg1", Role: "user", Content: userInput}, nil).Once()
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]*memory.Message{
		{Role: "user", Content: userInput},
	}, nil).Once() // First GetMessages
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "Okay, using tool.", mock.MatchedBy(func(metadata map[string]interface{}) bool {
		_, ok := metadata["tool_calls"]
		return ok // Check if tool_calls metadata exists
	})).Return(&memory.Message{ID: "msg2"}, nil).Once() // Save assistant message with tool call intent
	mockMemory.On("AddMessage", ctx, threadID, "tool", `{"result":"tool success"}`, mock.MatchedBy(func(metadata map[string]interface{}) bool {
		id, okId := metadata["tool_call_id"].(string)
		name, okName := metadata["tool_name"].(string)
		return okId && okName && id == "call1" && name == "mock_tool"
	})).Return(&memory.Message{ID: "msg3"}, nil).Once() // Save tool result message
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]*memory.Message{ // Second GetMessages after tool call
		{Role: "user", Content: userInput},
		{Role: "assistant", Content: "Okay, using tool.", Metadata: map[string]interface{}{"tool_calls": []interface{}{ /* dummy */ }}},
		// Removed Name field from memory.Message literal
		{Role: "tool", Content: `{"result":"tool success"}`, Metadata: map[string]interface{}{"tool_call_id": "call1", "tool_name": "mock_tool"}},
	}, nil).Once()
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "Tool finished.", mock.Anything).Return(&memory.Message{ID: "msg4"}, nil).Once() // Save final assistant message

	// Mock ModelProvider interactions
	// 1. First call: Model decides to use the tool
	toolCallArgs, _ := json.Marshal(map[string]interface{}{"input": "data"})
	modelResponse1 := &models.ResponseWithFunctionCalls{
		Text: "Okay, using tool.",
		ToolCalls: []models.ToolCall{
			{ID: "call1", Type: "function", Function: models.FunctionCall{Name: "mock_tool", Arguments: string(toolCallArgs)}},
		},
		FinishReason: "tool_calls",
	}
	mockModel.On("GenerateWithFunctionCalls", ctx, mock.MatchedBy(func(msgs []models.Message) bool {
		return len(msgs) > 0 && msgs[len(msgs)-1].Role == "user" // Check last message is user
	}), mock.Anything).Return(modelResponse1, nil).Once()

	// 2. Second call: Model responds after getting tool result
	modelResponse2 := &models.ResponseWithFunctionCalls{
		Text:         "Tool finished.",
		FinishReason: "stop",
	}
	mockModel.On("GenerateWithFunctionCalls", ctx, mock.MatchedBy(func(msgs []models.Message) bool {
		// Check last message is tool result
		if len(msgs) == 0 {
			return false
		}
		lastMsg := msgs[len(msgs)-1]
		return lastMsg.Role == "tool"
	}), mock.Anything).Return(modelResponse2, nil).Once()

	// Mock Tool execution
	mockTool.On("Execute", map[string]interface{}{"input": "data"}, mock.AnythingOfType("*tools.ExecuteOptions")).Return(map[string]string{"result": "tool success"}, nil).Once()
	// Mock tool schema validation
	mockTool.On("GetInputSchema").Return(mockSchema{}) // Ensure GetInputSchema is mocked

	runOpts := &RunOptions{
		ThreadID:            threadID,
		Input:               userInput,
		AvailableTools:      []tools.Tool{mockTool}, // Provide the tool
		MaxConsecutiveCalls: 5,                      // Allow tool calls
	}

	finalResponse, err := agent.Run(ctx, runOpts)

	assert.NoError(t, err)
	assert.Equal(t, "Tool finished.", finalResponse)

	// Verify mocks
	mockModel.AssertExpectations(t)
	mockMemory.AssertExpectations(t)
	mockTool.AssertExpectations(t)
}

func TestAgent_RunWithCallbacks_OnStepFinish(t *testing.T) {
	agent, mockModel, mockMemory, mockTool := setupTestAgent(t)
	ctx := context.Background()
	threadID := "thread-callbacks-step"
	userInput := "Use the tool with callbacks"

	// Mock MemoryProvider interactions
	mockMemory.On("AddMessage", ctx, threadID, "user", userInput, mock.Anything).Return(&memory.Message{ID: "msg1"}, nil).Once()
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]*memory.Message{
		{Role: "user", Content: userInput},
	}, nil).Once()
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "Using the tool.", mock.Anything).Return(&memory.Message{ID: "msg2"}, nil).Once()
	mockMemory.On("AddMessage", ctx, threadID, "tool", `{"result":"tool result"}`, mock.Anything).Return(&memory.Message{ID: "msg3"}, nil).Once()
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]*memory.Message{
		{Role: "user", Content: userInput},
		{Role: "assistant", Content: "Using the tool.", Metadata: map[string]interface{}{"tool_calls": []interface{}{}}},
		{Role: "tool", Content: `{"result":"tool result"}`, Metadata: map[string]interface{}{"tool_call_id": "call1", "tool_name": "mock_tool"}},
	}, nil).Once()
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "Final response", mock.Anything).Return(&memory.Message{ID: "msg4"}, nil).Once()

	// Mock ModelProvider
	toolCallArgs, _ := json.Marshal(map[string]interface{}{"input": "step-data"})
	modelResponse1 := &models.ResponseWithFunctionCalls{
		Text: "Using the tool.",
		ToolCalls: []models.ToolCall{
			{ID: "call1", Type: "function", Function: models.FunctionCall{Name: "mock_tool", Arguments: string(toolCallArgs)}},
		},
		FinishReason: "tool_calls",
	}
	mockModel.On("GenerateWithFunctionCalls", mock.Anything, mock.Anything, mock.Anything).Return(modelResponse1, nil).Once()

	modelResponse2 := &models.ResponseWithFunctionCalls{
		Text:         "Final response",
		FinishReason: "stop",
	}
	mockModel.On("GenerateWithFunctionCalls", mock.Anything, mock.Anything, mock.Anything).Return(modelResponse2, nil).Once()

	// Mock Tool execution
	mockTool.On("Execute", map[string]interface{}{"input": "step-data"}, mock.Anything).Return(map[string]string{"result": "tool result"}, nil).Once()
	mockTool.On("GetInputSchema").Return(mockSchema{}) // Ensure GetInputSchema is mocked

	// Track callbacks
	var stepFinishCalled bool
	var stepData *StepFinishData // Changed to pointer

	runOpts := &RunOptions{
		ThreadID:            threadID,
		Input:               userInput,
		AvailableTools:      []tools.Tool{mockTool},
		MaxConsecutiveCalls: 5,
		// Correct callback signature
		OnStepFinish: func(ctx context.Context, data *StepFinishData) error {
			stepFinishCalled = true
			stepData = data // Assign the pointer
			return nil      // Return nil error
		},
	}

	finalResponse, err := agent.RunWithCallbacks(ctx, runOpts)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, "Final response", finalResponse)
	assert.True(t, stepFinishCalled, "OnStepFinish callback was not called")
	assert.NotNil(t, stepData, "Step data should not be nil") // Check pointer is not nil
	assert.NotEmpty(t, stepData.Text, "Step text should not be empty")
	assert.NotEmpty(t, stepData.ToolCalls, "Tool calls should not be empty")
	assert.Equal(t, "mock_tool", stepData.ToolCalls[0].Function.Name, "Tool name should match")
	assert.NotEmpty(t, stepData.ToolResults, "Tool results should not be empty")

	// Verify mocks
	mockModel.AssertExpectations(t)
	mockMemory.AssertExpectations(t)
	mockTool.AssertExpectations(t)
}

func TestAgent_RunWithCallbacks_OnFinish(t *testing.T) {
	agent, mockModel, mockMemory, mockTool := setupTestAgent(t)
	ctx := context.Background()
	threadID := "thread-callbacks-finish"
	userInput := "Process this with onFinish"

	// Mock MemoryProvider interactions
	mockMemory.On("AddMessage", ctx, threadID, "user", userInput, mock.Anything).Return(&memory.Message{ID: "msg1"}, nil).Once()
	// First GetMessages for model call
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]*memory.Message{
		{Role: "user", Content: userInput},
	}, nil).Once()
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "Final response", mock.Anything).Return(&memory.Message{ID: "msg2"}, nil).Once()
	// Second GetMessages for conversation history in callback
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]*memory.Message{
		{Role: "user", Content: userInput},
		{Role: "assistant", Content: "Final response"},
	}, nil).Once()

	// Mock ModelProvider
	modelResponse := &models.ResponseWithFunctionCalls{
		Text:         "Final response",
		FinishReason: "stop",
	}
	mockModel.On("GenerateWithFunctionCalls", mock.Anything, mock.Anything, mock.Anything).Return(modelResponse, nil).Once()

	// Track callbacks
	var finishCalled bool
	var finishData *RunResult // Changed to pointer

	runOpts := &RunOptions{
		ThreadID:       threadID,
		Input:          userInput,
		AvailableTools: []tools.Tool{mockTool},
		// Correct callback signature
		OnFinish: func(ctx context.Context, data *RunResult) error {
			finishCalled = true
			finishData = data // Assign pointer
			return nil        // Return nil error
		},
	}

	finalResponse, err := agent.RunWithCallbacks(ctx, runOpts)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, "Final response", finalResponse)
	assert.True(t, finishCalled, "OnFinish callback was not called")
	assert.NotNil(t, finishData, "Finish data should not be nil") // Check pointer
	assert.Equal(t, finalResponse, finishData.Response, "Final response should match")
	assert.Equal(t, 0, finishData.NumberOfSteps, "Number of steps should be 0")
	assert.Len(t, finishData.Conversation, 2, "Conversation length should be 2")

	// Verify mocks
	mockModel.AssertExpectations(t)
	mockMemory.AssertExpectations(t)
}

// TestActorCallback tests the ActorCallbackMessage with the actor system
func TestActorCallback(t *testing.T) {
	// Create a new actor system
	system := actor.NewActorSystem()

	// Setup test components
	agent, mockModel, mockMemory, mockTool := setupTestAgent(t)
	ctx := context.Background()
	threadID := "thread-actor-callback"
	userInput := "Use a tool with actor callback"

	// Create actor props and spawn actor
	props := NewAgentActor(agent)
	pid := system.Root.Spawn(props)

	// Set up callback tracking
	stepFinishCalled := false // Now used
	finishCalled := false
	var resultText string

	// Setup run options with callbacks using correct signatures
	runOpts := &RunOptions{
		ThreadID:            threadID,
		Input:               userInput,
		AvailableTools:      []tools.Tool{mockTool},
		MaxConsecutiveCalls: 3,
		OnStepFinish: func(ctx context.Context, data *StepFinishData) error { // Correct signature
			stepFinishCalled = true
			return nil
		},
		OnFinish: func(ctx context.Context, data *RunResult) error { // Correct signature
			finishCalled = true
			return nil
		},
	}

	// Mock MemoryProvider interactions for the test
	mockMemory.On("AddMessage", ctx, threadID, "user", userInput, mock.Anything).Return(&memory.Message{ID: "msg1"}, nil)
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]*memory.Message{
		{Role: "user", Content: userInput},
	}, nil).Once() // For model call
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]*memory.Message{ // For finish callback conversation
		{Role: "user", Content: userInput},
		{Role: "assistant", Content: "Actor callback test response"},
	}, nil).Once()

	// Mock ModelProvider to return a simple response without tool calls
	modelResponse := &models.ResponseWithFunctionCalls{
		Text:         "Actor callback test response",
		FinishReason: "stop",
	}
	mockModel.On("GenerateWithFunctionCalls", mock.Anything, mock.Anything, mock.Anything).Return(modelResponse, nil).Once()

	// Mock for adding the assistant message
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "Actor callback test response", mock.Anything).Return(&memory.Message{ID: "msg2"}, nil)

	// Send the callback message to the actor
	callbackMsg := &ActorCallbackMessage{
		RunOptions: runOpts,
		Context:    ctx,
	}

	// Request response via actor message
	future := system.Root.RequestFuture(pid, callbackMsg, 5*time.Second)
	result, err := future.Result()

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	resultText, ok := result.(string)
	assert.True(t, ok)
	assert.Equal(t, "Actor callback test response", resultText)

	// Give some time for callbacks to execute (in real system this would be synchronous)
	time.Sleep(100 * time.Millisecond)

	// Verify callbacks were called
	assert.False(t, stepFinishCalled, "onStepFinish should NOT have been called (no steps)") // Assert it wasn't called
	assert.True(t, finishCalled, "onFinish should have been called")

	// Verify mocks
	mockModel.AssertExpectations(t)
	mockMemory.AssertExpectations(t)
}

// Implementing the first test from the TODO list
func TestAgent_Run_MaxConsecutiveCalls(t *testing.T) {
	agent, mockModel, mockMemory, mockTool := setupTestAgent(t)
	ctx := context.Background()
	threadID := "thread-max-consecutive-calls"
	userInput := "Use the tool repeatedly"

	// Mock MemoryProvider interactions
	mockMemory.On("AddMessage", ctx, threadID, "user", userInput, mock.Anything).Return("msg1", nil)
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]memory.Message{
		{Role: "user", Content: userInput},
	}, nil).Once() // First GetMessages

	// Set up model to always return tool calls
	// This will create a situation where the model always wants to call tools,
	// so we can test the max consecutive calls limit
	toolCallArgs, _ := json.Marshal(map[string]interface{}{"input": "test-data"})

	// Create a model response that always requests a tool call
	toolCallResponse := &models.ResponseWithFunctionCalls{
		Text: "I'll use the tool again.",
		ToolCalls: []models.ToolCall{
			{ID: "call1", Type: "function", Function: models.FunctionCall{Name: "mock_tool", Arguments: string(toolCallArgs)}},
		},
		FinishReason: "tool_calls",
	}

	// Make the model always return a tool call response
	// This will be called up to MaxConsecutiveCalls times
	mockModel.On("GenerateWithFunctionCalls", mock.Anything, mock.Anything, mock.Anything).Return(toolCallResponse, nil)

	// Set up mock memory for each round
	// 1. For the first tool call
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "I'll use the tool again.", mock.Anything).Return("msg2", nil)
	mockMemory.On("AddMessage", ctx, threadID, "tool", mock.Anything, mock.Anything).Return("msg3", nil)

	// 2. For the second round of messages retrieval
	mockMessages2 := []memory.Message{
		{Role: "user", Content: userInput},
		{Role: "assistant", Content: "I'll use the tool again.", Metadata: map[string]interface{}{"tool_calls": []interface{}{}}},
		{Role: "tool", Content: `{"result":"tool result"}`, Metadata: map[string]interface{}{"tool_call_id": "call1", "tool_name": "mock_tool"}},
	}
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return(mockMessages2, nil).Once()

	// 3. For the second tool call
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "I'll use the tool again.", mock.Anything).Return("msg4", nil)
	mockMemory.On("AddMessage", ctx, threadID, "tool", mock.Anything, mock.Anything).Return("msg5", nil)

	// 4. For the third round of messages retrieval
	mockMessages3 := append(mockMessages2,
		memory.Message{Role: "assistant", Content: "I'll use the tool again.", Metadata: map[string]interface{}{"tool_calls": []interface{}{}}},
		memory.Message{Role: "tool", Content: `{"result":"tool result"}`, Metadata: map[string]interface{}{"tool_call_id": "call1", "tool_name": "mock_tool"}},
	)
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return(mockMessages3, nil).Once()

	// Mock Tool execution - will be called MaxConsecutiveCalls times
	mockTool.On("Execute", map[string]interface{}{"input": "test-data"}, mock.Anything).Return(map[string]string{"result": "tool result"}, nil).Times(2)

	// Set MaxConsecutiveCalls to 2, which means it should stop after 2 tool calls
	runOpts := &RunOptions{
		ThreadID:            threadID,
		Input:               userInput,
		AvailableTools:      []tools.Tool{mockTool},
		MaxConsecutiveCalls: 2, // Key setting for this test
	}

	// Run the agent
	_, err := agent.Run(ctx, runOpts)

	// We expect an error because MaxConsecutiveCalls was exceeded
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeded maximum consecutive tool calls")

	// Verify that the model and tool were called exactly 2 times
	mock.AssertExpectationsForObjects(t, mockTool)
	// Note: mockModel expectations are not precisely verified here because
	// we're using Return() which doesn't limit call count
}

// Implementing the second test from the TODO list
func TestAgent_Run_ModelError(t *testing.T) {
	agent, mockModel, mockMemory, _ := setupTestAgent(t)
	ctx := context.Background()
	threadID := "thread-model-error"
	userInput := "This will cause a model error"

	// Mock MemoryProvider interactions
	mockMemory.On("AddMessage", ctx, threadID, "user", userInput, mock.Anything).Return("msg1", nil)
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]memory.Message{
		{Role: "user", Content: userInput},
	}, nil).Once()

	// Make the model return an error
	expectedError := errors.New("model service unavailable")
	mockModel.On("GenerateWithFunctionCalls", mock.Anything, mock.Anything, mock.Anything).Return(nil, expectedError).Once()

	runOpts := &RunOptions{
		ThreadID: threadID,
		Input:    userInput,
	}

	// Run the agent
	_, err := agent.Run(ctx, runOpts)

	// We expect the error from the model to be propagated
	assert.Error(t, err)
	assert.Equal(t, expectedError, err)

	// Verify mocks
	mockModel.AssertExpectations(t)
	mockMemory.AssertExpectations(t)
}

// Implementing the third test from the TODO list
func TestAgent_Run_ToolArgParseError(t *testing.T) {
	agent, mockModel, mockMemory, mockTool := setupTestAgent(t)
	ctx := context.Background()
	threadID := "thread-tool-arg-parse-error"
	userInput := "Call a tool with invalid arguments"

	// Mock MemoryProvider interactions
	mockMemory.On("AddMessage", ctx, threadID, "user", userInput, mock.Anything).Return("msg1", nil)
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]memory.Message{
		{Role: "user", Content: userInput},
	}, nil).Once()

	// Create an invalid JSON for tool arguments
	invalidToolCallArgs := "invalid-json-format"

	// Create a model response that requests a tool call with invalid arguments
	toolCallResponse := &models.ResponseWithFunctionCalls{
		Text: "I'll use the tool.",
		ToolCalls: []models.ToolCall{
			{ID: "call1", Type: "function", Function: models.FunctionCall{Name: "mock_tool", Arguments: invalidToolCallArgs}},
		},
		FinishReason: "tool_calls",
	}

	mockModel.On("GenerateWithFunctionCalls", mock.Anything, mock.Anything, mock.Anything).Return(toolCallResponse, nil).Once()

	// Mock for adding the assistant message
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "I'll use the tool.", mock.Anything).Return("msg2", nil)

	// Mock for adding the error tool message
	// This will contain the parsing error
	mockMemory.On("AddMessage", ctx, threadID, "tool", mock.Anything, mock.Anything).Return("msg3", nil)

	// For the second round, after the error
	mockMessages2 := []memory.Message{
		{Role: "user", Content: userInput},
		{Role: "assistant", Content: "I'll use the tool.", Metadata: map[string]interface{}{"tool_calls": []interface{}{}}},
		{Role: "tool", Content: mock.Anything, Metadata: map[string]interface{}{"tool_call_id": "call1", "tool_name": "mock_tool", "error": true}},
	}
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return(mockMessages2, nil).Once()

	// For the final response without tool calls
	finalResponse := &models.ResponseWithFunctionCalls{
		Text:         "I encountered an error with the tool.",
		FinishReason: "stop",
	}
	mockModel.On("GenerateWithFunctionCalls", mock.Anything, mock.Anything, mock.Anything).Return(finalResponse, nil).Once()

	// For the final assistant message
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "I encountered an error with the tool.", mock.Anything).Return("msg4", nil)

	runOpts := &RunOptions{
		ThreadID:       threadID,
		Input:          userInput,
		AvailableTools: []tools.Tool{mockTool},
	}

	// Run the agent
	result, err := agent.Run(ctx, runOpts)

	// We expect no error returned from Run itself, but the tool execution should have failed
	assert.NoError(t, err)
	assert.Equal(t, "I encountered an error with the tool.", result)

	// Verify mocks
	mockModel.AssertExpectations(t)
	mockMemory.AssertExpectations(t)
	// We don't expect the tool to be executed at all due to parse error
	mockTool.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
}

// Implementing the fourth test from the TODO list
func TestAgent_Run_ToolNotFoundError(t *testing.T) {
	agent, mockModel, mockMemory, _ := setupTestAgent(t)
	ctx := context.Background()
	threadID := "thread-tool-not-found"
	userInput := "Call a non-existent tool"

	// Mock MemoryProvider interactions
	mockMemory.On("AddMessage", ctx, threadID, "user", userInput, mock.Anything).Return("msg1", nil)
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]memory.Message{
		{Role: "user", Content: userInput},
	}, nil).Once()

	// Valid JSON arguments but for a non-existent tool
	toolCallArgs, _ := json.Marshal(map[string]interface{}{"input": "test-data"})

	// Create a model response that requests a non-existent tool
	toolCallResponse := &models.ResponseWithFunctionCalls{
		Text: "I'll use the non_existent_tool.",
		ToolCalls: []models.ToolCall{
			{ID: "call1", Type: "function", Function: models.FunctionCall{Name: "non_existent_tool", Arguments: string(toolCallArgs)}},
		},
		FinishReason: "tool_calls",
	}

	mockModel.On("GenerateWithFunctionCalls", mock.Anything, mock.Anything, mock.Anything).Return(toolCallResponse, nil).Once()

	// Mock for adding the assistant message
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "I'll use the non_existent_tool.", mock.Anything).Return("msg2", nil)

	// Mock for adding the error tool message
	mockMemory.On("AddMessage", ctx, threadID, "tool", mock.Anything, mock.Anything).Return("msg3", nil)

	// For the second round, after the error
	mockMessages2 := []memory.Message{
		{Role: "user", Content: userInput},
		{Role: "assistant", Content: "I'll use the non_existent_tool.", Metadata: map[string]interface{}{"tool_calls": []interface{}{}}},
		{Role: "tool", Content: mock.Anything, Metadata: map[string]interface{}{"tool_call_id": "call1", "tool_name": "non_existent_tool", "error": true}},
	}
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return(mockMessages2, nil).Once()

	// For the final response without tool calls
	finalResponse := &models.ResponseWithFunctionCalls{
		Text:         "I couldn't find the tool.",
		FinishReason: "stop",
	}
	mockModel.On("GenerateWithFunctionCalls", mock.Anything, mock.Anything, mock.Anything).Return(finalResponse, nil).Once()

	// For the final assistant message
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "I couldn't find the tool.", mock.Anything).Return("msg4", nil)

	// Run options without providing the "non_existent_tool"
	runOpts := &RunOptions{
		ThreadID: threadID,
		Input:    userInput,
		// Intentionally not providing any tools
	}

	// Run the agent
	result, err := agent.Run(ctx, runOpts)

	// We expect no error returned from Run itself, but the tool not found error should be handled
	assert.NoError(t, err)
	assert.Equal(t, "I couldn't find the tool.", result)

	// Verify mocks
	mockModel.AssertExpectations(t)
	mockMemory.AssertExpectations(t)
}

// Implementing the fifth test from the TODO list
func TestAgent_Run_ToolExecuteError(t *testing.T) {
	agent, mockModel, mockMemory, mockTool := setupTestAgent(t)
	ctx := context.Background()
	threadID := "thread-tool-execute-error"
	userInput := "Use a tool that will fail during execution"

	// Mock MemoryProvider interactions
	mockMemory.On("AddMessage", ctx, threadID, "user", userInput, mock.Anything).Return("msg1", nil)
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]memory.Message{
		{Role: "user", Content: userInput},
	}, nil).Once()

	// Valid JSON arguments for the tool
	toolCallArgs, _ := json.Marshal(map[string]interface{}{"input": "cause-error"})

	// Create a model response that requests the tool
	toolCallResponse := &models.ResponseWithFunctionCalls{
		Text: "I'll use the tool.",
		ToolCalls: []models.ToolCall{
			{ID: "call1", Type: "function", Function: models.FunctionCall{Name: "mock_tool", Arguments: string(toolCallArgs)}},
		},
		FinishReason: "tool_calls",
	}

	mockModel.On("GenerateWithFunctionCalls", mock.Anything, mock.Anything, mock.Anything).Return(toolCallResponse, nil).Once()

	// Mock for adding the assistant message
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "I'll use the tool.", mock.Anything).Return("msg2", nil)

	// Mock the tool to return an error
	expectedToolError := errors.New("tool execution failed")
	mockTool.On("Execute", map[string]interface{}{"input": "cause-error"}, mock.Anything).Return(nil, expectedToolError).Once()

	// Mock for adding the error tool message
	mockMemory.On("AddMessage", ctx, threadID, "tool", mock.Anything, mock.Anything).Return("msg3", nil)

	// For the second round, after the error
	mockMessages2 := []memory.Message{
		{Role: "user", Content: userInput},
		{Role: "assistant", Content: "I'll use the tool.", Metadata: map[string]interface{}{"tool_calls": []interface{}{}}},
		{Role: "tool", Content: mock.Anything, Metadata: map[string]interface{}{"tool_call_id": "call1", "tool_name": "mock_tool", "error": true}},
	}
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return(mockMessages2, nil).Once()

	// For the final response without tool calls
	finalResponse := &models.ResponseWithFunctionCalls{
		Text:         "The tool execution failed.",
		FinishReason: "stop",
	}
	mockModel.On("GenerateWithFunctionCalls", mock.Anything, mock.Anything, mock.Anything).Return(finalResponse, nil).Once()

	// For the final assistant message
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "The tool execution failed.", mock.Anything).Return("msg4", nil)

	runOpts := &RunOptions{
		ThreadID:       threadID,
		Input:          userInput,
		AvailableTools: []tools.Tool{mockTool},
	}

	// Run the agent
	result, err := agent.Run(ctx, runOpts)

	// We expect no error returned from Run itself, but the tool execution error should be handled
	assert.NoError(t, err)
	assert.Equal(t, "The tool execution failed.", result)

	// Verify mocks
	mockModel.AssertExpectations(t)
	mockMemory.AssertExpectations(t)
	mockTool.AssertExpectations(t)
}
