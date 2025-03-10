package middleware

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/greysquirr3l/lemmings/internal/testutils"
	"github.com/greysquirr3l/lemmings/pkg/worker"
)

const (
	successResult = "success" // Extracted constant for success string
)

func TestChain(t *testing.T) {
	t.Run("Chain combines middleware in correct order", func(t *testing.T) {
		// Create a recorder slice to track middleware execution order
		var execOrder []string

		// Create middleware that add to the recorder
		first := func(next TaskHandlerFunc) TaskHandlerFunc {
			return func(ctx context.Context, task worker.Task) (interface{}, error) {
				execOrder = append(execOrder, "first-before")
				result, err := next(ctx, task)
				execOrder = append(execOrder, "first-after")
				return result, err
			}
		}

		second := func(next TaskHandlerFunc) TaskHandlerFunc {
			return func(ctx context.Context, task worker.Task) (interface{}, error) {
				execOrder = append(execOrder, "second-before")
				result, err := next(ctx, task)
				execOrder = append(execOrder, "second-after")
				return result, err
			}
		}

		// Create a test handler
		handler := func(ctx context.Context, task worker.Task) (interface{}, error) {
			execOrder = append(execOrder, "handler")
			return "result", nil
		}

		// Chain the middleware
		chainedHandler := Chain(first, second)(handler)

		// Execute the chain
		task := testutils.NewMockTask("test-task", nil)
		result, err := chainedHandler(context.Background(), task)

		// Check for errors
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Check result
		if result != "result" {
			t.Errorf("Expected result 'result', got %v", result)
		}

		// Check execution order
		expected := []string{
			"first-before", "second-before", "handler", "second-after", "first-after",
		}

		if len(execOrder) != len(expected) {
			t.Errorf("Expected %d steps, got %d", len(expected), len(execOrder))
		}

		for i := 0; i < len(execOrder) && i < len(expected); i++ {
			if execOrder[i] != expected[i] {
				t.Errorf("Step %d: expected %s, got %s", i, expected[i], execOrder[i])
			}
		}
	})
}

