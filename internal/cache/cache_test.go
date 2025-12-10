package cache

import (
	"testing"
	"time"
)

// TEST 1: Basic Get/Set operations
// Tests that we can store and retrieve data from cache
func TestDNSCache_BasicGetSet(t *testing.T) {
	var (
		cache    *DNSCache = NewDNSCache()
		key      string    = "example.com"
		response []byte    = []byte("192.168.1.1")
		ttl      uint32    = 300
	)

	// Set a value
	cache.Set(key, response, ttl)

	// Get it back
	var (
		result       []byte
		found        bool
		needsRefresh bool
	)
	result, found, needsRefresh = cache.Get(key)

	if !found {
		t.Error("Expected to find cached entry")
	}
	if needsRefresh {
		t.Error("Fresh entry should not need refresh")
	}
	if string(result) != string(response) {
		t.Errorf("Expected %s, got %s", response, result)
	}
}

// TEST 2: Missing key returns not found
// Verifies that requesting non-existent keys returns false
func TestDNSCache_GetMissingKey(t *testing.T) {
	var (
		cache *DNSCache = NewDNSCache()
		key   string    = "nonexistent.com"
	)

	var (
		result       []byte
		found        bool
		needsRefresh bool
	)
	result, found, needsRefresh = cache.Get(key)

	if found {
		t.Error("Should not find non-existent key")
	}
	if result != nil {
		t.Error("Result should be nil for missing key")
	}
	if needsRefresh {
		t.Error("Missing key should not trigger refresh")
	}
}

// TEST 3: Popularity tracking
// Tests that accessing entries increases their popularity
func TestCacheEntry_PopularityTracking(t *testing.T) {
	var (
		cache    *DNSCache = NewDNSCache()
		key      string    = "popular.com"
		response []byte    = []byte("1.2.3.4")
		ttl      uint32    = 300
	)

	cache.Set(key, response, ttl)

	// Access multiple times to increase popularity
	var i int
	for i = 0; i < 6; i++ {
		_, _, _ = cache.Get(key)
	}

	// Get entry to check popularity
	cache.mu.RLock()
	var entry *CacheEntry = cache.entries[key]
	cache.mu.RUnlock()

	if !entry.IsPopular() {
		t.Errorf("Entry should be popular after %d accesses", i)
	}
}

// TEST 4: Stale detection within grace period
// Tests that expired entries within grace period are marked as stale
func TestCacheEntry_StaleDetection(t *testing.T) {
	var (
		now   time.Time   = time.Now()
		entry *CacheEntry = &CacheEntry{
			CreatedAt: now.Add(-10 * time.Minute),
			ExpiresAt: now.Add(-2 * time.Minute), // Expired 2 min ago
		}
		// Grace period is 5 minutes, so this is stale but not completely expired
	)

	if !entry.IsStale(now) {
		t.Error("Entry should be stale")
	}
	if entry.IsCompletelyExpired() {
		t.Error("Entry should not be completely expired yet")
	}
}

// TEST 5: Complete expiration removes entry
// Tests that entries beyond grace period are deleted
func TestDNSCache_CompleteExpiration(t *testing.T) {
	var (
		cache          *DNSCache     = NewDNSCache()
		key            string        = "expired.com"
		response       []byte        = []byte("5.6.7.8")
		ttl            uint32        = 1 // 1 second TTL
		oldGracePeriod time.Duration = GRACE_PERIOD
	)

	cache.Set(key, response, ttl)

	defer func() {
		GRACE_PERIOD = oldGracePeriod
	}()

	GRACE_PERIOD = 1 * time.Millisecond
	// Wait for complete expiration (TTL + grace period)
	time.Sleep(time.Duration(ttl)*time.Second + GRACE_PERIOD + 10*time.Millisecond)

	var (
		result []byte
		found  bool
	)
	result, found, _ = cache.Get(key)

	if found {
		t.Error("Completely expired entry should not be found")
	}
	if result != nil {
		t.Error("Result should be nil for expired entry")
	}

	// Verify it's removed from cache
	cache.mu.RLock()
	var exists bool
	_, exists = cache.entries[key]
	cache.mu.RUnlock()

	if exists {
		t.Error("Expired entry should be deleted from cache")
	}
}

// TEST 6: Prefetch threshold detection
// Tests that popular entries near expiration trigger prefetch
func TestCacheEntry_PrefetchThreshold(t *testing.T) {
	var (
		ttl       uint32        = 100 // 100 seconds
		threshold time.Duration = time.Duration(float64(ttl)*PREFETCH_THRESHOLD) * time.Second
		now       time.Time     = time.Now()
		entry     *CacheEntry   = &CacheEntry{
			CreatedAt:   now.Add(-threshold - time.Second), // Past 80% threshold
			ExpiresAt:   now.Add(time.Duration(ttl) * time.Second),
			originalTTL: ttl,
		}
	)

	// Make it popular first
	var i int
	for i = 0; i < 6; i++ {
		entry.increasePopularity()
	}

	if !entry.ShouldPrefetch() {
		t.Error("Popular entry past threshold should trigger prefetch")
	}
}

