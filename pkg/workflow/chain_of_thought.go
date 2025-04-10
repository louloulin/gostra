package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/louloulin/gostra/pkg/agent"
	"github.com/louloulin/gostra/pkg/models"
)

// ChainOfThoughtStep represents a step in a chain-of-thought reasoning workflow
type ChainOfThoughtStep struct {
	ID            string                                                                  // Unique identifier for this step
	Name          string                                                                  // Human-readable name
	Description   string                                                                  // Optional description
	PromptFormat  string                                                                  // Format string for generating the prompt
	OutputKey     string                                                                  // Key to store the output in the context
	InputKeys     []string                                                                // Keys from previous steps to use as input
	ExecuteFunc   func(ctx context.Context, args *ChainOfThoughtStepArgs) (string, error) // Custom execution function
	ModelProvider models.ModelProvider                                                    // Model provider to use for this step
	AgentID       string                                                                  // ID of the agent to call (if using AgentNetwork)
	Network       *agent.AgentNetwork                                                     // AgentNetwork for agent communication
	DependsOn     []string                                                                // IDs of steps this step depends on (for parallel execution)
	IsParallel    bool                                                                    // Whether this step can be executed in parallel with others
}

// ChainOfThoughtStepArgs contains the arguments for executing a chain-of-thought step
type ChainOfThoughtStepArgs struct {
	Context     map[string]interface{} // Current workflow context
	StepInputs  map[string]string      // Inputs from previous steps
	RawInputs   map[string]interface{} // Original inputs without string conversion
	Model       models.ModelProvider   // Model provider
	Agent       *agent.ActorAgent      // Optional agent
	UserQuery   string                 // The original user query
	StepResults map[string]interface{} // Results from previous steps
}

// ChainOfThoughtWorkflow represents a workflow that implements chain-of-thought reasoning
type ChainOfThoughtWorkflow struct {
	*Workflow
	Steps        []*ChainOfThoughtStep          // Ordered steps in the workflow
	StepMap      map[string]*ChainOfThoughtStep // Map of step ID to step
	AgentMap     map[string]*agent.ActorAgent   // Map of agent ID to agent
	ResultsMap   map[string]string              // Map of step ID to result
	InputSchema  *Schema                        // Schema for validating workflow inputs
	OutputSchema *Schema                        // Schema for validating workflow outputs
}

// NewChainOfThoughtWorkflow creates a new chain-of-thought workflow
func NewChainOfThoughtWorkflow(name, description string) *ChainOfThoughtWorkflow {
	// Create a basic workflow first
	baseWorkflow, _ := NewWorkflow(&WorkflowOptions{
		Name:        name,
		Description: description,
		Steps:       []*Step{}, // We'll add steps programmatically
		StartStep:   "trigger", // Default start step name
		Metadata: map[string]interface{}{
			"type": "chain_of_thought",
		},
	})

	// Create a default input schema
	inputSchema := NewSchema(TypeObject, "Workflow input schema")
	querySchema := NewSimpleSchema(TypeString, "The query or prompt to process")
	querySchema.MinLength = 1
	inputSchema.AddProperty("query", querySchema, true)

	// Create a default output schema (any object)
	outputSchema := NewSchema(TypeObject, "Workflow output schema")

	return &ChainOfThoughtWorkflow{
		Workflow:     baseWorkflow,
		Steps:        []*ChainOfThoughtStep{},
		StepMap:      make(map[string]*ChainOfThoughtStep),
		AgentMap:     make(map[string]*agent.ActorAgent),
		ResultsMap:   make(map[string]string),
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
	}
}

// AddStep adds a chain-of-thought step to the workflow
func (w *ChainOfThoughtWorkflow) AddStep(step *ChainOfThoughtStep) *ChainOfThoughtWorkflow {
	w.Steps = append(w.Steps, step)
	w.StepMap[step.ID] = step

	// Create a corresponding workflow step
	workflowStep := &Step{
		ID:          step.ID,
		Name:        step.Name,
		Description: step.Description,
		Execute:     w.createStepExecutor(step),
		Metadata: map[string]interface{}{
			"type":       "chain_of_thought_step",
			"prompt_fmt": step.PromptFormat,
			"output_key": step.OutputKey,
			"input_keys": step.InputKeys,
			"agent":      step.AgentID != "",
			"model":      step.ModelProvider != nil && step.ModelProvider.GetID() != "",
		},
	}

	// Initialize Steps map if it's nil
	if w.Workflow.Steps == nil {
		w.Workflow.Steps = make(map[string]*Step)
	}

	// Add to base workflow
	w.Workflow.Steps[step.ID] = workflowStep

	// If this is the first step, set it as the start step
	if len(w.Steps) == 1 {
		w.Workflow.StartStep = step.ID
	} else {
		// Otherwise, set it as the next step for the previous step
		prevStep := w.Steps[len(w.Steps)-2]
		prevWorkflowStep := w.Workflow.Steps[prevStep.ID]
		prevWorkflowStep.Next = append(prevWorkflowStep.Next, step.ID)
	}

	return w
}

