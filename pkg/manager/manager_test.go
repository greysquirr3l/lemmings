package manager

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/greysquirr3l/lemmings/internal/testutils"
	"github.com/greysquirr3l/lemmings/pkg/worker"
)

const (
	testTaskResult = "done"
)

func TestManagerGetters(t *testing.T) {
	t.Run("GetTaskQueueLength returns queue length", func(t *testing.T) {
		mockFactory := testutils.NewMockFactory(func(id int) (worker.Worker, error) {
			return testutils.NewMockWorker(id, nil), nil
		})

		config := DefaultConfig()
		mgr, _ := NewManager(mockFactory, config)
		mgr.Start()

		// Add tasks without workers processing them
		task1 := testutils.NewMockTask("task-1", func(ctx context.Context) (interface{}, error) {
			time.Sleep(1 * time.Second) // Long-running task
			return nil, nil
		})

		mgr.Submit(task1)
		mgr.Submit(task1)

		// Check queue length
		queueLen := mgr.GetTaskQueueLength()
		if queueLen < 1 {
			t.Errorf("Expected task queue length ≥ 1, got %d", queueLen)
		}

		mgr.Stop()
	})

	t.Run("GetResultQueueLength returns result queue length", func(t *testing.T) {
		mockFactory := testutils.NewMockFactory(func(id int) (worker.Worker, error) {
			return testutils.NewMockWorker(id, nil), nil
		})

		config := DefaultConfig()
		mgr, _ := NewManager(mockFactory, config)

		// Check initial queue length
		queueLen := mgr.GetResultQueueLength()
		if queueLen != 0 {
			t.Errorf("Expected initial result queue length 0, got %d", queueLen)
		}

		mgr.Stop()
	})
}

