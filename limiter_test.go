package govalve

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func TestLimiter(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(ctx)
	err := manager.RegisterProfile("test-profile",
		WithDedicatedResource(rate.Limit(10)),
		WithWorkerPool(1, 1),
	)
	assert.NoError(t, err)

	limiter, err := manager.GetLimiterForProfile(ctx, "test-profile", "test-user")
	assert.NoError(t, err)

	// Test submitting a task
	var wg sync.WaitGroup
	wg.Add(1)
	err = limiter.Submit(ctx, func() {
		wg.Done()
	})
	assert.NoError(t, err)
	wg.Wait()

	// Test closing the limiter
	limiter.Close()
	err = limiter.Submit(ctx, func() {})
	assert.Error(t, err)
}

func TestRateLimiting(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(ctx)
	err := manager.RegisterProfile("test-profile",
		WithDedicatedResource(rate.Limit(1)),
		WithWorkerPool(1, 1),
	)
	assert.NoError(t, err)

	limiter, err := manager.GetLimiterForProfile(ctx, "test-profile", "test-user")
	assert.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(2)

	start := time.Now()
	err = limiter.Submit(ctx, func() {
		wg.Done()
	})
	assert.NoError(t, err)
	err = limiter.Submit(ctx, func() {
		wg.Done()
	})
	assert.NoError(t, err)

	wg.Wait()
	duration := time.Since(start)

	assert.GreaterOrEqual(t, duration, time.Second)
}
