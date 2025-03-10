// Package manager provides worker management functionality.
package manager

import (
	"log"
	"sync"
	"time"

	"github.com/greysquirr3l/lemmings/internal/utils"
)

// ResourceController handles dynamic scaling of workers based on resource usage.
// It monitors system resources like memory and adjusts worker count accordingly.
type ResourceController struct {
	sync.Mutex
	monitor         *utils.ResourceMonitor
	activeWorkers   int
	maxWorkers      int
	minWorkers      int
	targetWorkers   int
	pauseProcessing chan bool
	workerControl   chan int
	config          Config

	// Scaling metrics
	successfulBatches   int
	consecutiveScaleUps int
	lastScaleAction     time.Time
	scaleHistory        []ScaleEvent
	scalingEnabled      bool
}

// ScaleEvent represents a scaling action taken by the ResourceController.
// It records details about when and why scaling occurred.
type ScaleEvent struct {
	Time        time.Time // When the scaling occurred
	FromWorkers int       // Previous worker count
	ToWorkers   int       // New worker count
	Reason      string    // Reason for scaling
	MemUsage    float64   // Memory usage percentage at time of scaling
}

// NewResourceController creates a new ResourceController with the specified configuration.
// Returns an initialized controller that can adjust worker count based on resource usage.
func NewResourceController(config Config) *ResourceController {
	monitorOpts := []utils.ResourceMonitorOption{
		utils.WithMemoryWatermarks(config.LowMemoryMark, config.HighMemoryMark),
		utils.WithCheckInterval(config.MonitorInterval),
		utils.WithCleanupConfig(config.ForceGCThreshold, config.MinCleanupInterval),
		utils.WithScaleDownDelay(config.ScaleUpDelay),
	}

	return &ResourceController{
		monitor:         utils.NewResourceMonitor(monitorOpts...),
		activeWorkers:   config.InitialWorkers,
		targetWorkers:   config.InitialWorkers,
		maxWorkers:      config.MaxWorkers,
		minWorkers:      config.MinWorkers,
		pauseProcessing: make(chan bool, 1),
		workerControl:   make(chan int, config.MaxWorkers),
		config:          config,
		lastScaleAction: time.Now(),
		scaleHistory:    make([]ScaleEvent, 0, 100),
		scalingEnabled:  true,
	}
}

// AdjustWorkerCount changes the number of active workers.
// This method ensures the new count is between minWorkers and maxWorkers,
// sends appropriate signals to the worker pool, and records the scaling event.
//
// The reason parameter provides a description of why scaling is occurring.
func (rc *ResourceController) AdjustWorkerCount(newCount int, reason string) {
	rc.Lock()
	defer rc.Unlock()

	if newCount < rc.minWorkers {
		newCount = rc.minWorkers
	}
	if newCount > rc.maxWorkers {
		newCount = rc.maxWorkers
	}
	if newCount == rc.activeWorkers {
		return
	}

	delta := newCount - rc.activeWorkers
	if delta > 0 {
		// Adding workers
		for i := 0; i < delta; i++ {
			select {
			case rc.workerControl <- 1:
				// Signal sent successfully
			default:
				log.Printf("Warning: Worker control channel full, scale-up incomplete")
				// Adjust our count to reflect reality - only scale up by i workers
				newCount = rc.activeWorkers + i

				// Record the scaling event before returning
				memUsage := rc.monitor.GetStats().GetMemUsagePercent()
				event := ScaleEvent{
					Time:        time.Now(),
					FromWorkers: rc.activeWorkers,
					ToWorkers:   newCount,
					Reason:      reason + " (partial)",
					MemUsage:    memUsage,
				}
				rc.scaleHistory = append(rc.scaleHistory, event)
				rc.activeWorkers = newCount
				rc.lastScaleAction = time.Now()
				log.Printf("Adjusted worker count from %d to %d (%s, memory: %.1f%%)",
					event.FromWorkers, event.ToWorkers, reason, memUsage)
				return
			}
		}
	} else {
		// Removing workers
		for i := 0; i < -delta; i++ {
			select {
			case rc.workerControl <- -1:
				// Signal sent successfully
			default:
				log.Printf("Warning: Worker control channel full, scale-down incomplete")
				// Adjust our count to reflect reality - only scale down by i workers
				newCount = rc.activeWorkers - i

				// Record the scaling event before returning
				memUsage := rc.monitor.GetStats().GetMemUsagePercent()
				event := ScaleEvent{
					Time:        time.Now(),
					FromWorkers: rc.activeWorkers,
					ToWorkers:   newCount,
					Reason:      reason + " (partial)",
					MemUsage:    memUsage,
				}
				rc.scaleHistory = append(rc.scaleHistory, event)
				rc.activeWorkers = newCount
				rc.lastScaleAction = time.Now()
				log.Printf("Adjusted worker count from %d to %d (%s, memory: %.1f%%)",
					event.FromWorkers, event.ToWorkers, reason, memUsage)
				return
			}
		}
		rc.monitor.RecordScaleDown()
	}

	// Record the scaling event
	memUsage := rc.monitor.GetStats().GetMemUsagePercent()
	event := ScaleEvent{
		Time:        time.Now(),
		FromWorkers: rc.activeWorkers,
		ToWorkers:   newCount,
		Reason:      reason,
		MemUsage:    memUsage,
	}
	rc.scaleHistory = append(rc.scaleHistory, event)

	// Trim history if it gets too large
	if len(rc.scaleHistory) > 100 {
		rc.scaleHistory = rc.scaleHistory[len(rc.scaleHistory)-100:]
	}

	rc.activeWorkers = newCount
	rc.lastScaleAction = time.Now()
	log.Printf("Adjusted worker count from %d to %d (%s, memory: %.1f%%)",
		event.FromWorkers, event.ToWorkers, reason, memUsage)
}

