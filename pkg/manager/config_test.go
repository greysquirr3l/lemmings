package manager

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Check default values
	if cfg.InitialWorkers != 4 {
		t.Errorf("Expected InitialWorkers to be 4, got %d", cfg.InitialWorkers)
	}

	if cfg.MaxWorkers != 32 {
		t.Errorf("Expected MaxWorkers to be 32, got %d", cfg.MaxWorkers)
	}

	if cfg.MinWorkers != 1 {
		t.Errorf("Expected MinWorkers to be 1, got %d", cfg.MinWorkers)
	}

	if cfg.MaxTasksInQueue != 1000 {
		t.Errorf("Expected MaxTasksInQueue to be 1000, got %d", cfg.MaxTasksInQueue)
	}

	if cfg.ScaleUpFactor != 1.5 {
		t.Errorf("Expected ScaleUpFactor to be 1.5, got %f", cfg.ScaleUpFactor)
	}

	if cfg.ScaleDownFactor != 0.5 {
		t.Errorf("Expected ScaleDownFactor to be 0.5, got %f", cfg.ScaleDownFactor)
	}

	if cfg.MonitorInterval != 5*time.Second {
		t.Errorf("Expected MonitorInterval to be 5s, got %v", cfg.MonitorInterval)
	}

	if cfg.TaskTimeout != 30*time.Second {
		t.Errorf("Expected TaskTimeout to be 30s, got %v", cfg.TaskTimeout)
	}

	if cfg.LowMemoryMark != 30.0 {
		t.Errorf("Expected LowMemoryMark to be 30.0, got %f", cfg.LowMemoryMark)
	}

	if cfg.HighMemoryMark != 70.0 {
		t.Errorf("Expected HighMemoryMark to be 70.0, got %f", cfg.HighMemoryMark)
	}
}

func TestConfigValidation(t *testing.T) {
	t.Run("Valid config passes validation", func(t *testing.T) {
		cfg := DefaultConfig()
		err := cfg.Validate()
		if err != nil {
			t.Errorf("Expected no validation error for default config, got %v", err)
		}
	})

	t.Run("Invalid InitialWorkers fails validation", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.InitialWorkers = 0
		err := cfg.Validate()
		if err == nil {
			t.Error("Expected validation error for InitialWorkers=0, got nil")
		}
	})

	t.Run("Invalid MaxWorkers fails validation", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxWorkers = 0
		err := cfg.Validate()
		if err == nil {
			t.Error("Expected validation error for MaxWorkers=0, got nil")
		}
	})

	t.Run("Invalid MinWorkers fails validation", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MinWorkers = 0
		err := cfg.Validate()
		if err == nil {
			t.Error("Expected validation error for MinWorkers=0, got nil")
		}
	})

	t.Run("Invalid worker counts fail validation", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MinWorkers = 10
		cfg.MaxWorkers = 5
		err := cfg.Validate()
		if err == nil {
			t.Error("Expected validation error for MinWorkers > MaxWorkers, got nil")
		}
	})

	t.Run("Invalid scale factors fail validation", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.ScaleUpFactor = 0.5
		err := cfg.Validate()
		if err == nil {
			t.Error("Expected validation error for ScaleUpFactor < 1.0, got nil")
		}

		cfg = DefaultConfig()
		cfg.ScaleDownFactor = 1.5
		err = cfg.Validate()
		if err == nil {
			t.Error("Expected validation error for ScaleDownFactor > 1.0, got nil")
		}
	})

	t.Run("Invalid memory marks fail validation", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.LowMemoryMark = -10
		err := cfg.Validate()
		if err == nil {
			t.Error("Expected validation error for negative LowMemoryMark, got nil")
		}

		cfg = DefaultConfig()
		cfg.HighMemoryMark = 110
		err = cfg.Validate()
		if err == nil {
			t.Error("Expected validation error for HighMemoryMark > 100, got nil")
		}

		cfg = DefaultConfig()
		cfg.LowMemoryMark = 80
		cfg.HighMemoryMark = 70
		err = cfg.Validate()
		if err == nil {
			t.Error("Expected validation error for LowMemoryMark > HighMemoryMark, got nil")
		}
	})
}

func TestConfigWithOptions(t *testing.T) {
	t.Run("ConfigWithOptions applies all options", func(t *testing.T) {
		cfg := ConfigWithOptions(
			WithInitialWorkers(10),
			WithMaxWorkers(50),
			WithMinWorkers(5),
			WithMaxTasksInQueue(500),
			WithScaleFactors(2.0, 0.25),
			WithMemoryWatermarks(20, 80),
			WithMonitorInterval(10*time.Second),
			WithTaskTimeout(1*time.Minute),
			WithScaleUpDelay(30*time.Second),
			WithForceGCThreshold(85),
			WithVerbose(true),
		)

		if cfg.InitialWorkers != 10 {
			t.Errorf("Expected InitialWorkers=10, got %d", cfg.InitialWorkers)
		}

		if cfg.MaxWorkers != 50 {
			t.Errorf("Expected MaxWorkers=50, got %d", cfg.MaxWorkers)
		}

		if cfg.MinWorkers != 5 {
			t.Errorf("Expected MinWorkers=5, got %d", cfg.MinWorkers)
		}

		if cfg.MaxTasksInQueue != 500 {
			t.Errorf("Expected MaxTasksInQueue=500, got %d", cfg.MaxTasksInQueue)
		}

		if cfg.ScaleUpFactor != 2.0 {
			t.Errorf("Expected ScaleUpFactor=2.0, got %f", cfg.ScaleUpFactor)
		}

		if cfg.ScaleDownFactor != 0.25 {
			t.Errorf("Expected ScaleDownFactor=0.25, got %f", cfg.ScaleDownFactor)
		}

		if cfg.LowMemoryMark != 20 {
			t.Errorf("Expected LowMemoryMark=20, got %f", cfg.LowMemoryMark)
		}

		if cfg.HighMemoryMark != 80 {
			t.Errorf("Expected HighMemoryMark=80, got %f", cfg.HighMemoryMark)
		}

		if cfg.MonitorInterval != 10*time.Second {
			t.Errorf("Expected MonitorInterval=10s, got %v", cfg.MonitorInterval)
		}

		if cfg.TaskTimeout != 1*time.Minute {
			t.Errorf("Expected TaskTimeout=1m, got %v", cfg.TaskTimeout)
		}

		if cfg.ScaleUpDelay != 30*time.Second {
			t.Errorf("Expected ScaleUpDelay=30s, got %v", cfg.ScaleUpDelay)
		}

		if cfg.ForceGCThreshold != 85 {
			t.Errorf("Expected ForceGCThreshold=85, got %f", cfg.ForceGCThreshold)
		}

		if !cfg.Verbose {
			t.Error("Expected Verbose=true, got false")
		}
	})
}
