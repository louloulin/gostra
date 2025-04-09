# Parallel Workflow Example

This example demonstrates how to create and use a parallel workflow in Gostra, allowing for concurrent execution of steps in a workflow based on their dependencies.

## Key Components

1. **ParallelWorkflowOptions**: A struct that defines the configuration options for a parallel workflow, including:
   - ID, Name, Description: Basic workflow identifiers
   - InputSchema, OutputSchema: Schema definitions for validating inputs and outputs
   - MaxParallelExecutions: Maximum number of steps that can execute concurrently
   - Metadata: Additional information about the workflow

2. **ParallelStep**: Represents a step in a parallel workflow with:
   - ID, Name, Description: Basic step identifiers
   - PromptFormat: Template string for generating prompts with variable substitution
   - OutputKey: Key to store the output in the workflow context
   - InputKeys: Keys from previous steps or inputs to use
   - DependsOn: IDs of steps this step depends on
   - IsParallel: Whether this step can be executed in parallel with others
   - Retry capabilities: MaxRetries and RetryDelay

3. **Execution Methods**:
   - `RunWithParallelExecution`: Executes the workflow with parallel execution of steps based on dependencies

## Implementation Details

- Steps are executed in parallel when possible, with a maximum concurrent execution defined by `MaxParallelExecutions`.
- Dependencies between steps are respected, ensuring a step only runs after all its dependencies have completed.
- Results from each step are collected and made available to subsequent steps.
- Template variables in prompts are automatically replaced with actual values from inputs or previous steps.

## Usage

1. Create a parallel workflow with options
2. Add steps with their dependencies
3. Run the workflow with input data
4. Process the results

## Example Workflow

This example implements a product research workflow with four steps:

1. Market Research (parallel)
2. Competitor Analysis (parallel)
3. Pricing Strategy (parallel)
4. Comprehensive Summary (depends on all previous steps)

The first three steps can run in parallel as they don't depend on each other, while the final summary step runs after all others are complete. 