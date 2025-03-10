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
- **Graceful Handling**: Properly manages shutdown, cancellation and timeouts
- **Factory Pattern**: Create custom worker implementations easily
- **Efficient Batching**: Submit tasks individually or in batches

## Installation

```bash
go get github.com/greysquirr3l/lemmings
```

## Quick Start

```go
package main

import (
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
    task := worker.NewFunctionTask("task-1", func() (interface{}, error) {
        // Your task logic here
        time.Sleep(500 * time.Millisecond)
        return "task completed", nil
    })

    if err := mgr.Submit(task); err != nil {
        log.Printf("Failed to submit task: %v", err)
    }

    // Wait for processing to complete
    time.Sleep(1 * time.Second)

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
task := worker.NewFunctionTask("task-id", func() (interface{}, error) {
    // Task logic here
    return result, nil
})

// With customization
task := worker.NewFunctionTask("task-id", taskFunc).
    WithTimeout(5 * time.Second).
    WithRetries(3).
    WithPriority(10)
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
2. **Simplicity of use** - Clear interfaces with sensible defaults
3. **Genericity** - Process any type of task with type safety
4. **Resilience** - Gracefully handle failures and recover from errors

## License

MIT

### Step 1: Initialize the Go Module

First, create a new directory for your project and initialize a Go module:

```bash
mkdir lemmings
cd lemmings
go mod init lemmings
```

### Step 2: Create the Manager and Worker Files

#### `manager/manager.go`

This file will contain the `Manager` struct and its methods for managing workers.

```go
package manager

import (
 "context"
 "log"
 "sync"
 "time"
)

// Manager manages the dynamic scaling of workers
type Manager struct {
 mu          sync.Mutex
 workers     []*Worker
 maxWorkers  int
 minWorkers  int
 activeCount int
}

// NewManager creates a new Manager instance
func NewManager(minWorkers, maxWorkers int) *Manager {
 return &Manager{
  minWorkers: minWorkers,
  maxWorkers: maxWorkers,
 }
}

// Start initializes and starts the workers
func (m *Manager) Start(ctx context.Context) {
 m.mu.Lock()
 defer m.mu.Unlock()

 for i := 0; i < m.minWorkers; i++ {
  worker := NewWorker(i)
  m.workers = append(m.workers, worker)
  go worker.Start(ctx)
  m.activeCount++
 }
}

// ScaleUp increases the number of active workers
func (m *Manager) ScaleUp() {
 m.mu.Lock()
 defer m.mu.Unlock()

 if m.activeCount < m.maxWorkers {
  worker := NewWorker(m.activeCount)
  m.workers = append(m.workers, worker)
  go worker.Start(context.Background())
  m.activeCount++
  log.Printf("Scaled up: %d active workers", m.activeCount)
 }
}

// ScaleDown decreases the number of active workers
func (m *Manager) ScaleDown() {
 m.mu.Lock()
 defer m.mu.Unlock()

 if m.activeCount > m.minWorkers {
  worker := m.workers[m.activeCount-1]
  worker.Stop()
  m.workers = m.workers[:m.activeCount-1]
  m.activeCount--
  log.Printf("Scaled down: %d active workers", m.activeCount)
 }
}
```

#### `manager/worker.go`

This file will define the `Worker` struct and its methods.

```go
package manager

import (
 "context"
 "log"
 "time"
)

// Worker represents a worker that processes tasks
type Worker struct {
 id     int
 active bool
}

// NewWorker creates a new Worker instance
func NewWorker(id int) *Worker {
 return &Worker{id: id, active: true}
}

// Start begins the worker's processing loop
func (w *Worker) Start(ctx context.Context) {
 log.Printf("Worker %d started", w.id)
 for w.active {
  select {
  case <-ctx.Done():
   log.Printf("Worker %d stopping", w.id)
   return
  default:
   // Simulate work
   time.Sleep(1 * time.Second)
   log.Printf("Worker %d is working", w.id)
  }
 }
}

// Stop halts the worker's processing
func (w *Worker) Stop() {
 w.active = false
}
```

### Step 3: Create the Main File

#### `main.go`

This file will serve as the entry point for your application.

```go
package main

import (
 "context"
 "log"
 "time"

 "lemmings/manager"
)