// AdjustBasedOnResourceUsage dynamically adjusts workers based on resource usage
func (rc *ResourceController) AdjustBasedOnResourceUsage() {
	if !rc.scalingEnabled {
		return
	}

	// Check if memory usage is too high
	if rc.monitor.ShouldScaleDown() {
		// Scale down by the configured factor
		newWorkers := int(float64(rc.activeWorkers) * rc.config.ScaleDownFactor)
		rc.AdjustWorkerCount(newWorkers, "high memory usage")

		// Pause processing if memory is critically high
		if rc.monitor.GetStats().GetMemUsagePercent() > rc.config.HighMemoryMark+10 {
			select {
			case rc.pauseProcessing <- true:
				log.Printf("Pausing processing due to critically high memory usage (%.1f%%)",
					rc.monitor.GetStats().GetMemUsagePercent())
			default:
				// Already paused
			}
		}
		return
	}

	// Check if memory usage is low enough to scale up
	if rc.monitor.ShouldScaleUp() {
		// Resume processing if previously paused
		select {
		case <-rc.pauseProcessing:
			log.Printf("Resuming processing, memory usage now %.1f%%",
				rc.monitor.GetStats().GetMemUsagePercent())
		default:
			// Already running
		}

		// Increase successful batches count
		rc.successfulBatches++

		// Scale up if we've had enough successful batches
		if rc.successfulBatches >= rc.config.MinSuccessfulBatches &&
			time.Since(rc.lastScaleAction) >= rc.config.ScaleUpDelay {

			if rc.consecutiveScaleUps < rc.config.MaxConsecutiveScales {
				// Scale up by the configured factor
				newWorkers := int(float64(rc.activeWorkers) * rc.config.ScaleUpFactor)
				rc.AdjustWorkerCount(newWorkers, "low memory usage")
				rc.consecutiveScaleUps++
				rc.successfulBatches = 0
			} else {
				// Reset consecutive scale-ups after hitting max
				rc.consecutiveScaleUps = 0
				rc.successfulBatches = 0
			}
		}
	} else {
		// Reset consecutive scale-ups if memory is in the normal range
		rc.consecutiveScaleUps = 0
	}
}

// GetWorkerControlChannel returns the channel used to control workers
func (rc *ResourceController) GetWorkerControlChannel() <-chan int {
	return rc.workerControl
}

// GetPauseChannel returns the channel used to pause processing
func (rc *ResourceController) GetPauseChannel() <-chan bool {
	return rc.pauseProcessing
}

// GetActiveWorkerCount returns the current number of active workers
func (rc *ResourceController) GetActiveWorkerCount() int {
	rc.Lock()
	defer rc.Unlock()
	return rc.activeWorkers
}

// EnableScaling turns on automatic scaling
func (rc *ResourceController) EnableScaling() {
	rc.Lock()
	defer rc.Unlock()
	rc.scalingEnabled = true
}

// DisableScaling turns off automatic scaling
func (rc *ResourceController) DisableScaling() {
	rc.Lock()
	defer rc.Unlock()
	rc.scalingEnabled = false
}

// GetScalingHistory returns a copy of the scaling history
func (rc *ResourceController) GetScalingHistory() []ScaleEvent {
	rc.Lock()
	defer rc.Unlock()
	history := make([]ScaleEvent, len(rc.scaleHistory))
	copy(history, rc.scaleHistory)
	return history
}

// GetResourceMonitor returns the underlying resource monitor
func (rc *ResourceController) GetResourceMonitor() *utils.ResourceMonitor {
	return rc.monitor
}
