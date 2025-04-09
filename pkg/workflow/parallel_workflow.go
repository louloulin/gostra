package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yourusername/gostra/pkg/agent"
	"github.com/yourusername/gostra/pkg/models"
)

// ParallelStep represents a step in a parallel workflow
type ParallelStep struct {
	ID            string                                                            // Unique identifier for this step
	Name          string                                                            // Human-readable name
	Description   string                                                            // Optional description
	PromptFormat  string                                                            // Format string for generating the prompt
	OutputKey     string                                                            // Key to store the output in the context
	InputKeys     []string                                                          // Keys from previous steps to use as input
	ExecuteFunc   func(ctx context.Context, args *ParallelStepArgs) (string, error) // Custom execution function
	ModelProvider models.ModelProvider                                              // Model provider to use for this step
	AgentID       string                                                            // ID of the agent to call (if using AgentNetwork)
	Network       *agent.AgentNetwork                                               // AgentNetwork for agent communication
	DependsOn     []string                                                          // IDs of steps this step depends on (for parallel execution)
	IsParallel    bool                                                              // Whether this step can be executed in parallel with others
	MaxRetries    int                                                               // Maximum number of retries if the step fails
	RetryDelay    time.Duration                                                     // Delay between retries
}

// ParallelStepArgs contains the arguments for executing a parallel step
type ParallelStepArgs struct {
	Context     map[string]interface{} // Current workflow context
	StepInputs  map[string]string      // Inputs from previous steps
	RawInputs   map[string]interface{} // Original inputs without string conversion
	Model       models.ModelProvider   // Model provider
	UserQuery   string                 // The original user query
	StepResults map[string]interface{} // Results from previous steps
}

// ParallelWorkflow represents a workflow that implements parallel execution of steps
// It implements the IWorkflow interface
type ParallelWorkflow struct {
	id            string                   // Workflow ID
	name          string                   // Workflow name
	description   string                   // Workflow description
	steps         []*ParallelStep          // Ordered steps in the workflow
	stepMap       map[string]*ParallelStep // Map of step ID to step
	workflowSteps map[string]*Step         // Map of step ID to workflow step
	startStep     string                   // ID of the start step
	status        Status                   // Current workflow status
	results       map[string]*StepResult   // Map of step ID to step result
	resultsMap    map[string]string        // Map of step ID to result string
	context       map[string]interface{}   // Workflow context
	metadata      map[string]interface{}   // Workflow metadata
	inputSchema   *Schema                  // Schema for validating workflow inputs
	outputSchema  *Schema                  // Schema for validating workflow outputs
	maxParallel   int                      // Maximum number of parallel executions
	mu            sync.RWMutex             // Mutex for thread safety
}

// ParallelWorkflowOptions represents the options for creating a parallel workflow
type ParallelWorkflowOptions struct {
	ID                    string                 `json:"id"`
	Name                  string                 `json:"name"`
	Description           string                 `json:"description,omitempty"`
	InputSchema           map[string]interface{} `json:"input_schema,omitempty"`
	OutputSchema          map[string]interface{} `json:"output_schema,omitempty"`
	MaxParallelExecutions int                    `json:"max_parallel_executions,omitempty"`
	Metadata              map[string]interface{} `json:"metadata,omitempty"`
}