// createStepExecutor creates an executor function for a chain-of-thought step
func (w *ChainOfThoughtWorkflow) createStepExecutor(step *ChainOfThoughtStep) StepExecuteFunc {
	return func(ctx context.Context, input map[string]interface{}, workflow *Workflow) (map[string]interface{}, error) {
		// Start timing the step execution
		startTime := time.Now()

		// Extract user query if available
		userQuery := ""
		if q, ok := input["query"].(string); ok {
			userQuery = q
		}

		// Prepare step inputs (convert to strings when possible)
		stepInputs := make(map[string]string)
		rawInputs := make(map[string]interface{})

		// Get all results from previous steps
		stepResults := make(map[string]interface{})
		for _, prevStep := range w.Steps {
			if result := workflow.GetStepResult(prevStep.ID); result != nil && result.Status == StatusCompleted {
				for k, v := range result.Output {
					stepResults[prevStep.ID+"."+k] = v
				}
			}
		}

		// Collect required inputs from previous steps
		for _, inputKey := range step.InputKeys {
			if val, ok := input[inputKey]; ok {
				rawInputs[inputKey] = val
				if strVal, canConvert := val.(string); canConvert {
					stepInputs[inputKey] = strVal
				} else {
					// Try to format non-string values
					stepInputs[inputKey] = fmt.Sprintf("%v", val)
				}
			} else {
				// Check if the input might be in step results
				for _, results := range stepResults {
					resultMap, isMap := results.(map[string]interface{})
					if isMap {
						if val, has := resultMap[inputKey]; has {
							rawInputs[inputKey] = val
							if strVal, canConvert := val.(string); canConvert {
								stepInputs[inputKey] = strVal
							} else {
								stepInputs[inputKey] = fmt.Sprintf("%v", val)
							}
							break
						}
					}
				}
			}
		}

		// Prepare arguments
		args := &ChainOfThoughtStepArgs{
			Context:     input,
			StepInputs:  stepInputs,
			RawInputs:   rawInputs,
			Model:       step.ModelProvider,
			UserQuery:   userQuery,
			StepResults: stepResults,
		}

		var result string
		var err error

		// Execute the step
		if step.ExecuteFunc != nil {
			// Use custom execution function if provided
			result, err = step.ExecuteFunc(ctx, args)
		} else {
			// Default execution: format prompt and call model/agent
			prompt := step.PromptFormat

			// Replace placeholders in the prompt format with actual values
			for key, value := range stepInputs {
				placeholder := fmt.Sprintf("${%s}", key)
				prompt = ReplaceAll(prompt, placeholder, value)
			}

			// Replace step result references
			for prefix, results := range stepResults {
				resultMap, isMap := results.(map[string]interface{})
				if isMap {
					for key, value := range resultMap {
						placeholder := fmt.Sprintf("${%s.%s}", prefix, key)
						if strVal, ok := value.(string); ok {
							prompt = ReplaceAll(prompt, placeholder, strVal)
						} else {
							prompt = ReplaceAll(prompt, placeholder, fmt.Sprintf("%v", value))
						}
					}
				}
			}

			// Replace user query
			prompt = ReplaceAll(prompt, "${query}", userQuery)

			// Call either agent through network or model directly
			if step.AgentID != "" && step.Network != nil {
				// Call agent through network
				req := &agent.TransmitRequest{
					Message: prompt,
					Agents:  []string{step.AgentID},
					Context: map[string]interface{}{
						"step_id":   step.ID,
						"workflow":  workflow.ID,
						"timestamp": time.Now().Format(time.RFC3339),
					},
				}

				resp, agentErr := step.Network.SendRequest(ctx, req)
				if agentErr != nil {
					err = fmt.Errorf("agent network error: %w", agentErr)
				} else if len(resp.Results) > 0 {
					result = resp.Results[0].Content
				} else {
					err = fmt.Errorf("empty response from agent %s", step.AgentID)
				}
			} else if step.ModelProvider != nil {
				// Call model directly
				messages := []models.Message{
					{
						Role:    "system",
						Content: "You are a reasoning engine performing a step in a chain-of-thought process.",
					},
					{
						Role:    "user",
						Content: prompt,
					},
				}
				result, err = step.ModelProvider.Generate(ctx, messages, &models.GenerateOptions{
					Temperature: 0.7,
				})
			} else {
				err = fmt.Errorf("no agent or model provider specified for step %s", step.ID)
			}
		}

		// Store the result
		w.ResultsMap[step.ID] = result

		// Prepare output
		output := map[string]interface{}{
			step.OutputKey: result,
			"duration_ms":  time.Since(startTime).Milliseconds(),
		}

		// Also add the step result to the context for subsequent steps
		input[step.OutputKey] = result

		if err != nil {
			return output, fmt.Errorf("step %s failed: %w", step.ID, err)
		}

		return output, nil
	}
}

