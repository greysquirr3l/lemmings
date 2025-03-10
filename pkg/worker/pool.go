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
func (p *Pool) Scale(delta int) (added, removed int) {
	p.Lock()
	defer p.Unlock()

	if !p.started {
		return 0, 0
	}

	// Calculate new size
	newSize := len(p.workers) + delta
	if newSize < 1 {
		newSize = 1
	}
	if newSize > p.maxSize {
		newSize = p.maxSize
	}

	// Adjust workers
	if delta > 0 {
		// Add workers
		for i := 0; i < delta && len(p.workers) < newSize; i++ {
			if err := p.addWorker(); err != nil {
				log.Printf("Error adding worker: %v", err)
				break
			}
			added++
		}
	} else if delta < 0 {
		// Remove workers
		for i := 0; i < -delta && len(p.workers) > newSize; i++ {
			if p.removeWorker() {
				removed++
			} else {
				break
			}
		}
	}

	return added, removed
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
