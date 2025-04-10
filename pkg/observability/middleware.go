package observability

import (
	"context"
	"strconv"
	"time"

	"github.com/louloulin/gostra/pkg/actor"
)

// MonitorMiddleware provides monitoring capabilities for actors
type MonitorMiddleware struct {
	monitor *Monitor
}

// NewMonitorMiddleware creates a new monitoring middleware
func NewMonitorMiddleware(monitor *Monitor) *MonitorMiddleware {
	return &MonitorMiddleware{
		monitor: monitor,
	}
}

// Receive handles incoming messages and collects metrics
func (m *MonitorMiddleware) Receive(ctx context.Context, envelope *actor.MessageEnvelope, next actor.ReceiverFunc) error {
	startTime := time.Now()
	actorID := envelope.Recipient.ID()

	// Record initial metrics
	m.monitor.RecordActorMetric(actorID, func(metrics *ActorMetrics) {
		metrics.MessageCount++
		metrics.State = string(envelope.Recipient.State())
	})

	// Call next middleware/handler
	err := next(ctx, envelope)

	// Record completion metrics
	m.monitor.RecordActorMetric(actorID, func(metrics *ActorMetrics) {
		metrics.ResponseTime = float64(time.Since(startTime).Milliseconds())
		if err != nil {
			metrics.ErrorCount++
		}
	})

	// Record general system metrics
	m.monitor.RecordMetric(&Metric{
		Name:  "actor_message_processing_time",
		Type:  HistogramMetric,
		Value: float64(time.Since(startTime).Milliseconds()),
		Labels: map[string]string{
			"actor_id": actorID,
			"success":  strconv.FormatBool(err == nil),
		},
	})

	return err
}

// WithMonitoring adds monitoring to an actor
func WithMonitoring(monitor *Monitor) actor.MiddlewareFunc {
	middleware := NewMonitorMiddleware(monitor)
	return func(next actor.ReceiverFunc) actor.ReceiverFunc {
		return func(ctx context.Context, envelope *actor.MessageEnvelope) error {
			return middleware.Receive(ctx, envelope, next)
		}
	}
}
