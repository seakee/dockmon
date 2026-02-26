// Copyright 2024 Seakee.  All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package trace

import (
	"runtime"
	"sync"
	"testing"
)

const (
	_   = 1 << (10 * iota)
	KiB // 1024
	MiB // 1048576
	GiB // 1073741824
)

var curMem uint64

// TestTraceID_New validates uniqueness under high concurrency.
//
// Parameters:
//   - t: testing context.
//
// Returns:
//   - None.
//
// Behavior:
//   - Generates a large number of IDs concurrently.
//   - Fails when any duplicate ID is detected.
func TestTraceID_New(t *testing.T) {
	// Create a trace ID generator for the stress test.
	tid := NewTraceID()

	// Protect shared map writes from concurrent goroutines.
	var mu sync.Mutex
	// Track all generated IDs to detect duplicates.
	uniqueIDs := make(map[string]struct{})
	// Wait for all goroutines to finish.
	var wg sync.WaitGroup

	// Configure worker count for concurrency simulation.
	const concurrency = 10000
	// Start concurrent workers.
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Generate IDs in a tight loop per worker.
			for j := 0; j < 10000; j++ {
				id := tid.New()
				mu.Lock()
				// Fail fast when a duplicate ID is found.
				if _, exists := uniqueIDs[id]; exists {
					mu.Unlock()
					t.Errorf("Duplicate ID found: %s", id)
					return
				}
				// Record generated ID for uniqueness checks.
				uniqueIDs[id] = struct{}{}
				mu.Unlock()
			}
		}()
	}

	// Wait for all workers before assertions and metrics.
	wg.Wait()
	// Emit metrics for observability.
	t.Logf("Unique IDs count: %d", len(uniqueIDs))
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

// BenchmarkTraceID_New benchmarks single-call ID generation throughput.
//
// Parameters:
//   - b: benchmark context.
//
// Returns:
//   - None.
func BenchmarkTraceID_New(b *testing.B) {
	id := NewTraceID()
	for i := 0; i < b.N; i++ {
		id.New()
	}
}
