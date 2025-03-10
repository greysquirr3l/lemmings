package worker

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBaseWorker(t *testing.T) {
	t.Run("NewBaseWorker initializes correctly", func(t *testing.T) {
		taskCh := make(chan Task, 10)
		resultCh := make(chan Result, 10)
		worker := NewBaseWorker(42, taskCh, resultCh, 3)

		if worker.ID() != 42 {
			t.Errorf("Expected ID to be 42, got %d", worker.ID())
		}

		if !worker.Available() {
			t.Error("Expected worker to be available by default")
		}
	})

	t.Run("SetAvailable updates availability", func(t *testing.T) {
		worker := NewBaseWorker(1, nil, nil, 3)
		worker.SetAvailable(false)

		if worker.Available() {
			t.Error("Expected worker to be unavailable after SetAvailable(false)")
		}
	})

	t.Run("ExecuteTask handles validation errors", func(t *testing.T) {
		resultCh := make(chan Result, 1)
		worker := NewBaseWorker(1, nil, resultCh, 3)

		// Create a task that will fail validation
		task := &SimpleTask{
			TaskID: "",
		}

		// Execute task
		worker.ExecuteTask(context.Background(), task)

		// Check result
		select {
		case result := <-resultCh:
			if result.Err == nil {
				t.Error("Expected error for invalid task, got nil")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Timed out waiting for result")
		}
	})

	t.Run("ExecuteTask handles successful execution", func(t *testing.T) {
		resultCh := make(chan Result, 1)
		worker := NewBaseWorker(1, nil, resultCh, 3)

		task := NewSimpleTask("test-task", "test-type", func(ctx context.Context) (interface{}, error) {
			return "success", nil
		})

		// Execute task
		worker.ExecuteTask(context.Background(), task)

		// Check result
		select {
		case result := <-resultCh:
			if result.Err != nil {
				t.Errorf("Expected no error, got %v", result.Err)
			}
			if result.Output != "success" {
				t.Errorf("Expected output 'success', got %v", result.Output)
			}
			if result.Worker != 1 {
				t.Errorf("Expected worker ID 1, got %d", result.Worker)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Timed out waiting for result")
		}

		// Check stats update
		stats := worker.Stats()
		if stats.TasksProcessed != 1 {
			t.Errorf("Expected 1 task processed, got %d", stats.TasksProcessed)
		}
	})

	t.Run("ExecuteTask handles task errors", func(t *testing.T) {
		resultCh := make(chan Result, 1)
		worker := NewBaseWorker(1, nil, resultCh, 3)

		expectedErr := errors.New("task error")
		task := NewSimpleTask("test-task", "test-type", func(ctx context.Context) (interface{}, error) {
			return nil, expectedErr
		})

		// Execute task
		worker.ExecuteTask(context.Background(), task)

		// Check result
		select {
		case result := <-resultCh:
			if result.Err == nil {
				t.Error("Expected error, got nil")
			} else if !errors.Is(result.Err, expectedErr) {
				t.Errorf("Expected error %v, got %v", expectedErr, result.Err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Timed out waiting for result")
		}

		// Check stats update
		stats := worker.Stats()
		if stats.TasksFailed != 1 {
			t.Errorf("Expected 1 task failed, got %d", stats.TasksFailed)
		}
	})

	t.Run("ExecuteTask respects context cancellation", func(t *testing.T) {
		resultCh := make(chan Result, 1)
		worker := NewBaseWorker(1, nil, resultCh, 3)

		ctx, cancel := context.WithCancel(context.Background())

		task := NewSimpleTask("test-task", "test-type", func(innerCtx context.Context) (interface{}, error) {
			<-innerCtx.Done() // Wait for cancellation
			return nil, innerCtx.Err()
		})

		// Execute task in a goroutine
		go worker.ExecuteTask(ctx, task)

		// Cancel context after a short delay
		time.Sleep(50 * time.Millisecond)
		cancel()

		// Check result
		select {
		case result := <-resultCh:
			if !errors.Is(result.Err, context.Canceled) {
				t.Errorf("Expected context.Canceled error, got %v", result.Err)
			}
		case <-time.After(500 * time.Millisecond):
			t.Error("Timed out waiting for result")
		}
	})

	t.Run("ExecuteTask performs retries", func(t *testing.T) {
		resultCh := make(chan Result, 1)
		worker := NewBaseWorker(1, nil, resultCh, 2) // Allow 2 retries

		attemptCount := 0
		task := NewSimpleTask("test-task", "test-type", func(ctx context.Context) (interface{}, error) {
			attemptCount++
			if attemptCount <= 2 {
				return nil, errors.New("temporary error")
			}
			return "success after retry", nil
		})

		// Execute task
		worker.ExecuteTask(context.Background(), task)

		// Check result
		select {
		case result := <-resultCh:
			if result.Err != nil {
				t.Errorf("Expected no error after retries, got %v", result.Err)
			}
			if result.Output != "success after retry" {
				t.Errorf("Expected output 'success after retry', got %v", result.Output)
			}
			if attemptCount != 3 {
				t.Errorf("Expected 3 attempts, got %d", attemptCount)
			}
		case <-time.After(500 * time.Millisecond):
			t.Error("Timed out waiting for result")
		}
	})
}

func TestSimpleWorker(t *testing.T) {
	t.Run("SimpleWorker starts and stops correctly", func(t *testing.T) {
		taskCh := make(chan Task, 10)
		resultCh := make(chan Result, 10)

		worker := NewSimpleWorker(1, taskCh, resultCh, 3)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start worker
		worker.Start(ctx)

		// Send a task
		task := NewSimpleTask("test-task", "test-type", func(ctx context.Context) (interface{}, error) {
			return "result", nil
		})
		taskCh <- task

		// Check result
		select {
		case result := <-resultCh:
			if result.Err != nil {
				t.Errorf("Expected no error, got %v", result.Err)
			}
			if result.Output != "result" {
				t.Errorf("Expected output 'result', got %v", result.Output)
			}
		case <-time.After(500 * time.Millisecond):
			t.Error("Timed out waiting for result")
		}

		// Stop worker
		worker.Stop()

		// Check worker is stopped by sending another task - should not be processed
		taskCh <- NewSimpleTask("test-task-2", "test-type", func(ctx context.Context) (interface{}, error) {
			return "should not be processed", nil
		})

		// No result should be received
		select {
		case result := <-resultCh:
			t.Errorf("Received unexpected result after stop: %v", result)
		case <-time.After(200 * time.Millisecond):
			// This is expected - no result should be received
		}
	})

	t.Run("SimpleWorker respects context cancellation", func(t *testing.T) {
		taskCh := make(chan Task, 10)
		resultCh := make(chan Result, 10)

		worker := NewSimpleWorker(1, taskCh, resultCh, 3)

		ctx, cancel := context.WithCancel(context.Background())

		// Start worker
		worker.Start(ctx)

		// Cancel context
		cancel()

		// Wait a bit for worker to notice cancellation
		time.Sleep(50 * time.Millisecond)

		// Send a task - should not be processed
		taskCh <- NewSimpleTask("test-task", "test-type", func(ctx context.Context) (interface{}, error) {
			return "should not be processed", nil
		})

		// No result should be received
		select {
		case result := <-resultCh:
			t.Errorf("Received unexpected result after context cancellation: %v", result)
		case <-time.After(200 * time.Millisecond):
			// This is expected - no result should be received
		}
	})
}
