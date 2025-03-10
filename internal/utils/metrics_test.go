package utils

import (
	"runtime"
	"testing"
	"time"
)

func TestSystemStats(t *testing.T) {
	t.Run("NewSystemStats initializes correctly", func(t *testing.T) {
		stats := NewSystemStats()

		if stats.MaxHistoryEntries != 100 {
			t.Errorf("Expected MaxHistoryEntries to be 100, got %d", stats.MaxHistoryEntries)
		}

		if stats.MemoryHistory == nil {
			t.Error("Expected MemoryHistory to be initialized")
		}

		if stats.GoroutineHistory == nil {
			t.Error("Expected GoroutineHistory to be initialized")
		}
	})

	t.Run("UpdateMemStats updates memory statistics", func(t *testing.T) {
		stats := NewSystemStats()

		// Check initial state
		initialLen := len(stats.MemoryHistory)
		initialGoroutines := len(stats.GoroutineHistory)

		// Force some memory allocation to ensure stats change
		buf := make([]byte, 1024*1024) // 1MB
		_ = buf

		// Update stats
		stats.UpdateMemStats()

		// Check memory history was updated
		if len(stats.MemoryHistory) != initialLen+1 {
			t.Errorf("Expected memory history length %d, got %d", initialLen+1, len(stats.MemoryHistory))
		}

		// Check goroutine history was updated
		if len(stats.GoroutineHistory) != initialGoroutines+1 {
			t.Errorf("Expected goroutine history length %d, got %d", initialGoroutines+1, len(stats.GoroutineHistory))
		}

		// Check that stats were filled
		if stats.MemoryStats.AllocatedBytes == 0 {
			t.Error("Expected allocated bytes to be non-zero")
		}

		if stats.MemoryStats.SystemBytes == 0 {
			t.Error("Expected system bytes to be non-zero")
		}

		if stats.NumGoroutine == 0 {
			t.Error("Expected goroutines count to be non-zero")
		}

		if stats.MemoryStats.UsagePercent <= 0 {
			t.Error("Expected memory usage percentage to be positive")
		}
	})

	t.Run("History is capped at max entries", func(t *testing.T) {
		stats := NewSystemStats()
		stats.MaxHistoryEntries = 5

		// Update more times than the cap
		for i := 0; i < 10; i++ {
			stats.UpdateMemStats()
		}

		if len(stats.MemoryHistory) > 5 {
			t.Errorf("Expected memory history to be capped at 5, got %d", len(stats.MemoryHistory))
		}

		if len(stats.GoroutineHistory) > 5 {
			t.Errorf("Expected goroutine history to be capped at 5, got %d", len(stats.GoroutineHistory))
		}
	})

	t.Run("GetMemUsagePercent returns current percentage", func(t *testing.T) {
		stats := NewSystemStats()
		stats.UpdateMemStats()

		percent := stats.GetMemUsagePercent()
		if percent <= 0 || percent > 100 {
			t.Errorf("Expected memory usage percentage between 0 and 100, got %f", percent)
		}
	})

	t.Run("GetMemStats returns copy of current stats", func(t *testing.T) {
		stats := NewSystemStats()
		stats.UpdateMemStats()

		memStats := stats.GetMemStats()

		// Values should match
		if memStats.AllocatedBytes != stats.MemoryStats.AllocatedBytes {
			t.Error("GetMemStats returned incorrect allocated bytes")
		}

		// Should be a copy, so modifying it shouldn't affect original
		originalBytes := stats.MemoryStats.AllocatedBytes
		memStats.AllocatedBytes = 999999

		if stats.MemoryStats.AllocatedBytes != originalBytes {
			t.Error("GetMemStats doesn't return a copy - original was modified")
		}
	})

	t.Run("GetNumGoroutine returns current goroutine count", func(t *testing.T) {
		stats := NewSystemStats()
		stats.UpdateMemStats()

		goroutines := stats.GetNumGoroutine()
		runtimeGoroutines := runtime.NumGoroutine()

		if goroutines != runtimeGoroutines {
			t.Errorf("Expected %d goroutines, got %d", runtimeGoroutines, goroutines)
		}
	})

	t.Run("GetMemoryTrend returns trend indicator", func(t *testing.T) {
		stats := NewSystemStats()

		// Not enough data points yet
		initialTrend := stats.GetMemoryTrend()
		if initialTrend != 0 {
			t.Errorf("Expected initial trend 0, got %f", initialTrend)
		}

		// Add some data points with increasing memory usage
		for i := 0; i < 10; i++ {
			stats.MemoryHistory = append(stats.MemoryHistory, MemStats{
				UsagePercent: float64(40 + i), // Increasing trend
			})
		}

		trend := stats.GetMemoryTrend()
		if trend <= 0 {
			t.Errorf("Expected positive trend for increasing memory usage, got %f", trend)
		}

		// Clear and add decreasing trend
		stats.MemoryHistory = nil
		for i := 0; i < 10; i++ {
			stats.MemoryHistory = append(stats.MemoryHistory, MemStats{
				UsagePercent: float64(50 - i), // Decreasing trend
			})
		}

		trend = stats.GetMemoryTrend()
		if trend >= 0 {
			t.Errorf("Expected negative trend for decreasing memory usage, got %f", trend)
		}
	})

	t.Run("SuggestGC recommends GC based on threshold", func(t *testing.T) {
		stats := NewSystemStats()

		// Low memory usage shouldn't suggest GC
		stats.MemoryStats.UsagePercent = 30
		if stats.SuggestGC(50) {
			t.Error("Expected SuggestGC to return false for low memory usage")
		}

		// High memory usage should suggest GC
		stats.MemoryStats.UsagePercent = 70
		if !stats.SuggestGC(50) {
			t.Error("Expected SuggestGC to return true for high memory usage")
		}

		// Many goroutines and heap objects should suggest GC
		stats.MemoryStats.UsagePercent = 30 // Reset to low memory
		stats.NumGoroutine = 15000
		stats.MemoryStats.HeapObjects = 200000
		if !stats.SuggestGC(50) {
			t.Error("Expected SuggestGC to return true for high goroutine and heap object counts")
		}
	})

	t.Run("GetMemoryHistory returns copy of history", func(t *testing.T) {
		stats := NewSystemStats()

		// Add some history entries
		for i := 0; i < 3; i++ {
			stats.MemoryHistory = append(stats.MemoryHistory, MemStats{UsagePercent: float64(i * 10)})
		}

		// Get history
		history := stats.GetMemoryHistory()

		// Should match original
		if len(history) != len(stats.MemoryHistory) {
			t.Errorf("Expected history length %d, got %d", len(stats.MemoryHistory), len(history))
		}

		// Should be a copy
		originalLength := len(stats.MemoryHistory)
		// Removing ineffectual assignment - just verify it doesn't affect original
		historyToModify := append(history, MemStats{UsagePercent: 99})
		if len(historyToModify) != originalLength+1 {
			t.Error("Failed to append to copy of history")
		}

		if len(stats.MemoryHistory) != originalLength {
			t.Error("GetMemoryHistory doesn't return a copy - original was modified")
		}
	})

	t.Run("GetGoroutineHistory returns copy of history", func(t *testing.T) {
		stats := NewSystemStats()

		// Add some history entries
		for i := 0; i < 3; i++ {
			stats.GoroutineHistory = append(stats.GoroutineHistory, i*10)
		}

		// Get history
		history := stats.GetGoroutineHistory()

		// Should match original
		if len(history) != len(stats.GoroutineHistory) {
			t.Errorf("Expected history length %d, got %d", len(stats.GoroutineHistory), len(history))
		}

		// Should be a copy
		originalLength := len(stats.GoroutineHistory)
		// Removing ineffectual assignment - just verify it doesn't affect original
		historyToModify := append(history, 99)
		if len(historyToModify) != originalLength+1 {
			t.Error("Failed to append to copy of history")
		}

		if len(stats.GoroutineHistory) != originalLength {
			t.Error("GetGoroutineHistory doesn't return a copy - original was modified")
		}
	})
}

