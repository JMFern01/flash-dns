package server

import (
	"sync"
	"testing"
)

// TEST 1: Initialize statistics with zero values
// Tests that new Statistics struct starts with all counters at zero
func TestStatistics_InitialState(t *testing.T) {
	var (
		stats       *Statistics = &Statistics{}
		blocked     uint64
		allowed     uint64
		cacheHits   uint64
		cacheMisses uint64
	)

	blocked, allowed, cacheHits, cacheMisses = stats.GetStats()

	if blocked != 0 {
		t.Errorf("Expected blocked=0, got %d", blocked)
	}
	if allowed != 0 {
		t.Errorf("Expected allowed=0, got %d", allowed)
	}
	if cacheHits != 0 {
		t.Errorf("Expected cacheHits=0, got %d", cacheHits)
	}
	if cacheMisses != 0 {
		t.Errorf("Expected cacheMisses=0, got %d", cacheMisses)
	}
}

// TEST 2: Increment blocked counter
// Tests that incrementBlocked increases the counter by 1
func TestStatistics_IncrementBlocked(t *testing.T) {
	var (
		stats   *Statistics = &Statistics{}
		blocked uint64
		_       uint64
	)

	stats.incrementBlocked()

	blocked, _, _, _ = stats.GetStats()

	if blocked != 1 {
		t.Errorf("Expected blocked=1, got %d", blocked)
	}
}

// TEST 3: Increment allowed counter
// Tests that incrementAllowed increases the counter by 1
func TestStatistics_IncrementAllowed(t *testing.T) {
	var (
		stats   *Statistics = &Statistics{}
		allowed uint64
		_       uint64
	)

	stats.incrementAllowed()

	_, allowed, _, _ = stats.GetStats()

	if allowed != 1 {
		t.Errorf("Expected allowed=1, got %d", allowed)
	}
}

// TEST 4: Increment cache hits counter
// Tests that incrementCacheHits increases the counter by 1
func TestStatistics_IncrementCacheHits(t *testing.T) {
	var (
		stats     *Statistics = &Statistics{}
		cacheHits uint64
		_         uint64
	)

	stats.incrementCacheHits()

	_, _, cacheHits, _ = stats.GetStats()

	if cacheHits != 1 {
		t.Errorf("Expected cacheHits=1, got %d", cacheHits)
	}
}

// TEST 5: Increment cache misses counter
// Tests that incrementCacheMisses increases the counter by 1
func TestStatistics_IncrementCacheMisses(t *testing.T) {
	var (
		stats       *Statistics = &Statistics{}
		cacheMisses uint64
		_           uint64
	)

	stats.incrementCacheMisses()

	_, _, _, cacheMisses = stats.GetStats()

	if cacheMisses != 1 {
		t.Errorf("Expected cacheMisses=1, got %d", cacheMisses)
	}
}

// TEST 6: Multiple increments to same counter
// Tests that counter accumulates correctly
func TestStatistics_MultipleIncrements(t *testing.T) {
	var (
		stats    *Statistics = &Statistics{}
		i        int
		blocked  uint64
		expected uint64 = 10
		_        uint64
	)

	for i = 0; i < int(expected); i++ {
		stats.incrementBlocked()
	}

	blocked, _, _, _ = stats.GetStats()

	if blocked != expected {
		t.Errorf("Expected blocked=%d, got %d", expected, blocked)
	}
}

// TEST 7: All counters work independently
// Tests that incrementing one counter doesn't affect others
func TestStatistics_IndependentCounters(t *testing.T) {
	var (
		stats       *Statistics = &Statistics{}
		blocked     uint64
		allowed     uint64
		cacheHits   uint64
		cacheMisses uint64
	)

	stats.incrementBlocked()
	stats.incrementBlocked()
	stats.incrementAllowed()
	stats.incrementCacheHits()
	stats.incrementCacheHits()
	stats.incrementCacheHits()
	stats.incrementCacheMisses()

	blocked, allowed, cacheHits, cacheMisses = stats.GetStats()

	if blocked != 2 {
		t.Errorf("Expected blocked=2, got %d", blocked)
	}
	if allowed != 1 {
		t.Errorf("Expected allowed=1, got %d", allowed)
	}
	if cacheHits != 3 {
		t.Errorf("Expected cacheHits=3, got %d", cacheHits)
	}
	if cacheMisses != 1 {
		t.Errorf("Expected cacheMisses=1, got %d", cacheMisses)
	}
}

// TEST 8: Concurrent increments are thread-safe
// Tests that atomic operations work correctly under concurrent access
func TestStatistics_ConcurrentIncrements(t *testing.T) {
	var (
		stats      *Statistics = &Statistics{}
		wg         sync.WaitGroup
		i          int
		goroutines int    = 100
		increments int    = 100
		expected   uint64 = uint64(goroutines * increments)
		blocked    uint64
		_          uint64
	)

	wg.Add(goroutines)

	// Launch multiple goroutines incrementing concurrently
	for i = 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			var k int
			for k = 0; k < increments; k++ {
				stats.incrementBlocked()
			}
		}()
	}

	wg.Wait()

	blocked, _, _, _ = stats.GetStats()

	if blocked != expected {
		t.Errorf("Expected blocked=%d, got %d (race condition detected)", expected, blocked)
	}
}

