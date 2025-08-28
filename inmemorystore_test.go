package govalve

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInMemoryStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()

	// Test SaveSubscription and GetSubscription
	sub := &Subscription{
		UserID:    "test-user",
		ProfileName: "test-profile",
		Status:      SubscriptionStatusActive,
		StartDate:   time.Now(),
		EndDate:     time.Now().Add(24 * time.Hour),
		Usage:       0,
		Quota:       100,
	}

	err := store.SaveSubscription(ctx, sub)
	assert.NoError(t, err)

	retrievedSub, err := store.GetSubscription(ctx, "test-user")
	assert.NoError(t, err)
	assert.Equal(t, sub, retrievedSub)

	// Test GetSubscription for non-existent user
	_, err = store.GetSubscription(ctx, "non-existent-user")
	assert.Error(t, err)

	// Test CheckAndIncrementUsage
	err = store.CheckAndIncrementUsage(ctx, "test-user", 100)
	assert.NoError(t, err)

	retrievedSub, err = store.GetSubscription(ctx, "test-user")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), retrievedSub.Usage)

	// Test CheckAndIncrementUsage with quota exceeded
	retrievedSub.Usage = 100
	err = store.SaveSubscription(ctx, retrievedSub)
	assert.NoError(t, err)

	err = store.CheckAndIncrementUsage(ctx, "test-user", 100)
	assert.Error(t, err)
}
