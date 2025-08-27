package govalve

import "time"

// SubscriptionStatus represents the status of a subscription.
type SubscriptionStatus string

const (
	// SubscriptionStatusActive means the subscription is active and can be used.
	SubscriptionStatusActive SubscriptionStatus = "active"
	// SubscriptionStatusExpired means the subscription has expired and cannot be used.
	SubscriptionStatusExpired SubscriptionStatus = "expired"
	// SubscriptionStatusCanceled means the subscription has been canceled and cannot be used.
	SubscriptionStatusCanceled SubscriptionStatus = "canceled"
)

// Subscription represents a user's subscription to a service.
type Subscription struct {
	UserID      string
	ProfileName string
	Status      SubscriptionStatus
	StartDate   time.Time
	EndDate     time.Time
	Usage       int64
}
