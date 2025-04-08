package observability

import (
	"testing"
)

func TestMonitor(t *testing.T) {
	monitor := NewMonitor()

	// Test metric recording
	metric := &Metric{
		Name:  "test_metric",
		Type:  CounterMetric,
		Value: 42.0,
		Labels: map[string]string{
			"test": "true",
		},
	}

	monitor.RecordMetric(metric)
	metrics := monitor.GetMetrics()

	if len(metrics) != 1 {
		t.Errorf("Expected 1 metric, got %d", len(metrics))
	}

	if m := metrics["test_metric"]; m == nil {
		t.Error("Expected test_metric to exist")
	} else {
		if m.Value != 42.0 {
			t.Errorf("Expected value 42.0, got %f", m.Value)
		}
		if m.Type != CounterMetric {
			t.Errorf("Expected type CounterMetric, got %s", m.Type)
		}
		if m.Labels["test"] != "true" {
			t.Errorf("Expected label test=true, got %s", m.Labels["test"])
		}
	}

	// Test actor metrics
	actorID := "test_actor"
	monitor.RecordActorMetric(actorID, func(metrics *ActorMetrics) {
		metrics.State = "active"
		metrics.MessageCount = 10
		metrics.ErrorCount = 2
		metrics.ResponseTime = 100.0
		metrics.MemoryUsage = 1024
	})

	actorMetrics := monitor.GetActorMetrics()
	if len(actorMetrics) != 1 {
		t.Errorf("Expected 1 actor metric, got %d", len(actorMetrics))
	}

	if am := actorMetrics[actorID]; am == nil {
		t.Error("Expected test_actor metrics to exist")
	} else {
		if am.State != "active" {
			t.Errorf("Expected state active, got %s", am.State)
		}
		if am.MessageCount != 10 {
			t.Errorf("Expected message count 10, got %d", am.MessageCount)
		}
		if am.ErrorCount != 2 {
			t.Errorf("Expected error count 2, got %d", am.ErrorCount)
		}
		if am.ResponseTime != 100.0 {
			t.Errorf("Expected response time 100.0, got %f", am.ResponseTime)
		}
		if am.MemoryUsage != 1024 {
			t.Errorf("Expected memory usage 1024, got %d", am.MemoryUsage)
		}
	}

	// Test reset
	monitor.Reset()
	metrics = monitor.GetMetrics()
	actorMetrics = monitor.GetActorMetrics()

	if len(metrics) != 0 {
		t.Errorf("Expected 0 metrics after reset, got %d", len(metrics))
	}
	if len(actorMetrics) != 0 {
		t.Errorf("Expected 0 actor metrics after reset, got %d", len(actorMetrics))
	}
}
