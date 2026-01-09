package hierarchy

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetClientMutex_SameServer verifies that the same mutex is returned
// for the same server name, ensuring proper serialization.
func TestGetClientMutex_SameServer(t *testing.T) {
	registry := NewServerRegistry(nil)

	mutex1 := registry.GetClientMutex("trello")
	mutex2 := registry.GetClientMutex("trello")

	assert.Same(t, mutex1, mutex2, "Same server should return same mutex")
}

// TestGetClientMutex_DifferentServers verifies that different mutexes are
// returned for different servers, allowing parallel execution across servers.
func TestGetClientMutex_DifferentServers(t *testing.T) {
	registry := NewServerRegistry(nil)

	trelloMutex := registry.GetClientMutex("trello")
	githubMutex := registry.GetClientMutex("github")
	gmailMutex := registry.GetClientMutex("gmail")

	assert.NotSame(t, trelloMutex, githubMutex, "Different servers should have different mutexes")
	assert.NotSame(t, trelloMutex, gmailMutex, "Different servers should have different mutexes")
	assert.NotSame(t, githubMutex, gmailMutex, "Different servers should have different mutexes")
}

// TestGetClientMutex_ConcurrentAccess verifies thread-safety of GetClientMutex
// when multiple goroutines request mutexes simultaneously.
func TestGetClientMutex_ConcurrentAccess(t *testing.T) {
	registry := NewServerRegistry(nil)

	const numGoroutines = 100
	var wg sync.WaitGroup
	mutexes := make([]*sync.Mutex, numGoroutines)

	// All goroutines request the same server's mutex concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mutexes[idx] = registry.GetClientMutex("shared-server")
		}(i)
	}

	wg.Wait()

	// All should be the same mutex
	for i := 1; i < numGoroutines; i++ {
		assert.Same(t, mutexes[0], mutexes[i],
			"All goroutines should receive the same mutex for the same server")
	}
}

// TestMutexSerializesExecution verifies that the mutex actually serializes
// concurrent operations, preventing interleaved execution.
func TestMutexSerializesExecution(t *testing.T) {
	registry := NewServerRegistry(nil)
	mutex := registry.GetClientMutex("test-server")

	const numOperations = 10
	var executionOrder []int
	var orderMu sync.Mutex
	var activeCount int32
	var maxConcurrent int32

	var wg sync.WaitGroup

	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(opNum int) {
			defer wg.Done()

			mutex.Lock()
			defer mutex.Unlock()

			// Track concurrent executions
			current := atomic.AddInt32(&activeCount, 1)
			for {
				old := atomic.LoadInt32(&maxConcurrent)
				if current <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, current) {
					break
				}
			}

			// Simulate work
			time.Sleep(1 * time.Millisecond)

			// Record execution order
			orderMu.Lock()
			executionOrder = append(executionOrder, opNum)
			orderMu.Unlock()

			atomic.AddInt32(&activeCount, -1)
		}(i)
	}

	wg.Wait()

	// Verify serialization: max concurrent should be 1
	assert.Equal(t, int32(1), maxConcurrent,
		"Only one operation should execute at a time (mutex serialization)")

	// All operations should have completed
	assert.Len(t, executionOrder, numOperations,
		"All operations should complete")
}

// TestDifferentServersMutexesAllowParallel verifies that different servers
// can execute in parallel (their mutexes don't block each other).
func TestDifferentServersMutexesAllowParallel(t *testing.T) {
	registry := NewServerRegistry(nil)

	server1Mutex := registry.GetClientMutex("server1")
	server2Mutex := registry.GetClientMutex("server2")

	var maxConcurrent int32
	var activeCount int32
	var wg sync.WaitGroup

	// Simulate parallel execution on different servers
	simulateWork := func(mutex *sync.Mutex, serverName string) {
		defer wg.Done()

		mutex.Lock()
		defer mutex.Unlock()

		current := atomic.AddInt32(&activeCount, 1)
		for {
			old := atomic.LoadInt32(&maxConcurrent)
			if current <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, current) {
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&activeCount, -1)
	}

	wg.Add(2)
	go simulateWork(server1Mutex, "server1")
	go simulateWork(server2Mutex, "server2")

	wg.Wait()

	// Both should have been able to run concurrently
	require.Equal(t, int32(2), maxConcurrent,
		"Different servers should execute in parallel (max concurrent = 2)")
}
