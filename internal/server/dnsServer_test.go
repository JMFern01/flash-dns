package server

import (
	"context"
	"encoding/binary"
	"flash-dns/internal/filter"
	"flash-dns/internal/utils"
	"net"
	"testing"
	"time"
)

// ============================================================================
// MOCK IMPLEMENTATIONS
// ============================================================================

// MockResolver simulates upstream DNS resolution
type MockResolver struct {
	response  []byte
	err       error
	callCount int
}

func (m *MockResolver) Resolve(ctx context.Context, query []byte) ([]byte, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

// MockCache simulates cache operations
type MockCache struct {
	data         map[string][]byte
	getCallCount int
	setCallCount int
}

func NewMockCache() *MockCache {
	return &MockCache{
		data: make(map[string][]byte),
	}
}

func (m *MockCache) Get(key string) ([]byte, bool, bool) {
	m.getCallCount++
	var (
		value []byte
		found bool
	)
	value, found = m.data[key]
	return value, found, false // needsRefresh always false for simplicity
}

func (m *MockCache) Set(key string, response []byte, ttl uint32) {
	m.setCallCount++
	m.data[key] = response
}

func (m *MockCache) Clean() {
	// No-op for mock
}

// MockFilter simulates domain filtering
type MockFilter struct {
	blockedDomains map[string]bool
	count          int
}

func NewMockFilter() *MockFilter {
	return &MockFilter{
		blockedDomains: make(map[string]bool),
	}
}

func (m *MockFilter) IsBlocked(domain string) bool {
	var blocked bool
	_, blocked = m.blockedDomains[domain]
	return blocked
}

func (m *MockFilter) Count() int {
	return m.count
}

func (m *MockFilter) AddBlocked(domain string) {
	m.blockedDomains[domain] = true
	m.count++
}

// ============================================================================
// TESTS
// ============================================================================

// TEST 1: Create new DNS server
// Tests that server is initialized with correct components
func TestNewDNSServer(t *testing.T) {
	var (
		config Config = Config{
			LocalAddr:   "127.0.0.1:5353",
			UpstreamDns: "8.8.8.8:53",
			FilterMode:  "nxdomain",
		}
		resolver   *MockResolver      = &MockResolver{}
		filterList *filter.FilterList = filter.NewFilterList()
		server     *DNSServer
	)

	server = NewDNSServer(config, resolver, filterList)

	if server == nil {
		t.Fatal("Server should not be nil")
	}
	if server.cache == nil {
		t.Error("Cache should be initialized")
	}
	if server.filter == nil {
		t.Error("Filter should be initialized")
	}
	if server.resolver == nil {
		t.Error("Resolver should be initialized")
	}
	if server.statistics == nil {
		t.Error("Statistics should be initialized")
	}
}

// TEST 2: Filter blocked domain
// Tests that blocked domains return true
func TestDNSServer_FilterDomain_Blocked(t *testing.T) {
	var (
		config Config = Config{
			LocalAddr:   "127.0.0.1:5353",
			UpstreamDns: "8.8.8.8:53",
			FilterMode:  "nxdomain",
		}
		resolver   *MockResolver      = &MockResolver{}
		mockFilter *MockFilter        = NewMockFilter()
		filterList *filter.FilterList = filter.NewFilterList()
		server     *DNSServer
		domain     string = "ads.example.com"
		blocked    bool
	)

	mockFilter.AddBlocked(domain)
	server = NewDNSServer(config, resolver, filterList)
	server.filter = mockFilter

	blocked = server.filterDomain(domain)

	if !blocked {
		t.Error("Domain should be blocked")
	}

	var (
		blockedCount uint64
		_            uint64
	)
	blockedCount, _, _, _ = server.statistics.GetStats()

	if blockedCount != 1 {
		t.Errorf("Expected 1 blocked request, got %d", blockedCount)
	}
}

// TEST 3: Filter allowed domain
// Tests that non-blocked domains return false
func TestDNSServer_FilterDomain_Allowed(t *testing.T) {
	var (
		config Config = Config{
			LocalAddr:   "127.0.0.1:5353",
			UpstreamDns: "8.8.8.8:53",
		}
		resolver   *MockResolver      = &MockResolver{}
		mockFilter *MockFilter        = NewMockFilter()
		filterList *filter.FilterList = filter.NewFilterList()
		server     *DNSServer
		domain     string = "google.com"
		blocked    bool
	)

	server = NewDNSServer(config, resolver, filterList)
	server.filter = mockFilter

	blocked = server.filterDomain(domain)

	if blocked {
		t.Error("Domain should not be blocked")
	}
}

// TEST 4: Cache hit returns cached response
// Tests that cached entries are returned without querying upstream
func TestDNSServer_GetCache_Hit(t *testing.T) {
	var (
		config Config = Config{
			LocalAddr:   "127.0.0.1:5353",
			UpstreamDns: "8.8.8.8:53",
		}
		resolver     *MockResolver      = &MockResolver{}
		mockCache    *MockCache         = NewMockCache()
		filterList   *filter.FilterList = filter.NewFilterList()
		server       *DNSServer
		cacheKey     string = "example.com:1"
		domain       string = "example.com"
		cachedData   []byte = []byte("cached response")
		response     []byte
		found        bool
		needsRefresh bool
	)

	mockCache.Set(cacheKey, cachedData, 300)
	server = NewDNSServer(config, resolver, filterList)
	server.cache = mockCache

	response, found, needsRefresh = server.getCache(cacheKey, domain)

	if !found {
		t.Error("Should find cached entry")
	}
	if needsRefresh {
		t.Error("Should not need refresh for this test")
	}
	if string(response) != string(cachedData) {
		t.Error("Should return cached data")
	}

	var (
		_         uint64
		cacheHits uint64
	)
	_, _, cacheHits, _ = server.statistics.GetStats()

	if cacheHits != 1 {
		t.Errorf("Expected 1 cache hit, got %d", cacheHits)
	}
}

// TEST 5: Cache miss returns not found
// Tests that missing entries return false
func TestDNSServer_GetCache_Miss(t *testing.T) {
	var (
		config Config = Config{
			LocalAddr:   "127.0.0.1:5353",
			UpstreamDns: "8.8.8.8:53",
		}
		resolver   *MockResolver      = &MockResolver{}
		mockCache  *MockCache         = NewMockCache()
		filterList *filter.FilterList = filter.NewFilterList()
		server     *DNSServer
		cacheKey   string = "nonexistent.com:1"
		domain     string = "nonexistent.com"
		response   []byte
		found      bool
	)

	server = NewDNSServer(config, resolver, filterList)
	server.cache = mockCache

	response, found, _ = server.getCache(cacheKey, domain)

	if found {
		t.Error("Should not find non-existent entry")
	}
	if response != nil {
		t.Error("Response should be nil for cache miss")
	}
}

// TEST 6: Query upstream and cache result
// Tests that upstream queries are cached
func TestDNSServer_QueryUpstream(t *testing.T) {
	var (
		ctx    context.Context = context.Background()
		config Config          = Config{
			LocalAddr:   "127.0.0.1:5353",
			UpstreamDns: "8.8.8.8:53",
		}
		query        []byte             = buildDNSQuery("example.com", 1, 1)
		mockResponse []byte             = buildDNSResponse("example.com", 1, 1, 3600, []byte{1, 2, 3, 4})
		resolver     *MockResolver      = &MockResolver{response: mockResponse}
		mockCache    *MockCache         = NewMockCache()
		filterList   *filter.FilterList = filter.NewFilterList()
		server       *DNSServer
		queryInfo    *utils.QueryInfo = &utils.QueryInfo{
			Domain:   "example.com",
			CacheKey: "example.com:1",
			QType:    1,
			QClass:   1,
		}
		response []byte
		err      error
	)

	server = NewDNSServer(config, resolver, filterList)
	server.cache = mockCache
	server.resolver = resolver

	response, err = server.queryUpstream(ctx, query, queryInfo)

	if err != nil {
		t.Fatalf("QueryUpstream failed: %v", err)
	}
	if response == nil {
		t.Error("Response should not be nil")
	}
	if resolver.callCount != 1 {
		t.Errorf("Expected 1 resolver call, got %d", resolver.callCount)
	}
	if mockCache.setCallCount != 1 {
		t.Errorf("Expected 1 cache set call, got %d", mockCache.setCallCount)
	}
}

// TEST 7: Create blocked response - NXDOMAIN mode
// Tests that NXDOMAIN response is created in default mode
func TestDNSServer_CreateBlockedResponse_NXDOMAIN(t *testing.T) {
	var (
		config Config = Config{
			LocalAddr:   "127.0.0.1:5353",
			UpstreamDns: "8.8.8.8:53",
			FilterMode:  "nxdomain",
		}
		resolver   *MockResolver      = &MockResolver{}
		filterList *filter.FilterList = filter.NewFilterList()
		server     *DNSServer
		query      []byte = buildDNSQuery("blocked.com", 1, 1)
		response   []byte
		flags      uint16
	)

	server = NewDNSServer(config, resolver, filterList)

	response = server.createBlockedResponse(query)

	if len(response) == 0 {
		t.Error("Response should not be empty")
	}

	// Check flags for NXDOMAIN (0x8183)
	flags = binary.BigEndian.Uint16(response[2:4])
	if flags != 0x8183 {
		t.Errorf("Expected NXDOMAIN flags 0x8183, got 0x%04X", flags)
	}
}

// TEST 8: Create blocked response - NULL mode
// Tests that 0.0.0.0 response is created in null mode
func TestDNSServer_CreateBlockedResponse_Null(t *testing.T) {
	var (
		config Config = Config{
			LocalAddr:   "127.0.0.1:5353",
			UpstreamDns: "8.8.8.8:53",
			FilterMode:  "null",
		}
		resolver   *MockResolver      = &MockResolver{}
		filterList *filter.FilterList = filter.NewFilterList()
		server     *DNSServer
		query      []byte = buildDNSQuery("blocked.com", 1, 1)
		response   []byte
		flags      uint16
		ancount    uint16
	)

	server = NewDNSServer(config, resolver, filterList)

	response = server.createBlockedResponse(query)

	if len(response) == 0 {
		t.Error("Response should not be empty")
	}

	// Check flags for successful response (0x8180)
	flags = binary.BigEndian.Uint16(response[2:4])
	if flags != 0x8180 {
		t.Errorf("Expected success flags 0x8180, got 0x%04X", flags)
	}

	// Check answer count should be 1
	ancount = binary.BigEndian.Uint16(response[6:8])
	if ancount != 1 {
		t.Errorf("Expected 1 answer, got %d", ancount)
	}
}

// TEST 9: Handle query with blocked domain
// Tests end-to-end blocked domain handling
func TestDNSServer_HandleQuery_BlockedDomain(t *testing.T) {
	var (
		ctx    context.Context = context.Background()
		config Config          = Config{
			LocalAddr:   "127.0.0.1:5353",
			UpstreamDns: "8.8.8.8:53",
			FilterMode:  "nxdomain",
		}
		query      []byte             = buildDNSQuery("ads.example.com", 1, 1)
		resolver   *MockResolver      = &MockResolver{}
		mockFilter *MockFilter        = NewMockFilter()
		filterList *filter.FilterList = filter.NewFilterList()
		server     *DNSServer
		addr       *net.UDPAddr
		conn       *net.UDPConn
		err        error
	)

	mockFilter.AddBlocked("ads.example.com")
	server = NewDNSServer(config, resolver, filterList)
	server.filter = mockFilter

	// Create a test UDP connection
	addr, err = net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to resolve address: %v", err)
	}

	conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("Failed to create UDP connection: %v", err)
	}
	defer conn.Close()

	var clientAddr *net.UDPAddr = conn.LocalAddr().(*net.UDPAddr)

	// Handle the query
	server.handleQuery(ctx, query, clientAddr, conn)

	// Verify statistics
	var (
		blocked uint64
		_       uint64
	)
	blocked, _, _, _ = server.statistics.GetStats()

	if blocked != 1 {
		t.Errorf("Expected 1 blocked request, got %d", blocked)
	}
}

