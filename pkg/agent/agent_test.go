package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

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

// --- Mock MemoryProvider ---

type MockMemoryProvider struct {
	mock.Mock
	mu       sync.Mutex
	messages map[string][]memory.Message // threadID -> messages
}

func NewMockMemoryProvider() *MockMemoryProvider {
	return &MockMemoryProvider{
		messages: make(map[string][]memory.Message),
	}
}

func (m *MockMemoryProvider) AddMessage(ctx context.Context, threadID, role, content string, metadata map[string]interface{}) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	args := m.Called(ctx, threadID, role, content, metadata)
	msg := memory.Message{
		ID:        fmt.Sprintf("msg-%d", len(m.messages[threadID])+1),
		Role:      role,
		Content:   content,
		Metadata:  metadata,
		CreatedAt: time.Now().Unix(),
	}
	m.messages[threadID] = append(m.messages[threadID], msg)
	return msg.ID, args.Error(1)
}

func (m *MockMemoryProvider) GetMessages(ctx context.Context, threadID string, limit, offset int) ([]memory.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	args := m.Called(ctx, threadID, limit, offset)
	msgs, exists := m.messages[threadID]
	if !exists {
		return []memory.Message{}, args.Error(1)
	}
	// Simple implementation for mock, doesn't handle limit/offset precisely for now
	return msgs, args.Error(1)
}

func (m *MockMemoryProvider) DeleteThread(ctx context.Context, threadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	args := m.Called(ctx, threadID)
	delete(m.messages, threadID)
	return args.Error(0)
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

type mockSchema struct{}

func (s mockSchema) JSONSchema() (map[string]interface{}, error) {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "input": map[string]interface{}{"type": "string"},
        },
    }, nil
}

func (m *MockTool) GetInputSchema() tools.Schema {
	args := m.Called()
	// Return a valid Schema implementation, even if simple
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
	mockMemory.On("AddMessage", ctx, threadID, "user", userInput, mock.Anything).Return("msg1", nil).Once()
	mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]memory.Message{
		{Role: "user", Content: userInput},
	}, nil).Once() // First GetMessages
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "Okay, using tool.", mock.MatchedBy(func(metadata map[string]interface{}) bool {
        _, ok := metadata["tool_calls"]
        return ok // Check if tool_calls metadata exists
    })).Return("msg2", nil).Once() // Save assistant message with tool call intent
    mockMemory.On("AddMessage", ctx, threadID, "tool", `{"result":"tool success"}`, mock.MatchedBy(func(metadata map[string]interface{}) bool {
        id, okId := metadata["tool_call_id"].(string)
        name, okName := metadata["tool_name"].(string)
        return okId && okName && id == "call1" && name == "mock_tool"
    })).Return("msg3", nil).Once() // Save tool result message
    mockMemory.On("GetMessages", ctx, threadID, 100, 0).Return([]memory.Message{ // Second GetMessages after tool call
		{Role: "user", Content: userInput},
		{Role: "assistant", Content: "Okay, using tool.", Metadata: map[string]interface{}{"tool_calls": []interface{}{ /* dummy */ }}},
		{Role: "tool", Content: `{"result":"tool success"}`, Name: "call1", Metadata: map[string]interface{}{"tool_call_id": "call1", "tool_name": "mock_tool"}},
	}, nil).Once()
	mockMemory.On("AddMessage", ctx, threadID, "assistant", "Tool finished.", mock.Anything).Return("msg4", nil).Once() // Save final assistant message

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
		return len(msgs) > 0 && msgs[len(msgs)-1].Role == "tool" // Check last message is tool result
	}), mock.Anything).Return(modelResponse2, nil).Once()


	// Mock Tool execution
	mockTool.On("Execute", map[string]interface{}{"input": "data"}, mock.AnythingOfType("*tools.ExecuteOptions")).Return(map[string]string{"result": "tool success"}, nil).Once()


	runOpts := &RunOptions{
		ThreadID:          threadID,
		Input:             userInput,
		AvailableTools:    []tools.Tool{mockTool}, // Provide the tool
		MaxConsecutiveCalls: 5, // Allow tool calls
	}

	finalResponse, err := agent.Run(ctx, runOpts)

	assert.NoError(t, err)
	assert.Equal(t, "Tool finished.", finalResponse)

	// Verify mocks
	mockModel.AssertExpectations(t)
	mockMemory.AssertExpectations(t)
	mockTool.AssertExpectations(t)
}

// TODO: Add more tests:
// - TestAgent_Run_ToolArgParseError
// - TestAgent_Run_ToolNotFoundError
// - TestAgent_Run_ToolExecuteError


``` 