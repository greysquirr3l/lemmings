// Package utils provides internal utility functions and types for resource monitoring.
package utils

import (
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// MemStats represents memory statistics collected from the Go runtime.
// It includes detailed memory usage metrics and garbage collection information.
type MemStats struct {
	UsagePercent     float64   // Percentage of system memory in use (0-100)
	AllocatedBytes   uint64    // Bytes allocated and still in use
	SystemBytes      uint64    // Total bytes obtained from system
	GCPauseTimeNs    uint64    // Nanoseconds spent in GC stop-the-world pauses
	LastGCTime       time.Time // Time of the last garbage collection
	NumGC            uint32    // Number of completed GC cycles
	LastCollectionMs float64   // Milliseconds spent in last collection
	HeapObjects      uint64    // Number of allocated heap objects
	HeapInUseBytes   uint64    // Bytes in use by the heap
	StackInUseBytes  uint64    // Bytes in use by the stack
	MSpanInUseBytes  uint64    // Bytes of allocated mspan structures
	MCacheInUseBytes uint64    // Bytes of allocated mcache structures
	BuckHashSysBytes uint64    // Bytes of profiling bucket hash table
	GCSysBytes       uint64    // Bytes of garbage collection system metadata
	OtherSysBytes    uint64    // Bytes of other system allocations
	NextGCSizeBytes  uint64    // Size target for next GC cycle
	LastSampleTime   time.Time // Time when these stats were sampled
	TotalAllocBytes  uint64    // Total bytes allocated (including freed)
	TotalAllocsCount uint64    // Total count of allocations
	TotalFreesCount  uint64    // Total count of frees
	PauseTotalNs     uint64    // Total nanoseconds spent in GC stop-the-world pauses
}

// SystemStats tracks system resource usage including memory and goroutine counts.
// It maintains historical data for trend analysis.
type SystemStats struct {
	mu           sync.RWMutex
	CPUUsage     float64
	MemoryStats  MemStats
	NumGoroutine int
	LastUpdate   time.Time

	// Historical data
	MemoryHistory     []MemStats
	GoroutineHistory  []int
	MaxHistoryEntries int
}

// NewSystemStats creates a new SystemStats instance with initialized fields.
// Returns a pointer to the created SystemStats.
func NewSystemStats() *SystemStats {
	return &SystemStats{
		LastUpdate:        time.Now(),
		MaxHistoryEntries: 100,
		MemoryHistory:     make([]MemStats, 0, 100),
		GoroutineHistory:  make([]int, 0, 100),
	}
}

// UpdateMemStats updates memory statistics
func (s *SystemStats) UpdateMemStats() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	// Calculate memory usage as a percentage
	usagePercent := float64(ms.Alloc) / float64(ms.Sys) * 100

	// Fix: Convert ms.LastGC to int64 safely
	var lastGCTime time.Time
	if ms.LastGC > 0 {
		// Check bounds before conversion to avoid overflow
		if ms.LastGC <= uint64(1<<63-1) {
			lastGCTime = time.Unix(0, int64(ms.LastGC))
		} else {
			// Use a fallback value if the value is too large
			lastGCTime = time.Now()
		}
	}

	// Create current memory stats snapshot
	currentStats := MemStats{
		UsagePercent:     usagePercent,
		AllocatedBytes:   ms.Alloc,
		SystemBytes:      ms.Sys,
		GCPauseTimeNs:    ms.PauseNs[(ms.NumGC+255)%256],
		LastGCTime:       lastGCTime,
		NumGC:            ms.NumGC,
		HeapObjects:      ms.HeapObjects,
		HeapInUseBytes:   ms.HeapInuse,
		StackInUseBytes:  ms.StackInuse,
		MSpanInUseBytes:  ms.MSpanInuse,
		MCacheInUseBytes: ms.MCacheInuse,
		BuckHashSysBytes: ms.BuckHashSys,
		GCSysBytes:       ms.GCSys,
		OtherSysBytes:    ms.OtherSys,
		NextGCSizeBytes:  ms.NextGC,
		LastSampleTime:   time.Now(),
		TotalAllocBytes:  ms.TotalAlloc,
		TotalAllocsCount: ms.Mallocs,
		TotalFreesCount:  ms.Frees,
		PauseTotalNs:     ms.PauseTotalNs,
	}

	if ms.NumGC > 0 {
		currentStats.LastCollectionMs = float64(ms.PauseNs[(ms.NumGC+255)%256]) / 1_000_000
	}

	// Update current stats
	s.MemoryStats = currentStats

	// Update goroutine count
	currentGoroutines := runtime.NumGoroutine()
	s.NumGoroutine = currentGoroutines

	// Update history with a maximum cap
	if len(s.MemoryHistory) >= s.MaxHistoryEntries {
		s.MemoryHistory = s.MemoryHistory[1:]
	}
	s.MemoryHistory = append(s.MemoryHistory, currentStats)

	if len(s.GoroutineHistory) >= s.MaxHistoryEntries {
		s.GoroutineHistory = s.GoroutineHistory[1:]
	}
	s.GoroutineHistory = append(s.GoroutineHistory, currentGoroutines)

	s.LastUpdate = time.Now()
}

