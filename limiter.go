package govalve

import (
	"context"
	"fmt"
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


func newLimiter(ctx context.Context, config *limiterConfig) (Limiter, error) {
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
		taskQueue:  make(chan Task, config.queueSize),
		rateLimiter: config.rateLimiterSize,
		ctx: limiterCtx,
		cancel: cancel,
	}
	l.startWorkers(config.workerPoolSize)

	return l, nil
}

func (l *limiterFunc) Submit(ctx context.Context, task Task) error {
	l.mu.Lock()
	
	if l.isClosed {
		l.mu.Unlock()
		return fmt.Errorf("limiter is closed")
	}
	l.mu.Unlock()
	
	select {
		case l.taskQueue <- task:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("failed to submit task: %w", ctx.Err())
		case <-l.ctx.Done():
			return fmt.Errorf("failed to submit task: limiter is shutting down")
	}
}

func (l *limiterFunc) startWorkers(workerCount int) {
	l.wg.Add(workerCount)
	for i := range workerCount {
		go func(workerID int) {
			defer l.wg.Done()

			for{
				select {
					case task, ok := <-l.taskQueue:
						if !ok {
							return
						}

						err := l.rateLimiter.Wait(l.ctx)
						if err != nil {
							return 
						}
						task()
					case <-l.ctx.Done():
						return
				}


			}
		}(i)

	}
}

func (l *limiterFunc) Close() {
	l.closeOnce.Do(func(){
		l.mu.Lock()
		l.isClosed = true
		l.mu.Unlock()

		close(l.taskQueue)
		l.wg.Wait()
	})
}