# Lemmings Architecture

## Communication Architecture

The Lemmings library implements real-time communication between the manager and workers through multiple Go channels and shared metrics.

### Manager-Worker Communication

The communication between the Manager and Workers is bidirectional and happens in real-time through several mechanisms:

#### 1. Task Distribution Channel

```plaintext
Manager ----[Tasks]----> Workers
```

- The Manager maintains a task queue channel (`taskQueue chan worker.Task`)
- Workers listen on this channel and pick up tasks as they become available
- Tasks flow from the Manager to Workers in real-time
- The channel is buffered to allow for queuing up to a configurable maximum

#### 2. Result Collection Channel

```plaintext
Workers ----[Results]----> Manager
```

- Workers send task execution results back to the Manager through a results channel (`results chan worker.Result`)
- Results include execution metrics, task output, and error information
- The Manager processes these results and updates its statistics

#### 3. Worker Scaling Channel

```plaintext
ResourceController ----[Scale Commands]----> WorkerPool
```

- The ResourceController can send scaling signals through a control channel (`workerControl chan int`)
- Positive values indicate to scale up, negative values to scale down
- The WorkerPool adjusts its worker count in response

#### 4. Pause Processing Channel

```plaintext
ResourceController ----[Pause Signals]----> WorkerPool
```

- Under high memory conditions, the ResourceController can send pause signals
- The WorkerPool can temporarily stop accepting new tasks while continuing to process existing ones

### Resource Monitoring Feedback Loop

```plaintext
┌─────────────────┐       ┌─────────────────┐
│     Manager     │◄─────►│Resource Monitor │
└────────┬────────┘       └─────────────────┘
         │                        ▲
         ▼                        │
┌─────────────────┐               │
│   Worker Pool   │───────────────┘
└─────────────────┘
```

The manager receives continuous feedback about:

- Memory utilization
- Worker count and utilization
- Task processing rates
- Worker statistics

This real-time monitoring allows the system to dynamically adjust resources as needed.

### Statistics Aggregation

Workers continuously update their internal statistics about:

- Tasks processed
- Task failures
- Processing duration
- Idle time

The Manager collects these statistics through the result channel and aggregates them to provide global insights into the system's performance.

## Real-Time Responsiveness

The channel-based communication architecture ensures:

1. **Immediate Task Distribution**: Tasks are sent to workers as soon as they're submitted (if workers are available)
2. **Real-Time Result Processing**: Results are processed as soon as tasks complete
3. **Dynamic Resource Adjustment**: Worker count adjusts based on real-time resource utilization
4. **Zero Polling**: The system operates without polling, using channels and event-driven architecture

This design makes Lemmings highly responsive to workload changes and resource constraints.
