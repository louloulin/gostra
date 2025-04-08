package evals

import (
	"context"
	"time"

	"github.com/yourusername/gostra/pkg/evals/metrics"
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
