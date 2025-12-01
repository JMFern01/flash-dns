package filter

import (
	"strings"
	"sync"
)

type FilterList struct {
	mu      *sync.RWMutex
	domains map[string]bool
}

func NewFilterList() *FilterList {
	return &FilterList{domains: make(map[string]bool, 1024)}
}

func (f *FilterList) Add(domain string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	domain = strings.ToLower(strings.TrimSpace(domain))
	b.domains[domain] = true
}