// NewParallelWorkflow creates a new parallel workflow
func NewParallelWorkflow(opts ParallelWorkflowOptions) (*ParallelWorkflow, error) {
	// Ensure we have an ID and name
	id := opts.ID
	if id == "" {
		return nil, fmt.Errorf("workflow ID is required")
	}

	name := opts.Name
	if name == "" {
		return nil, fmt.Errorf("workflow name is required")
	}

	// Default to 5 parallel executions if not specified
	maxParallel := 5
	if opts.MaxParallelExecutions > 0 {
		maxParallel = opts.MaxParallelExecutions
	}

	// Create input/output schemas if provided
	var inputSchema, outputSchema *Schema
	if opts.InputSchema != nil {
		schema := NewSchema(TypeObject, "Input Schema")
		// Schema properties would need to be processed here
		inputSchema = schema
	}

	if opts.OutputSchema != nil {
		schema := NewSchema(TypeObject, "Output Schema")
		// Schema properties would need to be processed here
		outputSchema = schema
	}

	return &ParallelWorkflow{
		id:            id,
		name:          name,
		description:   opts.Description,
		steps:         []*ParallelStep{},
		stepMap:       make(map[string]*ParallelStep),
		workflowSteps: make(map[string]*Step),
		status:        StatusPending,
		results:       make(map[string]*StepResult),
		resultsMap:    make(map[string]string),
		context:       make(map[string]interface{}),
		metadata:      opts.Metadata,
		inputSchema:   inputSchema,
		outputSchema:  outputSchema,
		maxParallel:   maxParallel,
	}, nil
}

// GetID returns the workflow ID
func (w *ParallelWorkflow) GetID() string {
	return w.id
}

// GetName returns the workflow name
func (w *ParallelWorkflow) GetName() string {
	return w.name
}

// GetDescription returns the workflow description
func (w *ParallelWorkflow) GetDescription() string {
	return w.description
}

// GetMetadata returns the workflow metadata
func (w *ParallelWorkflow) GetMetadata() map[string]interface{} {
	return w.metadata
}

// GetSteps returns all workflow steps
func (w *ParallelWorkflow) GetSteps() []Step {
	steps := make([]Step, 0, len(w.workflowSteps))
	for _, s := range w.workflowSteps {
		steps = append(steps, *s)
	}
	return steps
}

// GetStatus returns the current workflow status
func (w *ParallelWorkflow) GetStatus() Status {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.status
}

// AddStep adds a parallel step to the workflow
func (w *ParallelWorkflow) AddStep(step *ParallelStep) *ParallelWorkflow {
	w.steps = append(w.steps, step)
	w.stepMap[step.ID] = step

	// Create a corresponding workflow step
	workflowStep := &Step{
		ID:          step.ID,
		Name:        step.Name,
		Description: step.Description,
		Execute:     w.createStepExecutor(step),
		Metadata: map[string]interface{}{
			"type":       "parallel_step",
			"prompt_fmt": step.PromptFormat,
			"output_key": step.OutputKey,
			"input_keys": step.InputKeys,
			"agent":      step.AgentID != "",
			"model":      step.ModelProvider != nil && step.ModelProvider.GetID() != "",
			"depends_on": step.DependsOn,
			"parallel":   step.IsParallel,
		},
	}

	// Add to workflow steps
	w.workflowSteps[step.ID] = workflowStep

	// If this is the first step and no dependencies, set it as the start step
	if len(w.steps) == 1 && len(step.DependsOn) == 0 {
		w.startStep = step.ID
	}

	return w
}

// GetStepResult retrieves a result for a specific step
func (w *ParallelWorkflow) GetStepResult(stepID string) *StepResult {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.results[stepID]
}