// TEST 7: Eviction when cache is full
// Tests that least valuable entries are evicted when cache reaches max size
func TestDNSCache_Eviction(t *testing.T) {
	var (
		cache *DNSCache = NewDNSCache()
		ttl   uint32    = 300
	)

	// Fill cache to max
	var i int
	for i = 0; i < CACHE_MAX_SIZE; i++ {
		var key string = string(rune(i))
		cache.Set(key, []byte("data"), ttl)
	}

	// Access first entry multiple times to make it popular
	var popularKey string = string(rune(0))
	var j int
	for j = 0; j < 10; j++ {
		_, _, _ = cache.Get(popularKey)
	}

	// Add one more entry, should trigger eviction
	var newKey string = "newentry"
	cache.Set(newKey, []byte("newdata"), ttl)

	// Verify cache size is still at max
	cache.mu.RLock()
	var size int = len(cache.entries)
	cache.mu.RUnlock()

	if size > CACHE_MAX_SIZE {
		t.Errorf("Cache size %d exceeds max %d", size, CACHE_MAX_SIZE)
	}

	// Popular entry should still exist
	var (
		_     []byte
		found bool
	)
	_, found, _ = cache.Get(popularKey)

	if !found {
		t.Error("Popular entry should not be evicted")
	}
}

// TEST 8: Clean removes all expired entries
// Tests the Clean method removes entries beyond grace period
func TestDNSCache_Clean(t *testing.T) {
	var (
		c              *DNSCache     = NewDNSCache()
		ttl            uint32        = 1
		oldGracePeriod time.Duration = GRACE_PERIOD
	)

	// Add entries
	c.Set("keep.com", []byte("1.1.1.1"), 3600)  // Long TTL, keep
	c.Set("expire.com", []byte("2.2.2.2"), ttl) // Short TTL, expire

	defer func() {
		GRACE_PERIOD = oldGracePeriod
	}()

	GRACE_PERIOD = 1 * time.Millisecond
	// Wait for expiration
	time.Sleep(time.Duration(ttl)*time.Second + GRACE_PERIOD + 50*time.Millisecond)

	// Run cleanup
	c.Clean()

	// Check results
	c.mu.RLock()
	var (
		_            *CacheEntry
		keepExists   bool
		expireExists bool
	)
	_, keepExists = c.entries["keep.com"]
	_, expireExists = c.entries["expire.com"]
	c.mu.RUnlock()

	if !keepExists {
		t.Error("Non-expired entry should be kept")
	}
	if expireExists {
		t.Error("Expired entry should be removed")
	}
}

// TEST 9: Stale entries trigger refresh flag
// Tests that Get returns needsRefresh=true for stale entries
func TestDNSCache_StaleRefreshFlag(t *testing.T) {
	var (
		cache    *DNSCache = NewDNSCache()
		key      string    = "stale.com"
		response []byte    = []byte("3.3.3.3")
		ttl      uint32    = 1
	)

	cache.Set(key, response, ttl)

	// Wait for entry to become stale (expired but within grace period)
	time.Sleep(time.Duration(ttl)*time.Second + 10*time.Millisecond)

	var (
		result       []byte
		found        bool
		needsRefresh bool
	)
	result, found, needsRefresh = cache.Get(key)

	if !found {
		t.Error("Stale entry should still be found")
	}
	if !needsRefresh {
		t.Error("Stale entry should trigger refresh flag")
	}
	if string(result) != string(response) {
		t.Error("Should return stale data while waiting for refresh")
	}
}

// TEST 10: Updating existing key doesn't grow cache
// Tests that setting an existing key updates it without eviction
func TestDNSCache_UpdateExistingKey(t *testing.T) {
	var (
		cache        *DNSCache = NewDNSCache()
		key          string    = "update.com"
		response1    []byte    = []byte("old-data")
		response2    []byte    = []byte("new-data")
		ttl          uint32    = 300
		initialCount int
		finalCount   int
	)

	cache.Set(key, response1, ttl)

	cache.mu.RLock()
	initialCount = len(cache.entries)
	cache.mu.RUnlock()

	// Update the same key
	cache.Set(key, response2, ttl)

	cache.mu.RLock()
	finalCount = len(cache.entries)
	cache.mu.RUnlock()

	if finalCount != initialCount {
		t.Error("Updating existing key should not change cache size")
	}

	var (
		result []byte
		found  bool
	)
	result, found, _ = cache.Get(key)

	if !found || string(result) != string(response2) {
		t.Error("Should retrieve updated value")
	}
}