// ReplaceAll replaces all occurrences of old with new in s
func ReplaceAll(s, old, new string) string {
	// Standard library strings.ReplaceAll but we're defining it here
	// to avoid import conflicts
	var result string
	for {
		i := 0
		for ; i < len(s); i++ {
			if i+len(old) <= len(s) && s[i:i+len(old)] == old {
				result += s[:i] + new
				s = s[i+len(old):]
				break
			}
		}
		if i == len(s) {
			result += s
			break
		}
	}
	return result
}

// GetStepResult retrieves the result of a specific step by ID
func (w *ChainOfThoughtWorkflow) GetStepResult(stepID string) string {
	return w.ResultsMap[stepID]
}

// BuildRAGWorkflow creates a chain-of-thought workflow for RAG operations
func BuildRAGWorkflow(name string, model models.ModelProvider, agentID string, network *agent.AgentNetwork) *ChainOfThoughtWorkflow {
	workflow := NewChainOfThoughtWorkflow(name, "RAG workflow with chain-of-thought reasoning")

	// Step 1: Context Analysis
	workflow.AddStep(&ChainOfThoughtStep{
		ID:            "analyzeContext",
		Name:          "Analyze Context",
		Description:   "Analyze retrieved context chunks to identify key information",
		PromptFormat:  "${query} 1. First, carefully analyze the retrieved context chunks and identify key information.",
		OutputKey:     "initialAnalysis",
		InputKeys:     []string{"query"},
		ModelProvider: model,
		AgentID:       agentID,
		Network:       network,
	})

	// Step 2: Thought Breakdown
	workflow.AddStep(&ChainOfThoughtStep{
		ID:            "breakdownThoughts",
		Name:          "Break Down Thoughts",
		Description:   "Break down thinking process about how information relates to query",
		PromptFormat:  "Based on the initial analysis: ${initialAnalysis}\n\n2. Break down your thinking process about how the retrieved information relates to the query.",
		OutputKey:     "breakdown",
		InputKeys:     []string{"initialAnalysis"},
		ModelProvider: model,
		AgentID:       agentID,
		Network:       network,
	})

	// Step 3: Connect Pieces
	workflow.AddStep(&ChainOfThoughtStep{
		ID:            "connectPieces",
		Name:          "Connect Pieces",
		Description:   "Explain connections between different pieces of information",
		PromptFormat:  "Based on the breakdown: ${breakdown}\n\n3. Explain how you're connecting different pieces from the retrieved chunks.",
		OutputKey:     "connections",
		InputKeys:     []string{"breakdown"},
		ModelProvider: model,
		AgentID:       agentID,
		Network:       network,
	})

	// Step 4: Draw Conclusions
	workflow.AddStep(&ChainOfThoughtStep{
		ID:            "drawConclusions",
		Name:          "Draw Conclusions",
		Description:   "Draw conclusions based on evidence",
		PromptFormat:  "Based on your connections: ${connections}\n\n4. Draw conclusions based only on the evidence in the retrieved context.",
		OutputKey:     "conclusions",
		InputKeys:     []string{"connections"},
		ModelProvider: model,
		AgentID:       agentID,
		Network:       network,
	})

	// Step 5: Final Answer
	workflow.AddStep(&ChainOfThoughtStep{
		ID:            "finalAnswer",
		Name:          "Final Answer",
		Description:   "Provide final answer based on chain-of-thought reasoning",
		PromptFormat:  "Based on all your analysis:\n- Initial Analysis: ${initialAnalysis}\n- Thought Process: ${breakdown}\n- Connections: ${connections}\n- Conclusions: ${conclusions}\n\nProvide a final, comprehensive answer to the query: ${query}",
		OutputKey:     "answer",
		InputKeys:     []string{"query", "initialAnalysis", "breakdown", "connections", "conclusions"},
		ModelProvider: model,
		AgentID:       agentID,
		Network:       network,
	})

	return workflow
}

