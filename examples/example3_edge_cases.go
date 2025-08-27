package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bravian1/govalve"
	"golang.org/x/time/rate"
)

// EdgeCaseStore tests error conditions
type EdgeCaseStore struct {
	subscriptions map[string]*govalve.Subscription
	mu            sync.RWMutex
	failMode      string
}

func NewEdgeCaseStore() *EdgeCaseStore {
	return &EdgeCaseStore{
		subscriptions: make(map[string]*govalve.Subscription),
	}
}

func (s *EdgeCaseStore) GetSubscription(ctx context.Context, userID string) (*govalve.Subscription, error) {
	if s.failMode == "get_error" {
		return nil, fmt.Errorf("simulated database error")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.subscriptions[userID]
	if !ok {
		return nil, fmt.Errorf("subscription not found for user %s", userID)
	}
	return sub, nil
}

func (s *EdgeCaseStore) SaveSubscription(ctx context.Context, sub *govalve.Subscription) error {
	if s.failMode == "save_error" {
		return fmt.Errorf("simulated save error")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscriptions[sub.UserID] = sub
	return nil
}

func (s *EdgeCaseStore) CheckAndIncrementUsage(ctx context.Context, userID string, quota int64) error {
	if s.failMode == "usage_error" {
		return fmt.Errorf("simulated usage check error")
	}
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
	testInvalidConfigurations()
	testSubscriptionEdgeCases()
	testConcurrentAccess()
	testErrorHandling()
}

func testInvalidConfigurations() {
	fmt.Println("=== Testing Invalid Configurations ===")
	ctx := context.Background()
	manager := govalve.NewManager(ctx)
	defer manager.Shutdown()

	// Test invalid worker pool
	err := manager.RegisterProfile("invalid-workers", govalve.WithWorkerPool(0, 100))
	fmt.Printf("Zero workers: %v\n", err)

	// Test invalid queue size
	err = manager.RegisterProfile("invalid-queue", govalve.WithWorkerPool(5, 0))
	fmt.Printf("Zero queue: %v\n", err)

	// Test missing shared key
	err = manager.RegisterProfile("no-shared-key", govalve.WithWorkerPool(5, 100))
	fmt.Printf("Missing shared key: %v\n", err)

	// Test duplicate profile
	manager.RegisterProfile("duplicate", govalve.WithSharedResource("key", rate.Limit(1)), govalve.WithWorkerPool(1, 1))
	err = manager.RegisterProfile("duplicate", govalve.WithSharedResource("key2", rate.Limit(1)), govalve.WithWorkerPool(1, 1))
	fmt.Printf("Duplicate profile: %v\n", err)
}

func testSubscriptionEdgeCases() {
	fmt.Println("\n=== Testing Subscription Edge Cases ===")
	ctx := context.Background()
	store := NewEdgeCaseStore()
	manager := govalve.NewManager(ctx, govalve.WithStore(store))
	defer manager.Shutdown()

	manager.RegisterProfile("test", govalve.WithDedicatedResource(rate.Limit(10)), govalve.WithWorkerPool(1, 1))

	// Test subscription to non-existent profile
	_, err := manager.Subscribe(ctx, "user1", "non-existent", time.Hour)
	fmt.Printf("Non-existent profile: %v\n", err)

	// Test store save error
	store.failMode = "save_error"
	_, err = manager.Subscribe(ctx, "user2", "test", time.Hour)
	fmt.Printf("Save error: %v\n", err)
	store.failMode = ""

	// Test expired subscription
	manager.Subscribe(ctx, "user3", "test", -time.Hour) // Already expired
	_, err = manager.GetLimiter(ctx, "user3")
	fmt.Printf("Expired subscription: %v\n", err)

	// Test canceled subscription
	sub, _ := manager.Subscribe(ctx, "user4", "test", time.Hour)
	sub.Status = govalve.SubscriptionStatusCanceled
	store.SaveSubscription(ctx, sub)
	_, err = manager.GetLimiter(ctx, "user4")
	fmt.Printf("Canceled subscription: %v\n", err)
}

func testConcurrentAccess() {
	fmt.Println("\n=== Testing Concurrent Access ===")
	ctx := context.Background()
	store := NewEdgeCaseStore()
	manager := govalve.NewManager(ctx, govalve.WithStore(store))
	defer manager.Shutdown()

	manager.RegisterProfile("concurrent", govalve.WithSharedResource("shared", rate.Limit(100)), govalve.WithWorkerPool(10, 50))
	manager.Subscribe(ctx, "concurrent-user", "concurrent", time.Hour)

	var wg sync.WaitGroup
	// Multiple goroutines trying to get the same limiter
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			limiter, err := manager.GetLimiter(ctx, "concurrent-user")
			if err != nil {
				fmt.Printf("Concurrent access %d failed: %v\n", id, err)
				return
			}
			limiter.Submit(ctx, func() {
				fmt.Printf("Concurrent task %d executed\n", id)
			})
		}(i)
	}
	wg.Wait()
}

func testErrorHandling() {
	fmt.Println("\n=== Testing Error Handling ===")
	ctx := context.Background()
	store := NewEdgeCaseStore()
	manager := govalve.NewManager(ctx, govalve.WithStore(store))
	defer manager.Shutdown()

	manager.RegisterProfile("error-test", govalve.WithDedicatedResource(rate.Limit(10)), govalve.WithWorkerPool(1, 1), govalve.WithUsageQuota(2))
	manager.Subscribe(ctx, "error-user", "error-test", time.Hour)

	limiter, _ := manager.GetLimiter(ctx, "error-user")

	// Test usage quota exceeded
	limiter.Submit(ctx, func() { fmt.Println("Task 1") })
	limiter.Submit(ctx, func() { fmt.Println("Task 2") })
	err := limiter.Submit(ctx, func() { fmt.Println("Task 3 - should fail") })
	fmt.Printf("Quota exceeded: %v\n", err)

	// Test store errors during submit
	store.failMode = "usage_error"
	err = limiter.Submit(ctx, func() { fmt.Println("Should not execute") })
	fmt.Printf("Store error during submit: %v\n", err)
}