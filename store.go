package govalve

import "context"

// Store is the interface for persisting subscription data.
// The user of the library is responsible for providing a concrete
// implementation of this interface.
type Store interface {
	// GetSubscription retrieves a subscription for a given user ID.
	GetSubscription(ctx context.Context, userID string) (*Subscription, error)
	// SaveSubscription saves a subscription.
	SaveSubscription(ctx context.Context, sub *Subscription) error
	// CheckAndIncrementUsage checks if the usage quota is exceeded and increments the usage if it is not.
	// This should be an atomic operation.
	CheckAndIncrementUsage(ctx context.Context, userID string, quota int64) error
}
