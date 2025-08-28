package govalve

import (
	"context"
	"fmt"
	"sync"
)

// InMemoryStore is an in-memory implementation of the Store interface for demonstration purposes.
type InMemoryStore struct {
	subscriptions map[string]*Subscription
	mu            sync.RWMutex
}

// NewInMemoryStore creates a new InMemoryStore.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		subscriptions: make(map[string]*Subscription),
	}
}

// GetSubscription retrieves a subscription from the in-memory store.
func (s *InMemoryStore) GetSubscription(ctx context.Context, userID string) (*Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sub, ok := s.subscriptions[userID]
	if !ok {
		return nil, fmt.Errorf("subscription not found for user %s", userID)
	}

	return sub, nil
}

// SaveSubscription saves a subscription to the in-memory store.
func (s *InMemoryStore) SaveSubscription(ctx context.Context, sub *Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subscriptions[sub.UserID] = sub
	return nil
}

// CheckAndIncrementUsage checks quota and increments usage atomically.
func (s *InMemoryStore) CheckAndIncrementUsage(ctx context.Context, userID string, quota int64) error {
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