// BuildDynamicCOTWorkflow creates a dynamic chain-of-thought workflow that allows for flexible prompts and steps
func BuildDynamicCOTWorkflow(options *WorkflowOptions, model models.ModelProvider, ragAgentID string, network *agent.AgentNetwork) *ChainOfThoughtWorkflow {
	if options == nil {
		options = &WorkflowOptions{
			Name:        "Dynamic Chain of Thought",
			Description: "Dynamic multi-step reasoning workflow",
			Metadata: map[string]interface{}{
				"type": "dynamic_cot",
			},
		}
	}

	workflow := NewChainOfThoughtWorkflow(options.Name, options.Description)

	// Default steps if none provided
	var steps []*ChainOfThoughtStep

	// Get custom steps from metadata if provided
	if customSteps, ok := options.Metadata["steps"].([]map[string]interface{}); ok && len(customSteps) > 0 {
		for _, stepConfig := range customSteps {
			id, _ := stepConfig["id"].(string)
			name, _ := stepConfig["name"].(string)
			desc, _ := stepConfig["description"].(string)
			prompt, _ := stepConfig["prompt"].(string)
			outputKey, _ := stepConfig["output_key"].(string)
			inputKeysRaw, _ := stepConfig["input_keys"].([]interface{})

			// Convert input keys to string slice
			inputKeys := make([]string, 0, len(inputKeysRaw))
			for _, key := range inputKeysRaw {
				if keyStr, ok := key.(string); ok {
					inputKeys = append(inputKeys, keyStr)
				}
			}

			steps = append(steps, &ChainOfThoughtStep{
				ID:            id,
				Name:          name,
				Description:   desc,
				PromptFormat:  prompt,
				OutputKey:     outputKey,
				InputKeys:     inputKeys,
				ModelProvider: model,
				AgentID:       ragAgentID,
				Network:       network,
			})
		}
	} else {
		// Default Mastra-style steps
		steps = []*ChainOfThoughtStep{
			{
				ID:            "analyzeContext",
				Name:          "Analyze Context",
				Description:   "Analyze retrieved context chunks to identify key information",
				PromptFormat:  "${query} 1. First, carefully analyze the retrieved context chunks and identify key information.",
				OutputKey:     "initialAnalysis",
				InputKeys:     []string{"query"},
				ModelProvider: model,
				AgentID:       ragAgentID,
				Network:       network,
			},
			{
				ID:            "breakdownThoughts",
				Name:          "Break Down Thoughts",
				Description:   "Break down thinking process about how information relates to query",
				PromptFormat:  "Based on the initial analysis: ${initialAnalysis}\n\n2. Break down your thinking process about how the retrieved information relates to the query.",
				OutputKey:     "breakdown",
				InputKeys:     []string{"initialAnalysis"},
				ModelProvider: model,
				AgentID:       ragAgentID,
				Network:       network,
			},
			{
				ID:            "connectPieces",
				Name:          "Connect Pieces",
				Description:   "Explain connections between different pieces of information",
				PromptFormat:  "Based on the breakdown: ${breakdown}\n\n3. Explain how you're connecting different pieces from the retrieved chunks.",
				OutputKey:     "connections",
				InputKeys:     []string{"breakdown"},
				ModelProvider: model,
				AgentID:       ragAgentID,
				Network:       network,
			},
			{
				ID:            "drawConclusions",
				Name:          "Draw Conclusions",
				Description:   "Draw conclusions based on evidence",
				PromptFormat:  "Based on your connections: ${connections}\n\n4. Draw conclusions based only on the evidence in the retrieved context.",
				OutputKey:     "conclusions",
				InputKeys:     []string{"connections"},
				ModelProvider: model,
				AgentID:       ragAgentID,
				Network:       network,
			},
			{
				ID:            "finalAnswer",
				Name:          "Final Answer",
				Description:   "Provide final answer based on chain-of-thought reasoning",
				PromptFormat:  "Based on all your analysis:\n- Initial Analysis: ${initialAnalysis}\n- Thought Process: ${breakdown}\n- Connections: ${connections}\n- Conclusions: ${conclusions}\n\nProvide a final, comprehensive answer to the query: ${query}",
				OutputKey:     "answer",
				InputKeys:     []string{"query", "initialAnalysis", "breakdown", "connections", "conclusions"},
				ModelProvider: model,
				AgentID:       ragAgentID,
				Network:       network,
			},
		}
	}

	// Add all steps to the workflow
	for _, step := range steps {
		workflow.AddStep(step)
	}

	return workflow
}

