package cora

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ToolCache provides result caching for tool executions to avoid redundant calls.
type ToolCache struct {
	mu      sync.RWMutex
	cache   map[string]*cachedToolResult
	ttl     time.Duration
	maxSize int
	hits    int64
	misses  int64
}

type cachedToolResult struct {
	result    any
	err       error
	timestamp time.Time
}

// NewToolCache creates a new tool result cache with the specified TTL and max size.
func NewToolCache(ttl time.Duration, maxSize int) *ToolCache {
	return &ToolCache{
		cache:   make(map[string]*cachedToolResult),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// cacheKey generates a deterministic key from tool name and arguments.
func (tc *ToolCache) cacheKey(name string, args map[string]any) (string, error) {
	// Normalize args to JSON for consistent hashing
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("failed to marshal args for cache key: %w", err)
	}
	
	// Create hash of name + args
	h := sha256.New()
	h.Write([]byte(name))
	h.Write(argsJSON)
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Get retrieves a cached result if available and not expired.
func (tc *ToolCache) Get(name string, args map[string]any) (any, error, bool) {
	key, err := tc.cacheKey(name, args)
	if err != nil {
		return nil, nil, false
	}

	tc.mu.RLock()
	defer tc.mu.RUnlock()

	cached, exists := tc.cache[key]
	if !exists {
		tc.misses++
		return nil, nil, false
	}

	// Check if expired
	if time.Since(cached.timestamp) > tc.ttl {
		tc.misses++
		return nil, nil, false
	}

	tc.hits++
	return cached.result, cached.err, true
}

// Set stores a tool execution result in the cache.
func (tc *ToolCache) Set(name string, args map[string]any, result any, err error) {
	key, keyErr := tc.cacheKey(name, args)
	if keyErr != nil {
		return // Skip caching if we can't generate a key
	}

	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Evict oldest entry if cache is full
	if len(tc.cache) >= tc.maxSize {
		var oldestKey string
		var oldestTime time.Time
		for k, v := range tc.cache {
			if oldestTime.IsZero() || v.timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.timestamp
			}
		}
		if oldestKey != "" {
			delete(tc.cache, oldestKey)
		}
	}

	tc.cache[key] = &cachedToolResult{
		result:    result,
		err:       err,
		timestamp: time.Now(),
	}
}

// Stats returns cache hit/miss statistics.
func (tc *ToolCache) Stats() (hits, misses int64) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.hits, tc.misses
}

// Clear removes all cached entries.
func (tc *ToolCache) Clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cache = make(map[string]*cachedToolResult)
	tc.hits = 0
	tc.misses = 0
}