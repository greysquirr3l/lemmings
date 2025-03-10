package worker

import (
	"context"
	"errors"
	"time"
)

// Task represents a unit of work that can be executed by a Worker
type Task interface {
	Execute(ctx context.Context) (interface{}, error)
	ID() string
	Type() string
	Priority() int
	Validate() error
	MaxRetries() int
}

// SimpleTask is a basic implementation of the Task interface
type SimpleTask struct {
	TaskID       string
	TaskType     string
	TaskPriority int
	TaskRetries  int
	TaskFunc     func(ctx context.Context) (interface{}, error)
	Validator    func() error
}

// Execute runs the task function
func (t *SimpleTask) Execute(ctx context.Context) (interface{}, error) {
	if t.TaskFunc == nil {
		return nil, errors.New("task function is nil")
	}
	return t.TaskFunc(ctx)
}

// ID returns the task's identifier
func (t *SimpleTask) ID() string {
	return t.TaskID
}

// Type returns the task type
func (t *SimpleTask) Type() string {
	return t.TaskType
}

// Priority returns the task priority
func (t *SimpleTask) Priority() int {
	return t.TaskPriority
}

// Validate checks if the task is valid
func (t *SimpleTask) Validate() error {
	if t.TaskID == "" {
		return errors.New("task ID cannot be empty")
	}
	if t.TaskFunc == nil {
		return errors.New("task function cannot be nil")
	}
	if t.Validator != nil {
		return t.Validator()
	}
	return nil
}

// MaxRetries returns the maximum number of retries for the task
func (t *SimpleTask) MaxRetries() int {
	return t.TaskRetries
}

// NewSimpleTask creates a new simple task
func NewSimpleTask(id, taskType string, fn func(ctx context.Context) (interface{}, error)) *SimpleTask {
	return &SimpleTask{
		TaskID:      id,
		TaskType:    taskType,
		TaskFunc:    fn,
		TaskRetries: 3,
	}
}

// WithPriority sets the task priority
func (t *SimpleTask) WithPriority(priority int) *SimpleTask {
	t.TaskPriority = priority
	return t
}

// WithRetries sets the maximum number of retries
func (t *SimpleTask) WithRetries(retries int) *SimpleTask {
	t.TaskRetries = retries
	return t
}

// WithValidator sets a custom validation function
func (t *SimpleTask) WithValidator(validator func() error) *SimpleTask {
	t.Validator = validator
	return t
}

// FunctionTask is a task that executes a provided function
type FunctionTask struct {
	*SimpleTask
	startTime time.Time
	timeout   time.Duration
	callback  func(result Result)
}

// NewFunctionTask creates a new function task
func NewFunctionTask(id string, fn func(ctx context.Context) (interface{}, error)) *FunctionTask {
	// For backward compatibility
	ctxFn := fn
	if fn == nil {
		ctxFn = func(ctx context.Context) (interface{}, error) {
			return nil, errors.New("task function is nil")
		}
	}

	return &FunctionTask{
		SimpleTask: &SimpleTask{
			TaskID:      id,
			TaskType:    "function",
			TaskFunc:    ctxFn,
			TaskRetries: 3,
		},
		startTime: time.Now(),
	}
}

// WithTimeout sets a timeout for the task
func (t *FunctionTask) WithTimeout(timeout time.Duration) *FunctionTask {
	t.timeout = timeout
	return t
}

// WithCallback sets a callback function that will be called with the task result
func (t *FunctionTask) WithCallback(callback func(result Result)) *FunctionTask {
	t.callback = callback
	return t
}

// Execute runs the function with timeout if specified
func (t *FunctionTask) Execute(ctx context.Context) (interface{}, error) {
	// If no timeout is set, just execute normally
	if t.timeout <= 0 {
		return t.SimpleTask.Execute(ctx)
	}

	// Create a timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	// With timeout
	resultCh := make(chan struct {
		result interface{}
		err    error
	}, 1)

	go func() {
		result, err := t.SimpleTask.Execute(timeoutCtx)
		select {
		case resultCh <- struct {
			result interface{}
			err    error
		}{result, err}:
		case <-timeoutCtx.Done():
			// Context was cancelled, no need to send results
		}
	}()

	select {
	case res := <-resultCh:
		return res.result, res.err
	case <-timeoutCtx.Done():
		if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
			return nil, &TaskTimeoutError{
				TaskID:  t.TaskID,
				Timeout: t.timeout,
			}
		}
		return nil, timeoutCtx.Err()
	}
}

// GetCallback returns the callback function
func (t *FunctionTask) GetCallback() func(result Result) {
	return t.callback
}
