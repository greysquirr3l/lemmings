package testutils

import (
	"context"
	"sync"
	"time"

	"github.com/greysquirr3l/lemmings/pkg/worker"
)

// MockTask is a mock implementation of worker.Task for testing
type MockTask struct {
	id          string
	taskType    string
	priority    int
	maxRetries  int
	execFunc    func(ctx context.Context) (interface{}, error)
	validateErr error
	execCount   int
	mu          sync.Mutex
}

// NewMockTask creates a new mock task with the given parameters
func NewMockTask(id string, execFunc func(ctx context.Context) (interface{}, error)) *MockTask {
	return &MockTask{
		id:         id,
		taskType:   "mock",
		maxRetries: 1,
		execFunc:   execFunc,
	}
}

// ID returns the task's identifier
func (t *MockTask) ID() string {
	return t.id
}

// Type returns the task type
func (t *MockTask) Type() string {
	return t.taskType
}

// Priority returns the task priority
func (t *MockTask) Priority() int {
	return t.priority
}

// MaxRetries returns the maximum number of retries for the task
func (t *MockTask) MaxRetries() int {
	return t.maxRetries
}

// Validate checks if the task is valid
func (t *MockTask) Validate() error {
	return t.validateErr
}

// Execute runs the task function
func (t *MockTask) Execute(ctx context.Context) (interface{}, error) {
	t.mu.Lock()
	t.execCount++
	execCount := t.execCount
	t.mu.Unlock()

	if t.execFunc != nil {
		return t.execFunc(ctx)
	}
	return execCount, nil
}

// WithPriority sets the task priority
func (t *MockTask) WithPriority(priority int) *MockTask {
	t.priority = priority
	return t
}

// WithRetries sets the maximum number of retries
func (t *MockTask) WithRetries(retries int) *MockTask {
	t.maxRetries = retries
	return t
}

// WithValidateError sets a validation error
func (t *MockTask) WithValidateError(err error) *MockTask {
	t.validateErr = err
	return t
}

// GetExecutionCount returns the number of times Execute has been called
func (t *MockTask) GetExecutionCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.execCount
}

// MockWorker is a mock implementation of worker.Worker for testing
type MockWorker struct {
	id         int
	available  bool
	stats      worker.WorkerStats
	mu         sync.RWMutex
	tasks      []worker.Task
	started    bool
	stopped    bool
	resultChan chan<- worker.Result
}

// NewMockWorker creates a new mock worker
func NewMockWorker(id int, resultChan chan worker.Result) *MockWorker {
	return &MockWorker{
		id:         id,
		available:  true,
		resultChan: resultChan,
		tasks:      make([]worker.Task, 0),
	}
}

// ID returns the worker's identifier
func (w *MockWorker) ID() int {
	return w.id
}

// Start begins the worker's processing loop
func (w *MockWorker) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.started = true
}

// Stop stops the worker
func (w *MockWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stopped = true
}

// Available checks if the worker is available
func (w *MockWorker) Available() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.available
}

// SetAvailable updates worker availability
func (w *MockWorker) SetAvailable(available bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.available = available
}

// Stats returns worker statistics
func (w *MockWorker) Stats() worker.WorkerStats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stats
}

// ExecuteTask simulates task execution
func (w *MockWorker) ExecuteTask(ctx context.Context, task worker.Task) {
	w.mu.Lock()
	w.tasks = append(w.tasks, task)
	w.mu.Unlock()

	// Execute the task
	output, err := task.Execute(ctx)

	// Send a result
	if w.resultChan != nil {
		result := worker.Result{
			TaskID:   task.ID(),
			Output:   output,
			Err:      err,
			Duration: 1 * time.Millisecond,
			Worker:   w.id,
			Attempt:  1,
			Task:     task,
		}
		w.resultChan <- result
	}

	// Update stats
	w.mu.Lock()
	if err != nil {
		w.stats.TasksFailed++
	} else {
		w.stats.TasksProcessed++
	}
	w.stats.TotalDuration += 1 * time.Millisecond
	w.mu.Unlock()
}

// GetTasks returns the tasks executed by this worker
func (w *MockWorker) GetTasks() []worker.Task {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.tasks
}

// IsStarted returns true if the worker was started
func (w *MockWorker) IsStarted() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.started
}

// IsStopped returns true if the worker was stopped
func (w *MockWorker) IsStopped() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stopped
}

// MockFactory is a mock implementation of factory.WorkerFactory
type MockFactory struct {
	createFn        func(id int) (worker.Worker, error)
	createdWorkers  []worker.Worker
	creationError   error
	creationCounter int
}

// NewMockFactory creates a new mock factory with the given worker creation function
func NewMockFactory(fn func(id int) (worker.Worker, error)) *MockFactory {
	return &MockFactory{
		createFn:       fn,
		createdWorkers: make([]worker.Worker, 0),
	}
}

// Create implements the Factory interface
func (f *MockFactory) Create() (worker.Worker, error) {
	return f.CreateWithID(0)
}

// CreateWithID implements the WorkerFactory interface
func (f *MockFactory) CreateWithID(id int) (worker.Worker, error) {
	f.creationCounter++
	if f.creationError != nil {
		return nil, f.creationError
	}

	var w worker.Worker
	var err error
	if f.createFn != nil {
		w, err = f.createFn(id)
	} else {
		// Create a default mock worker if no function is provided
		resultChan := make(chan worker.Result, 10)
		w = NewMockWorker(id, resultChan)
	}

	if w != nil {
		f.createdWorkers = append(f.createdWorkers, w)
	}
	return w, err
}

// SetCreationError sets an error to be returned on worker creation
func (f *MockFactory) SetCreationError(err error) {
	f.creationError = err
}

// GetCreatedWorkers returns all workers created by this factory
func (f *MockFactory) GetCreatedWorkers() []worker.Worker {
	return f.createdWorkers
}

// GetCreationCount returns the number of times Create was called
func (f *MockFactory) GetCreationCount() int {
	return f.creationCounter
}