func TestManagerStressTests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress tests in short mode")
	}

	t.Run("High volume task processing", func(t *testing.T) {
		resultCh := make(chan worker.Result, 100)
		mockFactory := testutils.NewMockFactory(func(id int) (worker.Worker, error) {
			return testutils.NewMockWorker(id, resultCh), nil
		})

		config := DefaultConfig()
		config.InitialWorkers = 8
		config.MaxWorkers = 20
		config.MinWorkers = 2

		mgr, _ := NewManager(mockFactory, config)
		mgr.Start()
		mgr.EnableDynamicScaling()

		// Submit a large number of tasks
		taskCount := 500
		completedTasks := int32(0)

		// Create tasks with varying execution times
		for i := 0; i < taskCount; i++ {
			execTime := time.Duration(i%20+10) * time.Millisecond
			priority := i % 5 // Mix of priorities

			task := testutils.NewMockTask(
				fmt.Sprintf("stress-task-%d", i),
				func(ctx context.Context) (interface{}, error) {
					time.Sleep(execTime)
					atomic.AddInt32(&completedTasks, 1)
					return testTaskResult, nil
				},
			).WithPriority(priority)

			mgr.Submit(task)
		}

		// Wait for all tasks to be processed
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := mgr.WaitForCompletion(ctx)
		if err != nil {
			t.Errorf("Failed to complete all tasks: %v", err)
		}

		// Verify all tasks were processed
		stats := mgr.GetStats()
		if int(stats.TasksCompleted) != taskCount {
			t.Errorf("Expected %d completed tasks, got %d", taskCount, stats.TasksCompleted)
		}

		mgr.Stop()
	})

	t.Run("Dynamic scaling under load", func(t *testing.T) {
		mockFactory := testutils.NewMockFactory(func(id int) (worker.Worker, error) {
			return testutils.NewMockWorker(id, nil), nil
		})

		config := DefaultConfig()
		config.InitialWorkers = 2
		config.MaxWorkers = 20
		config.MinWorkers = 1
		config.MonitorInterval = 200 * time.Millisecond // Fast monitoring for testing
		config.ScaleUpFactor = 1.5
		config.MinSuccessfulBatches = 1

		mgr, _ := NewManager(mockFactory, config)
		mgr.Start()
		mgr.EnableDynamicScaling()

		// Record initial worker count
		initialCount := mgr.GetWorkerCount()

		// Submit a burst of tasks
		for i := 0; i < 100; i++ {
			task := testutils.NewMockTask(
				fmt.Sprintf("load-task-%d", i),
				func(ctx context.Context) (interface{}, error) {
					time.Sleep(100 * time.Millisecond)
					return testTaskResult, nil
				},
			)
			mgr.Submit(task)
		}

		// Wait a bit to allow scaling to occur
		time.Sleep(2 * time.Second)

		// Check if worker count increased
		midCount := mgr.GetWorkerCount()
		if midCount <= initialCount {
			t.Errorf("Expected worker count to increase from %d under load, got %d",
				initialCount, midCount)
		}

		// Wait for tasks to complete
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mgr.WaitForCompletion(ctx)

		// Wait a bit more to allow scaling down
		time.Sleep(3 * time.Second)

		// Check if worker count decreased after load
		finalCount := mgr.GetWorkerCount()
		if finalCount >= midCount {
			t.Errorf("Expected worker count to decrease from %d after load, got %d",
				midCount, finalCount)
		}

		mgr.Stop()
	})

	t.Run("Error handling and retry behavior", func(t *testing.T) {
		resultCh := make(chan worker.Result, 50)
		mockFactory := testutils.NewMockFactory(func(id int) (worker.Worker, error) {
			return testutils.NewMockWorker(id, resultCh), nil
		})

		config := DefaultConfig()
		config.InitialWorkers = 3

		mgr, _ := NewManager(mockFactory, config)
		mgr.Start()

		// Create tasks with different retry behavior
		successAfterRetryCount := 0
		totalFailures := 0

		// Task that succeeds after 2 retries
		retriableTask := testutils.NewMockTask(
			"retriable-task",
			func(ctx context.Context) (interface{}, error) {
				attemptCount := testutils.NewMockTask("", nil).GetExecutionCount() + 1
				if attemptCount <= 2 {
					return nil, errors.New("temporary error")
				}
				successAfterRetryCount++
				return "success after retry", nil
			},
		).WithRetries(3)

		// Task that always fails
		permanentFailTask := testutils.NewMockTask(
			"permanent-fail-task",
			func(ctx context.Context) (interface{}, error) {
				totalFailures++
				return nil, errors.New("permanent error")
			},
		)

		// Submit tasks
		mgr.Submit(retriableTask)
		mgr.Submit(permanentFailTask)

		// Wait for processing to complete
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		mgr.WaitForCompletion(ctx)

		// Process all results to count failures and retries
		close(resultCh)
		for result := range resultCh {
			// Just consume the results
			_ = result
		}

		// Check stats
		stats := mgr.GetStats()
		if successAfterRetryCount != 1 {
			t.Errorf("Expected 1 task to succeed after retry, got %d", successAfterRetryCount)
		}

		if stats.TasksCompleted != 1 {
			t.Errorf("Expected 1 completed task, got %d", stats.TasksCompleted)
		}

		if stats.TasksFailed != 1 {
			t.Errorf("Expected 1 failed task, got %d", stats.TasksFailed)
		}

		mgr.Stop()
	})

	t.Run("Context cancellation propagation", func(t *testing.T) {
		resultCh := make(chan worker.Result, 10)
		mockFactory := testutils.NewMockFactory(func(id int) (worker.Worker, error) {
			return testutils.NewMockWorker(id, resultCh), nil
		})

		config := DefaultConfig()
		mgr, _ := NewManager(mockFactory, config)
		mgr.Start()

		// Track cancellation
		var cancellationDetected int32

		// Create a context that we'll cancel; ignore the context since it isn't used
		_, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create a task that checks for cancellation
		task := testutils.NewMockTask(
			"cancellable-task",
			func(taskCtx context.Context) (interface{}, error) {
				// Long running task that checks for cancellation
				select {
				case <-time.After(10 * time.Second):
					return "completed", nil
				case <-taskCtx.Done():
					atomic.StoreInt32(&cancellationDetected, 1)
					return nil, taskCtx.Err()
				}
			},
		)

		// Submit the task
		mgr.Submit(task)

		// Wait a bit to ensure task is picked up
		time.Sleep(100 * time.Millisecond)

		// Cancel the context
		cancel()

		// Wait for result
		var result worker.Result
		select {
		case result = <-resultCh:
			// Got a result
		case <-time.After(1 * time.Second):
			t.Fatal("Timed out waiting for task result after cancellation")
		}

		// Verify task was canceled
		if !errors.Is(result.Err, context.Canceled) {
			t.Errorf("Expected context.Canceled error, got: %v", result.Err)
		}

		if atomic.LoadInt32(&cancellationDetected) != 1 {
			t.Error("Task did not detect context cancellation")
		}

		mgr.Stop()
	})

	t.Run("Manager shutdown with pending tasks", func(t *testing.T) {
		resultCh := make(chan worker.Result, 100)
		mockFactory := testutils.NewMockFactory(func(id int) (worker.Worker, error) {
			return testutils.NewMockWorker(id, resultCh), nil
		})

		config := DefaultConfig()
		config.InitialWorkers = 2

		mgr, _ := NewManager(mockFactory, config)
		mgr.Start()

		// Submit a bunch of tasks
		completedTasks := int32(0)
		taskCount := 50

		for i := 0; i < taskCount; i++ {
			// Mix of fast and slow tasks
			delay := time.Duration(10+i%40) * time.Millisecond

			task := testutils.NewMockTask(
				fmt.Sprintf("shutdown-task-%d", i),
				func(ctx context.Context) (interface{}, error) {
					time.Sleep(delay)
					atomic.AddInt32(&completedTasks, 1)
					return testTaskResult, nil
				},
			)

			mgr.Submit(task)
		}

		// Wait a bit for some tasks to start processing
		time.Sleep(100 * time.Millisecond)

		// Initiate graceful shutdown
		stopStart := time.Now()
		go mgr.Stop()

		// Check that tasks continue to be processed during shutdown
		for i := 0; i < 10; i++ {
			count := atomic.LoadInt32(&completedTasks)
			if count == int32(taskCount) {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Check shutdown time
		shutdownTime := time.Since(stopStart)

		// Should have processed all tasks
		completedCount := atomic.LoadInt32(&completedTasks)
		if completedCount != int32(taskCount) {
			t.Errorf("Expected all %d tasks to complete during shutdown, got %d",
				taskCount, completedCount)
		}

		// Shutdown should have taken some time to process tasks
		if shutdownTime < 100*time.Millisecond {
			t.Errorf("Shutdown happened too quickly (%v), expected graceful completion",
				shutdownTime)
		}

		// Manager should be stopped
		if mgr.started {
			t.Error("Manager should be marked as not started after Stop()")
		}
	})

	t.Run("Task prioritization", func(t *testing.T) {
		resultCh := make(chan worker.Result, 50)
		mockFactory := testutils.NewMockFactory(func(id int) (worker.Worker, error) {
			return testutils.NewMockWorker(id, resultCh), nil
		})

		config := DefaultConfig()
		config.InitialWorkers = 1 // Single worker to ensure sequential processing

		mgr, _ := NewManager(mockFactory, config)
		mgr.Start()

		// Create tasks with different priorities
		var results []string

		// Record task execution order
		recordMu := &sync.Mutex{}
		recordExecution := func(id string) {
			recordMu.Lock()
			defer recordMu.Unlock()
			results = append(results, id)
		}

		// Create and submit tasks with various priorities
		lowPriTask := testutils.NewMockTask(
			"low-priority",
			func(innerCtx context.Context) (interface{}, error) {
				recordExecution("low")
				return nil, nil
			},
		).WithPriority(1)

		medPriTask := testutils.NewMockTask(
			"medium-priority",
			func(innerCtx context.Context) (interface{}, error) {
				recordExecution("medium")
				return nil, nil
			},
		).WithPriority(5)

		highPriTask := testutils.NewMockTask(
			"high-priority",
			func(innerCtx context.Context) (interface{}, error) {
				recordExecution("high")
				return nil, nil
			},
		).WithPriority(10)

		// Submit in reverse priority order
		mgr.Submit(lowPriTask)
		mgr.Submit(medPriTask)
		mgr.Submit(highPriTask)

		// Wait for all to complete
		waitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		mgr.WaitForCompletion(waitCtx)

		// Clean up results channel
		close(resultCh)
		for range resultCh {
			// Just drain the channel
		}

		// Verify processing order
		if len(results) != 3 {
			t.Errorf("Expected 3 results, got %d", len(results))
		} else {
			// High priority should be first
			if results[0] != "high" {
				t.Errorf("Expected high priority task to be processed first, got %s", results[0])
			}

			// Medium priority should be second
			if results[1] != "medium" {
				t.Errorf("Expected medium priority task to be processed second, got %s", results[1])
			}

			// Low priority should be last
			if results[2] != "low" {
				t.Errorf("Expected low priority task to be processed last, got %s", results[2])
			}
		}

		mgr.Stop()
	})

	t.Run("Statistics accuracy under load", func(t *testing.T) {
		mockFactory := testutils.NewMockFactory(func(id int) (worker.Worker, error) {
			return testutils.NewMockWorker(id, nil), nil
		})

		config := DefaultConfig()
		config.InitialWorkers = 5

		mgr, _ := NewManager(mockFactory, config)
		mgr.Start()

		// Calculate expected stats
		expectedSubmitted := 200
		expectedCompleted := 180
		expectedFailed := 20

		// Submit tasks with predetermined success/fail ratio
		for i := 0; i < expectedSubmitted; i++ {
			shouldFail := i >= expectedSubmitted-expectedFailed

			task := testutils.NewMockTask(
				fmt.Sprintf("stats-task-%d", i),
				func(ctx context.Context) (interface{}, error) {
					if shouldFail {
						return nil, errors.New("intentional failure")
					}
					return "success", nil
				},
			)

			mgr.Submit(task)
		}

		// Wait for tasks to complete
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mgr.WaitForCompletion(ctx)

		// Check stats
		stats := mgr.GetStats()

		if stats.TasksSubmitted != int64(expectedSubmitted) {
			t.Errorf("Expected %d submitted tasks, got %d",
				expectedSubmitted, stats.TasksSubmitted)
		}

		if stats.TasksCompleted != int64(expectedCompleted) {
			t.Errorf("Expected %d completed tasks, got %d",
				expectedCompleted, stats.TasksCompleted)
		}

		if stats.TasksFailed != int64(expectedFailed) {
			t.Errorf("Expected %d failed tasks, got %d",
				expectedFailed, stats.TasksFailed)
		}

		// Verify average processing time is reasonable
		if stats.AvgProcessTime <= 0 {
			t.Errorf("Expected positive average processing time, got %v",
				stats.AvgProcessTime)
		}

		mgr.Stop()
	})
}
