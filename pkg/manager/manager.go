// Package manager provides a worker management system that automatically scales
// workers based on resource usage and task demand. The manager component is the
// central coordinator that distributes tasks to workers and collects results.
//
// The manager handles task submission, worker scaling, resource monitoring, and
// provides statistics about task processing. It's designed to be the primary
// interface point for applications using the lemmings library.
package manager

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/greysquirr3l/lemmings/internal/factory"
	"github.com/greysquirr3l/lemmings/pkg/worker"
)

// Manager coordinates worker pools and task distribution.
// It handles task submission, worker scaling, and result collection.
// Manager is the central orchestration component in the lemmings library.
type Manager struct {
	sync.RWMutex
	config          Config
	resourceCtrl    *ResourceController // handles dynamic scaling
	taskQueue       chan worker.Task    // tasks to be processed
	resultQueue     chan worker.Result  // results from task processing
	workerPool      *worker.Pool
	workerFactory   factory.WorkerFactory[worker.Worker]
	started         bool
	stats           *managerStats
	shutdownCh      chan struct{}
	resourceMonitor chan struct{}
	processorCtx    context.Context
	processorCancel context.CancelFunc
	waitCompleteCh  chan struct{} // signals when all tasks are complete
}

// WorkerFactory defines an interface for creating workers
type WorkerFactory interface {
	// CreateWithID creates a worker with the specified ID
	CreateWithID(id int) (worker.Worker, error)
}

// managerStats tracks statistics about task processing
type managerStats struct {
	sync.RWMutex
	tasksSubmitted  int64
	tasksCompleted  int64
	tasksFailed     int64
	tasksInProgress int32
	totalTaskTime   int64 // nanoseconds
	avgProcessTime  time.Duration
	peakWorkerCount int
}

// ManagerStats provides statistics about manager performance
type ManagerStats struct {
	TasksSubmitted  int64
	TasksCompleted  int64
	TasksFailed     int64
	TasksInProgress int32
	AvgProcessTime  time.Duration
	PeakWorkerCount int
}

// NewManager creates a new manager with the specified worker factory and configuration.
// Returns an initialized manager instance and any error encountered during creation.
//
// The workerFactory is used to create new workers as needed.
// The config parameter specifies settings like initial worker count, scaling parameters, etc.
func NewManager(workerFactory factory.WorkerFactory[worker.Worker], config Config) (*Manager, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	taskQueue := make(chan worker.Task, config.MaxTasksInQueue)
	resultQueue := make(chan worker.Result, config.MaxTasksInQueue)

	// Create worker pool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Fix: Always call cancel to avoid context leak

	pool, err := worker.NewPool(ctx, config.InitialWorkers, config.MaxWorkers, taskQueue, resultQueue, workerFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker pool: %w", err)
	}

	resourceCtrl := NewResourceController(config)

	return &Manager{
		config:          config,
		resourceCtrl:    resourceCtrl,
		taskQueue:       taskQueue,
		resultQueue:     resultQueue,
		workerPool:      pool,
		workerFactory:   workerFactory,
		shutdownCh:      make(chan struct{}),
		resourceMonitor: make(chan struct{}),
		stats:           &managerStats{},
		waitCompleteCh:  make(chan struct{}),
	}, nil
}

