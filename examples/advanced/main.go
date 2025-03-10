package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/greysquirr3l/lemmings/internal/factory"
	"github.com/greysquirr3l/lemmings/internal/utils"
	"github.com/greysquirr3l/lemmings/pkg/manager"
	"github.com/greysquirr3l/lemmings/pkg/worker"
)

// TaskResult holds the result of processing
type TaskResult struct {
	ID       string
	Value    int
	Duration time.Duration
	Error    error
}

// Create a custom worker that extends SimpleWorker
type CustomWorker struct {
	*worker.SimpleWorker
	processingTime int64 // in nanoseconds
}

// Create a custom worker factory
func newCustomWorkerFactory() factory.WorkerFactory[worker.Worker] {
	return factory.NewWorkerFactory(func(id int) (worker.Worker, error) {
		baseWorker := worker.NewSimpleWorker(id, nil, nil, 3)
		return &CustomWorker{
			SimpleWorker:   baseWorker,
			processingTime: 0,
		}, nil
	})
}

// TaskProcessor processes different types of tasks
type TaskProcessor struct {
	totalProcessed int64
	manager        *manager.Manager
	results        chan TaskResult
}

// NewTaskProcessor creates a new TaskProcessor
func NewTaskProcessor() (*TaskProcessor, error) {
	// Create custom worker factory
	workerFactory := newCustomWorkerFactory()

	// Create manager configuration
	config := manager.DefaultConfig()
	config.InitialWorkers = 2
	config.MaxWorkers = 50
	config.MinWorkers = 1
	config.Verbose = true
	config.ScaleUpFactor = 1.2
	config.ScaleDownFactor = 0.8
	config.MonitorInterval = 1 * time.Second

	// Create manager
	mgr, err := manager.NewManager(workerFactory, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	return &TaskProcessor{
		manager: mgr,
		results: make(chan TaskResult, 1000),
	}, nil
}

// Start starts the processor
func (tp *TaskProcessor) Start() error {
	return tp.manager.Start()
}

// Stop stops the processor
func (tp *TaskProcessor) Stop() {
	tp.manager.Stop()
	close(tp.results)
}

// ProcessMath submits math calculation tasks
func (tp *TaskProcessor) ProcessMath(count int, complexity int) error {
	log.Printf("Submitting %d math calculation tasks (complexity: %d)", count, complexity)

	for i := 0; i < count; i++ {
		taskID := fmt.Sprintf("math-%d", i)

		// Create a task that performs a CPU-intensive calculation
		task := worker.NewFunctionTask(taskID, func(ctx context.Context) (interface{}, error) {
			start := time.Now()

			// Simulate CPU-intensive work
			result := 0
			for j := 0; j < complexity*100000; j++ {
				result += j % 10
			}

			duration := time.Since(start)

			return TaskResult{
				ID:       taskID,
				Value:    result,
				Duration: duration,
			}, nil
		})

		if err := tp.manager.Submit(task); err != nil {
			return fmt.Errorf("failed to submit task %s: %w", taskID, err)
		}

		atomic.AddInt64(&tp.totalProcessed, 1)
	}

	return nil
}

// ProcessIO submits I/O simulation tasks
func (tp *TaskProcessor) ProcessIO(count int, ioTime time.Duration) error {
	log.Printf("Submitting %d I/O simulation tasks (io time: %v)", count, ioTime)

	for i := 0; i < count; i++ {
		taskID := fmt.Sprintf("io-%d", i)

		// Create a task that simulates I/O operations
		task := worker.NewFunctionTask(taskID, func(ctx context.Context) (interface{}, error) {
			start := time.Now()

			// Simulate I/O wait
			time.Sleep(ioTime)

			// Small amount of computation
			result := 0
			for j := 0; j < 1000; j++ {
				result += j
			}

			duration := time.Since(start)

			return TaskResult{
				ID:       taskID,
				Value:    result,
				Duration: duration,
			}, nil
		})

		if err := tp.manager.Submit(task); err != nil {
			return fmt.Errorf("failed to submit task %s: %w", taskID, err)
		}

		atomic.AddInt64(&tp.totalProcessed, 1)
	}

	return nil
}

// ProcessMixed submits a mix of tasks with different characteristics
func (tp *TaskProcessor) ProcessMixed(count int) error {
	log.Printf("Submitting %d mixed tasks", count)

	for i := 0; i < count; i++ {
		taskID := fmt.Sprintf("mixed-%d", i)

		// Randomly choose task type
		taskType, _ := utils.SecureIntn(3)

		var task worker.Task

		switch taskType {
		case 0:
			// CPU-intensive task
			task = worker.NewFunctionTask(taskID, func(ctx context.Context) (interface{}, error) {
				start := time.Now()

				// CPU-intensive work
				result := 0
				complexity, _ := utils.SecureIntn(10)
				complexity++ // Add 1 to ensure non-zero
				for j := 0; j < complexity*50000; j++ {
					result += j % 10
				}

				duration := time.Since(start)

				return TaskResult{
					ID:       taskID,
					Value:    result,
					Duration: duration,
				}, nil
			})

		case 1:
			// I/O-intensive task
			task = worker.NewFunctionTask(taskID, func(ctx context.Context) (interface{}, error) {
				start := time.Now()

				// I/O wait
				randVal, _ := utils.SecureIntn(500)
				sleepTime := time.Duration(randVal+10) * time.Millisecond
				time.Sleep(sleepTime)

				duration := time.Since(start)

				return TaskResult{
					ID:       taskID,
					Value:    int(sleepTime.Milliseconds()),
					Duration: duration,
				}, nil
			})

		case 2:
			// Error-prone task
			task = worker.NewFunctionTask(taskID, func(ctx context.Context) (interface{}, error) {
				// 30% chance to fail
				randVal, _ := utils.SecureIntn(10)
				if randVal < 3 {
					return nil, fmt.Errorf("random task failure")
				}

				start := time.Now()
				randVal, _ = utils.SecureIntn(100)
				time.Sleep(time.Duration(randVal+10) * time.Millisecond)
				duration := time.Since(start)

				return TaskResult{
					ID:       taskID,
					Value:    42,
					Duration: duration,
				}, nil
			}).WithRetries(2)
		}

		if err := tp.manager.Submit(task); err != nil {
			return fmt.Errorf("failed to submit task %s: %w", taskID, err)
		}

		atomic.AddInt64(&tp.totalProcessed, 1)
	}

	return nil
}

// GetStats returns the current processing statistics
func (tp *TaskProcessor) GetStats() manager.ManagerStats {
	return tp.manager.GetStats()
}

// GetProcessedCount returns the total number of processed tasks
func (tp *TaskProcessor) GetProcessedCount() int64 {
	return atomic.LoadInt64(&tp.totalProcessed)
}

func main() {
	log.Println("Starting advanced lemmings example")

	// Create a processor
	processor, err := NewTaskProcessor()
	if err != nil {
		log.Fatalf("Failed to create processor: %v", err)
	}

	// Set up cancellation on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Interrupt received, shutting down...")
		cancel()
	}()

	// Start the processor
	if err := processor.Start(); err != nil {
		log.Fatalf("Failed to start processor: %v", err)
	}
	defer processor.Stop()

	// Enable dynamic scaling
	processor.manager.EnableDynamicScaling()

	// Create stats printer
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats := processor.GetStats()
				workerCount := processor.manager.GetWorkerCount()
				queueLength := processor.manager.GetTaskQueueLength()

				log.Printf("Stats: Workers=%d, Queue=%d, Completed=%d, Failed=%d, Avg Time=%.2fms",
					workerCount, queueLength, stats.TasksCompleted, stats.TasksFailed,
					float64(stats.AvgProcessTime)/float64(time.Millisecond))
			}
		}
	}()

	// Submit some initial tasks
	if err := processor.ProcessMath(20, 1); err != nil {
		log.Printf("Error submitting math tasks: %v", err)
	}

	if err := processor.ProcessIO(20, 100*time.Millisecond); err != nil {
		log.Printf("Error submitting IO tasks: %v", err)
	}

	// Main processing loop
	for i := 0; i < 5; i++ {
		select {
		case <-ctx.Done():
			log.Println("Processing canceled")
			return
		default:
			// Submit a batch of mixed tasks
			if err := processor.ProcessMixed(100); err != nil {
				log.Printf("Error submitting mixed tasks: %v", err)
			}

			// Wait a bit before the next batch
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Wait for completion of the tasks
	log.Println("All tasks submitted, waiting for completion...")

	// Wait loop
	deadline := time.Now().Add(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			log.Println("Processing canceled")
			return
		case <-ticker.C:
			stats := processor.GetStats()
			queueLength := processor.manager.GetTaskQueueLength()
			processed := processor.GetProcessedCount()

			if stats.TasksCompleted+stats.TasksFailed >= processed && queueLength == 0 {
				log.Println("All tasks completed!")

				// Print final stats
				workerCount := processor.manager.GetWorkerCount()

				log.Printf("Final Stats: Workers=%d, Submitted=%d, Completed=%d, Failed=%d",
					workerCount, stats.TasksSubmitted, stats.TasksCompleted, stats.TasksFailed)
				log.Printf("Average Processing Time: %.2fms",
					float64(stats.AvgProcessTime)/float64(time.Millisecond))

				return
			}
		}
	}

	log.Println("Timeout waiting for task completion")
}
