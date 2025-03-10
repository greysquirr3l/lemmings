package manager

import (
	"testing"
	"time"

	"github.com/greysquirr3l/lemmings/internal/utils"
)

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
		rc.AdjustWorkerCount(2, "test")
		if rc.activeWorkers != 5 {
			t.Errorf("Expected worker count to be capped at min (5), got %d", rc.activeWorkers)
		}

		// Try to adjust above max
		rc.AdjustWorkerCount(15, "test")
		if rc.activeWorkers != 10 {
			t.Errorf("Expected worker count to be capped at max (10), got %d", rc.activeWorkers)
		}
	})

	t.Run("AdjustWorkerCount adds scale-up signals to channel", func(t *testing.T) {
		config := DefaultConfig()
		config.InitialWorkers = 3
		config.MaxWorkers = 10

		rc := NewResourceController(config)

		// Scale up by 2
		rc.AdjustWorkerCount(5, "scale up test")

		// Check that 2 scale-up signals were sent
		for i := 0; i < 2; i++ {
			select {
			case signal := <-rc.workerControl:
				if signal != 1 {
					t.Errorf("Expected scale-up signal 1, got %d", signal)
				}
			case <-time.After(100 * time.Millisecond):
				t.Error("Timed out waiting for scale-up signal")
			}
		}

		// Channel should be empty now
		select {
		case signal := <-rc.workerControl:
			t.Errorf("Unexpected signal: %d", signal)
		case <-time.After(50 * time.Millisecond):
			// Expected timeout - no more signals
		}
	})

	t.Run("AdjustWorkerCount adds scale-down signals to channel", func(t *testing.T) {
		config := DefaultConfig()
		config.InitialWorkers = 5
		config.MaxWorkers = 10
		config.MinWorkers = 1

		rc := NewResourceController(config)

		// Scale down by 2
		rc.AdjustWorkerCount(3, "scale down test")

		// Check that 2 scale-down signals were sent
		for i := 0; i < 2; i++ {
			select {
			case signal := <-rc.workerControl:
				if signal != -1 {
					t.Errorf("Expected scale-down signal -1, got %d", signal)
				}
			case <-time.After(100 * time.Millisecond):
				t.Error("Timed out waiting for scale-down signal")
			}
		}

		// Channel should be empty now
		select {
		case signal := <-rc.workerControl:
			t.Errorf("Unexpected signal: %d", signal)
		case <-time.After(50 * time.Millisecond):
			// Expected timeout - no more signals
		}
	})

	t.Run("AdjustWorkerCount records scaling events", func(t *testing.T) {
		config := DefaultConfig()
		config.InitialWorkers = 4
		config.MaxWorkers = 10

		rc := NewResourceController(config)

		initialEventCount := len(rc.scaleHistory)
		rc.AdjustWorkerCount(6, "test scaling")

		if len(rc.scaleHistory) != initialEventCount+1 {
			t.Errorf("Expected 1 new scaling event, got %d", len(rc.scaleHistory)-initialEventCount)
		}

		event := rc.scaleHistory[len(rc.scaleHistory)-1]
		if event.FromWorkers != 4 || event.ToWorkers != 6 {
			t.Errorf("Expected scaling from 4 to 6, got %d to %d",
				event.FromWorkers, event.ToWorkers)
		}

		if event.Reason != "test scaling" {
			t.Errorf("Expected reason 'test scaling', got '%s'", event.Reason)
		}
	})

	t.Run("AdjustWorkerCount handles channel full condition", func(t *testing.T) {
		config := DefaultConfig()
		config.InitialWorkers = 3
		config.MaxWorkers = 10

		rc := NewResourceController(config)

		// Fill the channel
		channelCapacity := cap(rc.workerControl)
		for i := 0; i < channelCapacity; i++ {
			rc.workerControl <- 1
		}

		// Try to adjust worker count - should handle full channel gracefully
		rc.AdjustWorkerCount(5, "should handle full channel")

		// The adjustment should happen anyway, even if signals couldn't be sent
		if rc.activeWorkers != 5 {
			t.Errorf("Expected worker count to be adjusted to 5 despite full channel, got %d", rc.activeWorkers)
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

		// Mock monitor to suggest scaling
		mockMonitor := utils.NewResourceMonitor()
		mockMonitor.GetStats().MemoryStats.UsagePercent = 90 // High memory - should scale down
		rc.monitor = mockMonitor

		// Try to adjust
		rc.AdjustBasedOnResourceUsage()

		// No change should occur
		if rc.activeWorkers != 5 {
			t.Errorf("Expected no change when scaling disabled, got %d", rc.activeWorkers)
		}
	})

	t.Run("AdjustBasedOnResourceUsage scales down on high memory", func(t *testing.T) {
		config := DefaultConfig()
		config.InitialWorkers = 10
		config.MaxWorkers = 20
		config.MinWorkers = 2
		config.ScaleDownFactor = 0.5
		rc := NewResourceController(config)

		// Mock monitor to suggest scaling down
		mockMonitor := utils.NewResourceMonitor()
		mockMonitor.GetStats().MemoryStats.UsagePercent = 90 // High memory - should scale down
		rc.monitor = mockMonitor

		// Empty the worker control channel
		for len(rc.workerControl) > 0 {
			<-rc.workerControl
		}

		// Adjust based on resource usage
		rc.AdjustBasedOnResourceUsage()

		// Should scale down by factor (10 * 0.5 = 5)
		expectedWorkers := 5
		if rc.activeWorkers != expectedWorkers {
			t.Errorf("Expected to scale down to %d workers, got %d", expectedWorkers, rc.activeWorkers)
		}

		// Check signals were sent
		signalCount := 0
		for len(rc.workerControl) > 0 {
			signal := <-rc.workerControl
			if signal != -1 {
				t.Errorf("Expected scale-down signal -1, got %d", signal)
			}
			signalCount++
		}

		if signalCount != 5 {
			t.Errorf("Expected 5 scale-down signals, got %d", signalCount)
		}
	})

	t.Run("AdjustBasedOnResourceUsage scales up on low memory", func(t *testing.T) {
		config := DefaultConfig()
		config.InitialWorkers = 5
		config.MaxWorkers = 20
		config.MinWorkers = 2
		config.ScaleUpFactor = 2.0
		config.MinSuccessfulBatches = 1
		config.ScaleUpDelay = 0 // No delay for testing
		rc := NewResourceController(config)

		// Mock monitor to suggest scaling up
		mockMonitor := utils.NewResourceMonitor()
		mockMonitor.GetStats().MemoryStats.UsagePercent = 10 // Low memory - should scale up
		rc.monitor = mockMonitor

		// Empty the worker control channel
		for len(rc.workerControl) > 0 {
			<-rc.workerControl
		}

		// Adjust based on resource usage
		rc.AdjustBasedOnResourceUsage()

		// Should scale up by factor (5 * 2.0 = 10)
		expectedWorkers := 10
		if rc.activeWorkers != expectedWorkers {
			t.Errorf("Expected to scale up to %d workers, got %d", expectedWorkers, rc.activeWorkers)
		}

		// Check signals were sent
		signalCount := 0
		for len(rc.workerControl) > 0 {
			signal := <-rc.workerControl
			if signal != 1 {
				t.Errorf("Expected scale-up signal 1, got %d", signal)
			}
			signalCount++
		}

		if signalCount != 5 {
			t.Errorf("Expected 5 scale-up signals, got %d", signalCount)
		}
	})

	t.Run("Pause processing on critically high memory", func(t *testing.T) {
		config := DefaultConfig()
		config.HighMemoryMark = 80
		rc := NewResourceController(config)

		// Mock monitor with critically high memory (>90%)
		mockMonitor := utils.NewResourceMonitor()
		mockMonitor.GetStats().MemoryStats.UsagePercent = 95
		rc.monitor = mockMonitor

		// Empty the pause channel
		select {
		case <-rc.pauseProcessing:
		default:
		}

		// Adjust based on resource usage
		rc.AdjustBasedOnResourceUsage()

		// Check if pause signal was sent
		select {
		case pause := <-rc.pauseProcessing:
			if !pause {
				t.Error("Expected pause signal to be true")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("No pause signal sent for critically high memory")
		}
	})

	t.Run("Consecutive scale up limitations", func(t *testing.T) {
		config := DefaultConfig()
		config.InitialWorkers = 2
		config.MaxWorkers = 20
		config.ScaleUpFactor = 2.0
		config.MinSuccessfulBatches = 1
		config.MaxConsecutiveScales = 2
		config.ScaleUpDelay = 0 // No delay for testing
		rc := NewResourceController(config)

		// Mock monitor to suggest scaling up
		mockMonitor := utils.NewResourceMonitor()
		mockMonitor.GetStats().MemoryStats.UsagePercent = 10 // Low memory - should scale up
		rc.monitor = mockMonitor

		// First scale-up: 2 -> 4
		rc.AdjustBasedOnResourceUsage()
		if rc.activeWorkers != 4 {
			t.Errorf("Expected first scale-up to 4 workers, got %d", rc.activeWorkers)
		}

		// Second scale-up: 4 -> 8
		rc.AdjustBasedOnResourceUsage()
		if rc.activeWorkers != 8 {
			t.Errorf("Expected second scale-up to 8 workers, got %d", rc.activeWorkers)
		}

		// Third scale-up: should be limited by MaxConsecutiveScales
		rc.AdjustBasedOnResourceUsage()
		if rc.activeWorkers != 8 {
			t.Errorf("Expected no scale-up due to MaxConsecutiveScales, got %d", rc.activeWorkers)
		}
	})
}

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

		// Add a scaling event
		rc.AdjustWorkerCount(rc.activeWorkers+1, "test")

		// Get the history
		history := rc.GetScalingHistory()

		// Check we have at least one event
		if len(history) == 0 {
			t.Error("Expected non-empty scaling history")
		}

		// Should be a copy - modifying shouldn't affect original
		originalLength := len(history)
		modifiedHistory := append(history, ScaleEvent{})

		if len(modifiedHistory) != originalLength+1 {
			t.Error("Failed to append to history copy")
		}

		if len(rc.scaleHistory) != originalLength {
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
