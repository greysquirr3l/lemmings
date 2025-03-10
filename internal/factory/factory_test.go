package factory

import (
	"context"
	"errors"
	"testing"
)

// Simple type for testing factories
type TestObject struct {
	ID      int
	Name    string
	IsValid bool
}

func TestFactoryFunc(t *testing.T) {
	t.Run("FactoryFunc.Create calls underlying function", func(t *testing.T) {
		called := false
		factory := FactoryFunc[TestObject](func() (TestObject, error) {
			called = true
			return TestObject{ID: 42, Name: "test"}, nil
		})

		obj, err := factory.Create()

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if !called {
			t.Error("Factory function was not called")
		}

		if obj.ID != 42 || obj.Name != "test" {
			t.Errorf("Got incorrect object: %+v", obj)
		}
	})

	t.Run("FactoryFunc.Create propagates errors", func(t *testing.T) {
		expectedErr := errors.New("factory error")
		factory := FactoryFunc[TestObject](func() (TestObject, error) {
			return TestObject{}, expectedErr
		})

		_, err := factory.Create()

		if !errors.Is(err, expectedErr) { // Fixed error comparison
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})
}

func TestWorkerFactory(t *testing.T) {
	t.Run("NewWorkerFactory creates factory with function", func(t *testing.T) {
		factory := NewWorkerFactory(func(id int) (TestObject, error) {
			return TestObject{ID: id, Name: "worker"}, nil
		})

		if factory == nil {
			t.Fatal("Expected factory to be created")
		}
	})

	t.Run("WorkerFactory.Create calls function with id 0", func(t *testing.T) {
		var capturedID int
		factory := NewWorkerFactory(func(id int) (TestObject, error) {
			capturedID = id
			return TestObject{ID: id, Name: "worker"}, nil
		})

		obj, err := factory.Create()

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if capturedID != 0 {
			t.Errorf("Expected id 0, got %d", capturedID)
		}

		if obj.ID != 0 || obj.Name != "worker" {
			t.Errorf("Got incorrect object: %+v", obj)
		}
	})

	t.Run("WorkerFactory.CreateWithID passes ID to function", func(t *testing.T) {
		var capturedID int
		factory := NewWorkerFactory(func(id int) (TestObject, error) {
			capturedID = id
			return TestObject{ID: id, Name: "worker"}, nil
		})

		obj, err := factory.CreateWithID(42)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if capturedID != 42 {
			t.Errorf("Expected id 42, got %d", capturedID)
		}

		if obj.ID != 42 || obj.Name != "worker" {
			t.Errorf("Got incorrect object: %+v", obj)
		}
	})

	t.Run("WorkerFactory.CreateWithID returns error when function is nil", func(t *testing.T) {
		factory := &WorkerFactoryFunc[TestObject]{
			createFn: nil,
		}

		_, err := factory.CreateWithID(1)

		if err == nil {
			t.Error("Expected error for nil factory function, got nil")
		}
	})

	t.Run("WorkerFactory.CreateWithID propagates errors", func(t *testing.T) {
		expectedErr := errors.New("factory error")
		factory := NewWorkerFactory(func(id int) (TestObject, error) {
			return TestObject{}, expectedErr
		})

		_, err := factory.CreateWithID(1)

		if !errors.Is(err, expectedErr) { // Fixed error comparison
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})
}

func TestPool(t *testing.T) {
	t.Run("NewPool initializes with objects", func(t *testing.T) {
		callCount := 0
		factory := FactoryFunc[TestObject](func() (TestObject, error) {
			callCount++
			return TestObject{ID: callCount, Name: "pooled"}, nil
		})

		pool, err := NewPool(factory, 3, 10)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if pool.Size() != 3 {
			t.Errorf("Expected pool size 3, got %d", pool.Size())
		}

		if callCount != 3 {
			t.Errorf("Expected factory to be called 3 times, got %d", callCount)
		}
	})

	t.Run("NewPool handles factory errors", func(t *testing.T) {
		expectedErr := errors.New("factory error")
		factory := FactoryFunc[TestObject](func() (TestObject, error) {
			return TestObject{}, expectedErr
		})

		_, err := NewPool(factory, 3, 10)

		if err == nil {
			t.Fatal("Expected error from factory, got nil")
		}

		if !errors.Is(err, expectedErr) {
			t.Errorf("Expected error to wrap %v, got %v", expectedErr, err)
		}
	})

	t.Run("Pool.Get returns object from pool", func(t *testing.T) {
		factory := FactoryFunc[TestObject](func() (TestObject, error) {
			return TestObject{ID: 42, Name: "pooled"}, nil
		})

		pool, _ := NewPool(factory, 1, 10)

		obj, err := pool.Get(context.Background())

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if obj.ID != 42 || obj.Name != "pooled" {
			t.Errorf("Got incorrect object: %+v", obj)
		}

		if pool.Size() != 0 {
			t.Errorf("Expected pool size to decrease to 0, got %d", pool.Size())
		}
	})

	t.Run("Pool.Get creates new object when pool empty", func(t *testing.T) {
		callCount := 0
		factory := FactoryFunc[TestObject](func() (TestObject, error) {
			callCount++
			return TestObject{ID: callCount, Name: "pooled"}, nil
		})

		pool, _ := NewPool(factory, 1, 10)

		// Get the first object (from pool)
		obj1, _ := pool.Get(context.Background())
		if obj1.ID != 1 {
			t.Errorf("Expected first object ID 1, got %d", obj1.ID)
		}

		// Get the second object (should be created)
		obj2, _ := pool.Get(context.Background())
		if obj2.ID != 2 {
			t.Errorf("Expected second object ID 2, got %d", obj2.ID)
		}

		if callCount != 2 {
			t.Errorf("Expected factory to be called 2 times, got %d", callCount)
		}
	})

	t.Run("Pool.Get respects max size", func(t *testing.T) {
		factory := FactoryFunc[TestObject](func() (TestObject, error) {
			return TestObject{}, nil
		})

		// Pool with 0 initial, 0 max size (will fail immediately)
		pool, _ := NewPool(factory, 0, 0)
		pool.initialized = true // Force initialized state

		_, err := pool.Get(context.Background())

		if err == nil {
			t.Error("Expected error for exhausted pool, got nil")
		}

		if err.Error() != "pool exhausted" {
			t.Errorf("Expected 'pool exhausted' error, got %v", err)
		}
	})

	t.Run("Pool.Get respects context cancellation", func(t *testing.T) {
		factory := FactoryFunc[TestObject](func() (TestObject, error) {
			return TestObject{}, nil
		})

		pool, _ := NewPool(factory, 0, 10)

		// Create canceled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := pool.Get(ctx)

		if err == nil {
			t.Error("Expected error for canceled context, got nil")
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled error, got %v", err)
		}
	})

	t.Run("Pool.Put adds object back to pool", func(t *testing.T) {
		factory := FactoryFunc[TestObject](func() (TestObject, error) {
			return TestObject{ID: 42, Name: "pooled"}, nil
		})

		pool, _ := NewPool(factory, 0, 10)

		// Initially empty
		if pool.Size() != 0 {
			t.Errorf("Expected initial size 0, got %d", pool.Size())
		}

		// Put an object
		obj := TestObject{ID: 99, Name: "returned"}
		pool.Put(obj)

		if pool.Size() != 1 {
			t.Errorf("Expected size 1 after Put, got %d", pool.Size())
		}

		// Get the object back
		retrievedObj, _ := pool.Get(context.Background())

		if retrievedObj.ID != 99 || retrievedObj.Name != "returned" {
			t.Errorf("Expected to get same object back, got %+v", retrievedObj)
		}
	})

	t.Run("Pool.Put respects max size", func(t *testing.T) {
		factory := FactoryFunc[TestObject](func() (TestObject, error) {
			return TestObject{}, nil
		})

		pool, _ := NewPool(factory, 0, 2)

		// Put three objects
		pool.Put(TestObject{ID: 1})
		pool.Put(TestObject{ID: 2})
		pool.Put(TestObject{ID: 3}) // Should be discarded

		// Size should be capped at 2
		if pool.Size() != 2 {
			t.Errorf("Expected size to be capped at 2, got %d", pool.Size())
		}
	})

	t.Run("Pool with unlimited max size", func(t *testing.T) {
		factory := FactoryFunc[TestObject](func() (TestObject, error) {
			return TestObject{}, nil
		})

		// Pool with negative max size meaning no limit
		pool, _ := NewPool(factory, 0, -1)

		// Put many objects
		for i := 0; i < 100; i++ {
			pool.Put(TestObject{ID: i})
		}

		// All should be stored
		if pool.Size() != 100 {
			t.Errorf("Expected size 100 for unlimited pool, got %d", pool.Size())
		}

		// Should be able to create new objects even with "full" pool
		_, err := pool.Get(context.Background())
		if err != nil {
			t.Errorf("Expected no error for unlimited pool, got %v", err)
		}
	})
}
