package manager

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/greysquirr3l/lemmings/internal/factory"
	"github.com/greysquirr3l/lemmings/pkg/worker"
)

// Manager coordinates workers and handles task distribution
type Manager struct {
	sync.RWMutex
	config           Config
	resourceCtrl     *ResourceController
	workerPool       *worker.Pool
	workerFactory    factory.WorkerFactory[worker.Worker]
	taskQueue        chan worker.Task
	results          chan worker.Result
	activeWorkers    map[int]worker.Worker
	activeWorkersMu  sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	started          bool
	stats            ManagerStats
	statsMu          sync.RWMutex
	shutdownCh       chan struct{}
	shutdownComplete chan struct{}
}

// ManagerStats tracks processing statistics
type ManagerStats struct {
	TasksSubmitted    int64
	TasksCompleted    int64
	TasksFailed       int64
	TotalProcessTime  time.Duration
	StartTime         time.Time
	AvgProcessTime    time.Duration
	PeakWorkerCount   int
	CurrentTaskCount  int
	TasksWaiting      int64
	CurrentMemPercent float64
	MemUtilization    []float64 // Records memory utilization over time
	WorkerUtilization []int     // Records worker count over time
	LastUpdated       time.Time
}

// NewManager creates a new manager
func NewManager(workerFactory factory.WorkerFactory[worker.Worker], config Config) (*Manager, error) {
	// Validate config
	if config.MaxWorkers < 1 {
		return nil, errors.New("max workers must be at least 1")
	}
	if config.InitialWorkers < 1 {
		config.InitialWorkers = 1
	}
	if config.InitialWorkers > config.MaxWorkers {
		config.InitialWorkers = config.MaxWorkers
	}
	if config.MinWorkers < 1 {
		config.MinWorkers = 1
	}
	if config.MinWorkers > config.InitialWorkers {
		config.MinWorkers = config.InitialWorkers
	}

	// Use default config for zero values
	defaultCfg := DefaultConfig()
	if config.TargetCPUPercent == 0 {
		config.TargetCPUPercent = defaultCfg.TargetCPUPercent
	}
	if config.TargetMemoryPercent == 0 {
		config.TargetMemoryPercent = defaultCfg.TargetMemoryPercent
	}

	// Create resource controller
	resourceCtrl := NewResourceController(config)

	// Create bounded task queue
	maxQueue := config.MaxTasksInQueue
	if maxQueue <= 0 {
		maxQueue = defaultCfg.MaxTasksInQueue
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		config:           config,
		resourceCtrl:     resourceCtrl,
		workerFactory:    workerFactory,
		taskQueue:        make(chan worker.Task, maxQueue),
		results:          make(chan worker.Result, maxQueue),
		activeWorkers:    make(map[int]worker.Worker),
		ctx:              ctx,
		cancel:           cancel,
		shutdownCh:       make(chan struct{}),
		shutdownComplete: make(chan struct{}),
		stats: ManagerStats{
			StartTime:         time.Now(),
			LastUpdated:       time.Now(),
			MemUtilization:    make([]float64, 0, 100),
			WorkerUtilization: make([]int, 0, 100),
		},
	}

	workerPool, err := worker.NewPool(
		ctx,
		config.InitialWorkers,
		config.MaxWorkers,
		m.taskQueue,
		m.results,
		workerFactory,
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create worker pool: %w", err)
	}
	m.workerPool = workerPool

	return m, nil
}

// Start begins processing tasks
func (m *Manager) Start() error {
	m.Lock()
	defer m.Unlock()

	if m.started {
		return errors.New("manager already started")
	}

	// Start the worker pool
	err := m.workerPool.Start()
	if err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	// Start monitoring resources
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.resourceCtrl.GetResourceMonitor().Start(m.ctx)
	}()

	// Start resource adjustment
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(m.config.MonitorInterval)
		defer ticker.Stop()

		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				m.resourceCtrl.AdjustBasedOnResourceUsage()

				// Update stats with current memory and worker usage
				memStats := m.resourceCtrl.GetResourceMonitor().GetStats().GetMemStats()

				m.statsMu.Lock()
				currentCount := m.workerPool.GetActiveWorkerCount()
				if currentCount > m.stats.PeakWorkerCount {
					m.stats.PeakWorkerCount = currentCount
				}

				// Record utilization metrics (keep last 100 points)
				m.stats.CurrentMemPercent = memStats.UsagePercent
				if len(m.stats.MemUtilization) >= 100 {
					m.stats.MemUtilization = m.stats.MemUtilization[1:100]
				}
				m.stats.MemUtilization = append(m.stats.MemUtilization, memStats.UsagePercent)

				if len(m.stats.WorkerUtilization) >= 100 {
					m.stats.WorkerUtilization = m.stats.WorkerUtilization[1:100]
				}
				m.stats.WorkerUtilization = append(m.stats.WorkerUtilization, currentCount)

				m.stats.LastUpdated = time.Now()
				m.statsMu.Unlock()
			}
		}
	}()

	// Start result handler
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.processResults()
	}()

	// Start graceful shutdown handler
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer close(m.shutdownComplete)
		<-m.shutdownCh

		if m.config.Verbose {
			log.Println("Manager beginning graceful shutdown")
		}

		// Stop accepting new tasks, but process existing ones
		close(m.taskQueue)

		// Set a timeout for remaining task completion
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Wait for tasks to complete or timeout
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-shutdownCtx.Done():
				if m.config.Verbose {
					log.Println("Manager shutdown timeout reached")
				}
				return
			case <-ticker.C:
				if len(m.taskQueue) == 0 && len(m.results) == 0 {
					if m.config.Verbose {
						log.Println("Manager completed graceful shutdown")
					}
					return
				}
			}
		}
	}()

	m.started = true
	return nil
}

