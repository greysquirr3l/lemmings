package manager

import (
	"errors"
	"time"
)

// Config defines the configuration parameters for the manager
type Config struct {
	// Worker configuration
	InitialWorkers  int
	MaxWorkers      int
	MinWorkers      int
	MaxTasksInQueue int

	// Scaling factors
	ScaleUpFactor        float64
	ScaleDownFactor      float64
	ScaleUpDelay         time.Duration
	MaxConsecutiveScales int
	MinSuccessfulBatches int

	// Resource monitoring
	LowMemoryMark      float64
	HighMemoryMark     float64
	ForceGCThreshold   float64
	MonitorInterval    time.Duration
	MinCleanupInterval time.Duration

	// Timeouts
	TaskTimeout time.Duration

	// System settings
	TargetCPUPercent    float64
	TargetMemoryPercent float64

	// Misc
	Verbose bool
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		InitialWorkers:       4,
		MaxWorkers:           32,
		MinWorkers:           1,
		MaxTasksInQueue:      1000,
		ScaleUpFactor:        1.5,
		ScaleDownFactor:      0.5,
		MonitorInterval:      5 * time.Second,
		TaskTimeout:          30 * time.Second,
		LowMemoryMark:        30.0, // below this percentage, scale up
		HighMemoryMark:       70.0, // above this percentage, scale down
		ForceGCThreshold:     80.0, // above this percentage, force GC
		ScaleUpDelay:         30 * time.Second,
		MaxConsecutiveScales: 3,
		MinSuccessfulBatches: 2,
		TargetCPUPercent:     70.0,
		TargetMemoryPercent:  60.0,
		MinCleanupInterval:   1 * time.Minute,
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if err := c.validateWorkerCounts(); err != nil {
		return err
	}

	if err := c.validateScalingFactors(); err != nil {
		return err
	}

	if err := c.validateMemorySettings(); err != nil {
		return err
	}

	return nil
}

// validateWorkerCounts validates the worker count settings
func (c *Config) validateWorkerCounts() error {
	if c.InitialWorkers <= 0 {
		return errors.New("initial workers must be greater than 0")
	}

	if c.MaxWorkers <= 0 {
		return errors.New("max workers must be greater than 0")
	}

	if c.MinWorkers <= 0 {
		return errors.New("min workers must be greater than 0")
	}

	if c.MinWorkers > c.MaxWorkers {
		return errors.New("min workers cannot be greater than max workers")
	}

	if c.InitialWorkers < c.MinWorkers || c.InitialWorkers > c.MaxWorkers {
		return errors.New("initial workers must be between min and max workers")
	}

	return nil
}

// validateScalingFactors validates the scaling factor settings
func (c *Config) validateScalingFactors() error {
	if c.ScaleUpFactor < 1.0 {
		return errors.New("scale up factor must be at least 1.0")
	}

	if c.ScaleDownFactor > 1.0 || c.ScaleDownFactor <= 0 {
		return errors.New("scale down factor must be between 0 and 1.0")
	}

	return nil
}

// validateMemorySettings validates the memory-related settings
func (c *Config) validateMemorySettings() error {
	if c.LowMemoryMark < 0 || c.LowMemoryMark > 100 {
		return errors.New("low memory mark must be between 0 and 100")
	}

	if c.HighMemoryMark < 0 || c.HighMemoryMark > 100 {
		return errors.New("high memory mark must be between 0 and 100")
	}

	if c.LowMemoryMark >= c.HighMemoryMark {
		return errors.New("low memory mark must be less than high memory mark")
	}

	return nil
}

// Option defines a configuration option function
type Option func(*Config)

// ConfigWithOptions creates a configuration with options
func ConfigWithOptions(opts ...Option) Config {
	cfg := DefaultConfig()

	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

// WithInitialWorkers sets the initial number of workers
func WithInitialWorkers(count int) Option {
	return func(c *Config) {
		c.InitialWorkers = count
	}
}

// WithMaxWorkers sets the maximum number of workers
func WithMaxWorkers(count int) Option {
	return func(c *Config) {
		c.MaxWorkers = count
	}
}

// WithMinWorkers sets the minimum number of workers
func WithMinWorkers(count int) Option {
	return func(c *Config) {
		c.MinWorkers = count
	}
}

// WithMaxTasksInQueue sets the maximum number of tasks in the queue
func WithMaxTasksInQueue(count int) Option {
	return func(c *Config) {
		c.MaxTasksInQueue = count
	}
}

// WithScaleFactors sets the scale up and down factors
func WithScaleFactors(up, down float64) Option {
	return func(c *Config) {
		c.ScaleUpFactor = up
		c.ScaleDownFactor = down
	}
}

// WithMemoryWatermarks sets the memory watermark percentages
func WithMemoryWatermarks(low, high float64) Option {
	return func(c *Config) {
		c.LowMemoryMark = low
		c.HighMemoryMark = high
	}
}

// WithMonitorInterval sets the resource monitoring interval
func WithMonitorInterval(d time.Duration) Option {
	return func(c *Config) {
		c.MonitorInterval = d
	}
}

// WithTaskTimeout sets the default task timeout
func WithTaskTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.TaskTimeout = d
	}
}

// WithScaleUpDelay sets the delay before scaling up
func WithScaleUpDelay(d time.Duration) Option {
	return func(c *Config) {
		c.ScaleUpDelay = d
	}
}

// WithForceGCThreshold sets the memory threshold for forced garbage collection
func WithForceGCThreshold(threshold float64) Option {
	return func(c *Config) {
		c.ForceGCThreshold = threshold
	}
}

// WithVerbose enables verbose logging
func WithVerbose(verbose bool) Option {
	return func(c *Config) {
		c.Verbose = verbose
	}
}
