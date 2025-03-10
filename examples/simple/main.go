package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/greysquirr3l/lemmings/internal/factory"
	"github.com/greysquirr3l/lemmings/pkg/manager"
	"github.com/greysquirr3l/lemmings/pkg/worker"
)

func main() {
	// Create a worker factory
	workerFactory := factory.NewWorkerFactory(func(id int) (worker.Worker, error) {
		return worker.NewSimpleWorker(id, nil, nil, 3), nil
	})

	// Create a manager with default configuration
	config := manager.DefaultConfig()
	config.InitialWorkers = 5
	config.MaxWorkers = 20
	config.Verbose = true

	mgr, err := manager.NewManager(workerFactory, config)
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	// Start the manager
	if err := mgr.Start(); err != nil {
		log.Fatalf("Failed to start manager: %v", err)
	}
	defer mgr.Stop()

	// Enable dynamic scaling
	mgr.EnableDynamicScaling()

	// Create and submit some tasks
	for i := 0; i < 100; i++ {
		taskID := fmt.Sprintf("task-%d", i)

		// Create a simple task
		task := worker.NewFunctionTask(taskID, func(ctx context.Context) (interface{}, error) {
			// Simulate work
			duration := time.Duration(rand.Intn(500)+100) * time.Millisecond
			time.Sleep(duration)

			// 10% chance of failure
			if rand.Intn(10) == 0 {
				return nil, fmt.Errorf("random task failure")
			}

			return fmt.Sprintf("Result from task %s", taskID), nil
		}).WithTimeout(2 * time.Second)

		if err := mgr.Submit(task); err != nil {
			log.Printf("Failed to submit task %s: %v", taskID, err)
		}

		// Add small delay between submissions
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for tasks to complete
	time.Sleep(1 * time.Second)

	// Monitor progress
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Timeout or cancellation
			fmt.Println("Monitoring stopped")
			return
		case <-ticker.C:
			stats := mgr.GetStats()
			activeWorkers := mgr.GetWorkerCount()
			queueLength := mgr.GetTaskQueueLength()

			fmt.Printf("Stats: Submitted=%d, Completed=%d, Failed=%d, Workers=%d, Queue=%d\n",
				stats.TasksSubmitted, stats.TasksCompleted, stats.TasksFailed,
				activeWorkers, queueLength)

			if stats.TasksCompleted+stats.TasksFailed >= stats.TasksSubmitted &&
				stats.TasksSubmitted > 0 && queueLength == 0 {
				fmt.Println("All tasks processed!")
				return
			}
		}
	}
}