// Run executes the chain-of-thought workflow with the given input
func (w *ChainOfThoughtWorkflow) Run(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// Validate the input against the schema
	if w.InputSchema != nil {
		if err := ValidateWorkflowInput(w.InputSchema, input); err != nil {
			return nil, err
		}
	}

	// Run the base workflow
	result, err := w.Workflow.Run(ctx, input)
	if err != nil {
		return nil, err
	}

	// Validate the output against the schema
	if w.OutputSchema != nil && w.OutputSchema.Properties != nil && len(w.OutputSchema.Properties) > 0 {
		validationErrors := w.OutputSchema.Validate(result)
		if len(validationErrors) > 0 {
			// Log the errors but don't fail the workflow
			errStrings := make([]string, len(validationErrors))
			for i, valErr := range validationErrors {
				errStrings[i] = fmt.Sprintf("%s: %s", valErr.Path, valErr.Message)
			}
			fmt.Printf("Warning: Workflow output validation errors: %s\n", strings.Join(errStrings, "; "))
		}
	}

	return result, nil
}

// SetInputSchema sets a custom input schema for the workflow
func (w *ChainOfThoughtWorkflow) SetInputSchema(schema *Schema) {
	w.InputSchema = schema
}

// SetOutputSchema sets a custom output schema for the workflow
func (w *ChainOfThoughtWorkflow) SetOutputSchema(schema *Schema) {
	w.OutputSchema = schema
}

// RunWithSharedContext executes a chain-of-thought workflow with context sharing through the agent network
func (w *ChainOfThoughtWorkflow) RunWithSharedContext(ctx context.Context, input map[string]interface{}, conversationID string) (map[string]interface{}, error) {
	// Validate the input against the schema
	if w.InputSchema != nil {
		if err := ValidateWorkflowInput(w.InputSchema, input); err != nil {
			return nil, err
		}
	}

	if w.Steps == nil || len(w.Steps) == 0 {
		return nil, fmt.Errorf("workflow has no steps")
	}

	// Make sure we have a network
	hasNetwork := false
	for _, step := range w.Steps {
		if step.Network != nil {
			hasNetwork = true
			break
		}
	}

	if !hasNetwork {
		// Fall back to regular Run method if no network is available
		return w.Run(ctx, input)
	}

	result := make(map[string]interface{})
	stepContext := make(map[string]interface{})

	// Initialize context with input
	for k, v := range input {
		stepContext[k] = v
	}

	// Add conversation ID for context tracking
	if conversationID == "" {
		conversationID = fmt.Sprintf("cot-%d", time.Now().UnixNano())
	}
	stepContext["conversation_id"] = conversationID
	stepContext["workflow_id"] = w.ID

	// Execute each step in sequence, preserving context between steps
	for i, step := range w.Steps {
		if step.Network == nil {
			continue
		}

		// Prepare request for this step
		stepMsg := step.PromptFormat

		// Replace placeholders in the prompt format with actual values
		for key, value := range stepContext {
			if strVal, ok := value.(string); ok {
				placeholder := fmt.Sprintf("${%s}", key)
				stepMsg = ReplaceAll(stepMsg, placeholder, strVal)
			} else {
				// Convert non-string values to string
				placeholder := fmt.Sprintf("${%s}", key)
				stepMsg = ReplaceAll(stepMsg, placeholder, fmt.Sprintf("%v", value))
			}
		}

		// Call agent through network
		req := &agent.TransmitRequest{
			Message:      stepMsg,
			Agents:       []string{step.AgentID},
			Context:      stepContext,
			ParallelCall: false,
		}

		resp, err := step.Network.SendRequest(ctx, req)
		if err != nil {
			return result, fmt.Errorf("step %s failed: %w", step.ID, err)
		}

		if len(resp.Results) == 0 {
			return result, fmt.Errorf("empty response from agent %s", step.AgentID)
		}

		// Store the result
		stepContent := resp.Results[0].Content
		w.ResultsMap[step.ID] = stepContent
		result[step.OutputKey] = stepContent

		// Update context for next steps
		if resp.Context != nil {
			// Merge with existing context, preserving earlier values
			for k, v := range resp.Context {
				stepContext[k] = v
			}
		}

		// Also add this step's output directly to the context
		stepContext[step.OutputKey] = stepContent

		// Record step result in workflow
		w.Workflow.Results[step.ID] = &StepResult{
			StepID:    step.ID,
			Status:    StatusCompleted,
			StartTime: time.Now().Add(-time.Second), // Approximate
			EndTime:   time.Now(),
			Output: map[string]interface{}{
				step.OutputKey: stepContent,
			},
		}

		// Log progress
		if w.Workflow.Status == "" {
			w.Workflow.Status = StatusRunning
		}

		// Set next step index or mark as completed
		if i == len(w.Steps)-1 {
			w.Workflow.Status = StatusCompleted
		}
	}

	return result, nil
}