// TEST 9: Concurrent increments to different counters
// Tests that all counters can be safely incremented concurrently
func TestStatistics_ConcurrentMixedIncrements(t *testing.T) {
	var (
		stats       *Statistics = &Statistics{}
		wg          sync.WaitGroup
		iterations  int = 1000
		blocked     uint64
		allowed     uint64
		cacheHits   uint64
		cacheMisses uint64
	)

	wg.Add(4)

	// Goroutine 1: increment blocked
	go func() {
		defer wg.Done()
		var k int
		for k = 0; k < iterations; k++ {
			stats.incrementBlocked()
		}
	}()

	// Goroutine 2: increment allowed
	go func() {
		defer wg.Done()
		var k int
		for k = 0; k < iterations; k++ {
			stats.incrementAllowed()
		}
	}()

	// Goroutine 3: increment cache hits
	go func() {
		defer wg.Done()
		var k int
		for k = 0; k < iterations; k++ {
			stats.incrementCacheHits()
		}
	}()

	// Goroutine 4: increment cache misses
	go func() {
		defer wg.Done()
		var k int
		for k = 0; k < iterations; k++ {
			stats.incrementCacheMisses()
		}
	}()

	wg.Wait()

	blocked, allowed, cacheHits, cacheMisses = stats.GetStats()

	if blocked != uint64(iterations) {
		t.Errorf("Expected blocked=%d, got %d", iterations, blocked)
	}
	if allowed != uint64(iterations) {
		t.Errorf("Expected allowed=%d, got %d", iterations, allowed)
	}
	if cacheHits != uint64(iterations) {
		t.Errorf("Expected cacheHits=%d, got %d", iterations, cacheHits)
	}
	if cacheMisses != uint64(iterations) {
		t.Errorf("Expected cacheMisses=%d, got %d", iterations, cacheMisses)
	}
}

// TEST 10: GetStats returns current state
// Tests that GetStats returns accurate snapshot of counters
func TestStatistics_GetStats(t *testing.T) {
	var (
		stats       *Statistics = &Statistics{}
		blocked     uint64
		allowed     uint64
		cacheHits   uint64
		cacheMisses uint64
	)

	stats.incrementBlocked()
	stats.incrementBlocked()
	stats.incrementBlocked()
	stats.incrementAllowed()
	stats.incrementAllowed()
	stats.incrementCacheHits()
	stats.incrementCacheMisses()
	stats.incrementCacheMisses()

	blocked, allowed, cacheHits, cacheMisses = stats.GetStats()

	if blocked != 3 {
		t.Errorf("Expected blocked=3, got %d", blocked)
	}
	if allowed != 2 {
		t.Errorf("Expected allowed=2, got %d", allowed)
	}
	if cacheHits != 1 {
		t.Errorf("Expected cacheHits=1, got %d", cacheHits)
	}
	if cacheMisses != 2 {
		t.Errorf("Expected cacheMisses=2, got %d", cacheMisses)
	}
}

// TEST 11: Log doesn't crash with zero stats
// Tests that Log handles division by zero gracefully
func TestStatistics_Log_ZeroStats(t *testing.T) {
	var stats *Statistics = &Statistics{}

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Log panicked with zero stats: %v", r)
		}
	}()

	stats.Log()
}

// TEST 12: Log doesn't crash with partial stats
// Tests that Log handles edge cases like zero cache operations
func TestStatistics_Log_PartialStats(t *testing.T) {
	var stats *Statistics = &Statistics{}

	stats.incrementBlocked()
	stats.incrementAllowed()
	// Note: No cache operations, so cacheHits + cacheMisses = 0

	// Should not panic (will have NaN or Inf in calculations)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Log panicked with partial stats: %v", r)
		}
	}()

	stats.Log()
}

// TEST 13: Concurrent reads and writes
// Tests that reading stats while incrementing is safe
func TestStatistics_ConcurrentReadsAndWrites(t *testing.T) {
	var (
		stats      *Statistics = &Statistics{}
		wg         sync.WaitGroup
		iterations int = 1000
	)

	wg.Add(2)

	// Goroutine 1: continuously increment
	go func() {
		defer wg.Done()
		var k int
		for k = 0; k < iterations; k++ {
			stats.incrementBlocked()
			stats.incrementAllowed()
			stats.incrementCacheHits()
			stats.incrementCacheMisses()
		}
	}()

	// Goroutine 2: continuously read
	go func() {
		defer wg.Done()
		var (
			blocked     uint64
			allowed     uint64
			cacheHits   uint64
			cacheMisses uint64
			k           int
		)
		for k = 0; k < iterations; k++ {
			blocked, allowed, cacheHits, cacheMisses = stats.GetStats()
			// Just reading, verify it doesn't panic
			_ = blocked
			_ = allowed
			_ = cacheHits
			_ = cacheMisses
		}
	}()

	wg.Wait()

	// Verify final counts are correct
	var (
		blocked uint64
		_       uint64
	)
	blocked, _, _, _ = stats.GetStats()

	if blocked != uint64(iterations) {
		t.Errorf("Expected blocked=%d, got %d (lost updates)", iterations, blocked)
	}
}

