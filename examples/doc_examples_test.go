// Package examples provides examples of using the lemmings library.
package examples

import (
	"context"
	"fmt"
	"time"

	"github.com/greysquirr3l/lemmings/internal/factory"
	"github.com/greysquirr3l/lemmings/pkg/manager"
	"github.com/greysquirr3l/lemmings/pkg/middleware"
	"github.com/greysquirr3l/lemmings/pkg/worker"
)

// This example demonstrates basic usage of the Manager to process tasks.
func Example_basicUsage() {
	// Create a worker factory
	workerFactory := factory.NewWorkerFactory(func(id int) (worker.Worker, error) {
		return worker.NewSimpleWorker(id, nil, nil, 3), nil
	})

	// Configure the manager
	config := manager.DefaultConfig()
	config.InitialWorkers = 5
	config.MaxWorkers = 20

	// Create and start the manager
	mgr, _ := manager.NewManager(workerFactory, config)
	mgr.Start()
	defer mgr.Stop()

	// Enable dynamic scaling
	mgr.EnableDynamicScaling()

	// Create a task
	task := worker.NewFunctionTask("task-1", func(ctx context.Context) (interface{}, error) {
		// Task logic goes here
		return "task result", nil
	})

	// Submit the task
	mgr.Submit(task)

	// Wait for task completion
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mgr.WaitForCompletion(ctx)

	// Output:
}

// This example shows how to use middleware to add behaviors to tasks.
func Example_usingMiddleware() {
	// Create middleware chain
	chain := middleware.Chain(
		middleware.LoggingMiddleware(),
		middleware.RecoveryMiddleware(),
		middleware.TimeoutMiddleware(5*time.Second),
		middleware.RetryMiddleware(3, 1.0),
	)

	// Create a task
	task := worker.NewFunctionTask("task-with-middleware", func(ctx context.Context) (interface{}, error) {
		// Check for timeout or cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return "success", nil
		}
	})

	// Wrap task with middleware
	wrappedTask := middleware.WrapTask(task, chain)

	// Use the wrapped task as you would a normal task
	fmt.Println("Task ready with middleware:", wrappedTask.ID())

	// Output:
	// Task ready with middleware: task-with-middleware
}

// This example shows how to create custom workers.
func Example_customWorker() {
	// Define custom worker type
	type CustomWorker struct {
		*worker.SimpleWorker
		customData string
	}

	// Create factory for custom worker
	factory := factory.NewWorkerFactory(func(id int) (worker.Worker, error) {
		baseWorker := worker.NewSimpleWorker(id, nil, nil, 3)
		return &CustomWorker{
			SimpleWorker: baseWorker,
			customData:   fmt.Sprintf("worker-%d-data", id),
		}, nil
	})

	// Use factory with manager
	config := manager.DefaultConfig()
	mgr, _ := manager.NewManager(factory, config)

	// Start the manager to verify our custom worker works
	if err := mgr.Start(); err != nil {
		fmt.Println("Error starting manager:", err)
	} else {
		fmt.Println("Manager started successfully with custom workers")
		mgr.Stop()
	}

	// Output:
	// Manager started successfully with custom workers
}

// This example shows how to use the priority queue for task prioritization.
func Example_priorityQueue() {
	// Create a priority queue
	queue := worker.NewPriorityQueue()

	// Create tasks with different priorities
	task1 := worker.NewSimpleTask("low-priority", "example", nil).WithPriority(1)
	task2 := worker.NewSimpleTask("medium-priority", "example", nil).WithPriority(5)
	task3 := worker.NewSimpleTask("high-priority", "example", nil).WithPriority(10)

	// Add tasks to queue
	queue.Push(task1)
	queue.Push(task2)
	queue.Push(task3)

	// Process tasks in priority order
	for queue.Len() > 0 {
		nextTask := queue.Pop()
		fmt.Println("Processing task:", nextTask.ID())
	}

	// Output:
	// Processing task: high-priority
	// Processing task: medium-priority
	// Processing task: low-priority
}