// createStepExecutor creates an executor function for a parallel step
func (w *ParallelWorkflow) createStepExecutor(step *ParallelStep) StepExecuteFunc {
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
		for _, prevStep := range w.steps {
			if result := w.GetStepResult(prevStep.ID); result != nil && result.Status == StatusCompleted {
				for k, v := range result.Output {
					stepResults[prevStep.ID+"."+k] = v
				}
			}
		}

		// Collect required inputs
		for _, inputKey := range step.InputKeys {
			if val, ok := input[inputKey]; ok {
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
		args := &ParallelStepArgs{
			Context:     input,
			StepInputs:  stepInputs,
			RawInputs:   rawInputs,
			Model:       step.ModelProvider,
			UserQuery:   userQuery,
			StepResults: stepResults,
		}

		var result string
		var err error

		// Execute the step with retry logic
		maxRetries := step.MaxRetries
		if maxRetries <= 0 {
			maxRetries = 1 // At least one attempt
		}

		for attempt := 0; attempt < maxRetries; attempt++ {
			if attempt > 0 && step.RetryDelay > 0 {
				select {
				case <-time.After(step.RetryDelay):
					// Continue after delay
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}

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
							"workflow":  w.GetID(),
							"timestamp": time.Now().Format(time.RFC3339),
							"attempt":   attempt + 1,
						},
					}

					resp, agentErr := step.Network.SendRequest(ctx, req)
					if agentErr != nil {
						err = fmt.Errorf("agent network error: %w", agentErr)
					} else if len(resp.Results) > 0 {
						result = resp.Results[0].Content
						err = nil
						break // Success, exit retry loop
					} else {
						err = fmt.Errorf("empty response from agent %s", step.AgentID)
					}
				} else if step.ModelProvider != nil {
					// Call model directly
					messages := []models.Message{
						{
							Role:    "system",
							Content: "You are a task executor performing a step in a parallel workflow.",
						},
						{
							Role:    "user",
							Content: prompt,
						},
					}
					result, err = step.ModelProvider.Generate(ctx, messages, &models.GenerateOptions{
						Temperature: 0.7,
					})
					if err == nil {
						break // Success, exit retry loop
					}
				} else {
					err = fmt.Errorf("no agent or model provider specified for step %s", step.ID)
				}
			}

			if err == nil {
				break // Success, exit retry loop
			}

			// Log retry attempt
			if attempt < maxRetries-1 {
				fmt.Printf("Step %s failed (attempt %d/%d): %v. Retrying...\n",
					step.ID, attempt+1, maxRetries, err)
			}
		}

		// Store the result
		w.resultsMap[step.ID] = result

		// Prepare output
		output := map[string]interface{}{
			step.OutputKey: result,
			"duration_ms":  time.Since(startTime).Milliseconds(),
		}

		if err != nil {
			return output, fmt.Errorf("step %s failed after %d attempts: %w", step.ID, maxRetries, err)
		}

		return output, nil
	}
}

// Run executes the workflow using the sequential method
func (w *ParallelWorkflow) Run(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	w.mu.Lock()
	if w.status == StatusRunning {
		w.mu.Unlock()
		return nil, errors.New("workflow is already running")
	}

	w.status = StatusRunning
	w.results = make(map[string]*StepResult)

	// Add input to context
	if input != nil {
		for k, v := range input {
			w.context[k] = v
		}
	}
	w.mu.Unlock()

	// Validate the input against the schema
	if w.inputSchema != nil {
		if err := ValidateWorkflowInput(w.inputSchema, input); err != nil {
			w.mu.Lock()
			w.status = StatusFailed
			w.mu.Unlock()
			return nil, err
		}
	}

	// Execute the workflow sequentially starting from the start step
	result, err := w.executeSequential(ctx, input)

	w.mu.Lock()
	defer w.mu.Unlock()

	if err != nil {
		w.status = StatusFailed
		return nil, err
	}

	w.status = StatusCompleted
	return result, nil
}

// executeSequential executes the workflow steps sequentially
func (w *ParallelWorkflow) executeSequential(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	if w.startStep == "" {
		return nil, fmt.Errorf("no start step defined")
	}

	// Execute steps in sequential order
	return w.executeStep(ctx, w.startStep, input)
}

