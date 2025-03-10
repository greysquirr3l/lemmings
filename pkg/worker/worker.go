package worker

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Result represents the outcome of a task execution
type Result struct {
	TaskID   string
	Output   interface{}
	Err      error
	Duration time.Duration
	Worker   int
	Attempt  int
	Task     Task // Reference to original task
}

// Worker interface defines the behavior of a worker
type Worker interface {
	ID() int
	Start(context.Context)
	Stop()
	Available() bool
	SetAvailable(bool)
	Stats() WorkerStats
}

// WorkerStats contains statistics for a worker
type WorkerStats struct {
	TasksProcessed int64
	TasksFailed    int64
	TotalDuration  time.Duration
	IdleTime       time.Duration
	LastTaskTime   time.Time
}

// BaseWorker implements common Worker functionality
type BaseWorker struct {
	sync.RWMutex
	id         int
	stats      WorkerStats
	available  bool
	taskCh     <-chan Task
	resultCh   chan<- Result
	maxRetries int
	wg         sync.WaitGroup
}

// NewBaseWorker creates a new base worker
func NewBaseWorker(id int, taskCh <-chan Task, resultCh chan<- Result, maxRetries int) *BaseWorker {
	return &BaseWorker{
		id:         id,
		available:  true,
		taskCh:     taskCh,
		resultCh:   resultCh,
		maxRetries: maxRetries,
		stats: WorkerStats{
			LastTaskTime: time.Now(),
		},
	}
}

// ID returns the worker's identifier
func (w *BaseWorker) ID() int {
	return w.id
}

// Available checks if the worker is available
func (w *BaseWorker) Available() bool {
	w.RLock()
	defer w.RUnlock()
	return w.available
}

// SetAvailable updates worker availability
func (w *BaseWorker) SetAvailable(available bool) {
	w.Lock()
	defer w.Unlock()
	w.available = available
}

// Stats returns worker statistics
func (w *BaseWorker) Stats() WorkerStats {
	w.RLock()
	defer w.RUnlock()
	return w.stats
}

// ExecuteTask executes a task and handles result routing
// Fix: Reduce cyclomatic complexity by extracting functions
func (w *BaseWorker) ExecuteTask(ctx context.Context, task Task) {
	w.Lock()
	w.stats.IdleTime += time.Since(w.stats.LastTaskTime)
	w.stats.LastTaskTime = time.Now()
	w.Unlock()

	var result Result
	result.TaskID = task.ID()
	result.Worker = w.id
	result.Task = task

	// Validate task
	if err := task.Validate(); err != nil {
		result.Err = fmt.Errorf("task validation failed: %w", err)
		w.sendResult(result)
		return
	}

	// Execute with retry logic
	w.executeWithRetries(ctx, task, &result)
}

// executeWithRetries handles the retry logic for task execution
func (w *BaseWorker) executeWithRetries(ctx context.Context, task Task, result *Result) {
	maxRetries := task.MaxRetries()
	if maxRetries <= 0 {
		maxRetries = w.maxRetries
	}

	var output interface{}
	var err error

	start := time.Now()
	for attempt := 0; attempt <= maxRetries; attempt++ {
		result.Attempt = attempt + 1

		// Check if context is canceled before each attempt
		select {
		case <-ctx.Done():
			result.Err = ctx.Err()
			result.Duration = time.Since(start)
			w.sendResult(*result)
			return
		default:
			// Continue with execution
		}

		// Execute the task using context
		output, err = task.Execute(ctx)

		if err == nil {
			// Task succeeded
			result.Output = output
			result.Duration = time.Since(start)
			break
		} else if attempt >= maxRetries {
			// Task failed after all retries
			result.Err = NewTaskError(task.ID(), attempt+1, err.Error(), err)
			result.Duration = time.Since(start)
		} else {
			// Prepare for retry with exponential backoff
			if !w.waitForRetry(ctx, attempt, start, result) {
				return // Context was canceled during wait
			}
		}
	}

	// Update statistics
	w.updateTaskStats(result)
	w.sendResult(*result)
}

// waitForRetry handles the backoff wait between retry attempts
func (w *BaseWorker) waitForRetry(ctx context.Context, attempt int, start time.Time, result *Result) bool {
	// Fix: Avoid integer overflow by using small shift values
	// Only shift up to 20 to avoid potential overflow
	shift := attempt
	if shift > 20 {
		shift = 20
	}
	backoffDuration := time.Duration(1<<shift) * 10 * time.Millisecond
	if backoffDuration > 1*time.Second {
		backoffDuration = 1 * time.Second
	}

	select {
	case <-ctx.Done():
		result.Err = ctx.Err()
		result.Duration = time.Since(start)
		w.sendResult(*result)
		return false
	case <-time.After(backoffDuration):
		return true // Continue to next attempt
	}
}

// updateTaskStats updates worker statistics after task execution
func (w *BaseWorker) updateTaskStats(result *Result) {
	w.Lock()
	defer w.Unlock()

	if result.Err != nil {
		w.stats.TasksFailed++
	} else {
		w.stats.TasksProcessed++
	}
	w.stats.TotalDuration += result.Duration
	w.stats.LastTaskTime = time.Now()
}

// sendResult sends the task result to the result channel
func (w *BaseWorker) sendResult(result Result) {
	select {
	case w.resultCh <- result:
		// Result sent successfully
	default:
		// Result channel is full or closed
	}
}

// SimpleWorker is a basic implementation of the Worker interface
type SimpleWorker struct {
	*BaseWorker
	ctx    context.Context
	cancel context.CancelFunc
}

// NewSimpleWorker creates a new simple worker
func NewSimpleWorker(id int, taskCh <-chan Task, resultCh chan<- Result, maxRetries int) *SimpleWorker {
	baseWorker := NewBaseWorker(id, taskCh, resultCh, maxRetries)
	ctx, cancel := context.WithCancel(context.Background())

	return &SimpleWorker{
		BaseWorker: baseWorker,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins the worker's processing loop
func (w *SimpleWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			case <-w.ctx.Done():
				return
			case task, ok := <-w.taskCh:
				if !ok {
					// Task channel closed
					return
				}

				w.ExecuteTask(ctx, task)
			}
		}
	}()
}

// Stop stops the worker
func (w *SimpleWorker) Stop() {
	w.cancel()
	w.wg.Wait()
}
