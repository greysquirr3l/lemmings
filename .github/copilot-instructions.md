# Lemmings Library - GitHub Copilot Instructions

## Project Overview

Lemmings is a dynamic worker/manager library for Go applications that need to distribute and process tasks efficiently. The library automatically scales workers based on resource usage, provides task retry capabilities, and offers a flexible framework for processing any type of task.

## Component Structure

- **Factory Pattern (`internal/factory`)**: Generic implementation of the factory pattern to create workers and tasks
- **Resource Monitoring (`internal/utils`)**: Utilities for monitoring system resources and memory usage
- **Manager (`pkg/manager`)**: Core component that coordinates workers and distributes tasks
- **Worker (`pkg/worker`)**: Worker implementation that processes tasks
- **Task (`pkg/worker`)**: Interface and implementations for different task types
- **Middleware (`pkg/middleware`)**: Middleware for cross-cutting concerns like logging and recovery

## Key Concepts

### Manager

The manager is responsible for:

- Managing the worker pool
- Distributing tasks to workers
- Scaling workers up/down based on resource usage
- Tracking task statistics

Example usage:

```go
workerFactory := factory.NewWorkerFactory(func(id int) (worker.Worker, error) {
    return worker.NewSimpleWorker(id, nil, nil, 3), nil
})
config := manager.DefaultConfig()
mgr, _ := manager.NewManager(workerFactory, config)
mgr.Start()
```

### Workers

Workers process tasks from a queue. They can be customized and extended:

```go
type CustomWorker struct {
    *worker.SimpleWorker
    customData string
}
```

### Tasks

Tasks represent units of work:

```go
task := worker.NewFunctionTask("task-id", func(ctx context.Context) (interface{}, error) {
    // Task implementation
    return result, nil
})
```

## Common Patterns

### Submitting Tasks

```go
// Submit a single task
err := mgr.Submit(task)

// Submit with timeout
err := mgr.SubmitWithTimeout(task, 5*time.Second)

// Submit a batch
count, err := mgr.SubmitBatch(tasks)
```

### Resource Control

```go
// Enable dynamic scaling
mgr.EnableDynamicScaling()

// Manually adjust worker count
mgr.ForceScaleWorkers(10)
```

### Task Creation

```go
// Simple task
task := worker.NewFunctionTask("id", taskFunc)

// With customization
task := worker.NewFunctionTask("id", taskFunc).
    WithTimeout(5*time.Second).
    WithRetries(3)
```

### Using Middleware

```go
// Create middleware chain
chain := middleware.Chain(
    middleware.LoggingMiddleware(),
    middleware.RecoveryMiddleware(),
)

// Apply middleware to task
wrappedTask := middleware.WrapTask(task, chain)
```

## Development Guidelines

1. **Context Awareness**: All task implementations should respect context cancellation
2. **Resource Efficiency**: Always consider memory usage and resource cleanup
3. **Concurrency Safety**: Use locks and atomic operations for shared data
4. **Error Handling**: Wrap errors with context, use appropriate error types
5. **Worker Scaling**: Ensure workers can be safely added/removed during operation

## Testing Approach

1. Test component interfaces independently
2. Use mocks for dependencies
3. Include stress tests for resource management
4. Test scaling under load
5. Verify task retry behavior
6. Ensure context cancellation is properly handled

## Architecture Diagram

```shell
┌─────────────────┐      ┌───────────────┐
│     Manager     │◄─────┤Configuration  │
└────────┬────────┘      └───────────────┘
         │
         ▼
┌─────────────────┐      ┌───────────────┐
│  Worker Pool    │◄─────┤Resource Ctrl  │
└────────┬────────┘      └───────────────┘
         │
         ▼
┌─────────────────┐      ┌───────────────┐
│    Workers      │      │   Tasks       │
└─────────────────┘      └───────────────┘
```

## Extension Points

- Custom worker implementations
- Specialized task types
- Resource monitoring strategies
- Scaling policies
- Custom middleware
- Priority-based scheduling
