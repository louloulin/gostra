package metrics

import (
	"sync"
)

// DefaultMetricProvider provides a basic implementation of metric collection
type DefaultMetricProvider struct {
	mu      sync.RWMutex
	metrics map[string]float64
}

// NewDefaultMetricProvider creates a new DefaultMetricProvider instance
func NewDefaultMetricProvider() *DefaultMetricProvider {
	return &DefaultMetricProvider{
		metrics: make(map[string]float64),
	}
}

// CollectMetric records a metric value
func (p *DefaultMetricProvider) CollectMetric(name string, value float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metrics[name] = value
}

// GetMetrics returns all collected metrics
func (p *DefaultMetricProvider) GetMetrics() map[string]float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	metrics := make(map[string]float64, len(p.metrics))
	for k, v := range p.metrics {
		metrics[k] = v
	}
	return metrics
}

// Reset clears all collected metrics
func (p *DefaultMetricProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metrics = make(map[string]float64)
}
