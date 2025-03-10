// Package worker provides interfaces and implementations for tasks and workers.
package worker

import (
	"context"
	"errors"
	"time"
)

// Task represents a unit of work that can be executed by a worker.
// Tasks have an identifier, type, and priority, and can be executed
// within a context. They also specify retry behavior and can be validated.
type Task interface {
	// ID returns the unique identifier for this task.
	ID() string

	// Type returns the type of this task, useful for categorization.
	Type() string

	// Priority returns the priority level of this task.
	// Higher values indicate higher priority.
	Priority() int

	// MaxRetries returns the maximum number of retry attempts allowed for this task.
	MaxRetries() int

	// Validate checks if the task is valid and can be executed.
	// Returns an error if the task is invalid.
	Validate() error

	// Execute runs the task and returns its result or an error.
	// The provided context can be used to signal cancellation or timeout.
	Execute(ctx context.Context) (interface{}, error)
}

// TaskFunc is a function that can be executed as a task.
// It accepts a context and returns a result or an error.
type TaskFunc func(ctx context.Context) (interface{}, error)

// SimpleTask is a basic implementation of the Task interface.
type SimpleTask struct {
	TaskID       string
	TaskType     string
	TaskPriority int
	MaxRetry     int
	Function     TaskFunc
	ValidateFunc func() error
	Timeout      time.Duration
	Callback     func(Result)
}

// NewSimpleTask creates a new SimpleTask with the specified ID, type, and function.
// Returns a pointer to the created SimpleTask.
func NewSimpleTask(id, taskType string, fn TaskFunc) *SimpleTask {
	return &SimpleTask{
		TaskID:   id,
		TaskType: taskType,
		Function: fn,
		MaxRetry: 3,
	}
}

// WithPriority sets the priority of the task and returns the modified task.
// Higher values indicate higher priority.
func (t *SimpleTask) WithPriority(priority int) *SimpleTask {
	t.TaskPriority = priority
	return t
}

// WithRetries sets the maximum number of retry attempts for the task and returns the modified task.
func (t *SimpleTask) WithRetries(retries int) *SimpleTask {
	t.MaxRetry = retries
	return t
}

// WithTimeout sets a timeout duration for task execution and returns the modified task.
func (t *SimpleTask) WithTimeout(timeout time.Duration) *SimpleTask {
	t.Timeout = timeout
	return t
}

// WithValidator sets a custom validation function for the task and returns the modified task.
// The validator will be called when Validate() is invoked.
func (t *SimpleTask) WithValidator(validator func() error) *SimpleTask {
	t.ValidateFunc = validator
	return t
}

// WithCallback sets a function to be called when the task completes and returns the modified task.
// The callback will receive the Result struct containing the task's output or error.
func (t *SimpleTask) WithCallback(callback func(Result)) *SimpleTask {
	t.Callback = callback
	return t
}

// ID returns the unique identifier for this task.
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
	if t.Function == nil {
		return errors.New("task function cannot be nil")
	}
	if t.ValidateFunc != nil {
		return t.ValidateFunc()
	}
	return nil
}

// MaxRetries returns the maximum number of retries for the task
func (t *SimpleTask) MaxRetries() int {
	return t.MaxRetry
}

// Execute runs the task function
func (t *SimpleTask) Execute(ctx context.Context) (interface{}, error) {
	if t.Function == nil {
		return nil, errors.New("task function is nil")
	}
	return t.Function(ctx)
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
			TaskID:   id,
			TaskType: "function",
			Function: ctxFn,
			MaxRetry: 3,
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
			// Context was canceled, no need to send results
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