// RunWithParallelSteps executes a chain-of-thought workflow with parallel step execution where possible
func (w *ChainOfThoughtWorkflow) RunWithParallelSteps(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// Validate the input against the schema
	if w.InputSchema != nil {
		if err := ValidateWorkflowInput(w.InputSchema, input); err != nil {
			return nil, err
		}
	}

	if w.Steps == nil || len(w.Steps) == 0 {
		return nil, fmt.Errorf("workflow has no steps")
	}

	// Initialize the workflow context with the input
	workflowContext := make(map[string]interface{})
	for k, v := range input {
		workflowContext[k] = v
	}

	// Create result map
	result := make(map[string]interface{})

	// Build a dependency graph
	dependencyGraph := make(map[string][]string)
	reverseDependencies := make(map[string][]string)

	// Initialize all steps as having no dependencies by default
	for _, step := range w.Steps {
		dependencyGraph[step.ID] = []string{}
		reverseDependencies[step.ID] = []string{}
	}

	// Build the graph
	for _, step := range w.Steps {
		// If DependsOn is specified, use it
		if len(step.DependsOn) > 0 {
			dependencyGraph[step.ID] = step.DependsOn
			for _, depID := range step.DependsOn {
				reverseDependencies[depID] = append(reverseDependencies[depID], step.ID)
			}
		} else {
			// Otherwise, infer dependencies from InputKeys
			for _, otherStep := range w.Steps {
				if otherStep.ID != step.ID {
					// Check if this step uses any outputs from the other step
					for _, inputKey := range step.InputKeys {
						if otherStep.OutputKey == inputKey {
							dependencyGraph[step.ID] = append(dependencyGraph[step.ID], otherStep.ID)
							reverseDependencies[otherStep.ID] = append(reverseDependencies[otherStep.ID], step.ID)
						}
					}
				}
			}
		}
	}

	// Create a list of completed steps
	completedSteps := make(map[string]bool)

	// Create a channel for results
	resultChan := make(chan struct {
		stepID string
		output map[string]interface{}
		err    error
	})

	// Function to check if a step is ready to run
	isStepReady := func(stepID string) bool {
		for _, depID := range dependencyGraph[stepID] {
			if !completedSteps[depID] {
				return false
			}
		}
		return true
	}

	// Execute steps until all are completed
	var activeRoutines int
	maxConcurrent := 5 // Maximum concurrent steps

	for len(completedSteps) < len(w.Steps) {
		// Find ready steps
		readySteps := make([]*ChainOfThoughtStep, 0)
		for _, step := range w.Steps {
			if !completedSteps[step.ID] && isStepReady(step.ID) && (step.IsParallel || activeRoutines == 0) {
				readySteps = append(readySteps, step)
			}
		}

		// Start execution of ready steps (up to max concurrent)
		for _, step := range readySteps {
			if activeRoutines < maxConcurrent {
				activeRoutines++

				// Execute the step in a goroutine
				go func(s *ChainOfThoughtStep) {
					stepOutput, err := w.executeStep(ctx, s, workflowContext)
					resultChan <- struct {
						stepID string
						output map[string]interface{}
						err    error
					}{
						stepID: s.ID,
						output: stepOutput,
						err:    err,
					}
				}(step)
			}
		}

		// Wait for a step to complete
		if activeRoutines > 0 {
			stepResult := <-resultChan
			activeRoutines--

			if stepResult.err != nil {
				// If a step fails, cancel the workflow
				return result, fmt.Errorf("step %s failed: %w", stepResult.stepID, stepResult.err)
			}

			// Mark the step as completed
			completedSteps[stepResult.stepID] = true

			// Add to results and update context
			for k, v := range stepResult.output {
				result[k] = v
				workflowContext[k] = v
			}
		} else if len(readySteps) == 0 {
			// If no steps are ready and no active routines, we might have a cycle
			return result, fmt.Errorf("workflow execution deadlock: circular dependencies detected")
		}
	}

	// Validate the output against the schema
	if w.OutputSchema != nil && w.OutputSchema.Properties != nil && len(w.OutputSchema.Properties) > 0 {
		validationErrors := w.OutputSchema.Validate(result)
		if len(validationErrors) > 0 {
			// Log the errors but don't fail the workflow
			errStrings := make([]string, len(validationErrors))
			for i, valErr := range validationErrors {
				errStrings[i] = fmt.Sprintf("%s: %s", valErr.Path, valErr.Message)
			}
			fmt.Printf("Warning: Workflow output validation errors: %s\n", strings.Join(errStrings, "; "))
		}
	}

	return result, nil
}