// TEST 10: Handle query with allowed domain and cache miss
// Tests that allowed domains query upstream
func TestDNSServer_HandleQuery_AllowedCacheMiss(t *testing.T) {
	var (
		ctx    context.Context = context.Background()
		config Config          = Config{
			LocalAddr:   "127.0.0.1:5353",
			UpstreamDns: "8.8.8.8:53",
		}
		query        []byte             = buildDNSQuery("google.com", 1, 1)
		mockResponse []byte             = buildDNSResponse("google.com", 1, 1, 3600, []byte{8, 8, 8, 8})
		resolver     *MockResolver      = &MockResolver{response: mockResponse}
		mockCache    *MockCache         = NewMockCache()
		filterList   *filter.FilterList = filter.NewFilterList()
		server       *DNSServer
		addr         *net.UDPAddr
		conn         *net.UDPConn
		err          error
	)

	server = NewDNSServer(config, resolver, filterList)
	server.resolver = resolver
	server.cache = mockCache

	addr, err = net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to resolve address: %v", err)
	}

	conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("Failed to create UDP connection: %v", err)
	}
	defer conn.Close()

	var clientAddr *net.UDPAddr = conn.LocalAddr().(*net.UDPAddr)

	server.handleQuery(ctx, query, clientAddr, conn)

	// Give goroutine time to complete
	time.Sleep(100 * time.Millisecond)

	// Verify resolver was called
	if resolver.callCount != 1 {
		t.Errorf("Expected 1 resolver call, got %d", resolver.callCount)
	}

	// Verify cache was updated
	if mockCache.setCallCount != 1 {
		t.Errorf("Expected 1 cache set, got %d", mockCache.setCallCount)
	}

	// Verify statistics
	var (
		_           uint64
		cacheMisses uint64
	)
	_, _, _, cacheMisses = server.statistics.GetStats()

	if cacheMisses != 1 {
		t.Errorf("Expected 1 cache miss, got %d", cacheMisses)
	}
}