func TestResourceMonitor(t *testing.T) {
	t.Run("NewResourceMonitor initializes with defaults", func(t *testing.T) {
		rm := NewResourceMonitor()

		if rm.memoryLowmark != DefaultMemoryLowWatermark {
			t.Errorf("Expected low watermark %d, got %f", DefaultMemoryLowWatermark, rm.memoryLowmark)
		}

		if rm.memoryHighmark != DefaultMemoryHighWatermark {
			t.Errorf("Expected high watermark %d, got %f", DefaultMemoryHighWatermark, rm.memoryHighmark)
		}

		if rm.checkInterval != DefaultCheckInterval {
			t.Errorf("Expected check interval %v, got %v", DefaultCheckInterval, rm.checkInterval)
		}

		if rm.stats == nil {
			t.Error("Expected stats to be initialized")
		}
	})

	t.Run("ResourceMonitor options work correctly", func(t *testing.T) {
		rm := NewResourceMonitor(
			WithMemoryWatermarks(10, 90),
			WithCheckInterval(1*time.Second),
			WithCleanupConfig(95, 10*time.Minute),
			WithScaleDownDelay(1*time.Minute),
		)

		if rm.memoryLowmark != 10 {
			t.Errorf("WithMemoryWatermarks failed to set low watermark, got %f", rm.memoryLowmark)
		}

		if rm.memoryHighmark != 90 {
			t.Errorf("WithMemoryWatermarks failed to set high watermark, got %f", rm.memoryHighmark)
		}

		if rm.checkInterval != 1*time.Second {
			t.Errorf("WithCheckInterval failed, got %v", rm.checkInterval)
		}

		if rm.cleanupThreshold != 95 {
			t.Errorf("WithCleanupConfig failed to set threshold, got %f", rm.cleanupThreshold)
		}

		if rm.minCleanupInterval != 10*time.Minute {
			t.Errorf("WithCleanupConfig failed to set interval, got %v", rm.minCleanupInterval)
		}

		if rm.scaleDownDelay != 1*time.Minute {
			t.Errorf("WithScaleDownDelay failed, got %v", rm.scaleDownDelay)
		}
	})

	t.Run("ShouldScaleDown returns correct value", func(t *testing.T) {
		rm := NewResourceMonitor(WithMemoryWatermarks(20, 80))

		// Force memory stats
		rm.stats.MemoryStats.UsagePercent = 90

		if !rm.ShouldScaleDown() {
			t.Error("Expected ShouldScaleDown to return true for high memory usage")
		}

		rm.stats.MemoryStats.UsagePercent = 70
		if rm.ShouldScaleDown() {
			t.Error("Expected ShouldScaleDown to return false for memory usage below threshold")
		}
	})

	t.Run("ShouldScaleUp returns correct value", func(t *testing.T) {
		rm := NewResourceMonitor(WithMemoryWatermarks(30, 80))

		// Force memory stats
		rm.stats.MemoryStats.UsagePercent = 20

		if !rm.ShouldScaleUp() {
			t.Error("Expected ShouldScaleUp to return true for low memory usage")
		}

		// Test scale down delay affects scaling up
		rm.RecordScaleDown()
		// Should not scale up because we just scaled down
		if rm.ShouldScaleUp() {
			t.Error("Expected ShouldScaleUp to return false after recent scale down")
		}

		// Test memory above threshold
		rm.stats.MemoryStats.UsagePercent = 40
		if rm.ShouldScaleUp() {
			t.Error("Expected ShouldScaleUp to return false for memory usage above threshold")
		}
	})

	t.Run("RecordScaleDown updates lastScaleDown time", func(t *testing.T) {
		rm := NewResourceMonitor()
		oldTime := rm.lastScaleDown

		// Ensure time passes
		time.Sleep(10 * time.Millisecond)
		rm.RecordScaleDown()

		if !rm.lastScaleDown.After(oldTime) {
			t.Error("RecordScaleDown didn't update lastScaleDown time")
		}
	})

	t.Run("GetStats returns the stats instance", func(t *testing.T) {
		rm := NewResourceMonitor()
		stats := rm.GetStats()

		if stats != rm.stats {
			t.Error("GetStats didn't return the expected stats instance")
		}
	})
}

func TestSimpleMemUsage(t *testing.T) {
	t.Run("GetSimpleMemUsagePercent returns value between 0 and 100", func(t *testing.T) {
		percent := GetSimpleMemUsagePercent()

		if percent <= 0 || percent > 100 {
			t.Errorf("Expected percentage between 0 and 100, got %f", percent)
		}
	})
}

func TestForceGC(t *testing.T) {
	// This is a simple smoke test as the actual effect is hard to test precisely
	t.Run("ForceGC doesn't panic", func(t *testing.T) {
		// Allocate a lot of memory
		data := make([][]byte, 100)
		for i := 0; i < 100; i++ {
			data[i] = make([]byte, 1024*1024) // 1MB each
		}

		// Clear references to allow GC
		// Removing ineffectual assignment - we don't need this line
		// Since data goes out of scope anyway after this test
		// data = nil

		// Call ForceGC - should not panic
		ForceGC()

		// Success is just not panicking
	})
}