// GetMemUsagePercent returns the current memory usage percentage
func (s *SystemStats) GetMemUsagePercent() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.MemoryStats.UsagePercent
}

// GetMemStats returns a copy of the current memory statistics
func (s *SystemStats) GetMemStats() MemStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.MemoryStats
}

// GetNumGoroutine returns the current number of goroutines
func (s *SystemStats) GetNumGoroutine() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.NumGoroutine
}

// GetMemoryTrend analyzes memory usage trend over the past samples
// Returns a value between -1 and 1, where:
// - Negative values indicate decreasing trend
// - Positive values indicate increasing trend
// - Values close to 0 indicate stable memory usage
func (s *SystemStats) GetMemoryTrend() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.MemoryHistory) < 2 {
		return 0
	}

	// Use the last 5 samples or fewer if not available
	sampleCount := 5
	if len(s.MemoryHistory) < sampleCount {
		sampleCount = len(s.MemoryHistory)
	}

	samples := s.MemoryHistory[len(s.MemoryHistory)-sampleCount:]

	// Calculate trend using linear regression slope
	var sumX, sumY, sumXY, sumXX float64
	n := float64(len(samples))

	for i, sample := range samples {
		x := float64(i)
		y := sample.UsagePercent

		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}

	// Calculate slope
	slope := (n*sumXY - sumX*sumY) / (n*sumXX - sumX*sumX)

	// Normalize to range between -1 and 1
	maxSlope := 10.0 // Arbitrary scaling factor
	normalizedSlope := slope / maxSlope

	if normalizedSlope > 1.0 {
		return 1.0
	} else if normalizedSlope < -1.0 {
		return -1.0
	}
	return normalizedSlope
}

// SuggestGC suggests whether garbage collection should be triggered
func (s *SystemStats) SuggestGC(threshold float64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Suggest GC if memory usage exceeds threshold OR
	// we have a lot of goroutines AND heap objects
	return s.MemoryStats.UsagePercent > threshold ||
		(s.NumGoroutine > 10000 && s.MemoryStats.HeapObjects > 100000)
}

// GetMemoryHistory returns a copy of the memory history
func (s *SystemStats) GetMemoryHistory() []MemStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := make([]MemStats, len(s.MemoryHistory))
	copy(history, s.MemoryHistory)
	return history
}

// GetGoroutineHistory returns a copy of the goroutine history
func (s *SystemStats) GetGoroutineHistory() []int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := make([]int, len(s.GoroutineHistory))
	copy(history, s.GoroutineHistory)
	return history
}

// ForceGC forces garbage collection
func ForceGC() {
	runtime.GC()
	debug.FreeOSMemory()
}

// GetSimpleMemUsagePercent returns the current memory usage percentage without using SystemStats
func GetSimpleMemUsagePercent() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / float64(m.Sys) * 100
}