// TEST 11: Refresh cache in background
// Tests that cache refresh works asynchronously
func TestDNSServer_RefreshCache(t *testing.T) {
	var (
		ctx    context.Context = context.Background()
		config Config          = Config{
			LocalAddr:   "127.0.0.1:5353",
			UpstreamDns: "8.8.8.8:53",
		}
		query        []byte             = buildDNSQuery("example.com", 1, 1)
		mockResponse []byte             = buildDNSResponse("example.com", 1, 1, 1800, []byte{1, 2, 3, 4})
		resolver     *MockResolver      = &MockResolver{response: mockResponse}
		mockCache    *MockCache         = NewMockCache()
		filterList   *filter.FilterList = filter.NewFilterList()
		server       *DNSServer
		queryInfo    *utils.QueryInfo = &utils.QueryInfo{
			Domain:   "example.com",
			CacheKey: "example.com:1",
			QType:    1,
			QClass:   1,
		}
	)

	server = NewDNSServer(config, resolver, filterList)
	server.resolver = resolver
	server.cache = mockCache

	server.refreshCache(ctx, query, queryInfo)

	// Give goroutine time to complete
	time.Sleep(100 * time.Millisecond)

	if resolver.callCount != 1 {
		t.Errorf("Expected 1 resolver call, got %d", resolver.callCount)
	}
	if mockCache.setCallCount != 1 {
		t.Errorf("Expected 1 cache set, got %d", mockCache.setCallCount)
	}
}