// Start initializes and starts the manager.
// This launches worker goroutines and begins processing tasks.
// Returns an error if the manager is already started or if initialization fails.
func (m *Manager) Start() error {
	m.Lock()
	defer m.Unlock()

	if m.started {
		return errors.New("manager already started")
	}

	// Create a new context for task processing
	m.processorCtx, m.processorCancel = context.WithCancel(context.Background())

	// Start the worker pool
	if err := m.workerPool.Start(); err != nil {
		m.processorCancel()
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	// Start result processor
	go m.processResults()

	// Start resource monitor
	go m.monitorResources()

	m.started = true
	return nil
}

// Stop gracefully stops the manager.
// This waits for all current tasks to complete before stopping workers.
// Tasks submitted during shutdown may be rejected depending on the shutdown stage.
func (m *Manager) Stop() {
	m.Lock()
	if !m.started {
		m.Unlock()
		return
	}
	m.Unlock()

	// Signal shutdown
	close(m.shutdownCh)

	// Wait for processors to finish
	m.processorCancel()

	// Stop the worker pool
	m.workerPool.Stop()

	m.Lock()
	m.started = false
	m.Unlock()

	log.Println("Manager stopped")
}

// Submit adds a task to the queue for processing.
// Returns an error if the submission fails, e.g., due to task validation failure
// or if the queue is full.
//
// Tasks will be processed according to their priority, with higher priorities
// processed first. For tasks with equal priority, they'll be processed in FIFO order.
func (m *Manager) Submit(task worker.Task) error {
	return m.SubmitWithTimeout(task, 0)
}

// SubmitWithTimeout adds a task to the queue with a timeout.
// If the task cannot be added to the queue within the specified duration,
// it returns a timeout error.
//
// The timeout refers to the queue submission time, not the task execution time.
func (m *Manager) SubmitWithTimeout(task worker.Task, timeout time.Duration) error {
	m.RLock()
	if !m.started {
		m.RUnlock()
		return errors.New("manager not started")
	}
	m.RUnlock()

	// Validate the task
	if err := task.Validate(); err != nil {
		return fmt.Errorf("invalid task: %w", err)
	}

	// Add to task queue with optional timeout
	if timeout > 0 {
		select {
		case m.taskQueue <- task:
			// Task added successfully
		case <-time.After(timeout):
			return errors.New("timeout submitting task to queue")
		case <-m.shutdownCh:
			return errors.New("manager is shutting down")
		}
	} else {
		select {
		case m.taskQueue <- task:
			// Task added successfully
		case <-m.shutdownCh:
			return errors.New("manager is shutting down")
		}
	}

	m.stats.Lock()
	m.stats.tasksSubmitted++
	atomic.AddInt32(&m.stats.tasksInProgress, 1)
	m.stats.Unlock()

	if m.config.Verbose {
		log.Printf("Task %s submitted", task.ID())
	}

	return nil
}

// processResults handles processing of task results from workers
func (m *Manager) processResults() {
	for {
		select {
		case <-m.shutdownCh:
			return
		case result := <-m.resultQueue:
			m.handleResult(result)
		}
	}
}

// handleResult processes a single task result
func (m *Manager) handleResult(result worker.Result) {
	m.stats.Lock()
	defer m.stats.Unlock()

	if result.Err != nil {
		// Task failed
		m.stats.tasksFailed++
	} else {
		// Task succeeded
		m.stats.tasksCompleted++
	}

	// Update stats
	atomic.AddInt32(&m.stats.tasksInProgress, -1)
	m.stats.totalTaskTime += result.Duration.Nanoseconds()

	if m.stats.tasksCompleted > 0 {
		m.stats.avgProcessTime = time.Duration(m.stats.totalTaskTime / m.stats.tasksCompleted)
	}

	// Check if a task callback exists and invoke it
	if result.Task != nil {
		if task, ok := result.Task.(*worker.SimpleTask); ok && task.Callback != nil {
			go task.Callback(result)
		}
	}
}

// monitorResources monitors system resources and adjusts worker count accordingly
func (m *Manager) monitorResources() {
	ticker := time.NewTicker(m.config.MonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.shutdownCh:
			return
		case <-ticker.C:
			m.resourceCtrl.AdjustBasedOnResourceUsage()
		}
	}
}

// EnableDynamicScaling turns on automatic worker scaling
func (m *Manager) EnableDynamicScaling() {
	m.resourceCtrl.EnableScaling()
}

// DisableDynamicScaling turns off automatic worker scaling
func (m *Manager) DisableDynamicScaling() {
	m.resourceCtrl.DisableScaling()
}

// GetStats returns statistics about manager performance
func (m *Manager) GetStats() ManagerStats {
	m.stats.RLock()
	defer m.stats.RUnlock()

	return ManagerStats{
		TasksSubmitted:  m.stats.tasksSubmitted,
		TasksCompleted:  m.stats.tasksCompleted,
		TasksFailed:     m.stats.tasksFailed,
		TasksInProgress: m.stats.tasksInProgress,
		AvgProcessTime:  m.stats.avgProcessTime,
		PeakWorkerCount: m.stats.peakWorkerCount,
	}
}

// GetWorkerCount returns the current number of active workers
func (m *Manager) GetWorkerCount() int {
	return m.workerPool.Size()
}

// GetTaskQueueLength returns the number of tasks waiting to be processed
func (m *Manager) GetTaskQueueLength() int {
	return len(m.taskQueue)
}

// GetResultQueueLength returns the number of results waiting to be processed
func (m *Manager) GetResultQueueLength() int {
	return len(m.resultQueue)
}

// WaitForCompletion blocks until all submitted tasks are completed or context is done
func (m *Manager) WaitForCompletion(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			m.stats.RLock()
			inProgress := m.stats.tasksInProgress
			m.stats.RUnlock()

			queueLen := m.GetTaskQueueLength()

			// No tasks in queue and none in progress means we're done
			if inProgress == 0 && queueLen == 0 {
				return nil
			}
		}
	}
}

// ForceScaleWorkers manually adjusts the worker count
func (m *Manager) ForceScaleWorkers(count int) {
	m.resourceCtrl.AdjustWorkerCount(count, "manual scaling")
}

// SubmitBatch submits a batch of tasks
func (m *Manager) SubmitBatch(tasks []worker.Task) (int, error) {
	if !m.started {
		return 0, errors.New("manager not started")
	}

	submitted := 0
	for _, task := range tasks {
		if err := m.Submit(task); err != nil {
			return submitted, err
		}
		submitted++
	}

	return submitted, nil
}
