// Package lemmings is the main package for the Lemmings worker pool library.
// It provides convenient access to all the core functionality.
package lemmings

import (
	"github.com/greysquirr3l/lemmings/internal/factory"
	"github.com/greysquirr3l/lemmings/internal/utils"
	"github.com/greysquirr3l/lemmings/pkg/manager"
	"github.com/greysquirr3l/lemmings/pkg/middleware"
	"github.com/greysquirr3l/lemmings/pkg/worker"
)

// Version information
const (
	// Version is the current version of the Lemmings library
	Version = "0.1.1"
)

// NewManager creates a new Manager with the given worker factory and configuration.
// This is a convenience function that wraps manager.NewManager.
func NewManager(factory factory.WorkerFactory[worker.Worker], config manager.Config) (*manager.Manager, error) {
	return manager.NewManager(factory, config)
}

// DefaultConfig returns the default configuration for a Manager.
// This is a convenience function that wraps manager.DefaultConfig.
func DefaultConfig() manager.Config {
	return manager.DefaultConfig()
}

// NewWorkerFactory creates a new worker factory with the given creation function.
// This is a convenience function that wraps factory.NewWorkerFactory.
func NewWorkerFactory(fn func(id int) (worker.Worker, error)) factory.WorkerFactory[worker.Worker] {
	return factory.NewWorkerFactory(fn)
}

// CreateSimpleWorker creates a new simple worker with the given parameters.
// This is a convenience function that simplifies worker creation.
func CreateSimpleWorker(id int, resultChan chan<- worker.Result) worker.Worker {
	// Pass nil for taskCh since it's the first parameter
	return worker.NewSimpleWorker(id, nil, resultChan, 0)
}

// NewFunctionTask creates a new task from a function.
// This is a convenience function that wraps worker.NewFunctionTask.
func NewFunctionTask(id string, fn worker.TaskFunc) *worker.FunctionTask {
	return worker.NewFunctionTask(id, fn)
}

// CreateMiddlewareChain creates a new middleware chain with the given middleware.
// This is a convenience function that wraps middleware.Chain.
func CreateMiddlewareChain(middlewares ...middleware.TaskMiddleware) middleware.TaskMiddleware {
	return middleware.Chain(middlewares...)
}

// WrapTask wraps a task with middleware.
// This is a convenience function that wraps middleware.WrapTask.
func WrapTask(task worker.Task, middlewareFn middleware.TaskMiddleware) worker.Task {
	return middleware.WrapTask(task, middlewareFn)
}

// Common middleware
var (
	// LoggingMiddleware logs task execution details
	LoggingMiddleware = middleware.LoggingMiddleware
	// RecoveryMiddleware recovers from panics in task execution
	RecoveryMiddleware = middleware.RecoveryMiddleware
	// TimeoutMiddleware adds a timeout to task execution
	TimeoutMiddleware = middleware.TimeoutMiddleware
	// RetryMiddleware adds retry capability to task execution
	RetryMiddleware = middleware.RetryMiddleware
)

// ForceGC forces garbage collection.
// This is a convenience function that wraps utils.ForceGC.
func ForceGC() {
	utils.ForceGC()
}

// GetMemoryUsagePercent returns the current memory usage percentage.
// This is a convenience function that wraps utils.GetSimpleMemUsagePercent.
func GetMemoryUsagePercent() float64 {
	return utils.GetSimpleMemUsagePercent()
}
