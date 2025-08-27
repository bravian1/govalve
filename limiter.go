package govalve

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Task is a function to be executed by a worker.
// It is the basic unit of work in govalve.
// The user of the library provides the implementation of the task.
// For example, a task could be an API call, a database query, or any other operation that needs to be rate-limited.
type Task func()

// Limiter is the interface for submitting tasks to be executed.
// It is responsible for rate-limiting and managing the task queue.
// The user of the library interacts with the Limiter to submit tasks.
// The Limiter implementation is not exposed to the user.
// The user gets a Limiter instance from the Manager.
type Limiter interface {
	// Submit submits a task to the limiter.
	// It blocks until the task is submitted or the context is canceled.
	// If the subscription has a usage quota, it checks if the quota is exceeded.
	// If the quota is exceeded, it returns an error.
	// If the limiter is closed, it returns an error.
	// If the context is canceled, it returns an error.
	Submit(ctx context.Context, task Task) error

	// Close closes the limiter and waits for all workers to finish.
	// After a limiter is closed, no more tasks can be submitted.
	Close()
}

// limiterFunc is the internal implementation of the Limiter interface.
// It is not exposed to the user of the library.
// It manages the task queue, the worker pool, and the rate limiter.
// It also interacts with the Manager to get subscription information.
type limiterFunc struct {
	taskQueue   chan Task
	wg          sync.WaitGroup
	rateLimiter *rate.Limiter
	closeOnce   sync.Once
	mu 		sync.Mutex
	isClosed	bool

	ctx    context.Context
	cancel context.CancelFunc

	manager *Manager
	userID  string
}

// newLimiter creates a new limiter.
// It is called by the Manager when a new limiter is needed.
// It is not exposed to the user of the library.
func newLimiter(ctx context.Context, config *limiterConfig, manager *Manager, userID string) (Limiter, error) {
	if config.workerPoolSize <= 0 {
		return nil, fmt.Errorf("workerPoolSize must be greater than 0, got %d", config.workerPoolSize)
	}
	if config.queueSize <= 0 {
		return nil, fmt.Errorf("queueSize must be greater than 0, got %d", config.queueSize)
	}
	if config.rateLimiterSize == nil {
		return nil, fmt.Errorf("rateLimiterSize must not be nil")
	}
	limiterCtx, cancel := context.WithCancel(ctx)

	l := &limiterFunc{
		taskQueue:   make(chan Task, config.queueSize),
		rateLimiter: config.rateLimiterSize,
		ctx:         limiterCtx,
		cancel:      cancel,
		manager:     manager,
		userID:      userID,
	}
	l.startWorkers(config.workerPoolSize)

	return l, nil
}

// Submit submits a task to the limiter.
// It is the implementation of the Submit method of the Limiter interface.
func (l *limiterFunc) Submit(ctx context.Context, task Task) error {
	l.mu.Lock()

	if l.isClosed {
		l.mu.Unlock()
		return fmt.Errorf("limiter is closed")
	}
	l.mu.Unlock()

	// If a store is configured, check and increment the subscription's usage.
	if l.manager != nil && l.manager.store != nil {
		sub, err := l.manager.GetSubscription(ctx, l.userID)
		if err != nil {
			return fmt.Errorf("failed to get subscription: %w", err)
		}
		if sub == nil {
			return fmt.Errorf("subscription not found for user %q", l.userID)
		}
		now := time.Now()
		if sub.Status != SubscriptionStatusActive || now.Before(sub.StartDate) || now.After(sub.EndDate) {
			return fmt.Errorf("subscription is not active for user %q", l.userID)
		}

		profile, ok := l.manager.profiles[sub.ProfileName]
		if !ok {
			return fmt.Errorf("profile '%s' not found", sub.ProfileName)
		}

		err = l.manager.store.CheckAndIncrementUsage(ctx, l.userID, profile.usageQuota)
		if err != nil {
			return err
		}
	}

	// Submit the task to the task queue.
	// It blocks until the task is submitted or the context is canceled.
	select {
	case l.taskQueue <- task:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("failed to submit task: %w", ctx.Err())
	case <-l.ctx.Done():
		return fmt.Errorf("failed to submit task: limiter is shutting down")
	}
}

// startWorkers starts the worker pool.
// It is called by newLimiter when a new limiter is created.
func (l *limiterFunc) startWorkers(workerCount int) {
	l.wg.Add(workerCount)
	for i := range workerCount {
		go func(workerID int) {
			defer l.wg.Done()

			for {
				select {
				case task, ok := <-l.taskQueue:
					if !ok {
						return
					}

					// Wait for the rate limiter.
					err := l.rateLimiter.Wait(l.ctx)
					if err != nil {
						return
					}

					// Execute the task.
					task()

				case <-l.ctx.Done():
					return
				}

			}
		}(i)

	}
}

// Close closes the limiter and waits for all workers to finish.
// It is the implementation of the Close method of the Limiter interface.
func (l *limiterFunc) Close() {
	l.closeOnce.Do(func() {
		l.mu.Lock()
		l.isClosed = true
		l.mu.Unlock()

		close(l.taskQueue)
		l.wg.Wait()
	})
}
