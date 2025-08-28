package govalve

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FailedTask represents a task that failed to execute
type FailedTask struct {
	ID        string
	Task      Task
	UserID    string
	ProfileName string
	Error     error
	Timestamp time.Time
	Attempts  int
}

// DeadLetterQueue handles failed tasks that couldn't be processed
type DeadLetterQueue interface {
	// Add adds a failed task to the dead letter queue
	Add(ctx context.Context, failedTask *FailedTask) error
	// GetFailed retrieves failed tasks for inspection/reprocessing
	GetFailed(ctx context.Context, limit int) ([]*FailedTask, error)
	// Remove removes a task from the dead letter queue
	Remove(ctx context.Context, taskID string) error
}

// InMemoryDeadLetterQueue is a simple in-memory implementation
type InMemoryDeadLetterQueue struct {
	tasks map[string]*FailedTask
	mu    sync.RWMutex
}

func NewInMemoryDeadLetterQueue() *InMemoryDeadLetterQueue {
	return &InMemoryDeadLetterQueue{
		tasks: make(map[string]*FailedTask),
	}
}

func (dlq *InMemoryDeadLetterQueue) Add(ctx context.Context, failedTask *FailedTask) error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()

	failedTask.ID = uuid.NewString()
	dlq.tasks[failedTask.ID] = failedTask
	return nil
}

func (dlq *InMemoryDeadLetterQueue) GetFailed(ctx context.Context, limit int) ([]*FailedTask, error) {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()

	var failed []*FailedTask
	count := 0
	for _, task := range dlq.tasks {
		if count >= limit {
			break
		}
		failed = append(failed, task)
		count++
	}
	return failed, nil
}

func (dlq *InMemoryDeadLetterQueue) Remove(ctx context.Context, taskID string) error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()

	delete(dlq.tasks, taskID)
	return nil
}