// executeStep executes a single step and continues to the next steps
func (w *ParallelWorkflow) executeStep(ctx context.Context, stepID string, input map[string]interface{}) (map[string]interface{}, error) {
	w.mu.RLock()
	step, exists := w.workflowSteps[stepID]
	w.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("step '%s' not found", stepID)
	}

	// Create step result
	result := &StepResult{
		StepID:    stepID,
		Status:    StatusRunning,
		StartTime: time.Now(),
		Output:    make(map[string]interface{}),
	}

	w.mu.Lock()
	w.results[stepID] = result
	w.mu.Unlock()

	// Execute the step
	fmt.Printf("Executing step '%s' of workflow '%s'\n", stepID, w.id)

	// Create a dummy workflow for the step executor
	dummyWorkflow := NewDummyWorkflow()

	output, err := step.Execute(ctx, input, dummyWorkflow)

	w.mu.Lock()
	result.EndTime = time.Now()

	if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		w.mu.Unlock()
		return nil, fmt.Errorf("step '%s' failed: %w", stepID, err)
	}

	result.Status = StatusCompleted
	result.Output = output
	w.mu.Unlock()

	return output, nil
}

// RunWithParallelSteps executes the workflow with parallel execution of independent steps
func (w *ParallelWorkflow) RunWithParallelSteps(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	return w.RunWithParallelExecution(ctx, input)
}

