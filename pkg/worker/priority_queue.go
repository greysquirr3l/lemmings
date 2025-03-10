// Package worker provides implementations for tasks and workers.
package worker

import (
	"container/heap"
	"sync"
	"time"
)

// PriorityQueue implements a thread-safe priority queue for tasks.
// It ensures that tasks with higher priority values are processed first.
// When priorities are equal, tasks are processed in FIFO order based on insertion time.
type PriorityQueue struct {
	mu    sync.RWMutex
	queue taskHeap
	count int
}

// NewPriorityQueue creates a new empty priority queue.
// Returns a pointer to the created PriorityQueue.
func NewPriorityQueue() *PriorityQueue {
	pq := &PriorityQueue{
		queue: make(taskHeap, 0),
	}
	heap.Init(&pq.queue)
	return pq
}

// Push adds a task to the queue.
// Tasks with higher priority values will be popped before tasks with lower priorities.
// Tasks with the same priority will be popped in FIFO order.
func (pq *PriorityQueue) Push(task Task) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	item := &taskItem{
		task:      task,
		priority:  task.Priority(),
		timestamp: time.Now().UnixNano(),
		index:     pq.count,
	}

	pq.count++
	heap.Push(&pq.queue, item)
}

// Pop removes and returns the highest priority task.
// If the queue is empty, it returns nil.
// Tasks with higher priority values will be popped first.
func (pq *PriorityQueue) Pop() Task {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.queue.Len() == 0 {
		return nil
	}

	// Fix: Add type assertion error check
	rawItem := heap.Pop(&pq.queue)
	item, ok := rawItem.(*taskItem)
	if !ok {
		return nil
	}
	return item.task
}

// Len returns the number of tasks currently in the queue.
func (pq *PriorityQueue) Len() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return pq.queue.Len()
}

// Peek returns the highest priority task without removing it.
// If the queue is empty, it returns nil.
func (pq *PriorityQueue) Peek() Task {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	if pq.queue.Len() == 0 {
		return nil
	}

	return pq.queue[0].task
}

// taskItem represents an item in the priority queue.
// It wraps a Task with additional metadata needed for queue operations.
type taskItem struct {
	task      Task
	priority  int
	timestamp int64 // For FIFO ordering of same-priority items
	index     int   // Index in the heap array
}

// taskHeap implements heap.Interface for task priority management.
type taskHeap []*taskItem

func (h *taskHeap) Len() int { return len(*h) }

// Higher priority values have higher priority in the queue
func (h *taskHeap) Less(i, j int) bool {
	// First compare by priority (higher value = higher priority)
	if (*h)[i].priority != (*h)[j].priority {
		return (*h)[i].priority > (*h)[j].priority
	}
	// If priorities are equal, compare by timestamp (older = higher priority)
	return (*h)[i].timestamp < (*h)[j].timestamp
}

func (h *taskHeap) Swap(i, j int) {
	(*h)[i], (*h)[j] = (*h)[j], (*h)[i]
	(*h)[i].index = i
	(*h)[j].index = j
}

func (h *taskHeap) Push(x interface{}) {
	// Fix: Add type assertion error check
	item, ok := x.(*taskItem)
	if !ok {
		return
	}
	*h = append(*h, item)
	item.index = len(*h) - 1
}

func (h *taskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*h = old[0 : n-1]
	return item
}
