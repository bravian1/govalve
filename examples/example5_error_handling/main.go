package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bravian1/govalve"
	"golang.org/x/time/rate"
)

// FlakyStore simulates store failures for testing retry logic
type FlakyStore struct {
	subscriptions map[string]*govalve.Subscription
	mu            sync.RWMutex
	failCount     int
	maxFails      int
}

func NewFlakyStore(maxFails int) *FlakyStore {
	return &FlakyStore{
		subscriptions: make(map[string]*govalve.Subscription),
		maxFails:      maxFails,
	}
}

func (s *FlakyStore) GetSubscription(ctx context.Context, userID string) (*govalve.Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.subscriptions[userID]
	if !ok {
		return nil, fmt.Errorf("subscription not found for user %s", userID)
	}
	return sub, nil
}

func (s *FlakyStore) SaveSubscription(ctx context.Context, sub *govalve.Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Simulate failures for first few attempts
	if s.failCount < s.maxFails {
		s.failCount++
		fmt.Printf("Store failure %d/%d - simulating network error\n", s.failCount, s.maxFails)
		return govalve.RetryableError{Err: fmt.Errorf("network timeout")}
	}
	
	s.subscriptions[sub.UserID] = sub
	fmt.Println("Store operation succeeded after retries")
	return nil
}

func (s *FlakyStore) CheckAndIncrementUsage(ctx context.Context, userID string, quota int64) error {
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

	// Create flaky store that fails first 2 attempts
	store := NewFlakyStore(2)
	dlq := govalve.NewInMemoryDeadLetterQueue()
	
	// Configure custom retry settings
	retryConfig := govalve.RetryConfig{
		MaxAttempts: 4,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    1 * time.Second,
		Multiplier:  2.0,
	}

	// Create manager with error handling
	manager := govalve.NewManager(appCtx,
		govalve.WithStore(store),
		govalve.WithRetryConfig(retryConfig),
		govalve.WithDeadLetterQueue(dlq))
	defer manager.Shutdown()

	// Register profile
	manager.RegisterProfile("error-test",
		govalve.WithDedicatedResource(rate.Limit(5)),
		govalve.WithWorkerPool(3, 10),
		govalve.WithUsageQuota(5))

	fmt.Println("=== Testing Store Retry Logic ===")
	// This should succeed after retries
	userID := "retry-user"
	_, err := manager.Subscribe(appCtx, userID, "error-test", 10*time.Second)
	if err != nil {
		fmt.Printf("Subscription failed: %v\n", err)
	} else {
		fmt.Println("Subscription succeeded with retries!")
	}

	fmt.Println("\n=== Testing Task Error Handling ===")
	limiter, _ := manager.GetLimiter(appCtx, userID)
	
	var wg sync.WaitGroup
	
	// Submit normal task
	wg.Add(1)
	limiter.Submit(appCtx, func() {
		defer wg.Done()
		fmt.Println("Normal task executed successfully")
	})
	
	// Submit panicking task
	wg.Add(1)
	limiter.Submit(appCtx, func() {
		defer wg.Done()
		fmt.Println("About to panic...")
		panic("simulated task failure")
	})
	
	// Submit another normal task
	wg.Add(1)
	limiter.Submit(appCtx, func() {
		defer wg.Done()
		fmt.Println("Another normal task executed")
	})
	
	wg.Wait()
	
	// Check dead letter queue
	fmt.Println("\n=== Checking Dead Letter Queue ===")
	time.Sleep(100 * time.Millisecond) // Let DLQ operations complete
	
	failed, err := dlq.GetFailed(appCtx, 10)
	if err != nil {
		fmt.Printf("Error getting failed tasks: %v\n", err)
	} else {
		fmt.Printf("Found %d failed tasks in dead letter queue\n", len(failed))
		for i, task := range failed {
			fmt.Printf("Failed task %d: User=%s, Error=%v, Time=%s\n", 
				i+1, task.UserID, task.Error, task.Timestamp.Format("15:04:05"))
		}
	}
}