func main() {
 ctx, cancel := context.WithCancel(context.Background())
 defer cancel()

 // Create a new manager with a minimum of 2 workers and a maximum of 5
 mgr := manager.NewManager(2, 5)
 mgr.Start(ctx)

 // Simulate dynamic scaling
 for i := 0; i < 10; i++ {
  time.Sleep(2 * time.Second)
  mgr.ScaleUp()
 }

 for i := 0; i < 5; i++ {
  time.Sleep(3 * time.Second)
  mgr.ScaleDown()
 }

 // Allow some time for workers to finish
 time.Sleep(5 * time.Second)
 cancel()
 log.Println("Shutting down...")
}
```

### Step 4: Create the README File

#### `README.md`

Now let's create a README.md file that explains the purpose and usage of the library:

```markdown
# Lemmings

Lemmings is a lightweight, highly configurable worker/manager library for Go that makes it easy to distribute any job to workers. It features dynamic scaling based on resource usage, keeping your application performant under varying loads.

## Features

- **Dynamic Worker Scaling**: Automatically scales workers up and down based on memory usage
- **Flexible Task Framework**: Submit any job type to workers with minimal boilerplate
- **Resource Monitoring**: Built-in monitoring of memory and system resources
- **Graceful Handling**: Properly handles shutdown, cancellation and timeouts
- **Generic Worker Factory**: Create custom worker implementations with the factory pattern
- **Task Retries**: Configurable retry mechanism with exponential backoff
- **Batch Processing**: Submit individual tasks or batches for efficient processing

## Installation

```bash
go get github.com/greysquirr3l/lemmings
```

## Quick Start

```go
package main

import (
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

 // Create manager with default configuration
 config := manager.DefaultConfig()
 config.InitialWorkers = 5
 config.MaxWorkers = 20

 mgr, err := manager.NewManager(workerFactory, config)
 if err != nil {
  log.Fatal(err)
 }

 // Start the manager
 if err := mgr.Start(); err != nil {
  log.Fatal(err)
 }
 defer mgr.Stop()

 // Enable dynamic scaling
 mgr.EnableDynamicScaling()

 // Submit a task
 task := worker.NewFunctionTask("task-1", func() (interface{}, error) {
  // Your task logic here
  time.Sleep(500 * time.Millisecond)
  return "task completed", nil
 })

 if err := mgr.Submit(task); err != nil {
  log.Printf("Failed to submit task: %v", err)
 }

 // Wait for processing to complete
 time.Sleep(1 * time.Second)

 // Get statistics
 stats := mgr.GetStats()
 fmt.Printf("Tasks completed: %d, Failed: %d\n",
  stats.TasksCompleted, stats.TasksFailed)
}
```

## Documentation

### Creating Custom Workers

You can create custom worker implementations by embedding the `SimpleWorker`:

```go
type CustomWorker struct {
 *worker.SimpleWorker
 customData string
}

// Create a custom worker factory
workerFactory := factory.NewWorkerFactory(func(id int) (worker.Worker, error) {
 baseWorker := worker.NewSimpleWorker(id, nil, nil, 3)
 return &CustomWorker{
  SimpleWorker: baseWorker,
  customData:   "worker data",
 }, nil
})
```

### Creating Tasks

The library comes with built-in task types:

```go
// Simple function task
task := worker.NewFunctionTask("task-id", func() (interface{}, error) {
 // Task logic here
 return result, nil
})

// With customization
task := worker.NewFunctionTask("task-id", taskFunc).
 WithTimeout(5 * time.Second).
 WithRetries(3).
 WithPriority(10)
```

### Configuration

The manager can be configured through the `manager.Config` struct:

```go
config := manager.DefaultConfig()
config.InitialWorkers = 10
config.MaxWorkers = 50
config.ScaleUpFactor = 1.5
config.ScaleDownFactor = 0.5
config.TaskTimeout = 1 * time.Minute
```

## Examples

Check out the `examples/` directory for more detailed examples:

- `examples/simple/main.go`: Basic usage example
- `examples/advanced/main.go`: Advanced features like custom workers and different task types

## License

MIT

### Step 5: Run the Project

To run your project, use the following command:

```bash
go run main.go
```

### Summary

This implementation provides a basic structure for a manager/worker library that dynamically scales the number of workers based on the defined limits. The `Manager` can scale up and down the number of `Worker` instances, and each worker simulates processing tasks in a loop. You can expand this further by adding task queues, error handling, and more sophisticated scaling logic based on actual workload metrics.
