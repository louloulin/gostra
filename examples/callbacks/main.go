package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/louloulin/gostra"
	"github.com/louloulin/gostra/pkg/agent"
	"github.com/louloulin/gostra/pkg/memory"
	"github.com/louloulin/gostra/pkg/models/openai"
	"github.com/louloulin/gostra/pkg/tools"
)

// Define a simple tool for demonstration
type CurrentTimeTool struct{}

func (t *CurrentTimeTool) GetID() string {
	return "current_time"
}

func (t *CurrentTimeTool) GetDescription() string {
	return "Get the current time"
}

// JSONSchema implements the tools.Schema interface
type TimeToolSchema struct{}

func (s *TimeToolSchema) JSONSchema() (map[string]interface{}, error) {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"format": map[string]interface{}{
				"type":        "string",
				"description": "The format to return the time in (e.g., 'full', 'date', 'time')",
				"enum":        []string{"full", "date", "time"},
			},
		},
		"required": []string{},
	}, nil
}

// Validate validates the input against the schema
func (s *TimeToolSchema) Validate(params map[string]interface{}) error {
	// Simple validation, in a real implementation you would check the format value
	if format, exists := params["format"]; exists {
		formatStr, ok := format.(string)
		if !ok {
			return fmt.Errorf("format is not a string")
		}

		// Check if format is one of the allowed values
		validFormats := []string{"full", "date", "time"}
		isValid := false
		for _, validFormat := range validFormats {
			if formatStr == validFormat {
				isValid = true
				break
			}
		}

		if !isValid {
			return fmt.Errorf("invalid format: %s", formatStr)
		}
	}

	return nil
}

func (t *CurrentTimeTool) GetInputSchema() tools.Schema {
	return &TimeToolSchema{}
}

func (t *CurrentTimeTool) Execute(params map[string]interface{}, options *tools.ExecuteOptions) (interface{}, error) {
	now := time.Now()

	format, ok := params["format"].(string)
	if !ok {
		format = "full"
	}

	switch format {
	case "date":
		return now.Format("2006-01-02"), nil
	case "time":
		return now.Format("15:04:05"), nil
	default:
		return now.Format("2006-01-02 15:04:05"), nil
	}
}

func main() {
	// Create a new Gostra instance
	g := gostra.NewGostra(&gostra.Options{})

	// Register OpenAI model provider
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("OPENAI_API_KEY environment variable is not set")
		os.Exit(1)
	}

	// Create OpenAI provider
	provider, err := openai.NewOpenAIProvider(&openai.Options{
		APIKey: apiKey,
		Model:  "gpt-3.5-turbo",
	})

	if err != nil {
		fmt.Printf("Failed to create OpenAI provider: %v\n", err)
		os.Exit(1)
	}

	g.RegisterModelProvider("openai", provider)

	// Create a simple in-memory provider
	memoryProvider := memory.NewInMemoryProvider()

	// Register a tool
	timeTool := &CurrentTimeTool{}
	g.RegisterTool(timeTool)

	// Create an agent directly instead of using RegisterAgent
	assistant, err := agent.NewAgent(&agent.Options{
		ID:             "assistant",
		Name:           "Time Assistant",
		SystemPrompt:   "You are a helpful assistant with access to tools. Your responses should be concise and helpful.",
		ModelProvider:  provider,
		MemoryProvider: memoryProvider,
		Tools:          []tools.Tool{timeTool},
	})

	if err != nil {
		fmt.Printf("Failed to create agent: %v\n", err)
		os.Exit(1)
	}

	// Prepare step counter
	stepCount := 0

	// Define callbacks
	onStepFinish := func(ctx context.Context, data *agent.StepFinishData) error {
		stepCount++
		fmt.Printf("\n----- Step %d Complete -----\n", stepCount)
		fmt.Printf("Model response: %s\n", data.Text)

		if len(data.ToolCalls) > 0 {
			fmt.Println("\nTool Calls:")
			for i, call := range data.ToolCalls {
				fmt.Printf("  %d. %s\n", i+1, call.Function.Name)
				fmt.Printf("     Arguments: %v\n", call.Function.Arguments)

				if result, ok := data.ToolResults[call.ID]; ok {
					fmt.Printf("     Result: %v\n", result)
				}
			}
		}

		if data.Error != nil {
			fmt.Printf("\nError: %s\n", data.Error)
		}

		fmt.Println("--------------------------")
		return nil // No error in callback
	}

	onFinish := func(ctx context.Context, data *agent.RunResult) error {
		fmt.Printf("\n===== Execution Complete =====\n")
		fmt.Printf("Final response: %s\n", data.Response)

		if data.Conversation != nil {
			fmt.Printf("Total messages: %d\n", len(data.Conversation))
		}

		if data.Error != nil {
			fmt.Printf("Terminal error: %s\n", data.Error)
		}

		fmt.Printf("Total steps executed: %d\n", stepCount)
		fmt.Println("=============================")
		return nil // No error in callback
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run the agent with callbacks
	fmt.Println("Running agent with callbacks...")
	response, err := assistant.RunWithCallbacks(ctx, &agent.RunOptions{
		ThreadID:            "demo-thread",
		Input:               "What's the current time? Then tell me what day of the week it is today.",
		MaxConsecutiveCalls: 5,
		OnStepFinish:        onStepFinish,
		OnFinish:            onFinish,
	})

	// Print the final result
	if err != nil {
		fmt.Printf("Agent run failed: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Final agent response: %s\n", response)
}
