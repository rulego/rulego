/*
 * Copyright 2025 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cache

import (
	"sync"
	"time"

	"github.com/rulego/rulego/api/types"
)

var DefaultCache = NewMemoryCache(time.Minute * 5)

// MemoryCache is an in-memory cache implementation.
// It stores key-value pairs with optional expiration.
type MemoryCache struct {
	items      map[string]item
	mu         sync.RWMutex
	stopGc     chan struct{} // Channel to signal GC to stop
	ticker     *time.Ticker  // Ticker for GC
	gcInterval time.Duration // GC interval duration
}

// item represents a cached item with its value and expiration time.
// The expiration time is stored as Unix nano timestamp (int64).
// If expiration is 0, the item will never expire.
type item struct {
	value      interface{}
	expiration int64
}

// NewMemoryCache creates a new MemoryCache instance.
// The returned cache is initialized with:
// - An empty items map
// - A stopGc channel for controlling garbage collection
// Note: Garbage collection is not started automatically, call StartGC() to enable it.
func NewMemoryCache(gcInterval time.Duration) *MemoryCache {
	c := &MemoryCache{
		items:      make(map[string]item),
		stopGc:     make(chan struct{}),
		gcInterval: time.Minute * 5, // Default 5 minute
	}
	if gcInterval > 0 {
		c.gcInterval = gcInterval
	}
	// GC is no longer started automatically
	return c
}

// Set stores a value in the cache with the given key and an optional expiration duration.
// Parameters:
//   - key: The cache key (string)
//   - value: The value to store (interface{})
//   - ttl: Time-to-live duration as string (e.g. "10m", "1h")
//
// Returns:
//   - error if ttl parsing fails
//
// If ttl is 0, the item will not expire.
// ttl should be a string (e.g. "10m").
func (c *MemoryCache) Set(key string, value interface{}, ttl string) error {
	var expiration int64
	var dur time.Duration
	var err error

	if ttl != "" {
		dur, err = time.ParseDuration(ttl)
		if err != nil {
			return err
		}
	}

	if dur > 0 {
		expiration = time.Now().Add(dur).UnixNano()
	}

	c.mu.Lock()
	c.items[key] = item{
		value:      value,
		expiration: expiration,
	}
	// If an expirable item was added and GC is not running (ticker is nil),
	// set flag to start GC after releasing the lock.
	shouldStartGC := expiration > 0 && c.ticker == nil
	c.mu.Unlock()

	if shouldStartGC {
		c.StartGC() // StartGC handles its own locking
	}

	return nil
}

// Get retrieves a value from the cache by its key.
// Parameters:
//   - key: The cache key to retrieve (string)
//
// Returns:
//   - value: The stored value if found and not expired
//   - bool: true if value exists and is valid, false otherwise
//
// It returns the value and true if the key exists and has not expired, otherwise it returns nil and false.
func (c *MemoryCache) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	it, found := c.items[key]
	if !found {
		return nil
	}

	if it.expiration > 0 && time.Now().UnixNano() > it.expiration {
		// Item has expired
		// We can also delete it here, but the GC will take care of it
		return nil
	}

	return it.value
}

// Has checks if a prefixed key exists in the cache
// Parameters:
//   - key: Cache key (will be automatically prefixed)
//
// Returns:
//   - bool: Whether the key exists and is valid
func (c *MemoryCache) Has(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	it, found := c.items[key]
	if !found {
		return false
	}

	if it.expiration > 0 && time.Now().UnixNano() > it.expiration {
		return false
	}

	return true
}

func (c *MemoryCache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
	return nil
}

// DeleteByPrefix removes all cache items with the given prefix.
// Parameters:
//   - prefix: The key prefix to match (string)
//
// Returns:
//   - error: Always nil in current implementation
func (c *MemoryCache) DeleteByPrefix(prefix string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.items {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.items, k)
		}
	}
	return nil
}

// GetByPrefix retrieves all values with keys matching the specified prefix
// Parameters:
//   - prefix: key prefix to match (string)
//
// Returns:
//   - map[string]interface{}: map of matching key-value pairs
func (c *MemoryCache) GetByPrefix(prefix string) map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]interface{})
	now := time.Now().UnixNano()

	for k, v := range c.items {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			if v.expiration == 0 || now <= v.expiration {
				result[k] = v.value
			}
		}
	}

	return result
}

// StartGC starts the garbage collection process if not already running and if there are expirable items.
// It runs a goroutine that periodically checks for expired items (every c.gcInterval).
// If GC is already running, or if there are no items with an expiration time, this is a no-op.
func (c *MemoryCache) StartGC() {
	c.mu.Lock()
	if c.ticker != nil { // GC already running
		c.mu.Unlock()
		return
	}

	// Check if there are any expirable items. If not, don't start GC.
	hasExpirable := false
	for _, itm := range c.items {
		if itm.expiration > 0 {
			hasExpirable = true
			break
		}
	}

	if !hasExpirable {
		c.mu.Unlock()
		return
	}

	c.ticker = time.NewTicker(c.gcInterval) // Use configured interval
	c.stopGc = make(chan struct{})          // Create new stopGc channel
	c.mu.Unlock()

	go func() {
		for {
			select {
			case <-c.ticker.C:
				c.deleteExpired()
			case <-c.stopGc:
				c.ticker.Stop()
				c.mu.Lock()
				c.ticker = nil // Mark GC as stopped
				c.mu.Unlock()
				return
			}
		}
	}()
}

// StopGC stops the garbage collection process.
// It sends a signal to the GC goroutine to stop.
// If GC is not running, this is a no-op.
// This function is safe to call multiple times; it will only attempt to close the stop channel once.
func (c *MemoryCache) StopGC() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ticker != nil && c.stopGc != nil { // If GC is effectively running and stop channel exists
		select {
		case <-c.stopGc:
			// Channel already closed or signal already sent.
		default:
			// Channel is open, close it to signal the GC goroutine.
			close(c.stopGc)
		}
		// The GC goroutine is responsible for stopping the ticker and setting c.ticker = nil.
	}
}

// deleteExpired removes all expired items from the cache.
// This is called periodically by the GC goroutine.
// It locks the cache during operation to ensure thread safety.

// deleteExpired removes all expired items from the cache.
// This is called periodically by the GC goroutine.
// It first collects all expired keys under a read lock to minimize write lock contention,
// then deletes them in batches under a write lock, re-checking expiration status before deletion.
// After cleaning, it checks if GC should be paused.
func (c *MemoryCache) deleteExpired() {
	now := time.Now().UnixNano() // Use this timestamp for the entire deletion cycle.

	// Step 1: Collect all expired keys under a read lock.
	// This allows other read operations to proceed concurrently.
	c.mu.RLock()
	var expiredKeys []string
	for k, v := range c.items {
		if v.expiration > 0 && now > v.expiration {
			expiredKeys = append(expiredKeys, k)
		}
	}
	c.mu.RUnlock()

	if len(expiredKeys) == 0 {
		return
	}

	// Step 2: Delete collected keys in batches under a write lock.
	// Batching helps to avoid holding the write lock for too long if there are many expired keys.
	const batchSize = 300 // Number of keys to delete in each batch.
	for i := 0; i < len(expiredKeys); i += batchSize {
		c.mu.Lock()
		// Determine the end of the current batch
		end := i + batchSize
		if end > len(expiredKeys) {
			end = len(expiredKeys)
		}
		currentBatch := expiredKeys[i:end]

		for _, k := range currentBatch {
			// Re-check if the item still exists and is still expired.
			// This is important because the item might have been updated or deleted
			// by another goroutine between the RUnlock (after collecting keys) and this Lock.
			if item, found := c.items[k]; found && item.expiration > 0 && now > item.expiration {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
		// Consider a small sleep here if GC is aggressive or many batches,
		// to yield to other goroutines, e.g., time.Sleep(time.Millisecond).
		// For now, keeping it simple without the sleep.
	}

	// After deleting expired items, check if there are any expirable items left.
	c.mu.RLock()
	hasExpirableRemaining := false
	for _, itm := range c.items {
		if itm.expiration > 0 { // If any item has a non-zero expiration, GC should continue.
			hasExpirableRemaining = true
			break
		}
	}
	c.mu.RUnlock()

	if !hasExpirableRemaining {
		c.StopGC() // If no expirable items left, stop the GC.
	}
}

// NamespaceCache is a namespace-based cache wrapper
// It implements the Cache interface, adding namespace prefix functionality to the underlying cache
// Key features:
// - Automatically adds specified prefix to all operations
// - Supports batch deletion by prefix
// - Thread-safe (depends on underlying cache implementation)
// It prepends all keys in the underlying cache with a specified namespace for isolation
// Suitable for scenarios requiring cache data separation between different businesses or modules
type NamespaceCache struct {
	Cache     types.Cache // 底层缓存实现
	Namespace string      // 命名空间前缀
}

// NewNamespaceCache creates a new namespace-based cache instance
// Parameters:
//   - cache: Underlying cache implementation (must implement Cache interface)
//   - namespace: Namespace string to use
//
// Returns:
//   - *NamespaceCache: New namespace cache instance
//   - nil: If cache parameter is nil
func NewNamespaceCache(cache types.Cache, namespace string) *NamespaceCache {
	if cache == nil {
		return nil
	}
	return &NamespaceCache{
		Cache:     cache,
		Namespace: namespace,
	}
}

// Set stores a namespace-prefixed key-value pair in the cache
// Parameters:
//   - key: Cache key (will be automatically prefixed)
//   - value: Value to store
//   - ttl: Expiration duration string (e.g. "10m", "1h")
//
// Returns:
//   - error: If underlying cache returns error or cache is not initialized
func (c *NamespaceCache) Set(key string, value interface{}, ttl string) error {
	if c == nil || c.Cache == nil {
		return types.ErrCacheNotInitialized
	}
	return c.Cache.Set(c.Namespace+key, value, ttl)
}

// Get retrieves the value for a prefixed key
// Parameters:
//   - key: Cache key (will be automatically prefixed)
//
// Returns:
//   - interface{}: Stored value
//   - bool: Whether a valid value was found
func (c *NamespaceCache) Get(key string) interface{} {
	if c == nil || c.Cache == nil {
		return nil
	}
	return c.Cache.Get(c.Namespace + key)
}

// Delete removes a prefixed key from the cache
// Parameters:
//   - key: Cache key (will be automatically prefixed)
//
// Returns:
//   - error: If underlying cache returns error or cache is not initialized
func (c *NamespaceCache) Delete(key string) error {
	if c == nil || c.Cache == nil {
		return types.ErrCacheNotInitialized
	}
	return c.Cache.Delete(c.Namespace + key)
}

// Has checks if a prefixed key exists in the cache
// Parameters:
//   - key: Cache key (will be automatically prefixed)
//
// Returns:
//   - bool: Whether the key exists and is valid
func (c *NamespaceCache) Has(key string) bool {
	if c == nil || c.Cache == nil {
		return false
	}
	return c.Cache.Has(c.Namespace + key)
}

// DeleteByPrefix removes all cache items matching the specified prefix
// Parameters:
//   - prefix: Prefix to delete, must match the namespace prefix
//
// Returns:
//   - error: If underlying cache returns error or cache is not initialized
//     Returns types.ErrCacheNotInitialized if cache is nil
func (c *NamespaceCache) DeleteByPrefix(prefix string) error {
	if c == nil || c.Cache == nil {
		return types.ErrCacheNotInitialized
	}
	return c.Cache.DeleteByPrefix(c.Namespace + prefix)
}

func (c *NamespaceCache) GetByPrefix(prefix string) map[string]interface{} {
	if c == nil || c.Cache == nil {
		return map[string]interface{}{}
	}
	result := c.Cache.GetByPrefix(c.Namespace + prefix)
	newResult := make(map[string]interface{})
	for k, v := range result {
		if len(k) > len(c.Namespace) {
			newResult[k[len(c.Namespace):]] = v
		}
	}
	return newResult
}

// Ensure NamespaceCache implements the Cache interface.
var _ types.Cache = (*NamespaceCache)(nil)

// Ensure MemoryCache implements the Cache interface.
var _ types.Cache = (*MemoryCache)(nil)