// Stop stops the manager and all workers
func (m *Manager) Stop() {
	m.Lock()

	if !m.started {
		m.Unlock()
		return
	}

	// Begin graceful shutdown
	select {
	case m.shutdownCh <- struct{}{}:
		if m.config.Verbose {
			log.Println("Manager initiating graceful shutdown")
		}
	default:
		// Shutdown already initiated
	}

	m.Unlock()

	// Wait for graceful shutdown to complete with a timeout
	select {
	case <-m.shutdownComplete:
		// Graceful shutdown completed
	case <-time.After(32 * time.Second):
		// Forceful shutdown after timeout
		if m.config.Verbose {
			log.Println("Manager forcefully shutting down")
		}
	}

	// Now cancel everything and wait
	m.cancel()
	m.wg.Wait()

	m.Lock()
	m.started = false
	m.Unlock()
}

// Submit submits a task for processing
func (m *Manager) Submit(task worker.Task) error {
	if !m.started {
		return worker.ErrManagerNotStarted
	}

	select {
	case m.taskQueue <- task:
		m.statsMu.Lock()
		m.stats.TasksSubmitted++
		m.stats.CurrentTaskCount++
		m.stats.TasksWaiting++
		m.statsMu.Unlock()
		return nil
	case <-m.ctx.Done():
		return worker.ErrManagerShuttingDown
	default:
		return worker.ErrTaskQueueFull
	}
}

// SubmitWithTimeout submits a task with a timeout
func (m *Manager) SubmitWithTimeout(task worker.Task, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if !m.started {
		return worker.ErrManagerNotStarted
	}

	select {
	case m.taskQueue <- task:
		m.statsMu.Lock()
		m.stats.TasksSubmitted++
		m.stats.CurrentTaskCount++
		m.stats.TasksWaiting++
		m.statsMu.Unlock()
		return nil
	case <-m.ctx.Done():
		return worker.ErrManagerShuttingDown
	case <-ctx.Done():
		return errors.New("submission timeout")
	}
}

// SubmitBatch submits multiple tasks
func (m *Manager) SubmitBatch(tasks []worker.Task) (int, error) {
	if !m.started {
		return 0, worker.ErrManagerNotStarted
	}

	submitted := 0
	for _, task := range tasks {
		select {
		case m.taskQueue <- task:
			submitted++
		case <-m.ctx.Done():
			m.updateSubmittedStats(submitted)
			return submitted, worker.ErrManagerShuttingDown
		default:
			m.updateSubmittedStats(submitted)
			return submitted, worker.ErrTaskQueueFull
		}
	}

	m.updateSubmittedStats(submitted)
	return submitted, nil
}

// updateSubmittedStats updates statistics after task submission
func (m *Manager) updateSubmittedStats(count int) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()

	m.stats.TasksSubmitted += int64(count)
	m.stats.CurrentTaskCount += count
	m.stats.TasksWaiting += int64(count)
}

// processResults processes task results
func (m *Manager) processResults() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case result, ok := <-m.results:
			if !ok {
				return // Channel closed
			}

			// Handle callback if task has one
			if fnTask, ok := result.Task.(*worker.FunctionTask); ok {
				if callback := fnTask.GetCallback(); callback != nil {
					go callback(result)
				}
			}

			m.statsMu.Lock()
			m.stats.CurrentTaskCount--
			m.stats.TasksWaiting--
			if result.Err != nil {
				m.stats.TasksFailed++
			} else {
				m.stats.TasksCompleted++
				m.stats.TotalProcessTime += result.Duration
				if m.stats.TasksCompleted > 0 {
					m.stats.AvgProcessTime = time.Duration(int64(m.stats.TotalProcessTime) / m.stats.TasksCompleted)
				}
			}
			m.statsMu.Unlock()
		}
	}
}

// GetStats returns the current processing statistics
func (m *Manager) GetStats() ManagerStats {
	m.statsMu.RLock()
	defer m.statsMu.RUnlock()
	return m.stats
}

// GetWorkerCount returns the current number of workers
func (m *Manager) GetWorkerCount() int {
	return m.workerPool.GetActiveWorkerCount()
}

// GetTaskQueueLength returns the current number of tasks in the queue
func (m *Manager) GetTaskQueueLength() int {
	return len(m.taskQueue)
}

// GetResultQueueLength returns the current number of results in the queue
func (m *Manager) GetResultQueueLength() int {
	return len(m.results)
}

// EnableDynamicScaling enables automatic worker scaling
func (m *Manager) EnableDynamicScaling() {
	m.resourceCtrl.EnableScaling()
}

// DisableDynamicScaling disables automatic worker scaling
func (m *Manager) DisableDynamicScaling() {
	m.resourceCtrl.DisableScaling()
}

// ForceScaleWorkers manually scales workers to the specified count
func (m *Manager) ForceScaleWorkers(count int) {
	m.resourceCtrl.AdjustWorkerCount(count, "manual scaling")
}

// WaitForCompletion blocks until all tasks are processed or context is canceled
func (m *Manager) WaitForCompletion(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			stats := m.GetStats()
			if stats.TasksSubmitted == stats.TasksCompleted+stats.TasksFailed &&
				m.GetTaskQueueLength() == 0 &&
				stats.TasksSubmitted > 0 {
				return nil
			}
		}
	}
}
