package worker

import (
	"context"
	"testing"
	"time"
)

const (
	task1ID = "task1" // Extracted constant for task1
	task2ID = "task2" // Extracted constant for task2
)

func TestPriorityQueue(t *testing.T) {
	t.Run("NewPriorityQueue initializes empty queue", func(t *testing.T) {
		pq := NewPriorityQueue()

		if pq.Len() != 0 {
			t.Errorf("Expected length 0, got %d", pq.Len())
		}

		if pq.Peek() != nil {
			t.Errorf("Expected Peek() to return nil for empty queue, got %v", pq.Peek())
		}

		if pq.Pop() != nil {
			t.Errorf("Expected Pop() to return nil for empty queue, got %v", pq.Pop())
		}
	})

	t.Run("Push and Pop maintain priority order", func(t *testing.T) {
		pq := NewPriorityQueue()

		// Create tasks with different priorities
		task1 := NewSimpleTask(task1ID, "test", nil).WithPriority(1)
		task2 := NewSimpleTask(task2ID, "test", nil).WithPriority(3)
		task3 := NewSimpleTask("task3", "test", nil).WithPriority(2)

		// Push tasks
		pq.Push(task1)
		pq.Push(task2)
		pq.Push(task3)

		// Verify queue length
		if pq.Len() != 3 {
			t.Errorf("Expected length 3, got %d", pq.Len())
		}

		// Verify Pop returns tasks in priority order (highest first)
		popped := pq.Pop()
		if popped == nil || popped.ID() != task2ID {
			t.Errorf("Expected %s (priority 3), got %v", task2ID, popped)
		}

		popped = pq.Pop()
		if popped == nil || popped.ID() != "task3" {
			t.Errorf("Expected task3 (priority 2), got %v", popped)
		}

		popped = pq.Pop()
		if popped == nil || popped.ID() != task1ID {
			t.Errorf("Expected %s (priority 1), got %v", task1ID, popped)
		}

		// Queue should be empty now
		if pq.Len() != 0 {
			t.Errorf("Expected empty queue, got length %d", pq.Len())
		}
	})

	t.Run("Peek returns highest priority task without removing", func(t *testing.T) {
		pq := NewPriorityQueue()

		task1 := NewSimpleTask(task1ID, "test", nil).WithPriority(1)
		task2 := NewSimpleTask(task2ID, "test", nil).WithPriority(2)

		pq.Push(task1)
		pq.Push(task2)

		// Peek should return highest priority task
		peeked := pq.Peek()
		if peeked == nil || peeked.ID() != task2ID {
			t.Errorf("Expected Peek() to return task2, got %v", peeked)
		}

		// Queue length should not change after Peek
		if pq.Len() != 2 {
			t.Errorf("Expected length to remain 2 after Peek(), got %d", pq.Len())
		}
	})

	t.Run("Items with same priority ordered by insertion time", func(t *testing.T) {
		pq := NewPriorityQueue()

		// Create tasks with same priority
		task1 := NewSimpleTask("task1", "test", nil).WithPriority(1)

		// Add first task
		pq.Push(task1)

		// Sleep to ensure different timestamps
		time.Sleep(10 * time.Millisecond)

		// Add second task with same priority
		task2 := NewSimpleTask("task2", "test", nil).WithPriority(1)
		pq.Push(task2)

		// Verify FIFO order for same priority
		popped := pq.Pop()
		if popped == nil || popped.ID() != "task1" {
			t.Errorf("Expected task1 (inserted first), got %v", popped)
		}

		popped = pq.Pop()
		if popped == nil || popped.ID() != "task2" {
			t.Errorf("Expected task2 (inserted second), got %v", popped)
		}
	})

	t.Run("Queue is thread-safe", func(t *testing.T) {
		pq := NewPriorityQueue()

		// Number of goroutines and operations
		goroutines := 10
		tasksPerGoroutine := 100

		// Create synchronization channels
		done := make(chan struct{})
		start := make(chan struct{})

		// Create goroutines that push tasks
		for i := 0; i < goroutines; i++ {
			go func(id int) {
				// Wait for start signal
				<-start

				// Push tasks with varying priorities
				for j := 0; j < tasksPerGoroutine; j++ {
					priority := j % 5 // Use 5 different priority levels
					taskID := "task-" + string(rune(id+'0')) + "-" + string(rune(j+'0'))

					task := NewSimpleTask(taskID, "test", func(ctx context.Context) (interface{}, error) {
						return nil, nil
					}).WithPriority(priority)

					pq.Push(task)
				}

				done <- struct{}{}
			}(i)
		}

		// Start all goroutines at once
		close(start)

		// Wait for all goroutines to finish
		for i := 0; i < goroutines; i++ {
			<-done
		}

		// Verify queue length
		expected := goroutines * tasksPerGoroutine
		if pq.Len() != expected {
			t.Errorf("Expected length %d, got %d", expected, pq.Len())
		}

		// Verify items can be popped
		var prevPriority int = 100 // Start with high value
		for i := 0; i < expected; i++ {
			task := pq.Pop()
			if task == nil {
				t.Errorf("Unexpected nil task at position %d", i)
				break
			}

			// Priority should be non-increasing
			if task.Priority() > prevPriority {
				t.Errorf("Priority order violated: got priority %d after %d",
					task.Priority(), prevPriority)
			}
			prevPriority = task.Priority()
		}

		// Queue should be empty after popping all items
		if pq.Len() != 0 {
			t.Errorf("Expected empty queue after popping all items, got length %d", pq.Len())
		}
	})
}

func TestTaskHeap(t *testing.T) {
	t.Run("taskHeap implements heap.Interface correctly", func(t *testing.T) {
		h := &taskHeap{}

		// Create task items
		item1 := &taskItem{
			task:      NewSimpleTask("task1", "test", nil),
			priority:  1,
			timestamp: time.Now().UnixNano(),
			index:     0,
		}

		item2 := &taskItem{
			task:      NewSimpleTask("task2", "test", nil),
			priority:  2,
			timestamp: time.Now().UnixNano() + 1,
			index:     0,
		}

		// Test Push
		h.Push(item1)
		if len(*h) != 1 {
			t.Errorf("Expected length 1 after Push, got %d", len(*h))
		}

		h.Push(item2)
		if len(*h) != 2 {
			t.Errorf("Expected length 2 after Push, got %d", len(*h))
		}

		// Test Less (higher priority should be "less" so it comes first)
		if !h.Less(1, 0) {
			t.Error("Expected item2 (higher priority) to be Less than item1")
		}

		// Test Pop
		popped := h.Pop()
		if popped == nil {
			t.Error("Unexpected nil from Pop()")
		}

		item, ok := popped.(*taskItem)
		if !ok {
			t.Error("Expected *taskItem from Pop(), got different type")
		} else if item.task.ID() != task2ID {
			t.Errorf("Expected task2 from Pop(), got %s", item.task.ID())
		}

		// Test Swap and index updates
		*h = append(*h, item1, item2)
		h.Swap(0, 1)

		if (*h)[0].task.ID() != "task2" || (*h)[1].task.ID() != "task1" {
			t.Error("Swap did not correctly swap elements")
		}

		if (*h)[0].index != 0 || (*h)[1].index != 1 {
			t.Error("Swap did not update indices correctly")
		}
	})

	t.Run("taskHeap rejects invalid type in Push", func(t *testing.T) {
		h := &taskHeap{}

		// Push a non-*taskItem value
		initialLen := len(*h)
		h.Push("not a task item")

		// Length should not change
		if len(*h) != initialLen {
			t.Error("Expected heap to reject invalid type")
		}
	})
}
