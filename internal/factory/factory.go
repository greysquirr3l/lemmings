// Package factory implements generic factory patterns for creating objects.
// It provides type-safe factories for creating workers and other components.
package factory

import (
	"context"
	"fmt"
	"sync"
)

// Factory is a generic interface for creating objects of type T.
type Factory[T any] interface {
	// Create creates and returns a new instance of type T.
	// Returns the created object or an error if creation fails.
	Create() (T, error)
}

// FactoryFunc is a function type that implements the Factory interface.
// It allows simple functions to be used as factories.
type FactoryFunc[T any] func() (T, error)

// Create calls the factory function to create a new instance.
// Implements the Factory interface.
func (f FactoryFunc[T]) Create() (T, error) {
	return f()
}

// WorkerFactory is a factory that creates workers with sequential IDs.
type WorkerFactory[T any] interface {
	Factory[T]

	// CreateWithID creates a worker with the specified ID.
	// Returns the created worker or an error if creation fails.
	CreateWithID(id int) (T, error)
}

// WorkerFactoryFunc is a function type that implements WorkerFactory
type WorkerFactoryFunc[T any] struct {
	createFn func(id int) (T, error)
}

// NewWorkerFactory creates a new worker factory from a function
func NewWorkerFactory[T any](fn func(id int) (T, error)) WorkerFactory[T] {
	return &WorkerFactoryFunc[T]{
		createFn: fn,
	}
}

// Create implements the Factory interface
func (f *WorkerFactoryFunc[T]) Create() (T, error) {
	return f.CreateWithID(0)
}

// CreateWithID implements the WorkerFactory interface
func (f *WorkerFactoryFunc[T]) CreateWithID(id int) (T, error) {
	if f.createFn == nil {
		var zero T
		return zero, fmt.Errorf("worker factory function is nil")
	}
	return f.createFn(id)
}

// Pool is a generic object pool
type Pool[T any] struct {
	factory     Factory[T]
	objects     []T
	mu          sync.Mutex
	initialized bool
	maxSize     int
}

// NewPool creates a new object pool
func NewPool[T any](factory Factory[T], initialSize, maxSize int) (*Pool[T], error) {
	p := &Pool[T]{
		factory:     factory,
		objects:     make([]T, 0, initialSize),
		maxSize:     maxSize,
		initialized: false,
	}

	// Pre-initialize objects
	if initialSize > 0 {
		for i := 0; i < initialSize; i++ {
			obj, err := factory.Create()
			if err != nil {
				return nil, fmt.Errorf("failed to initialize pool object: %w", err)
			}
			p.objects = append(p.objects, obj)
		}
	}
	p.initialized = true

	return p, nil
}

// Get retrieves an object from the pool or creates a new one
func (p *Pool[T]) Get(ctx context.Context) (T, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check context cancellation
	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	default:
		// Continue
	}

	// If we have available objects, return one
	if len(p.objects) > 0 {
		obj := p.objects[len(p.objects)-1]
		p.objects = p.objects[:len(p.objects)-1]
		return obj, nil
	}

	// Create a new object if we haven't reached max size
	if !p.initialized || p.maxSize <= 0 || len(p.objects) < p.maxSize {
		return p.factory.Create()
	}

	// No objects available and at max size
	var zero T
	return zero, fmt.Errorf("pool exhausted")
}

// Put returns an object to the pool
func (p *Pool[T]) Put(obj T) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.maxSize <= 0 || len(p.objects) < p.maxSize {
		p.objects = append(p.objects, obj)
	}
}

// Size returns the current size of the pool
func (p *Pool[T]) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.objects)
}