// TEST 14: High volume stress test
// Tests that counters work correctly under high load
func TestStatistics_HighVolumeStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	var (
		stats      *Statistics = &Statistics{}
		wg         sync.WaitGroup
		i          int
		goroutines int    = 50
		increments int    = 10000
		expected   uint64 = uint64(goroutines * increments)
		blocked    uint64
		_          uint64
	)

	wg.Add(goroutines)

	for i = 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			var k int
			for k = 0; k < increments; k++ {
				stats.incrementBlocked()
				stats.incrementAllowed()
				stats.incrementCacheHits()
				stats.incrementCacheMisses()
			}
		}()
	}

	wg.Wait()

	blocked, _, _, _ = stats.GetStats()

	if blocked != expected {
		t.Errorf("Expected blocked=%d, got %d", expected, blocked)
	}
}

// TEST 15: Multiple GetStats calls return consistent results
// Tests that GetStats can be called multiple times safely
func TestStatistics_MultipleGetStats(t *testing.T) {
	var (
		stats        *Statistics = &Statistics{}
		blocked1     uint64
		allowed1     uint64
		cacheHits1   uint64
		cacheMisses1 uint64
		blocked2     uint64
		allowed2     uint64
		cacheHits2   uint64
		cacheMisses2 uint64
	)

	stats.incrementBlocked()
	stats.incrementAllowed()
	stats.incrementCacheHits()
	stats.incrementCacheMisses()

	blocked1, allowed1, cacheHits1, cacheMisses1 = stats.GetStats()
	blocked2, allowed2, cacheHits2, cacheMisses2 = stats.GetStats()

	if blocked1 != blocked2 {
		t.Errorf("GetStats returned different blocked values: %d vs %d", blocked1, blocked2)
	}
	if allowed1 != allowed2 {
		t.Errorf("GetStats returned different allowed values: %d vs %d", allowed1, allowed2)
	}
	if cacheHits1 != cacheHits2 {
		t.Errorf("GetStats returned different cacheHits values: %d vs %d", cacheHits1, cacheHits2)
	}
	if cacheMisses1 != cacheMisses2 {
		t.Errorf("GetStats returned different cacheMisses values: %d vs %d", cacheMisses1, cacheMisses2)
	}
}

// TEST 16: Zero total doesn't cause panic in Log
// Tests division by zero handling in block rate calculation
func TestStatistics_Log_ZeroTotal(t *testing.T) {
	var stats *Statistics = &Statistics{}

	// No blocked or allowed, so total = 0
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Log panicked with zero total: %v", r)
		}
	}()

	stats.Log()
}

// TEST 17: Only cache hits, no misses
// Tests cache hit rate calculation edge case
func TestStatistics_Log_OnlyCacheHits(t *testing.T) {
	var stats *Statistics = &Statistics{}

	stats.incrementBlocked()
	stats.incrementCacheHits()
	stats.incrementCacheHits()
	stats.incrementCacheHits()
	// No cache misses

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Log panicked with only cache hits: %v", r)
		}
	}()

	stats.Log()
}

// TEST 18: Realistic usage scenario
// Tests a realistic combination of operations
func TestStatistics_RealisticScenario(t *testing.T) {
	var (
		stats       *Statistics = &Statistics{}
		blocked     uint64
		allowed     uint64
		cacheHits   uint64
		cacheMisses uint64
	)

	// Simulate real usage: some blocked, many allowed, lots of cache activity
	stats.incrementBlocked()
	stats.incrementBlocked()
	stats.incrementBlocked()

	stats.incrementAllowed()
	stats.incrementAllowed()
	stats.incrementAllowed()
	stats.incrementAllowed()
	stats.incrementAllowed()
	stats.incrementAllowed()
	stats.incrementAllowed()

	stats.incrementCacheHits()
	stats.incrementCacheHits()
	stats.incrementCacheHits()
	stats.incrementCacheHits()
	stats.incrementCacheHits()

	stats.incrementCacheMisses()
	stats.incrementCacheMisses()

	blocked, allowed, cacheHits, cacheMisses = stats.GetStats()

	if blocked != 3 {
		t.Errorf("Expected 3 blocked, got %d", blocked)
	}
	if allowed != 7 {
		t.Errorf("Expected 7 allowed, got %d", allowed)
	}
	if cacheHits != 5 {
		t.Errorf("Expected 5 cache hits, got %d", cacheHits)
	}
	if cacheMisses != 2 {
		t.Errorf("Expected 2 cache misses, got %d", cacheMisses)
	}

	// Verify totals
	var total uint64 = blocked + allowed
	if total != 10 {
		t.Errorf("Expected total=10, got %d", total)
	}

	// Log shouldn't panic
	stats.Log()
}
