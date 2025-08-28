package govalve

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func TestManager(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore()
	manager := NewManager(ctx, WithStore(store))

	// Test profile registration
	err := manager.RegisterProfile("test-profile",
		WithDedicatedResource(rate.Limit(10)),
		WithWorkerPool(1, 1),
	)
	assert.NoError(t, err)

	// Test duplicate profile registration
	err = manager.RegisterProfile("test-profile",
		WithDedicatedResource(rate.Limit(10)),
		WithWorkerPool(1, 1),
	)
	assert.Error(t, err)

	// Test subscription management
	sub, err := manager.Subscribe(ctx, "test-user", "test-profile", time.Hour)
	assert.NoError(t, err)
	assert.NotNil(t, sub)

	retrievedSub, err := manager.GetSubscription(ctx, "test-user")
	assert.NoError(t, err)
	assert.Equal(t, sub, retrievedSub)

	// Test getting a limiter
	limiter, err := manager.GetLimiter(ctx, "test-user")
	assert.NoError(t, err)
	assert.NotNil(t, limiter)

	// Test getting a limiter for a user with an expired subscription
	sub.EndDate = time.Now().Add(-time.Hour)
	err = store.SaveSubscription(ctx, sub)
	assert.NoError(t, err)

	_, err = manager.GetLimiter(ctx, "test-user")
	assert.Error(t, err)
}
