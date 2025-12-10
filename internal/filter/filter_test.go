package filter

import (
	"encoding/binary"
	"os"
	"testing"
)

// TEST 1: Basic Add and IsBlocked
// Tests that we can add a domain and check if it's blocked
func TestFilterList_BasicAddAndBlock(t *testing.T) {
	var (
		f      *FilterList = NewFilterList()
		domain string      = "example.com"
	)

	f.Add(domain)

	var blocked bool = f.IsBlocked(domain)
	if !blocked {
		t.Errorf("Domain %s should be blocked", domain)
	}
}

// TEST 2: Non-blocked domain returns false
// Verifies that domains not in the list are not blocked
func TestFilterList_NonBlockedDomain(t *testing.T) {
	var (
		f      *FilterList = NewFilterList()
		domain string      = "safe.com"
	)

	var blocked bool = f.IsBlocked(domain)
	if blocked {
		t.Errorf("Domain %s should not be blocked", domain)
	}
}

// TEST 3: Wildcard matching - subdomain blocking
// Tests that blocking parent domain blocks all subdomains
func TestFilterList_WildcardMatching(t *testing.T) {
	var (
		f          *FilterList = NewFilterList()
		parent     string      = "ads.com"
		subdomain1 string      = "tracker.ads.com"
		subdomain2 string      = "analytics.tracker.ads.com"
	)

	f.Add(parent)

	if !f.IsBlocked(parent) {
		t.Error("Parent domain should be blocked")
	}
	if !f.IsBlocked(subdomain1) {
		t.Error("Subdomain should be blocked when parent is blocked")
	}
	if !f.IsBlocked(subdomain2) {
		t.Error("Deep subdomain should be blocked when parent is blocked")
	}
}

// TEST 4: Domain normalization
// Tests that domains are normalized (lowercase, trimmed, no trailing dot)
func TestFilterList_DomainNormalization(t *testing.T) {
	var (
		f       *FilterList = NewFilterList()
		domain1 string      = "EXAMPLE.COM"
		domain2 string      = "  example.com  "
		domain3 string      = "example.com."
		check   string      = "example.com"
	)

	f.Add(domain1)

	if !f.IsBlocked(check) {
		t.Error("Uppercase domain should be normalized and blocked")
	}
	if !f.IsBlocked(domain2) {
		t.Error("Domain with spaces should be normalized and blocked")
	}
	if !f.IsBlocked(domain3) {
		t.Error("Domain with trailing dot should be normalized and blocked")
	}
}

// TEST 5: Count returns correct number of domains
// Tests that Count() returns the number of unique domains
func TestFilterList_Count(t *testing.T) {
	var (
		f        *FilterList = NewFilterList()
		expected int         = 3
	)

	f.Add("domain1.com")
	f.Add("domain2.com")
	f.Add("domain3.com")
	f.Add("domain1.com") // Duplicate, should not increase count

	var count int = f.Count()
	if count != expected {
		t.Errorf("Expected count %d, got %d", expected, count)
	}
}

// TEST 6: Load from file with valid AdBlock format
// Tests loading domains from a file with AdBlock Plus format
func TestFilterList_LoadFromFile(t *testing.T) {
	var (
		f           *FilterList = NewFilterList()
		filename    string      = "test_blocklist.txt"
		fileContent string      = `! Comment line
[AdBlock Plus]
||ads.example.com^
||tracker.com^
! Another comment
@@whitelist.com^
||malware.net^

invalid line without format
||analytics.example.org^
`
		file *os.File
		err  error
	)

	// Create temporary test file
	file, err = os.Create(filename)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	_, err = file.WriteString(fileContent)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	file.Close()
	defer os.Remove(filename)

	// Load the file
	err = f.LoadFromFile(filename)
	if err != nil {
		t.Fatalf("Failed to load file: %v", err)
	}

	// Verify loaded domains
	var (
		blocked1 bool = f.IsBlocked("ads.example.com")
		blocked2 bool = f.IsBlocked("tracker.com")
		blocked3 bool = f.IsBlocked("malware.net")
		blocked4 bool = f.IsBlocked("analytics.example.org")
		notFound bool = f.IsBlocked("whitelist.com")
	)

	if !blocked1 {
		t.Error("ads.example.com should be blocked")
	}
	if !blocked2 {
		t.Error("tracker.com should be blocked")
	}
	if !blocked3 {
		t.Error("malware.net should be blocked")
	}
	if !blocked4 {
		t.Error("analytics.example.org should be blocked")
	}
	if notFound {
		t.Error("whitelist.com should not be blocked (@@prefix)")
	}

	// Verify count (should have 4 domains)
	var count int = f.Count()
	if count != 4 {
		t.Errorf("Expected 4 domains loaded, got %d", count)
	}
}

// TEST 7: Load from non-existent file returns error
// Tests that loading from missing file returns error
func TestFilterList_LoadFromNonExistentFile(t *testing.T) {
	var (
		f        *FilterList = NewFilterList()
		filename string      = "nonexistent_file.txt"
		err      error
	)

	err = f.LoadFromFile(filename)
	if err == nil {
		t.Error("Should return error for non-existent file")
	}
}

