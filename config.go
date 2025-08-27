package govalve

import (
	"fmt"

	"golang.org/x/time/rate"
)

type Option func(*limiterConfig) error

type limiterConfig struct {
	workerPoolSize  int
	queueSize       int
	rateLimiterSize *rate.Limiter
	usageQuota      int64

	isDedicated bool
	sharedKey   string
}

// WithWorkerPool sets the number of concurrent workers and the size of the task queue.
func WithWorkerPool(size, queueSize int) Option {
	return func(c *limiterConfig) error {
		if size <= 0 {
			return fmt.Errorf("worker pool size must be greater than 0")
		}
		if queueSize <= 0 {
			return fmt.Errorf("queue size must be greater than 0")
		}
		c.workerPoolSize = size
		c.queueSize = queueSize
		return nil
	}
}

// WithSharedResource sets up a shared resource with a rate limiter.
func WithSharedResource(key string, limit rate.Limit) Option {
	return func(c *limiterConfig) error {
		if key == "" {
			return fmt.Errorf("shared resource key cannot be empty")
		}
		c.isDedicated = false
		c.sharedKey = key
		c.rateLimiterSize = rate.NewLimiter(limit, int(limit))
		return nil
	}
}

// WithDedicatedResource sets up a dedicated resource with a rate limiter.
func WithDedicatedResource(limit rate.Limit) Option {
	return func(c *limiterConfig) error {
		c.isDedicated = true
		c.rateLimiterSize = rate.NewLimiter(limit, int(limit))
		return nil
	}
}

// WithUsageQuota sets the usage quota for a subscription.
func WithUsageQuota(quota int64) Option {
	return func(c *limiterConfig) error {
		if quota <= 0 {
			return fmt.Errorf("usage quota must be greater than 0")
		}
		c.usageQuota = quota
		return nil
	}
}
