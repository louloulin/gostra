package observability

import (
	"sync"
	"time"
)

// MetricType defines the type of metric being collected
type MetricType string

const (
	CounterMetric   MetricType = "counter"
	GaugeMetric     MetricType = "gauge"
	HistogramMetric MetricType = "histogram"
)

// Metric represents a single metric data point
type Metric struct {
	Name      string                 `json:"name"`
	Type      MetricType             `json:"type"`
	Value     float64                `json:"value"`
	Labels    map[string]string      `json:"labels"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Monitor provides system monitoring capabilities
type Monitor struct {
	mu      sync.RWMutex
	metrics map[string]*Metric
	actors  map[string]*ActorMetrics
}

// ActorMetrics stores metrics specific to an actor
type ActorMetrics struct {
	ID           string    `json:"id"`
	State        string    `json:"state"`
	LastActive   time.Time `json:"last_active"`
	MessageCount int64     `json:"message_count"`
	ErrorCount   int64     `json:"error_count"`
	ResponseTime float64   `json:"response_time_ms"`
	MemoryUsage  int64     `json:"memory_usage_bytes"`
}

// NewMonitor creates a new monitoring instance
func NewMonitor() *Monitor {
	return &Monitor{
		metrics: make(map[string]*Metric),
		actors:  make(map[string]*ActorMetrics),
	}
}

// RecordMetric records a new metric value
func (m *Monitor) RecordMetric(metric *Metric) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metric.Timestamp = time.Now()
	m.metrics[metric.Name] = metric
}

// RecordActorMetric records metrics for a specific actor
func (m *Monitor) RecordActorMetric(actorID string, update func(*ActorMetrics)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, exists := m.actors[actorID]
	if !exists {
		metrics = &ActorMetrics{
			ID:         actorID,
			LastActive: time.Now(),
		}
		m.actors[actorID] = metrics
	}

	update(metrics)
	metrics.LastActive = time.Now()
}

// GetMetrics returns all recorded metrics
func (m *Monitor) GetMetrics() map[string]*Metric {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make(map[string]*Metric, len(m.metrics))
	for k, v := range m.metrics {
		metrics[k] = v
	}
	return metrics
}

// GetActorMetrics returns metrics for all actors
func (m *Monitor) GetActorMetrics() map[string]*ActorMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make(map[string]*ActorMetrics, len(m.actors))
	for k, v := range m.actors {
		metrics[k] = v
	}
	return metrics
}

// Reset clears all recorded metrics
func (m *Monitor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics = make(map[string]*Metric)
	m.actors = make(map[string]*ActorMetrics)
}
