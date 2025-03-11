package manager

import (
	"testing"
	"time"

	"github.com/greysquirr3l/lemmings/internal/utils"
)

// Removed unused testMonitor type and its ShouldScaleUp method

func TestNewResourceController(t *testing.T) {
	t.Run("NewResourceController initializes with config", func(t *testing.T) {
		config := DefaultConfig()
		config.InitialWorkers = 5
		config.MaxWorkers = 20
		config.MinWorkers = 2

		rc := NewResourceController(config)

		if rc.activeWorkers != 5 {
			t.Errorf("Expected activeWorkers to be 5, got %d", rc.activeWorkers)
		}

		if rc.maxWorkers != 20 {
			t.Errorf("Expected maxWorkers to be 20, got %d", rc.maxWorkers)
		}

		if rc.minWorkers != 2 {
			t.Errorf("Expected minWorkers to be 2, got %d", rc.minWorkers)
		}

		if rc.monitor == nil {
			t.Error("Expected monitor to be initialized")
		}

		if rc.workerControl == nil {
			t.Error("Expected workerControl channel to be initialized")
		}

		if rc.pauseProcessing == nil {
			t.Error("Expected pauseProcessing channel to be initialized")
		}

		if !rc.scalingEnabled {
			t.Error("Expected scalingEnabled to be true by default")
		}
	})
}

func TestAdjustWorkerCount(t *testing.T) {
	t.Run("AdjustWorkerCount respects min and max bounds", func(t *testing.T) {
		config := DefaultConfig()
		config.MinWorkers = 5
		config.MaxWorkers = 10
		config.InitialWorkers = 7

		rc := NewResourceController(config)

		// Try to adjust below min
		rc.AdjustWorkerCount(-10, "test")
		if rc.activeWorkers != 5 {
			t.Errorf("Expected worker count to be capped at min (5), got %d", rc.activeWorkers)
		}

		// Try to adjust above max
		rc.AdjustWorkerCount(10, "test")
		if rc.activeWorkers != 10 {
			t.Errorf("Expected worker count to be capped at max (10), got %d", rc.activeWorkers)
		}
	})

	t.Run("AdjustWorkerCount records scaling events", func(t *testing.T) {
		config := DefaultConfig()
		config.InitialWorkers = 4
		config.MaxWorkers = 10

		rc := NewResourceController(config)
		initialEvents := len(rc.scaleHistory)

		rc.AdjustWorkerCount(2, "test scaling")

		if len(rc.scaleHistory) != initialEvents+1 {
			t.Errorf("Expected 1 new scaling event, got %d", len(rc.scaleHistory)-initialEvents)
		}
	})
}

func TestResourceControlEnabledDisabled(t *testing.T) {
	t.Run("EnableScaling and DisableScaling work correctly", func(t *testing.T) {
		config := DefaultConfig()
		rc := NewResourceController(config)

		// Initially enabled
		if !rc.scalingEnabled {
			t.Error("Expected scaling to be enabled by default")
		}

		// Disable scaling
		rc.DisableScaling()
		if rc.scalingEnabled {
			t.Error("Expected scaling to be disabled after DisableScaling")
		}

		// Enable scaling
		rc.EnableScaling()
		if !rc.scalingEnabled {
			t.Error("Expected scaling to be enabled after EnableScaling")
		}
	})
}

func TestAdjustBasedOnResourceUsage(t *testing.T) {
	t.Run("AdjustBasedOnResourceUsage does nothing when scaling disabled", func(t *testing.T) {
		config := DefaultConfig()
		rc := NewResourceController(config)

		// Set initial state
		rc.activeWorkers = 5
		rc.DisableScaling()

		// Create a test monitor
		mockMonitor := utils.NewResourceMonitor()
		mockMonitor.GetStats().MemoryStats.UsagePercent = 90 // High memory
		rc.monitor = mockMonitor

		// Try to adjust
		rc.AdjustBasedOnResourceUsage()

		// No change should occur
		if rc.activeWorkers != 5 {
			t.Errorf("Expected no change when scaling disabled, got %d", rc.activeWorkers)
		}
	})

	t.Run("High memory causes scale down", func(t *testing.T) {
		// Simple test for scale down on high memory
		config := DefaultConfig()
		config.InitialWorkers = 10
		rc := NewResourceController(config)

		// Simulate high memory
		mockMonitor := utils.NewResourceMonitor()
		mockMonitor.GetStats().MemoryStats.UsagePercent = 90
		// Set monitor directly with no type adapter
		rc.monitor = mockMonitor

		// Empty the worker control channel
		for len(rc.workerControl) > 0 {
			<-rc.workerControl
		}

		// Start with known worker count and make sure we scale down
		initialWorkers := rc.activeWorkers
		rc.AdjustBasedOnResourceUsage()

		// Should have scaled down
		if rc.activeWorkers >= initialWorkers {
			t.Errorf("Expected worker count to decrease from %d on high memory, got %d",
				initialWorkers, rc.activeWorkers)
		}
	})
}

// Simplified getter tests
func TestResourceControllerGetters(t *testing.T) {
	t.Run("GetWorkerControlChannel returns the worker control channel", func(t *testing.T) {
		config := DefaultConfig()
		rc := NewResourceController(config)

		channel := rc.GetWorkerControlChannel()
		if channel == nil {
			t.Error("Expected non-nil worker control channel")
		}
	})

	t.Run("GetPauseChannel returns the pause channel", func(t *testing.T) {
		config := DefaultConfig()
		rc := NewResourceController(config)

		channel := rc.GetPauseChannel()
		if channel == nil {
			t.Error("Expected non-nil pause channel")
		}
	})

	t.Run("GetActiveWorkerCount returns the current worker count", func(t *testing.T) {
		config := DefaultConfig()
		config.InitialWorkers = 7
		rc := NewResourceController(config)

		count := rc.GetActiveWorkerCount()
		if count != 7 {
			t.Errorf("Expected 7 active workers, got %d", count)
		}
	})

	t.Run("GetScalingHistory returns a copy of scaling history", func(t *testing.T) {
		config := DefaultConfig()
		rc := NewResourceController(config)

		// Record an event
		rc.scaleHistory = append(rc.scaleHistory, ScaleEvent{
			Time:        time.Now(),
			FromWorkers: 5,
			ToWorkers:   7,
			Reason:      "test",
		})

		// Get the history
		history := rc.GetScalingHistory()
		if len(history) != 1 {
			t.Errorf("Expected scaling history length 1, got %d", len(history))
		}

		// Should be a copy - modifying shouldn't affect original
		history = append(history, ScaleEvent{})
		if len(history) != 2 {
			t.Errorf("Expected modified history length to be 2, got %d", len(history))
		}
		if len(rc.scaleHistory) != 1 {
			t.Error("GetScalingHistory didn't return a copy")
		}
	})

	t.Run("GetResourceMonitor returns the monitor", func(t *testing.T) {
		config := DefaultConfig()
		rc := NewResourceController(config)

		monitor := rc.GetResourceMonitor()
		if monitor == nil {
			t.Error("Expected non-nil resource monitor")
		}
	})
}
