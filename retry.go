package govalve

import (
	"context"
	"fmt"
	"math"
	"time"
)

// RetryConfig defines retry behavior for operations
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

// DefaultRetryConfig provides sensible defaults for retry behavior
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		Multiplier:  2.0,
	}
}

// RetryableError indicates an operation can be retried
type RetryableError struct {
	Err error
}

func (e RetryableError) Error() string {
	return fmt.Sprintf("retryable error: %v", e.Err)
}

func (e RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable checks if an error should trigger a retry
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(RetryableError)
	return ok
}

// WithRetry executes a function with exponential backoff retry logic
func WithRetry(ctx context.Context, config RetryConfig, operation func() error) error {
	var lastErr error
	
	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if attempt > 0 {
			// Calculate delay with exponential backoff
			delay := time.Duration(float64(config.BaseDelay) * math.Pow(config.Multiplier, float64(attempt-1)))
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
			
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		
		err := operation()
		if err == nil {
			return nil
		}
		
		lastErr = err
		if !IsRetryable(err) {
			return err
		}
	}
	
	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}