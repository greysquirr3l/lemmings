# Lemmings

A lightweight, dynamically scaling worker management system for Go applications.

[![Go Reference](https://pkg.go.dev/badge/github.com/greysquirr3l/lemmings.svg)](https://pkg.go.dev/github.com/greysquirr3l/lemmings)

## Overview

Lemmings provides a robust framework for distributing and processing tasks across a flexible pool of workers. It automatically scales the number of workers based on system resource usage, making it ideal for applications with varying workloads.

## Key Features

- **Dynamic Worker Scaling**: Automatically adjusts worker count based on memory usage
- **Resource-Aware**: Monitors system resources to make intelligent scaling decisions
- **Generic Task Support**: Process any type of task through a simple interface
- **Fault Tolerance**: Configurable retry mechanism for failed tasks
- **Context-Aware**: Full support for context cancellation and timeouts
- **Graceful Handling**: Properly manages shutdown, cancellation, and timeouts
- **Factory Pattern**: Create custom worker implementations easily
- **Efficient Batching**: Submit tasks individually or in batches
- **Middleware Support**: Add cross-cutting concerns like logging and recovery

## Installation

```bash
go get github.com/greysquirr3l/lemmings
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/greysquirr3l/lemmings/internal/factory"
    "github.com/greysquirr3l/lemmings/pkg/manager"
    "github.com/greysquirr3l/lemmings/pkg/worker"
)

func main() {
    // Create a worker factory
    workerFactory := factory.NewWorkerFactory(func(id int) (worker.Worker, error) {
        return worker.NewSimpleWorker(id, nil, nil, 3), nil
    })

    // Configure the manager
    config := manager.DefaultConfig()
    config.InitialWorkers = 5
    config.MaxWorkers = 20

    // Create and start the manager
    mgr, err := manager.NewManager(workerFactory, config)
    if err != nil {
        log.Fatal(err)
    }

    if err := mgr.Start(); err != nil {
        log.Fatal(err)
    }
    defer mgr.Stop()

    // Enable dynamic scaling
    mgr.EnableDynamicScaling()

    // Create and submit a task
    task := worker.NewFunctionTask("task-1", func(ctx context.Context) (interface{}, error) {
        // Your task logic here
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-time.After(500 * time.Millisecond):
            return "task completed", nil
        }
    })

    if err := mgr.Submit(task); err != nil {
        log.Printf("Failed to submit task: %v", err)
    }

    // Wait for processing to complete
    waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := mgr.WaitForCompletion(waitCtx); err != nil {
        log.Printf("Error waiting for completion: %v", err)
    }

    // Get statistics
    stats := mgr.GetStats()
    fmt.Printf("Tasks completed: %d, Failed: %d\n",
        stats.TasksCompleted, stats.TasksFailed)
}
```

## Custom Workers

Create custom worker implementations by embedding the `SimpleWorker`:

```go
type CustomWorker struct {
    *worker.SimpleWorker
    customData string
}

workerFactory := factory.NewWorkerFactory(func(id int) (worker.Worker, error) {
    baseWorker := worker.NewSimpleWorker(id, nil, nil, 3)
    return &CustomWorker{
        SimpleWorker: baseWorker,
        customData:   "worker data",
    }, nil
})
```

## Task Creation

Create tasks using the built-in task types:

```go
// Simple function task
task := worker.NewFunctionTask("task-id", func(ctx context.Context) (interface{}, error) {
    // Task logic here
    return result, nil
})

// With customization
task := worker.NewFunctionTask("task-id", taskFunc).
    WithTimeout(5 * time.Second).
    WithRetries(3).
    WithPriority(10).
    WithCallback(func(result worker.Result) {
        log.Printf("Task %s completed with result: %v", result.TaskID, result.Output)
    })
```

## Middleware

Apply middleware to tasks for cross-cutting concerns:

```go
// Create middleware chain
chain := middleware.Chain(
    middleware.LoggingMiddleware(),
    middleware.RecoveryMiddleware(),
    middleware.TimeoutMiddleware(5 * time.Second),
    middleware.RetryMiddleware(3, 1.0),
)

// Apply middleware to task
wrappedTask := middleware.WrapTask(task, chain)

// Submit the wrapped task
mgr.Submit(wrappedTask)
```

## Priority Queue

Use the priority queue for more sophisticated task scheduling:

```go
queue := worker.NewPriorityQueue()

// Add tasks with different priorities
queue.Push(task1) // Default priority (0)
queue.Push(task2.WithPriority(10)) // Higher priority

// Process tasks in priority order
for queue.Len() > 0 {
    nextTask := queue.Pop()
    mgr.Submit(nextTask)
}
```

## Configuration

Configure the manager through the `manager.Config` struct:

```go
config := manager.DefaultConfig()
config.InitialWorkers = 10
config.MaxWorkers = 50
config.ScaleUpFactor = 1.5
config.ScaleDownFactor = 0.5
config.TaskTimeout = 1 * time.Minute
```

## Examples

Check out the examples directory for more detailed usage patterns:

- `examples/simple/main.go`: Basic usage example
- `examples/advanced/main.go`: Advanced features like custom workers and different task types

## Design Philosophy

Lemmings is built around a few core principles:

1. **Resource efficiency** - Workers scale based on system resources
2. **Context awareness** - Support for proper cancellation and timeouts
3. **Simplicity of use** - Clear interfaces with sensible defaults
4. **Genericity** - Process any type of task with type safety
5. **Resilience** - Gracefully handle failures and recover from errors

## License

MIT
