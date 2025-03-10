package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/greysquirr3l/lemmings/internal/factory"
)

// Pool manages a group of workers
type Pool struct {
	sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	workers       map[int]Worker
	workerFactory factory.WorkerFactory[Worker]
	taskChan      chan Task
	resultChan    chan Result
	size          int
	maxSize       int
	nextWorkerID  int32
	started       bool
	workerWg      sync.WaitGroup
}

// NewPool creates a new worker pool
func NewPool(ctx context.Context, initialSize, maxSize int, taskChan chan Task, resultChan chan Result,
	workerFactory factory.WorkerFactory[Worker]) (*Pool, error) {

	if initialSize < 0 {
		initialSize = 0
	}
	if maxSize <= 0 {
		return nil, fmt.Errorf("max size must be greater than 0")
	}
	if initialSize > maxSize {
		initialSize = maxSize
	}

	poolCtx, cancel := context.WithCancel(ctx)

	p := &Pool{
		ctx:           poolCtx,
		cancel:        cancel,
		workers:       make(map[int]Worker),
		workerFactory: workerFactory,
		taskChan:      taskChan,
		resultChan:    resultChan,
		size:          initialSize,
		maxSize:       maxSize,
		nextWorkerID:  0,
		started:       false,
	}

	return p, nil
}

// Start initializes and starts the worker pool
func (p *Pool) Start() error {
	p.Lock()
	defer p.Unlock()

	if p.started {
		return fmt.Errorf("pool already started")
	}

	// Initialize workers
	for i := 0; i < p.size; i++ {
		if err := p.addWorker(); err != nil {
			p.stopAllWorkers()
			return fmt.Errorf("failed to add worker: %w", err)
		}
	}

	p.started = true
	return nil
}

// Stop stops all workers in the pool
func (p *Pool) Stop() {
	p.Lock()
	defer p.Unlock()

	if !p.started {
		return
	}

	p.cancel()
	p.stopAllWorkers()
	p.started = false
}

// addWorker creates and starts a new worker
func (p *Pool) addWorker() error {
	id := int(atomic.AddInt32(&p.nextWorkerID, 1))

	worker, err := p.workerFactory.CreateWithID(id)
	if err != nil {
		return fmt.Errorf("failed to create worker %d: %w", id, err)
	}

	p.workers[id] = worker
	worker.Start(p.ctx)

	log.Printf("Added worker %d, pool size now %d", id, len(p.workers))
	return nil
}

// removeWorker removes a worker from the pool
func (p *Pool) removeWorker() bool {
	if len(p.workers) == 0 {
		return false
	}

	// Find a worker to remove (any one)
	var workerToRemove Worker
	var workerID int
	for id, w := range p.workers {
		workerToRemove = w
		workerID = id
		break
	}

	// Remove and stop the worker
	if workerToRemove != nil {
		delete(p.workers, workerID)
		workerToRemove.Stop()
		log.Printf("Removed worker %d, pool size now %d", workerID, len(p.workers))
		return true
	}

	return false
}

// stopAllWorkers stops all workers in the pool
func (p *Pool) stopAllWorkers() {
	for _, worker := range p.workers {
		worker.Stop()
	}
	p.workers = make(map[int]Worker)
	p.workerWg.Wait()
}

// Scale adjusts the pool size
// Fix: Reduce cyclomatic complexity by extracting functions
func (p *Pool) Scale(delta int) (added, removed int) {
	p.Lock()
	defer p.Unlock()

	if !p.started {
		return 0, 0
	}

	// Calculate new size with bounds checking
	newSize := p.calculateNewSize(delta)

	// Handle scaling up or down
	if delta > 0 {
		added = p.scaleUp(newSize)
	} else if delta < 0 {
		removed = p.scaleDown(newSize)
	}

	return added, removed
}

// calculateNewSize determines the new pool size based on constraints
func (p *Pool) calculateNewSize(delta int) int {
	newSize := len(p.workers) + delta
	if newSize < 1 {
		newSize = 1
	}
	if newSize > p.maxSize {
		newSize = p.maxSize
	}
	return newSize
}

// scaleUp adds workers to the pool
func (p *Pool) scaleUp(targetSize int) int {
	added := 0
	currentSize := len(p.workers)

	// Don't try to add more than needed
	toAdd := targetSize - currentSize
	if toAdd <= 0 {
		return 0
	}

	for i := 0; i < toAdd && len(p.workers) < targetSize; i++ {
		if err := p.addWorker(); err != nil {
			log.Printf("Error adding worker: %v", err)
			break
		}
		added++
	}

	return added
}

// scaleDown removes workers from the pool
func (p *Pool) scaleDown(targetSize int) int {
	removed := 0
	currentSize := len(p.workers)

	// Don't try to remove more than needed
	toRemove := currentSize - targetSize
	if toRemove <= 0 {
		return 0
	}

	for i := 0; i < toRemove && len(p.workers) > targetSize; i++ {
		if p.removeWorker() {
			removed++
		} else {
			break
		}
	}

	return removed
}

// Size returns the current pool size
func (p *Pool) Size() int {
	p.RLock()
	defer p.RUnlock()
	return len(p.workers)
}

// GetActiveWorkerCount returns the number of currently active workers
func (p *Pool) GetActiveWorkerCount() int {
	return p.Size()
}

// GetMaxSize returns the maximum allowed pool size
func (p *Pool) GetMaxSize() int {
	p.RLock()
	defer p.RUnlock()
	return p.maxSize
}
