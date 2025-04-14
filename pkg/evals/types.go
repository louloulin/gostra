package evals

import (
	"context"
	"time"

	"github.com/louloulin/gostra/pkg/evals/metrics"
)

// EvalResult represents the result of a single evaluation run
type EvalResult struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     time.Time              `json:"end_time"`
	Duration    time.Duration          `json:"duration"`
	Success     bool                   `json:"success"`
	Score       float64                `json:"score"`
	Metrics     map[string]float64     `json:"metrics"`
	Metadata    map[string]interface{} `json:"metadata"`
	Error       string                 `json:"error,omitempty"`
}

// EvalConfig defines configuration options for an evaluation
type EvalConfig struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Timeout     time.Duration          `json:"timeout"`
	MaxRetries  int                    `json:"max_retries"`
	Metadata    map[string]interface{} `json:"metadata"`
	CustomRules []CustomEvalRule       `json:"-"` // Slice for custom rules (ignored by JSON)
}

// MetricProvider defines the interface for collecting evaluation metrics
type MetricProvider interface {
	CollectMetric(name string, value float64)
	GetMetrics() map[string]float64
	Reset()
}

// AgentEvaluable defines the interface that an agent must implement to be evaluated
type AgentEvaluable interface {
	GetID() string
	GetSampleInteraction(ctx context.Context, taskIdentifier string) (input, output, referenceOutput string, err error)
	GetMultipleOutputs(ctx context.Context, input string, count int) (outputs []string, err error)
}

// CustomEvalRule defines the interface for custom evaluation logic
type CustomEvalRule interface {
	GetName() string // Name used as the key in the results map
	// Evaluate receives the input, actual output, and reference output for a specific test case
	Evaluate(ctx context.Context, input, output, referenceOutput string) (score float64, err error)
}

// Evaluator defines the interface for evaluating agents
type Evaluator interface {
	Evaluate(ctx context.Context, agent AgentEvaluable) (*EvalResult, error)
	GetConfig() *EvalConfig
	Reset()
}

// BaseEvaluator provides common functionality for evaluators
type BaseEvaluator struct {
	config  *EvalConfig
	metrics MetricProvider
}

// NewBaseEvaluator creates a new BaseEvaluator instance
func NewBaseEvaluator(config *EvalConfig) *BaseEvaluator {
	return &BaseEvaluator{
		config:  config,
		metrics: metrics.NewDefaultMetricProvider(),
	}
}

// GetConfig returns the evaluator configuration
func (e *BaseEvaluator) GetConfig() *EvalConfig {
	return e.config
}

// Reset resets the evaluator state
func (e *BaseEvaluator) Reset() {
	e.metrics.Reset()
}

// GetMetrics returns the collected metrics
func (e *BaseEvaluator) GetMetrics() map[string]float64 {
	return e.metrics.GetMetrics()
}

// --- Batch Evaluation Types ---

// EvalTestCase represents a single test case in a batch evaluation
type EvalTestCase struct {
	ID              string                 `json:"id"`               // Unique identifier for this test case
	Input           string                 `json:"input"`            // The input/prompt for the agent
	ReferenceOutput string                 `json:"reference_output"` // The ideal/expected output for comparison
	Metadata        map[string]interface{} `json:"metadata"`         // Any additional metadata for this case
}

// BatchEvalSummary holds summary statistics for a batch run
type BatchEvalSummary struct {
	TotalCases      int           `json:"total_cases"`
	Successful      int           `json:"successful"` // Count of cases where EvalResult.Success was true
	Failed          int           `json:"failed"`     // Count of cases where EvalResult.Success was false
	AverageScore    float64       `json:"average_score"`
	AverageDuration time.Duration `json:"average_duration"`
	// TODO: Add average/distribution for specific metrics?
}

// BatchEvalResult holds the results of evaluating an agent over a batch of test cases
type BatchEvalResult struct {
	AgentID       string           `json:"agent_id"`
	EvalConfig    *EvalConfig      `json:"eval_config"` // Configuration used for the batch
	StartTime     time.Time        `json:"start_time"`
	EndTime       time.Time        `json:"end_time"`
	TotalDuration time.Duration    `json:"total_duration"`
	Results       []*EvalResult    `json:"results"` // Results for each individual test case
	Summary       BatchEvalSummary `json:"summary"` // Summary statistics
}
