package manager

import (
	"context"
	"errors"
	"fmt"
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
}

// ManagerStats tracks processing statistics
type ManagerStats struct {
	TasksSubmitted   int64
	TasksCompleted   int64
	TasksFailed      int64
	TotalProcessTime time.Duration
	StartTime        time.Time
	AvgProcessTime   time.Duration
	PeakWorkerCount  int
	CurrentTaskCount int
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
		config:        config,
		resourceCtrl:  resourceCtrl,
		workerFactory: workerFactory,
		taskQueue:     make(chan worker.Task, maxQueue),
		results:       make(chan worker.Result, maxQueue),
		activeWorkers: make(map[int]worker.Worker),
		ctx:           ctx,
		cancel:        cancel,
		stats: ManagerStats{
			StartTime: time.Now(),
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

				m.activeWorkersMu.Lock()
				currentCount := m.workerPool.GetActiveWorkerCount()
				if currentCount > m.stats.PeakWorkerCount {
					m.stats.PeakWorkerCount = currentCount
				}
				m.activeWorkersMu.Unlock()
			}
		}
	}()

	// Start result handler
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.processResults()
	}()

	m.started = true
	return nil
}

// Stop stops the manager and all workers
func (m *Manager) Stop() {
	m.Lock()
	defer m.Unlock()

	if !m.started {
		return
	}

	m.cancel()
	m.wg.Wait()
	m.started = false
}

// Submit submits a task for processing
func (m *Manager) Submit(task worker.Task) error {
	if !m.started {
		return errors.New("manager not started")
	}

	select {
	case m.taskQueue <- task:
		m.statsMu.Lock()
		m.stats.TasksSubmitted++
		m.stats.CurrentTaskCount++
		m.statsMu.Unlock()
		return nil
	case <-m.ctx.Done():
		return errors.New("manager shutting down")
	default:
		return errors.New("task queue full")
	}
}

// SubmitWithTimeout submits a task with a timeout
func (m *Manager) SubmitWithTimeout(task worker.Task, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if !m.started {
		return errors.New("manager not started")
	}

	select {
	case m.taskQueue <- task:
		m.statsMu.Lock()
		m.stats.TasksSubmitted++
		m.stats.CurrentTaskCount++
		m.statsMu.Unlock()
		return nil
	case <-m.ctx.Done():
		return errors.New("manager shutting down")
	case <-ctx.Done():
		return errors.New("submission timeout")
	}
}

// SubmitBatch submits multiple tasks
func (m *Manager) SubmitBatch(tasks []worker.Task) (int, error) {
	if !m.started {
		return 0, errors.New("manager not started")
	}

	submitted := 0
	for _, task := range tasks {
		select {
		case m.taskQueue <- task:
			submitted++
		case <-m.ctx.Done():
			return submitted, errors.New("manager shutting down")
		default:
			return submitted, errors.New("task queue full")
		}
	}

	m.statsMu.Lock()
	m.stats.TasksSubmitted += int64(submitted)
	m.stats.CurrentTaskCount += submitted
	m.statsMu.Unlock()

	return submitted, nil
}

// processResults processes task results
func (m *Manager) processResults() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case result := <-m.results:
			m.statsMu.Lock()
			m.stats.CurrentTaskCount--
			if result.Err != nil {
				m.stats.TasksFailed++
			} else {
				m.stats.TasksCompleted++
				m.stats.TotalProcessTime += result.Duration
				m.stats.AvgProcessTime = time.Duration(int64(m.stats.TotalProcessTime) / m.stats.TasksCompleted)
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
