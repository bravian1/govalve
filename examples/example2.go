package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bravian1/govalve"
	"golang.org/x/time/rate"
)

// MockStore is an in-memory implementation of the govalve.Store interface for demonstration purposes.
type MockStore struct {
	subscriptions map[string]*govalve.Subscription
	mu            sync.RWMutex
}

// NewMockStore creates a new MockStore.
func NewMockStore() *MockStore {
	return &MockStore{
		subscriptions: make(map[string]*govalve.Subscription),
	}
}

// GetSubscription retrieves a subscription from the in-memory store.
func (s *MockStore) GetSubscription(ctx context.Context, userID string) (*govalve.Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sub, ok := s.subscriptions[userID]
	if !ok {
		return nil, fmt.Errorf("subscription not found for user %s", userID)
	}

	return sub, nil
}

// SaveSubscription saves a subscription to the in-memory store.
func (s *MockStore) SaveSubscription(ctx context.Context, sub *govalve.Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subscriptions[sub.UserID] = sub
	return nil
}

// CheckAndIncrementUsage checks if the usage quota is exceeded and increments the usage if it is not.
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

	// 1. Create a new MockStore.
	store := NewMockStore()

	// 2. Create a new Manager with the MockStore.
	manager := govalve.NewManager(appCtx, govalve.WithStore(store))
	defer manager.Shutdown()

	// 3. Register a "pro-tier" profile with a usage quota.
	err := manager.RegisterProfile("pro-tier",
		govalve.WithDedicatedResource(rate.Limit(10)),
		govalve.WithWorkerPool(10, 200),
		govalve.WithUsageQuota(5),
	)
	if err != nil {
		panic(err)
	}

	// 4. Subscribe a user to the "pro-tier" for a short duration.
	userID := "pro-user-123"
	_, err = manager.Subscribe(appCtx, userID, "pro-tier", 5*time.Second)
	if err != nil {
		panic(err)
	}

	fmt.Printf("User %s subscribed to pro-tier with a quota of 5 requests.\n", userID)

	// 5. Get a limiter for the user.
	limiter, err := manager.GetLimiter(appCtx, userID)
	if err != nil {
		panic(err)
	}

	// 6. Submit tasks for execution.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		taskNum := i
		go func() {
			defer wg.Done()
			err := limiter.Submit(appCtx, func() {
				fmt.Printf("[%s] [Pro User]  Executing task %d\n", time.Now().Format("15:04:05.000"), taskNum)
				time.Sleep(50 * time.Millisecond) // Simulate work
			})
			if err != nil {
				fmt.Printf("[Pro User] Failed to submit task %d: %v\n", taskNum, err)
			}
		}()
	}

	wg.Wait()

	// 7. Check the user's subscription usage.
	sub, err := manager.GetSubscription(appCtx, userID)
	if err != nil {
		panic(err)
	}

	fmt.Printf("User %s usage: %d/%d\n", userID, sub.Usage, 5)

	// 8. Wait for the subscription to expire.
	fmt.Println("Waiting for subscription to expire...")
	time.Sleep(6 * time.Second)

	// 9. Try to get a limiter for the user again.
	_, err = manager.GetLimiter(appCtx, userID)
	if err != nil {
		fmt.Printf("Successfully blocked user after subscription expired: %v\n", err)
	}
}