// TEST 12: Context cancellation stops operations
// Tests that cancelled context stops query processing
func TestDNSServer_QueryUpstream_ContextCancelled(t *testing.T) {
	var (
		ctx    context.Context
		cancel context.CancelFunc
		config Config = Config{
			LocalAddr:   "127.0.0.1:5353",
			UpstreamDns: "8.8.8.8:53",
		}
		query      []byte             = buildDNSQuery("example.com", 1, 1)
		resolver   *MockResolver      = &MockResolver{}
		filterList *filter.FilterList = filter.NewFilterList()
		server     *DNSServer
		queryInfo  *utils.QueryInfo = &utils.QueryInfo{
			Domain:   "example.com",
			CacheKey: "example.com:1",
		}
		response []byte
		err      error
	)

	ctx, cancel = context.WithCancel(context.Background())
	cancel() // Cancel immediately

	server = NewDNSServer(config, resolver, filterList)
	server.resolver = resolver

	response, err = server.queryUpstream(ctx, query, queryInfo)

	if response != nil {
		t.Error("Response should be nil for cancelled context")
	}
	if err != nil {
		t.Error("Error should be nil for cancelled context")
	}
}

// ============================================================================
// HELPER FUNCTIONS FOR BUILDING DNS PACKETS
// ============================================================================