// executeStep executes a single step with the given context
func (w *ChainOfThoughtWorkflow) executeStep(ctx context.Context, step *ChainOfThoughtStep, workflowContext map[string]interface{}) (map[string]interface{}, error) {
	// Start timing the step execution
	startTime := time.Now()

	// Extract user query if available
	userQuery := ""
	if q, ok := workflowContext["query"].(string); ok {
		userQuery = q
	}

	// Prepare step inputs (convert to strings when possible)
	stepInputs := make(map[string]string)
	rawInputs := make(map[string]interface{})

	// Collect required inputs
	for _, inputKey := range step.InputKeys {
		if val, ok := workflowContext[inputKey]; ok {
			rawInputs[inputKey] = val
			if strVal, canConvert := val.(string); canConvert {
				stepInputs[inputKey] = strVal
			} else {
				// Try to format non-string values
				stepInputs[inputKey] = fmt.Sprintf("%v", val)
			}
		}
	}

	// Prepare arguments
	args := &ChainOfThoughtStepArgs{
		Context:    workflowContext,
		StepInputs: stepInputs,
		RawInputs:  rawInputs,
		Model:      step.ModelProvider,
		UserQuery:  userQuery,
	}

	var result string
	var err error

	// Execute the step
	if step.ExecuteFunc != nil {
		// Use custom execution function if provided
		result, err = step.ExecuteFunc(ctx, args)
	} else {
		// Default execution: format prompt and call model/agent
		prompt := step.PromptFormat

		// Replace placeholders in the prompt format with actual values
		for key, value := range stepInputs {
			placeholder := fmt.Sprintf("${%s}", key)
			prompt = ReplaceAll(prompt, placeholder, value)
		}

		// Replace user query
		prompt = ReplaceAll(prompt, "${query}", userQuery)

		// Call either agent through network or model directly
		if step.AgentID != "" && step.Network != nil {
			// Call agent through network
			req := &agent.TransmitRequest{
				Message: prompt,
				Agents:  []string{step.AgentID},
				Context: map[string]interface{}{
					"step_id":   step.ID,
					"workflow":  w.ID,
					"timestamp": time.Now().Format(time.RFC3339),
				},
			}

			resp, agentErr := step.Network.SendRequest(ctx, req)
			if agentErr != nil {
				err = fmt.Errorf("agent network error: %w", agentErr)
			} else if len(resp.Results) > 0 {
				result = resp.Results[0].Content
			} else {
				err = fmt.Errorf("empty response from agent %s", step.AgentID)
			}
		} else if step.ModelProvider != nil {
			// Call model directly
			messages := []models.Message{
				{
					Role:    "system",
					Content: "You are a reasoning engine performing a step in a chain-of-thought process.",
				},
				{
					Role:    "user",
					Content: prompt,
				},
			}
			result, err = step.ModelProvider.Generate(ctx, messages, &models.GenerateOptions{
				Temperature: 0.7,
			})
		} else {
			err = fmt.Errorf("no agent or model provider specified for step %s", step.ID)
		}
	}

	// Store the result
	w.ResultsMap[step.ID] = result

	// Prepare output
	output := map[string]interface{}{
		step.OutputKey: result,
		"duration_ms":  time.Since(startTime).Milliseconds(),
	}

	if err != nil {
		return output, fmt.Errorf("step %s failed: %w", step.ID, err)
	}

	return output, nil
}

