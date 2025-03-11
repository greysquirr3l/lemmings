// Package worker provides worker and task implementations.
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

	// ErrTaskCanceled is returned when a task is canceled during execution.
	ErrTaskCanceled = errors.New("task canceled")

	// ErrTaskTimeout is returned when a task exceeds its execution time limit.
	ErrTaskTimeout = errors.New("task timeout")

	// ErrTaskValidation is returned when a task fails validation.
	ErrTaskValidation = errors.New("task validation failed")

	// ErrInvalidTask is returned when a task is invalid.
	ErrInvalidTask = errors.New("invalid task")

	// ErrWorkerBusy is returned when a worker is already processing a task.
	ErrWorkerBusy = errors.New("worker is busy")

	// ErrWorkerStopped is returned when attempting to use a stopped worker.
	ErrWorkerStopped = errors.New("worker is stopped")
)

// TaskError represents an error that occurred during task execution.
// It includes information about the task and the attempt number.
type TaskError struct {
	TaskID  string // The ID of the task that failed
	Attempt int    // The attempt number when the failure occurred
	Message string // Description of the error
	Cause   error  // The underlying error
}

// Error implements the error interface.
// It returns a formatted error message including task ID and attempt number.
func (e *TaskError) Error() string {
	return fmt.Sprintf("task %s failed on attempt %d: %s", e.TaskID, e.Attempt, e.Message)
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
func (e *TaskError) Unwrap() error {
	return e.Cause
}

// NewTaskError creates a TaskError with the given parameters.
// It returns a pointer to the created TaskError.
func NewTaskError(taskID string, attempt int, message string, cause error) *TaskError {
	return &TaskError{
		TaskID:  taskID,
		Attempt: attempt,
		Message: message,
		Cause:   cause,
	}
}

// TaskTimeoutError represents a timeout error during task execution.
type TaskTimeoutError struct {
	TaskID  string        // The ID of the task that timed out
	Timeout time.Duration // The duration after which the task timed out
}

// Error implements the error interface.
// It returns a formatted error message including task ID and timeout duration.
func (e *TaskTimeoutError) Error() string {
	return fmt.Sprintf("task %s timed out after %v", e.TaskID, e.Timeout)
}