func TestLoggingMiddleware(t *testing.T) {
	t.Run("LoggingMiddleware passes through results and errors", func(t *testing.T) {
		// Create middleware
		logging := LoggingMiddleware()

		// Create success handler
		successHandler := func(ctx context.Context, task worker.Task) (interface{}, error) {
			return successResult, nil
		}

		// Create error handler
		expectedErr := errors.New("test error")
		errorHandler := func(ctx context.Context, task worker.Task) (interface{}, error) {
			return nil, expectedErr
		}

		// Test success case
		task := testutils.NewMockTask("success-task", nil)
		wrappedSuccess := logging(successHandler)

		result, err := wrappedSuccess(context.Background(), task)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != successResult {
			t.Errorf("Expected result '%s', got %v", successResult, result)
		}

		// Test error case
		task = testutils.NewMockTask("error-task", nil)
		wrappedError := logging(errorHandler)

		result, err = wrappedError(context.Background(), task)
		if !errors.Is(err, expectedErr) {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
		if result != nil {
			t.Errorf("Expected nil result, got %v", result)
		}
	})
}

func TestRecoveryMiddleware(t *testing.T) {
	t.Run("RecoveryMiddleware recovers from panics", func(t *testing.T) {
		// Create middleware
		recovery := RecoveryMiddleware()

		// Create handler that panics with different values
		panicStringHandler := func(ctx context.Context, task worker.Task) (interface{}, error) {
			panic("string panic")
		}

		panicErrorHandler := func(ctx context.Context, task worker.Task) (interface{}, error) {
			panic(errors.New("error panic"))
		}

		panicOtherHandler := func(ctx context.Context, task worker.Task) (interface{}, error) {
			panic(123) // Not a string or error
		}

		// Test panic with string
		task := testutils.NewMockTask("panic-string-task", nil)
		wrappedString := recovery(panicStringHandler)

		_, err := wrappedString(context.Background(), task)
		if err == nil {
			t.Error("Expected error from recovered panic, got nil")
		}
		if !strings.Contains(err.Error(), "string panic") {
			t.Errorf("Expected error to contain panic message, got %v", err)
		}

		// Test panic with error
		task = testutils.NewMockTask("panic-error-task", nil)
		wrappedError := recovery(panicErrorHandler)

		_, err = wrappedError(context.Background(), task)
		if err == nil {
			t.Error("Expected error from recovered panic, got nil")
		}
		if !strings.Contains(err.Error(), "error panic") {
			t.Errorf("Expected error to contain panic message, got %v", err)
		}

		// Test panic with other value
		task = testutils.NewMockTask("panic-other-task", nil)
		wrappedOther := recovery(panicOtherHandler)

		_, err = wrappedOther(context.Background(), task)
		if err == nil {
			t.Error("Expected error from recovered panic, got nil")
		}
		if !strings.Contains(err.Error(), "unknown") {
			t.Errorf("Expected error to mention 'unknown', got %v", err)
		}
	})
}

func TestTimeoutMiddleware(t *testing.T) {
	t.Run("TimeoutMiddleware adds timeout to context", func(t *testing.T) {
		// Create middleware with short timeout
		timeout := TimeoutMiddleware(50 * time.Millisecond)

		// Create handler that checks if context has deadline
		handler := func(ctx context.Context, task worker.Task) (interface{}, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Error("Expected context to have deadline")
			}
			return deadline, nil
		}

		// Test timeout is added
		task := testutils.NewMockTask("timeout-task", nil)
		wrappedHandler := timeout(handler)

		result, err := wrappedHandler(context.Background(), task)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Check that the deadline is in the future
		deadline, ok := result.(time.Time)
		if !ok {
			t.Fatalf("Expected time.Time result, got %T", result)
		}

		if time.Until(deadline) > 50*time.Millisecond {
			t.Errorf("Expected deadline to be ~50ms in the future, got %v", time.Until(deadline))
		}
	})

	t.Run("TimeoutMiddleware cancels context after timeout", func(t *testing.T) {
		// Create middleware with short timeout
		timeout := TimeoutMiddleware(50 * time.Millisecond)

		// Create handler that takes longer than the timeout
		handler := func(ctx context.Context, task worker.Task) (interface{}, error) {
			select {
			case <-time.After(200 * time.Millisecond):
				return "completed", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// Test timeout works
		task := testutils.NewMockTask("timeout-task", nil)
		wrappedHandler := timeout(handler)

		_, err := wrappedHandler(context.Background(), task)
		if err == nil {
			t.Error("Expected context timeout error, got nil")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected DeadlineExceeded error, got %v", err)
		}
	})
}

func TestRetryMiddleware(t *testing.T) {
	t.Run("RetryMiddleware retries on failure", func(t *testing.T) {
		// Create middleware with 2 retries
		retry := RetryMiddleware(2, 1.0)

		// Create handler that fails a certain number of times
		attempts := 0
		handler := func(ctx context.Context, task worker.Task) (interface{}, error) {
			attempts++
			if attempts <= 2 {
				return nil, errors.New("temporary error")
			}
			return successResult, nil
		}

		// Test retries
		task := testutils.NewMockTask("retry-task", nil)
		wrappedHandler := retry(handler)

		result, err := wrappedHandler(context.Background(), task)
		if err != nil {
			t.Errorf("Expected no error after retries, got %v", err)
		}
		if result != successResult {
			t.Errorf("Expected result '%s', got %v", successResult, result)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("RetryMiddleware respects max retries", func(t *testing.T) {
		// Create middleware with 1 retry
		retry := RetryMiddleware(1, 1.0)

		// Create handler that always fails
		attempts := 0
		handler := func(ctx context.Context, task worker.Task) (interface{}, error) {
			attempts++
			return nil, errors.New("persistent error")
		}

		// Test max retries
		task := testutils.NewMockTask("retry-task", nil)
		wrappedHandler := retry(handler)

		_, err := wrappedHandler(context.Background(), task)
		if err == nil {
			t.Error("Expected error after max retries, got nil")
		}

		// Should try initial + 1 retry = 2 attempts
		if attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", attempts)
		}

		var taskErr *worker.TaskError
		if !errors.As(err, &taskErr) {
			t.Errorf("Expected TaskError, got %T", err)
		} else {
			if taskErr.Attempt != 2 {
				t.Errorf("Expected attempt count 2, got %d", taskErr.Attempt)
			}
		}
	})

	t.Run("RetryMiddleware respects context cancellation", func(t *testing.T) {
		// Create middleware with many retries
		retry := RetryMiddleware(10, 1.0)

		// Create handler that fails but checks context
		attempts := 0
		handler := func(ctx context.Context, task worker.Task) (interface{}, error) {
			attempts++
			return nil, errors.New("error")
		}

		// Create cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		// Test context cancellation
		task := testutils.NewMockTask("retry-task", nil)
		wrappedHandler := retry(handler)

		// Run in goroutine
		errChan := make(chan error)
		go func() {
			_, err := wrappedHandler(ctx, task)
			errChan <- err
		}()

		// Cancel after a short delay
		time.Sleep(50 * time.Millisecond)
		cancel()

		// Wait for result
		var err error
		select {
		case err = <-errChan:
			// Got result
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Test timed out")
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled error, got %v", err)
		}
	})
}

func TestWrapTask(t *testing.T) {
	t.Run("WrapTask creates wrapped task with middleware", func(t *testing.T) {
		// Create a middleware that modifies the result
		middleware := func(next TaskHandlerFunc) TaskHandlerFunc {
			return func(ctx context.Context, task worker.Task) (interface{}, error) {
				result, err := next(ctx, task)
				if str, ok := result.(string); ok {
					return "wrapped:" + str, err
				}
				return result, err
			}
		}

		// Create a task
		originalTask := testutils.NewMockTask("test-task", func(ctx context.Context) (interface{}, error) {
			return "original", nil
		})

		// Wrap the task
		wrappedTask := WrapTask(originalTask, middleware)

		// Test that the task's Execute method applies the middleware
		result, err := wrappedTask.Execute(context.Background())
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		expected := "wrapped:original"
		if result != expected {
			t.Errorf("Expected result '%s', got '%v'", expected, result)
		}

		// Test that other methods delegate to the original task
		if wrappedTask.ID() != "test-task" {
			t.Errorf("Expected ID 'test-task', got '%s'", wrappedTask.ID())
		}
	})
}
