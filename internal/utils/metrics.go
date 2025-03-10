package utils

import (
	"runtime"
	"runtime/debug" // Add debug import
	"sync"
	"time"
)

// MemStats represents memory statistics
type MemStats struct {
	UsagePercent      float64
	AllocatedBytes    uint64
	SystemBytes       uint64
	GCPauseTimeNs     uint64
	LastGCTime        time.Time
	NumGC             uint32
	LastCollectionMs  float64
	HeapObjects       uint64
	HeapInUseBytes    uint64
	StackInUseBytes   uint64
	MSpanInUseBytes   uint64
	MCacheInUseBytes  uint64
	BuckHashSysBytes  uint64
	GCSysBytes        uint64
	OtherSysBytes     uint64
	NextGCSizeBytes   uint64
	LastSampleTime    time.Time
	TotalAllocBytes   uint64
	TotalAllocsCount  uint64
	TotalFreesCount   uint64
	PauseTotalNs      uint64
}

// SystemStats tracks system resource usage
type SystemStats struct {
	mu           sync.RWMutex
	CPUUsage     float64
	MemoryStats  MemStats
	NumGoroutine int
	LastUpdate   time.Time
}

// New creates a new SystemStats instance
func NewSystemStats() *SystemStats {
	return &SystemStats{
		LastUpdate: time.Now(),
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

	s.MemoryStats = MemStats{
		UsagePercent:      usagePercent,
		AllocatedBytes:    ms.Alloc,
		SystemBytes:       ms.Sys,
		GCPauseTimeNs:     ms.PauseNs[(ms.NumGC+255)%256],
		LastGCTime:        time.Unix(0, int64(ms.LastGC)),
		NumGC:             ms.NumGC,
		HeapObjects:       ms.HeapObjects,
		HeapInUseBytes:    ms.HeapInuse,
		StackInUseBytes:   ms.StackInuse,
		MSpanInUseBytes:   ms.MSpanInuse,
		MCacheInUseBytes:  ms.MCacheInuse,
		BuckHashSysBytes:  ms.BuckHashSys,
		GCSysBytes:        ms.GCSys,
		OtherSysBytes:     ms.OtherSys,
		NextGCSizeBytes:   ms.NextGC,
		LastSampleTime:    time.Now(),
		TotalAllocBytes:   ms.TotalAlloc,
		TotalAllocsCount:  ms.Mallocs,
		TotalFreesCount:   ms.Frees,
		PauseTotalNs:      ms.PauseTotalNs,
	}

	if ms.NumGC > 0 {
		s.MemoryStats.LastCollectionMs = float64(ms.PauseNs[(ms.NumGC+255)%256]) / 1_000_000
	}

	s.NumGoroutine = runtime.NumGoroutine()
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

// SuggestGC suggests whether garbage collection should be triggered
func (s *SystemStats) SuggestGC(threshold float64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return s.MemoryStats.UsagePercent > threshold || 
		(s.NumGoroutine > 10000 && s.MemoryStats.HeapObjects > 100000)
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