// TEST 8: CreateBlockedResponse sets correct flags
// Tests that blocked response has proper DNS flags (NXDOMAIN)
func TestCreateBlockedResponse(t *testing.T) {
	var (
		query    []byte = make([]byte, 12)
		response []byte
		flags    uint16
		ancount  uint16
	)

	// Set up a minimal DNS query
	binary.BigEndian.PutUint16(query[0:2], 0x1234) // Transaction ID

	response = CreateBlockedResponse(query)

	// Verify response structure
	if len(response) != len(query) {
		t.Errorf("Response length should equal query length")
	}

	// Check flags (should be 0x8183: QR=1, RCODE=3)
	flags = binary.BigEndian.Uint16(response[2:4])
	if flags != 0x8183 {
		t.Errorf("Expected flags 0x8183, got 0x%04X", flags)
	}

	// Check answer count (should be 0)
	ancount = binary.BigEndian.Uint16(response[6:8])
	if ancount != 0 {
		t.Errorf("Expected answer count 0, got %d", ancount)
	}

	// Transaction ID should be preserved
	var txID uint16 = binary.BigEndian.Uint16(response[0:2])
	if txID != 0x1234 {
		t.Error("Transaction ID should be preserved")
	}
}

// TEST 9: CreateBlockedResponse handles short queries
// Tests that short queries are returned unchanged
func TestCreateBlockedResponse_ShortQuery(t *testing.T) {
	var (
		query    []byte = make([]byte, 5) // Less than 12 bytes
		response []byte
	)

	response = CreateBlockedResponse(query)

	if len(response) != len(query) {
		t.Error("Short query should be returned unchanged")
	}
}

// TEST 10: CreateNullResponse returns 0.0.0.0
// Tests that null response contains A record pointing to 0.0.0.0
func TestCreateNullResponse(t *testing.T) {
	var (
		query    []byte = make([]byte, 12)
		response []byte
		flags    uint16
		ancount  uint16
		position int
	)

	// Set up a minimal DNS query
	binary.BigEndian.PutUint16(query[0:2], 0x5678) // Transaction ID

	response = CreateNullResponse(query)

	// Response should be longer than query (query + answer record)
	if len(response) <= len(query) {
		t.Error("Null response should be longer than query")
	}

	// Check flags (should be 0x8180: QR=1, RCODE=0)
	flags = binary.BigEndian.Uint16(response[2:4])
	if flags != 0x8180 {
		t.Errorf("Expected flags 0x8180, got 0x%04X", flags)
	}

	// Check answer count (should be 1)
	ancount = binary.BigEndian.Uint16(response[6:8])
	if ancount != 1 {
		t.Errorf("Expected answer count 1, got %d", ancount)
	}

	// Verify the answer section starts with pointer (0xC00C)
	position = len(query)
	if response[position] != 0xC0 || response[position+1] != 0x0C {
		t.Error("Answer should start with name pointer 0xC00C")
	}

	// Verify Type A (0x0001)
	position += 2
	var recordType uint16 = binary.BigEndian.Uint16(response[position : position+2])
	if recordType != 1 {
		t.Errorf("Expected record type A (1), got %d", recordType)
	}

	// Verify IP address is 0.0.0.0 (last 4 bytes)
	var ipStart int = len(response) - 4
	if response[ipStart] != 0 || response[ipStart+1] != 0 ||
		response[ipStart+2] != 0 || response[ipStart+3] != 0 {
		t.Error("IP address should be 0.0.0.0")
	}

	// Transaction ID should be preserved
	var txID uint16 = binary.BigEndian.Uint16(response[0:2])
	if txID != 0x5678 {
		t.Error("Transaction ID should be preserved")
	}
}

// TEST 11: CreateNullResponse handles short queries
// Tests that short queries are returned unchanged
func TestCreateNullResponse_ShortQuery(t *testing.T) {
	var (
		query    []byte = make([]byte, 8) // Less than 12 bytes
		response []byte
	)

	response = CreateNullResponse(query)

	if len(response) != len(query) {
		t.Error("Short query should be returned unchanged")
	}
}

// TEST 12: Wildcard doesn't match unrelated domains
// Tests that blocking a domain doesn't block unrelated domains
func TestFilterList_WildcardNonMatch(t *testing.T) {
	var (
		f             *FilterList = NewFilterList()
		blocked       string      = "ads.com"
		unrelated     string      = "example.com"
		similarButNot string      = "adsnotblocked.com"
	)

	f.Add(blocked)

	if f.IsBlocked(unrelated) {
		t.Error("Unrelated domain should not be blocked")
	}
	if f.IsBlocked(similarButNot) {
		t.Error("Similar but different domain should not be blocked")
	}
}

// TEST 13: normalizeDomain function
// Tests the normalizeDomain helper function directly
func TestNormalizeDomain(t *testing.T) {
	var tests = []struct {
		input    string
		expected string
	}{
		{"EXAMPLE.COM", "example.com"},
		{"  example.com  ", "example.com"},
		{"example.com.", "example.com"},
		{"  EXAMPLE.COM.  ", "example.com"},
		{"ExAmPlE.CoM", "example.com"},
	}

	var (
		i    int
		test struct {
			input    string
			expected string
		}
		result string
	)

	for i, test = range tests {
		result = normalizeDomain(test.input)
		if result != test.expected {
			t.Errorf("Test %d: normalizeDomain(%q) = %q, want %q",
				i, test.input, result, test.expected)
		}
	}
}
