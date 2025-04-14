package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/louloulin/gostra"
	"github.com/louloulin/gostra/pkg/agent"
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

	provider := openai.NewProvider(&openai.ProviderOptions{
		APIKey: apiKey,
	})
	g.RegisterModelProvider("openai", provider)

	// Register a tool
	timeTool := &CurrentTimeTool{}
	g.RegisterTool(timeTool)

	// Register an agent
	assistant, err := g.RegisterAgent("assistant", &agent.Options{
		ID:            "assistant",
		Name:          "Time Assistant",
		SystemPrompt:  "You are a helpful assistant with access to tools. Your responses should be concise and helpful.",
		ModelProvider: provider,
		Tools:         []tools.Tool{timeTool},
	})

	if err != nil {
		fmt.Printf("Failed to register agent: %v\n", err)
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
		fmt.Println("=============================\n")
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