// BuildParallelCOTWorkflow creates a workflow with parallel step execution
func BuildParallelCOTWorkflow(options *WorkflowOptions, model models.ModelProvider, agentID string, network *agent.AgentNetwork) *ChainOfThoughtWorkflow {
	if options == nil {
		options = &WorkflowOptions{
			Name:        "Parallel Chain of Thought",
			Description: "Chain of thought workflow with parallel step execution",
			Metadata: map[string]interface{}{
				"type": "parallel_cot",
			},
		}
	}

	workflow := NewChainOfThoughtWorkflow(options.Name, options.Description)

	// Define parallel steps
	// Step 1: Context Analysis (No dependencies, can run in parallel)
	contextStep := &ChainOfThoughtStep{
		ID:            "contextAnalysis",
		Name:          "Context Analysis",
		Description:   "Analyze the context and extract key information",
		PromptFormat:  "Analyze the following query and extract key information: ${query}",
		OutputKey:     "context_analysis",
		InputKeys:     []string{"query"},
		ModelProvider: model,
		AgentID:       agentID,
		Network:       network,
		IsParallel:    true, // Can run in parallel
	}
	workflow.AddStep(contextStep)

	// Step 2: Topic Exploration (No dependencies, can run in parallel with step 1)
	topicStep := &ChainOfThoughtStep{
		ID:            "topicExploration",
		Name:          "Topic Exploration",
		Description:   "Explore the main topics and concepts in the query",
		PromptFormat:  "Explore the main topics and concepts in the following query: ${query}",
		OutputKey:     "topic_exploration",
		InputKeys:     []string{"query"},
		ModelProvider: model,
		AgentID:       agentID,
		Network:       network,
		IsParallel:    true, // Can run in parallel
	}
	workflow.AddStep(topicStep)

	// Step 3: Critical Analysis (Depends on steps 1 and 2)
	analysisStep := &ChainOfThoughtStep{
		ID:            "criticalAnalysis",
		Name:          "Critical Analysis",
		Description:   "Perform critical analysis based on context and topic exploration",
		PromptFormat:  "Perform a critical analysis based on:\nContext Analysis: ${context_analysis}\nTopic Exploration: ${topic_exploration}\nQuery: ${query}",
		OutputKey:     "critical_analysis",
		InputKeys:     []string{"query", "context_analysis", "topic_exploration"},
		ModelProvider: model,
		AgentID:       agentID,
		Network:       network,
		DependsOn:     []string{"contextAnalysis", "topicExploration"}, // Explicitly define dependencies
		IsParallel:    false,                                           // Cannot run in parallel (depends on previous steps)
	}
	workflow.AddStep(analysisStep)

	// Step 4 & 5: Pros and Cons Analysis (Both depend on step 3, but can run in parallel with each other)
	prosStep := &ChainOfThoughtStep{
		ID:            "prosAnalysis",
		Name:          "Pros Analysis",
		Description:   "Analyze the positive aspects and supportive arguments",
		PromptFormat:  "Based on the critical analysis (${critical_analysis}), identify and elaborate on the positive aspects and supportive arguments:",
		OutputKey:     "pros_analysis",
		InputKeys:     []string{"critical_analysis"},
		ModelProvider: model,
		AgentID:       agentID,
		Network:       network,
		DependsOn:     []string{"criticalAnalysis"},
		IsParallel:    true, // Can run in parallel with cons analysis
	}
	workflow.AddStep(prosStep)

	consStep := &ChainOfThoughtStep{
		ID:            "consAnalysis",
		Name:          "Cons Analysis",
		Description:   "Analyze the negative aspects and counter-arguments",
		PromptFormat:  "Based on the critical analysis (${critical_analysis}), identify and elaborate on the negative aspects and counter-arguments:",
		OutputKey:     "cons_analysis",
		InputKeys:     []string{"critical_analysis"},
		ModelProvider: model,
		AgentID:       agentID,
		Network:       network,
		DependsOn:     []string{"criticalAnalysis"},
		IsParallel:    true, // Can run in parallel with pros analysis
	}
	workflow.AddStep(consStep)

	// Step 6: Synthesis (Depends on steps 4 and 5)
	synthesisStep := &ChainOfThoughtStep{
		ID:            "synthesis",
		Name:          "Synthesis",
		Description:   "Synthesize all analyses into a comprehensive answer",
		PromptFormat:  "Synthesize the following analyses into a comprehensive answer to the query (${query}):\n\nContext Analysis: ${context_analysis}\nTopic Exploration: ${topic_exploration}\nCritical Analysis: ${critical_analysis}\nPros Analysis: ${pros_analysis}\nCons Analysis: ${cons_analysis}",
		OutputKey:     "final_answer",
		InputKeys:     []string{"query", "context_analysis", "topic_exploration", "critical_analysis", "pros_analysis", "cons_analysis"},
		ModelProvider: model,
		AgentID:       agentID,
		Network:       network,
		DependsOn:     []string{"prosAnalysis", "consAnalysis"},
		IsParallel:    false,
	}
	workflow.AddStep(synthesisStep)

	return workflow
}