func buildDNSQuery(domain string, qtype uint16, qclass uint16) []byte {
	var (
		query    []byte = make([]byte, 12)
		labels   []string
		i        int
		label    string
		labelLen int
	)

	binary.BigEndian.PutUint16(query[0:2], 0x1234)
	binary.BigEndian.PutUint16(query[4:6], 1)

	labels = splitDomain(domain)
	for i, label = range labels {
		_ = i
		labelLen = len(label)
		query = append(query, byte(labelLen))
		query = append(query, []byte(label)...)
	}
	query = append(query, 0)

	var typeClass []byte = make([]byte, 4)
	binary.BigEndian.PutUint16(typeClass[0:2], qtype)
	binary.BigEndian.PutUint16(typeClass[2:4], qclass)
	query = append(query, typeClass...)

	return query
}

func buildDNSResponse(domain string, qtype uint16, qclass uint16, ttl uint32, rdata []byte) []byte {
	var (
		response []byte = make([]byte, 12)
		labels   []string
		i        int
		label    string
		labelLen int
	)

	binary.BigEndian.PutUint16(response[0:2], 0x1234)
	binary.BigEndian.PutUint16(response[2:4], 0x8180)
	binary.BigEndian.PutUint16(response[4:6], 1)
	binary.BigEndian.PutUint16(response[6:8], 1)

	labels = splitDomain(domain)
	for i, label = range labels {
		_ = i
		labelLen = len(label)
		response = append(response, byte(labelLen))
		response = append(response, []byte(label)...)
	}
	response = append(response, 0)

	var typeClass []byte = make([]byte, 4)
	binary.BigEndian.PutUint16(typeClass[0:2], qtype)
	binary.BigEndian.PutUint16(typeClass[2:4], qclass)
	response = append(response, typeClass...)

	response = append(response, 0xC0, 0x0C)

	var answerData []byte = make([]byte, 10)
	binary.BigEndian.PutUint16(answerData[0:2], qtype)
	binary.BigEndian.PutUint16(answerData[2:4], qclass)
	binary.BigEndian.PutUint32(answerData[4:8], ttl)
	binary.BigEndian.PutUint16(answerData[8:10], uint16(len(rdata)))
	response = append(response, answerData...)
	response = append(response, rdata...)

	return response
}

func splitDomain(domain string) []string {
	var (
		labels []string
		start  int = 0
		i      int
		ch     rune
	)

	for i, ch = range domain {
		if ch == '.' {
			if i > start {
				labels = append(labels, domain[start:i])
			}
			start = i + 1
		}
	}

	if len(domain) > start {
		labels = append(labels, domain[start:])
	}

	return labels
}
