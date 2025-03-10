package worker

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrTaskQueueFull is returned when the task queue is full
	ErrTaskQueueFull = errors.New("task queue is full")

	// ErrManagerNotStarted is returned when trying to use a manager that hasn't been started
	ErrManagerNotStarted = errors.New("manager not started")

	// ErrManagerShuttingDown is returned when trying to use a manager that is shutting down
	ErrManagerShuttingDown = errors.New("manager is shutting down")
)

// TaskError represents an error that occurred during task execution
type TaskError struct {
	TaskID  string
	Message string
	Err     error
	Attempt int
}

// Error implements the error interface
func (e *TaskError) Error() string {
	return fmt.Sprintf("task %s failed on attempt %d: %s", e.TaskID, e.Attempt, e.Message)
}

// Unwrap returns the underlying error
func (e *TaskError) Unwrap() error {
	return e.Err
}

// NewTaskError creates a new TaskError
func NewTaskError(taskID string, attempt int, message string, err error) *TaskError {
	return &TaskError{
		TaskID:  taskID,
		Message: message,
		Err:     err,
		Attempt: attempt,
	}
}

// TaskTimeoutError is returned when a task execution times out
type TaskTimeoutError struct {
	TaskID  string
	Timeout time.Duration
}

// Error implements the error interface
func (e *TaskTimeoutError) Error() string {
	return fmt.Sprintf("task %s timed out after %v", e.TaskID, e.Timeout)
}
