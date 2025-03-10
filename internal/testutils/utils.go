package testutils

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// WaitForCondition waits for a condition to be true or times out
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, message string) bool {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Errorf("Timed out waiting for condition: %s", message)
	return false
}

// CounterTask is a simple task that increments a counter
type CounterTask struct {
	*MockTask
	counter   *int
	counterMu sync.Mutex
}

// NewCounterTask creates a new counter task
func NewCounterTask(id string, counter *int) *CounterTask {
	task := &CounterTask{
		counter: counter,
	}

	task.MockTask = NewMockTask(id, func(ctx context.Context) (interface{}, error) {
		task.counterMu.Lock()
		defer task.counterMu.Unlock()
		*task.counter++
		return *task.counter, nil
	})

	return task
}

// ErrorAfterNExecutionsTask returns error after N executions
type ErrorAfterNExecutionsTask struct {
	*MockTask
	n         int
	execCount int
	execMu    sync.Mutex
}

// NewErrorAfterNExecutionsTask creates a new task that fails after N executions
func NewErrorAfterNExecutionsTask(id string, n int) *ErrorAfterNExecutionsTask {
	task := &ErrorAfterNExecutionsTask{
		n: n,
	}

	task.MockTask = NewMockTask(id, func(ctx context.Context) (interface{}, error) {
		task.execMu.Lock()
		defer task.execMu.Unlock()

		task.execCount++
		if task.execCount > task.n {
			return nil, errors.New("error after N executions")
		}
		return task.execCount, nil
	})

	return task
}

// ContextCheckTask is a task that checks if context is respected
type ContextCheckTask struct {
	*MockTask
	canceled     bool
	canceledLock sync.Mutex
}

// NewContextCheckTask creates a new task that checks context cancellation
func NewContextCheckTask(id string) *ContextCheckTask {
	task := &ContextCheckTask{}

	task.MockTask = NewMockTask(id, func(ctx context.Context) (interface{}, error) {
		select {
		case <-time.After(100 * time.Millisecond):
			return "completed", nil
		case <-ctx.Done():
			task.canceledLock.Lock()
			task.canceled = true
			task.canceledLock.Unlock()
			return nil, ctx.Err()
		}
	})

	return task
}

// WasCanceled returns true if the task detected context cancellation
func (t *ContextCheckTask) WasCanceled() bool {
	t.canceledLock.Lock()
	defer t.canceledLock.Unlock()
	return t.canceled
}
