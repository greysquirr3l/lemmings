package manager

import "time"

// Config defines configuration options for the Manager
type Config struct {
	// Initial and max number of workers
	InitialWorkers int
	MaxWorkers     int
	MinWorkers     int

	// Resources
	TargetCPUPercent    float64
	TargetMemoryPercent float64
	LowMemoryMark       float64
	HighMemoryMark      float64

	// Scaling
	ScaleUpFactor        float64
	ScaleDownFactor      float64
	ScaleUpDelay         time.Duration
	MinSuccessfulBatches int
	MaxConsecutiveScales int

	// Monitoring
	MonitorInterval  time.Duration
	DebugLogInterval time.Duration
	StuckTaskTimeout time.Duration
	ProgressInterval time.Duration

	// Memory cleanup
	ForceGCThreshold   float64
	MinCleanupInterval time.Duration

	// Task handling
	MaxTaskRetries     int
	TaskTimeout        time.Duration
	MaxTasksInQueue    int
	MaxConcurrentTasks int

	// Debugging
	Verbose bool
}

// DefaultConfig returns a Config with reasonable defaults
func DefaultConfig() Config {
	return Config{
		InitialWorkers:       10,
		MaxWorkers:           100,
		MinWorkers:           1,
		TargetCPUPercent:     70,
		TargetMemoryPercent:  70,
		LowMemoryMark:        60,
		HighMemoryMark:       75,
		ScaleUpFactor:        1.5,
		ScaleDownFactor:      0.5,
		ScaleUpDelay:         30 * time.Second,
		MinSuccessfulBatches: 5,
		MaxConsecutiveScales: 3,
		MonitorInterval:      500 * time.Millisecond,
		DebugLogInterval:     30 * time.Second,
		StuckTaskTimeout:     5 * time.Minute,
		ProgressInterval:     10 * time.Second,
		ForceGCThreshold:     85,
		MinCleanupInterval:   5 * time.Minute,
		MaxTaskRetries:       3,
		TaskTimeout:          2 * time.Minute,
		MaxTasksInQueue:      10000,
		MaxConcurrentTasks:   0, // 0 means no limit
		Verbose:              false,
	}
}
