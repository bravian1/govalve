package govalve

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

type Task func()

type Limiter interface {
	Submit(ctx context.Context, task Task) error

	Close()
}

type limiterFunc struct {
	taskQueue   chan Task
	wg          sync.WaitGroup
	rateLimiter *rate.Limiter
	closeOnce   sync.Once
	mu 		sync.Mutex
	isClosed	bool

	ctx context.Context
	cancel context.CancelFunc
}


