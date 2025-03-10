package worker

import (
	"context"
	"errors"
	"testing"
	"time"
)

const (
	testResult = "result" // Extracted constant for repeated string
)

func TestSimpleTask(t *testing.T) {
	t.Run("Execute returns task function result", func(t *testing.T) {
		task := NewSimpleTask("test-task", "test-type", func(ctx context.Context) (interface{}, error) {
			return testResult, nil
		})

		result, err := task.Execute(context.Background())

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != testResult {
			t.Errorf("Expected result to be '%s', got %v", testResult, result)
		}
	})

	t.Run("Execute returns error from task function", func(t *testing.T) {
		expectedErr := errors.New("test error")
		task := NewSimpleTask("test-task", "test-type", func(ctx context.Context) (interface{}, error) {
			return nil, expectedErr
		})

		_, err := task.Execute(context.Background())

		if !errors.Is(err, expectedErr) { // Fixed error comparison
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("Validate checks for empty ID", func(t *testing.T) {
		task := NewSimpleTask("", "test-type", func(ctx context.Context) (interface{}, error) {
			return testResult, nil
		})

		err := task.Validate()

		if err == nil {
			t.Error("Expected error for empty task ID, got nil")
		}
	})

	t.Run("Validate checks for nil function", func(t *testing.T) {
		task := &SimpleTask{
			TaskID:   "test-task",
			TaskType: "test-type",
		}

		err := task.Validate()

		if err == nil {
			t.Error("Expected error for nil task function, got nil")
		}
	})

	t.Run("Validate runs custom validator", func(t *testing.T) {
		expectedErr := errors.New("custom validation error")
		task := NewSimpleTask("test-task", "test-type", func(ctx context.Context) (interface{}, error) {
			return testResult, nil
		}).WithValidator(func() error {
			return expectedErr
		})

		err := task.Validate()

		if !errors.Is(err, expectedErr) { // Fixed error comparison
			t.Errorf("Expected custom validation error %v, got %v", expectedErr, err)
		}
	})

	t.Run("ID returns task ID", func(t *testing.T) {
		task := NewSimpleTask("test-task", "test-type", nil)

		if task.ID() != "test-task" {
			t.Errorf("Expected ID to be 'test-task', got %s", task.ID())
		}
	})

	t.Run("Type returns task type", func(t *testing.T) {
		task := NewSimpleTask("test-task", "test-type", nil)

		if task.Type() != "test-type" {
			t.Errorf("Expected Type to be 'test-type', got %s", task.Type())
		}
	})

	t.Run("Priority returns task priority", func(t *testing.T) {
		task := NewSimpleTask("test-task", "test-type", nil).WithPriority(5)

		if task.Priority() != 5 {
			t.Errorf("Expected Priority to be 5, got %d", task.Priority())
		}
	})

	t.Run("MaxRetries returns task retries", func(t *testing.T) {
		task := NewSimpleTask("test-task", "test-type", nil).WithRetries(3)

		if task.MaxRetries() != 3 {
			t.Errorf("Expected MaxRetries to be 3, got %d", task.MaxRetries())
		}
	})
}

func TestFunctionTask(t *testing.T) {
	t.Run("Execute with timeout - successful completion", func(t *testing.T) {
		task := NewFunctionTask("test-task", func(ctx context.Context) (interface{}, error) {
			// Return quickly
			return testResult, nil
		}).WithTimeout(100 * time.Millisecond)

		result, err := task.Execute(context.Background())

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != testResult {
			t.Errorf("Expected result to be '%s', got %v", testResult, result)
		}
	})

	t.Run("Execute with timeout - timeout exceeded", func(t *testing.T) {
		task := NewFunctionTask("test-task", func(ctx context.Context) (interface{}, error) {
			// Take longer than timeout
			time.Sleep(200 * time.Millisecond)
			return testResult, nil
		}).WithTimeout(50 * time.Millisecond)

		_, err := task.Execute(context.Background())

		if err == nil {
			t.Error("Expected timeout error, got nil")
		}

		var timeoutErr *TaskTimeoutError
		if !errors.As(err, &timeoutErr) {
			t.Errorf("Expected error to be TaskTimeoutError, got %T", err)
		}

		if timeoutErr != nil && timeoutErr.TaskID != "test-task" {
			t.Errorf("Expected timeout error TaskID to be 'test-task', got %s", timeoutErr.TaskID)
		}
	})

	t.Run("Execute with timeout - respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		task := NewFunctionTask("test-task", func(innerCtx context.Context) (interface{}, error) {
			// Wait for context cancellation
			<-innerCtx.Done()
			return nil, innerCtx.Err()
		}).WithTimeout(500 * time.Millisecond)

		// Run task in a goroutine
		resultChan := make(chan error, 1)
		go func() {
			_, err := task.Execute(ctx)
			resultChan <- err
		}()

		// Cancel context
		time.Sleep(50 * time.Millisecond)
		cancel()

		// Wait for result
		select {
		case err := <-resultChan:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("Expected context.Canceled error, got %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Error("Test timed out waiting for task to respect context cancellation")
		}
	})

	t.Run("WithCallback sets callback function", func(t *testing.T) {
		var called bool
		var resultOutput interface{}

		task := NewFunctionTask("test-task", func(ctx context.Context) (interface{}, error) {
			return testResult, nil
		}).WithCallback(func(result Result) {
			called = true
			resultOutput = result.Output
		})

		callback := task.GetCallback()
		if callback == nil {
			t.Error("Expected callback to be set, got nil")
		}

		callback(Result{Output: testResult})

		if !called {
			t.Error("Expected callback to be called")
		}
		if resultOutput != testResult {
			t.Errorf("Expected resultOutput to be '%s', got %v", testResult, resultOutput)
		}
	})
}

func TestTaskError(t *testing.T) {
	original := errors.New("original error")
	taskErr := NewTaskError("task-1", 3, "task failed", original)

	if taskErr.TaskID != "task-1" {
		t.Errorf("Expected TaskID to be 'task-1', got %s", taskErr.TaskID)
	}

	if taskErr.Attempt != 3 {
		t.Errorf("Expected Attempt to be 3, got %d", taskErr.Attempt)
	}

	if taskErr.Message != "task failed" {
		t.Errorf("Expected Message to be 'task failed', got %s", taskErr.Message)
	}

	if !errors.Is(taskErr, original) {
		t.Error("Expected errors.Is(taskErr, original) to be true")
	}

	errorMsg := taskErr.Error()
	expected := "task task-1 failed on attempt 3: task failed"
	if errorMsg != expected {
		t.Errorf("Expected Error() to return '%s', got '%s'", expected, errorMsg)
	}
}

func TestTaskTimeoutError(t *testing.T) {
	timeoutErr := &TaskTimeoutError{
		TaskID:  "task-1",
		Timeout: 500 * time.Millisecond,
	}

	errorMsg := timeoutErr.Error()
	expected := "task task-1 timed out after 500ms"
	if errorMsg != expected {
		t.Errorf("Expected Error() to return '%s', got '%s'", expected, errorMsg)
	}
}
