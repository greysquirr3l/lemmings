package middleware

import (
	"context"
	"log"
	"math"
	"time"

	"github.com/greysquirr3l/lemmings/pkg/worker"
)

// TaskMiddleware defines a function that wraps task execution
type TaskMiddleware func(next TaskHandlerFunc) TaskHandlerFunc

// TaskHandlerFunc defines the function signature for handling tasks
type TaskHandlerFunc func(ctx context.Context, task worker.Task) (interface{}, error)

// Chain combines multiple middleware into a single middleware
func Chain(middleware ...TaskMiddleware) TaskMiddleware {
	return func(next TaskHandlerFunc) TaskHandlerFunc {
		for i := len(middleware) - 1; i >= 0; i-- {
			next = middleware[i](next)
		}
		return next
	}
}

// LoggingMiddleware logs execution time and errors
func LoggingMiddleware() TaskMiddleware {
	return func(next TaskHandlerFunc) TaskHandlerFunc {
		return func(ctx context.Context, task worker.Task) (interface{}, error) {
			start := time.Now()
			result, err := next(ctx, task)
			duration := time.Since(start)

			if err != nil {
				// Log error
				log.Printf("Task %s failed after %v: %v", task.ID(), duration, err)
			} else {
				// Log success
				log.Printf("Task %s completed in %v", task.ID(), duration)
			}

			return result, err
		}
	}
}

// RecoveryMiddleware recovers from panics during task execution
func RecoveryMiddleware() TaskMiddleware {
	return func(next TaskHandlerFunc) TaskHandlerFunc {
		return func(ctx context.Context, task worker.Task) (result interface{}, err error) {
			defer func() {
				if r := recover(); r != nil {
					// Convert panic to error
					switch t := r.(type) {
					case error:
						err = t
					case string:
						err = worker.NewTaskError(task.ID(), 0, "panic: "+t, nil)
					default:
						err = worker.NewTaskError(task.ID(), 0, "panic: unknown", nil)
					}
				}
			}()

			return next(ctx, task)
		}
	}
}

// TimeoutMiddleware adds a timeout to task execution
func TimeoutMiddleware(timeout time.Duration) TaskMiddleware {
	return func(next TaskHandlerFunc) TaskHandlerFunc {
		return func(ctx context.Context, task worker.Task) (interface{}, error) {
			timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			return next(timeoutCtx, task)
		}
	}
}

// RetryMiddleware adds retry capability to task execution
func RetryMiddleware(maxRetries int, backoffFactor float64) TaskMiddleware {
	return func(next TaskHandlerFunc) TaskHandlerFunc {
		return func(ctx context.Context, task worker.Task) (interface{}, error) {
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				result, err := next(ctx, task)
				if err == nil {
					return result, nil
				}

				lastErr = err

				// Check context before retry
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
					// Calculate backoff with exponential increase
					if attempt < maxRetries {
						// Fix: Use math.Pow instead of bit shifting with floats
						backoffMs := int(50 * math.Pow(2, float64(attempt)) * backoffFactor)
						backoff := time.Duration(backoffMs) * time.Millisecond

						// Cap maximum backoff
						if backoff > 5*time.Second {
							backoff = 5 * time.Second
						}

						timer := time.NewTimer(backoff)
						select {
						case <-ctx.Done():
							timer.Stop()
							return nil, ctx.Err()
						case <-timer.C:
							// Continue with next attempt
						}
					}
				}
			}

			return nil, worker.NewTaskError(task.ID(), maxRetries+1, "max retries exceeded", lastErr)
		}
	}
}

// WrapTask wraps a Task with middleware
func WrapTask(task worker.Task, middleware TaskMiddleware) worker.Task {
	return &wrappedTask{
		Task:       task,
		middleware: middleware,
	}
}

// wrappedTask is a Task implementation that applies middleware
type wrappedTask struct {
	worker.Task
	middleware TaskMiddleware
}

// Execute runs the task with middleware applied
func (t *wrappedTask) Execute(ctx context.Context) (interface{}, error) {
	handler := t.middleware(func(ctx context.Context, task worker.Task) (interface{}, error) {
		return task.Execute(ctx)
	})

	return handler(ctx, t.Task)
}