// RunWithParallelExecution executes the workflow with parallel execution of steps
func (w *ParallelWorkflow) RunWithParallelExecution(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	w.mu.Lock()
	if w.status == StatusRunning {
		w.mu.Unlock()
		return nil, errors.New("workflow is already running")
	}

	w.status = StatusRunning
	w.results = make(map[string]*StepResult)

	// Add input to context
	if input != nil {
		for k, v := range input {
			w.context[k] = v
		}
	}
	w.mu.Unlock()

	// Validate the input against the schema
	if w.inputSchema != nil {
		if err := ValidateWorkflowInput(w.inputSchema, input); err != nil {
			w.mu.Lock()
			w.status = StatusFailed
			w.mu.Unlock()
			return nil, err
		}
	}

	if w.steps == nil || len(w.steps) == 0 {
		w.mu.Lock()
		w.status = StatusFailed
		w.mu.Unlock()
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
	for _, step := range w.steps {
		dependencyGraph[step.ID] = []string{}
		reverseDependencies[step.ID] = []string{}
	}

	// Build the graph
	for _, step := range w.steps {
		// If DependsOn is specified, use it
		if len(step.DependsOn) > 0 {
			dependencyGraph[step.ID] = step.DependsOn
			for _, depID := range step.DependsOn {
				reverseDependencies[depID] = append(reverseDependencies[depID], step.ID)
			}
		} else {
			// Otherwise, infer dependencies from InputKeys
			for _, otherStep := range w.steps {
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
	maxConcurrent := w.maxParallel // Use the configured maximum

	for len(completedSteps) < len(w.steps) {
		// Find ready steps
		readySteps := make([]*ParallelStep, 0)
		for _, step := range w.steps {
			if !completedSteps[step.ID] && isStepReady(step.ID) && (step.IsParallel || activeRoutines == 0) {
				readySteps = append(readySteps, step)
			}
		}

		// Start execution of ready steps (up to max concurrent)
		for _, step := range readySteps {
			if activeRoutines < maxConcurrent {
				activeRoutines++

				// Execute the step in a goroutine
				go func(s *ParallelStep) {
					// Create a copy of the context for this step
					stepCtx := make(map[string]interface{})
					for k, v := range workflowContext {
						stepCtx[k] = v
					}

					// Create a dummy workflow
					dummyWorkflow := NewDummyWorkflow()

					executor := w.createStepExecutor(s)
					stepOutput, err := executor(ctx, stepCtx, dummyWorkflow)

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
				w.mu.Lock()
				w.status = StatusFailed
				w.mu.Unlock()
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
			w.mu.Lock()
			w.status = StatusFailed
			w.mu.Unlock()
			return result, fmt.Errorf("workflow execution deadlock: circular dependencies detected")
		}
	}

	// Validate the output against the schema
	if w.outputSchema != nil && w.outputSchema.Properties != nil && len(w.outputSchema.Properties) > 0 {
		validationErrors := w.outputSchema.Validate(result)
		if len(validationErrors) > 0 {
			// Log the errors but don't fail the workflow
			errStrings := make([]string, len(validationErrors))
			for i, valErr := range validationErrors {
				errStrings[i] = fmt.Sprintf("%s: %s", valErr.Path, valErr.Message)
			}
			fmt.Printf("Warning: Workflow output validation errors: %s\n", strings.Join(errStrings, "; "))
		}
	}

	w.mu.Lock()
	w.status = StatusCompleted
	w.mu.Unlock()

	return result, nil
}

// SetInputSchema sets a custom input schema for the workflow
func (w *ParallelWorkflow) SetInputSchema(schema *Schema) {
	w.inputSchema = schema
}

// SetOutputSchema sets a custom output schema for the workflow
func (w *ParallelWorkflow) SetOutputSchema(schema *Schema) {
	w.outputSchema = schema
}

// BuildParallelWorkflow creates a generic parallel workflow
func BuildParallelWorkflow(options *WorkflowOptions, model models.ModelProvider, agentID string, network *agent.AgentNetwork) (*ParallelWorkflow, error) {
	// Convert to ParallelWorkflowOptions
	parallelOptions := ParallelWorkflowOptions{
		ID:                    options.ID,
		Name:                  options.Name,
		Description:           options.Description,
		Metadata:              options.Metadata,
		MaxParallelExecutions: 5, // Default value
	}

	// Check if max parallel is specified in metadata
	if options.Metadata != nil {
		if mp, ok := options.Metadata["max_parallel"].(int); ok && mp > 0 {
			parallelOptions.MaxParallelExecutions = mp
		}
	}

	workflow, err := NewParallelWorkflow(parallelOptions)
	if err != nil {
		return nil, err
	}

	// Get custom steps from metadata if provided
	if customSteps, ok := options.Metadata["steps"].([]map[string]interface{}); ok && len(customSteps) > 0 {
		// Add each step to workflow based on metadata
		for _, stepConfig := range customSteps {
			id, _ := stepConfig["id"].(string)
			if id == "" {
				continue // Skip invalid step
			}

			name, _ := stepConfig["name"].(string)
			desc, _ := stepConfig["description"].(string)
			prompt, _ := stepConfig["prompt"].(string)
			outputKey, _ := stepConfig["output_key"].(string)
			if outputKey == "" {
				outputKey = id + "_result"
			}

			// Get input keys
			inputKeysRaw, _ := stepConfig["input_keys"].([]interface{})
			inputKeys := make([]string, 0, len(inputKeysRaw))
			for _, key := range inputKeysRaw {
				if keyStr, ok := key.(string); ok {
					inputKeys = append(inputKeys, keyStr)
				}
			}

			// Get dependencies
			dependsOnRaw, _ := stepConfig["depends_on"].([]interface{})
			dependsOn := make([]string, 0, len(dependsOnRaw))
			for _, dep := range dependsOnRaw {
				if depStr, ok := dep.(string); ok {
					dependsOn = append(dependsOn, depStr)
				}
			}

			// Get parallel flag
			isParallel := false
			if parallelVal, ok := stepConfig["is_parallel"].(bool); ok {
				isParallel = parallelVal
			}

			// Create and add the step
			step := &ParallelStep{
				ID:            id,
				Name:          name,
				Description:   desc,
				PromptFormat:  prompt,
				OutputKey:     outputKey,
				InputKeys:     inputKeys,
				DependsOn:     dependsOn,
				IsParallel:    isParallel,
				ModelProvider: model,
				AgentID:       agentID,
				Network:       network,
			}

			workflow.AddStep(step)
		}
	}

	return workflow, nil
}

// GetStepResultString retrieves the string result of a specific step by ID
func (w *ParallelWorkflow) GetStepResultString(stepID string) string {
	return w.resultsMap[stepID]
}
