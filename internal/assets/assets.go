// Package assets handles game asset loading and caching.
package assets

import (
	"fmt"
	"sync"

	"github.com/Faultbox/midgard-ro/pkg/grf"
)

// Manager handles asset loading from GRF files.
type Manager struct {
	archives []*grf.Archive
	cache    *Cache
	mu       sync.RWMutex
}

// NewManager creates a new asset manager.
func NewManager() *Manager {
	return &Manager{
		cache: NewCache(),
	}
}

// AddArchive adds a GRF archive to the manager.
// Archives are searched in reverse order (last added = highest priority).
func (m *Manager) AddArchive(path string) error {
	archive, err := grf.Open(path)
	if err != nil {
		return fmt.Errorf("opening archive %s: %w", path, err)
	}

	m.mu.Lock()
	m.archives = append(m.archives, archive)
	m.mu.Unlock()

	return nil
}

// Load loads a file from the archives.
func (m *Manager) Load(path string) ([]byte, error) {
	// Check cache first
	if data, ok := m.cache.Get(path); ok {
		return data, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Search archives in reverse order
	for i := len(m.archives) - 1; i >= 0; i-- {
		data, err := m.archives[i].Read(path)
		if err == nil {
			m.cache.Set(path, data)
			return data, nil
		}
	}

	return nil, fmt.Errorf("file not found: %s", path)
}

// Close closes all archives.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, archive := range m.archives {
		archive.Close()
	}
	m.archives = nil
	m.cache.Clear()
}

// Cache is a simple in-memory cache for loaded assets.
type Cache struct {
	data map[string][]byte
	mu   sync.RWMutex

	// Stats
	hits   int
	misses int
}

// NewCache creates a new cache.
func NewCache() *Cache {
	return &Cache{
		data: make(map[string][]byte),
	}
}

// Get retrieves an item from cache.
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, ok := c.data[key]
	if ok {
		c.hits++
	} else {
		c.misses++
	}
	return data, ok
}

// Set stores an item in cache.
func (c *Cache) Set(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = data
}

// Clear clears the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string][]byte)
	c.hits = 0
	c.misses = 0
}

// Stats returns cache statistics.
func (c *Cache) Stats() (hits, misses int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses
}
