// internal/utils/resource.go

package utils

import (
	"context"
	"log"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

const (
	// Memory watermarks
	DefaultMemoryLowWatermark  = 60 // Percentage of memory usage that's considered safe
	DefaultMemoryHighWatermark = 75 // Percentage that triggers aggressive GC

	// Default interval between resource checks
	DefaultCheckInterval = 500 * time.Millisecond

	// Default scale factors
	DefaultScaleUpFactor   = 1.5 // Multiply current size by this when scaling up
	DefaultScaleDownFactor = 0.5 // Multiply current size by this when scaling down
)

// ResourceMonitor monitors system resources and provides scaling recommendations
type ResourceMonitor struct {
	sync.RWMutex
	stats              *SystemStats
	memoryLowmark      float64
	memoryHighmark     float64
	checkInterval      time.Duration
	lastCleanup        time.Time
	cleanupThreshold   float64
	minCleanupInterval time.Duration
	scaleDownDelay     time.Duration
	lastScaleDown      time.Time
	lastScaleUp        time.Time
	scaleUpCooldown    time.Duration
}

// NewResourceMonitor creates a new ResourceMonitor
func NewResourceMonitor(opts ...ResourceMonitorOption) *ResourceMonitor {
	rm := &ResourceMonitor{
		stats:              NewSystemStats(),
		memoryLowmark:      DefaultMemoryLowWatermark,
		memoryHighmark:     DefaultMemoryHighWatermark,
		checkInterval:      DefaultCheckInterval,
		lastCleanup:        time.Now(),
		cleanupThreshold:   85.0,
		minCleanupInterval: 5 * time.Minute,
		scaleDownDelay:     30 * time.Second,
		lastScaleDown:      time.Now(),
		lastScaleUp:        time.Now(),
		scaleUpCooldown:    30 * time.Second,
	}

	// Apply options
	for _, opt := range opts {
		opt(rm)
	}

	return rm
}

// ResourceMonitorOption configures a ResourceMonitor
type ResourceMonitorOption func(*ResourceMonitor)

// WithMemoryWatermarks sets the memory watermarks
func WithMemoryWatermarks(low, high float64) ResourceMonitorOption {
	return func(rm *ResourceMonitor) {
		rm.memoryLowmark = low
		rm.memoryHighmark = high
	}
}

// WithCheckInterval sets the resource check interval
func WithCheckInterval(interval time.Duration) ResourceMonitorOption {
	return func(rm *ResourceMonitor) {
		rm.checkInterval = interval
	}
}

// WithCleanupConfig sets the cleanup configuration
func WithCleanupConfig(threshold float64, minInterval time.Duration) ResourceMonitorOption {
	return func(rm *ResourceMonitor) {
		rm.cleanupThreshold = threshold
		rm.minCleanupInterval = minInterval
	}
}

// WithScaleDownDelay sets the delay before scaling up after a scale down
func WithScaleDownDelay(delay time.Duration) ResourceMonitorOption {
	return func(rm *ResourceMonitor) {
		rm.scaleDownDelay = delay
	}
}

// Start begins monitoring system resources
func (rm *ResourceMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(rm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rm.stats.UpdateMemStats()
			rm.monitorMemory()
		}
	}
}

// monitorMemory monitors memory usage and performs cleanup if needed
func (rm *ResourceMonitor) monitorMemory() {
	memUsage := rm.stats.GetMemUsagePercent()

	if memUsage > rm.cleanupThreshold &&
		time.Since(rm.lastCleanup) > rm.minCleanupInterval {
		log.Printf("Memory usage high (%.1f%%), performing cleanup", memUsage)
		rm.cleanupMemory()
		rm.Lock()
		rm.lastCleanup = time.Now()
		rm.Unlock()
	}
}

// cleanupMemory performs memory cleanup
func (rm *ResourceMonitor) cleanupMemory() {
	// Force GC
	runtime.GC()
	debug.FreeOSMemory()
}

// ShouldScaleDown returns true if resources indicate scaling down is needed
func (rm *ResourceMonitor) ShouldScaleDown() bool {
	rm.RLock()
	defer rm.RUnlock()

	return rm.stats.GetMemUsagePercent() > rm.memoryHighmark
}

// ShouldScaleUp returns true if resources indicate scaling up is possible
func (rm *ResourceMonitor) ShouldScaleUp() bool {
	rm.RLock()
	defer rm.RUnlock()

	// First check memory condition - memory must be below low watermark
	memUsage := rm.stats.GetMemUsagePercent()
	if memUsage >= rm.memoryLowmark {
		return false
	}

	// Check if we're in a cooldown period after recent scaling events
	if time.Since(rm.lastScaleDown) < rm.scaleDownDelay {
		return false
	}

	if time.Since(rm.lastScaleUp) < rm.scaleUpCooldown {
		return false
	}

	// All conditions passed, scaling up is possible
	return true
}

// RecordScaleUp records that a scale-up operation occurred
func (rm *ResourceMonitor) RecordScaleUp() {
	rm.Lock()
	defer rm.Unlock()
	rm.lastScaleUp = time.Now()
}

// RecordScaleDown records that a scale down has occurred
func (rm *ResourceMonitor) RecordScaleDown() {
	rm.Lock()
	defer rm.Unlock()
	rm.lastScaleDown = time.Now()
}

// GetStats returns the current system statistics
func (rm *ResourceMonitor) GetStats() *SystemStats {
	return rm.stats
}
