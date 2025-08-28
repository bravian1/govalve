package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bravian1/govalve"
	"golang.org/x/time/rate"
)

// SimpleMetrics is a basic metrics implementation for demonstration
type SimpleMetrics struct {
	counters   map[string]int64
	histograms map[string][]float64
	gauges     map[string]float64
	mu         sync.RWMutex
}

func NewSimpleMetrics() *SimpleMetrics {
	return &SimpleMetrics{
		counters:   make(map[string]int64),
		histograms: make(map[string][]float64),
		gauges:     make(map[string]float64),
	}
}

func (m *SimpleMetrics) IncrementCounter(name string, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.buildKey(name, labels)
	m.counters[key]++
	fmt.Printf("[METRIC] Counter %s = %d\n", key, m.counters[key])
}

func (m *SimpleMetrics) RecordHistogram(name string, value float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.buildKey(name, labels)
	m.histograms[key] = append(m.histograms[key], value)
	fmt.Printf("[METRIC] Histogram %s recorded %.4f\n", key, value)
}

func (m *SimpleMetrics) SetGauge(name string, value float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.buildKey(name, labels)
	m.gauges[key] = value
	fmt.Printf("[METRIC] Gauge %s = %.2f\n", key, value)
}

func (m *SimpleMetrics) buildKey(name string, labels map[string]string) string {
	key := name
	for k, v := range labels {
		key += fmt.Sprintf(",%s=%s", k, v)
	}
	return key
}

func (m *SimpleMetrics) PrintSummary() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	fmt.Println("\n=== METRICS SUMMARY ===")
	fmt.Println("Counters:")
	for k, v := range m.counters {
		fmt.Printf("  %s: %d\n", k, v)
	}
	
	fmt.Println("Gauges:")
	for k, v := range m.gauges {
		fmt.Printf("  %s: %.2f\n", k, v)
	}
}

// MockStore with metrics
type MockStore struct {
	subscriptions map[string]*govalve.Subscription
	mu            sync.RWMutex
}

func NewMockStore() *MockStore {
	return &MockStore{
		subscriptions: make(map[string]*govalve.Subscription),
	}
}

func (s *MockStore) GetSubscription(ctx context.Context, userID string) (*govalve.Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.subscriptions[userID]
	if !ok {
		return nil, fmt.Errorf("subscription not found for user %s", userID)
	}
	return sub, nil
}

func (s *MockStore) SaveSubscription(ctx context.Context, sub *govalve.Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscriptions[sub.UserID] = sub
	return nil
}

func (s *MockStore) CheckAndIncrementUsage(ctx context.Context, userID string, quota int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.subscriptions[userID]
	if !ok {
		return fmt.Errorf("subscription not found for user %s", userID)
	}
	if quota > 0 && sub.Usage >= quota {
		return fmt.Errorf("usage quota exceeded")
	}
	sub.Usage++
	return nil
}

func main() {
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// Create metrics collector
	metrics := NewSimpleMetrics()
	store := NewMockStore()

	// Create manager with metrics
	manager := govalve.NewManager(appCtx, 
		govalve.WithStore(store),
		govalve.WithMetrics(metrics))
	defer manager.Shutdown()

	// Register profile
	manager.RegisterProfile("monitored-tier",
		govalve.WithDedicatedResource(rate.Limit(5)),
		govalve.WithWorkerPool(5, 50),
		govalve.WithUsageQuota(3),
	)

	// Subscribe user
	userID := "monitored-user"
	manager.Subscribe(appCtx, userID, "monitored-tier", 10*time.Second)

	// Get limiter and submit tasks
	limiter, _ := manager.GetLimiter(appCtx, userID)
	
	fmt.Println("=== Submitting Tasks ===")
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		taskNum := i
		go func() {
			defer wg.Done()
			err := limiter.Submit(appCtx, func() {
				fmt.Printf("Task %d executed\n", taskNum)
				time.Sleep(100 * time.Millisecond)
			})
			if err != nil {
				fmt.Printf("Task %d failed: %v\n", taskNum, err)
			}
		}()
	}
	wg.Wait()

	// Test subscription modifications
	fmt.Println("\n=== Testing Subscription Modifications ===")
	manager.UpdateSubscriptionQuota(appCtx, userID, 10)
	manager.ExtendSubscription(appCtx, userID, 5*time.Second)
	
	time.Sleep(100 * time.Millisecond) // Let metrics settle
	metrics.PrintSummary()
}