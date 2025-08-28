package govalve

import (
	"time"
)

// Metrics defines the interface for collecting govalve metrics.
// Implement this interface to integrate with your monitoring system (Prometheus, DataDog, etc.).
type Metrics interface {
	// IncrementCounter increments a counter metric
	IncrementCounter(name string, labels map[string]string)
	// RecordHistogram records a histogram/timing metric
	RecordHistogram(name string, value float64, labels map[string]string)
	// SetGauge sets a gauge metric value
	SetGauge(name string, value float64, labels map[string]string)
}

// NoOpMetrics is a no-op implementation of Metrics for when monitoring is disabled.
type NoOpMetrics struct{}

func (m *NoOpMetrics) IncrementCounter(name string, labels map[string]string) {}
func (m *NoOpMetrics) RecordHistogram(name string, value float64, labels map[string]string) {}
func (m *NoOpMetrics) SetGauge(name string, value float64, labels map[string]string) {}

// WithMetrics is a ManagerOption that configures metrics collection.
func WithMetrics(metrics Metrics) ManagerOption {
	return func(m *Manager) {
		m.metrics = metrics
	}
}

// recordTaskSubmission records metrics for task submission
func (m *Manager) recordTaskSubmission(userID, profileName string, success bool, duration time.Duration) {
	if m.metrics == nil {
		return
	}
	
	status := "success"
	if !success {
		status = "failed"
	}
	
	labels := map[string]string{
		"user_id": userID,
		"profile": profileName,
		"status":  status,
	}
	
	m.metrics.IncrementCounter("govalve_task_submissions_total", labels)
	m.metrics.RecordHistogram("govalve_task_duration_seconds", duration.Seconds(), labels)
}

// recordSubscriptionEvent records subscription lifecycle events
func (m *Manager) recordSubscriptionEvent(userID, profileName, event string) {
	if m.metrics == nil {
		return
	}
	
	labels := map[string]string{
		"user_id": userID,
		"profile": profileName,
		"event":   event,
	}
	
	m.metrics.IncrementCounter("govalve_subscription_events_total", labels)
}

// recordUsageQuota records current usage vs quota
func (m *Manager) recordUsageQuota(userID, profileName string, usage, quota int64) {
	if m.metrics == nil {
		return
	}
	
	labels := map[string]string{
		"user_id": userID,
		"profile": profileName,
	}
	
	m.metrics.SetGauge("govalve_usage_current", float64(usage), labels)
	m.metrics.SetGauge("govalve_quota_limit", float64(quota), labels)
	
	if quota > 0 {
		utilization := float64(usage) / float64(quota)
		m.metrics.SetGauge("govalve_quota_utilization", utilization, labels)
	}
}