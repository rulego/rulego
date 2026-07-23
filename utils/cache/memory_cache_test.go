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
	"strings"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

func TestMemoryCache(t *testing.T) {
	c := NewMemoryCache(time.Minute)

	t.Run("SetAndGet", func(t *testing.T) {
		err := c.Set("key1", "value1", "1m")
		v, errGet := c.Get("key1")
		assert.Nil(t, errGet)
		assert.Equal(t, "value1", v)
		assert.Nil(t, err)

		// Test expiration time
		err = c.Set("key2", "value2", "1s")
		assert.Nil(t, err)
		time.Sleep(2 * time.Second)
		_, errGet = c.Get("key2")
		assert.Equal(t, types.ErrCacheMiss, errGet)
	})

	t.Run("Has", func(t *testing.T) {
		c.Set("key1", "value1", "1m")
		if !c.Has("key1") {
			t.Errorf("c.Has(\"key1\") should be true")
		}
		if c.Has("nonexistent") {
			t.Errorf("c.Has(\"nonexistent\") should be false")
		}

		// After the test expires, HaAs returns false
		c.Set("key2", "value2", "1s")
		time.Sleep(2 * time.Second)
		if c.Has("key2") {
			t.Errorf("c.Has(\"key2\") should be false after expiration")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		c.Set("key1", "value1", "1m")
		assert.Nil(t, c.Delete("key1"))
		_, err := c.Get("key1")
		assert.Equal(t, types.ErrCacheMiss, err)
		if c.Has("key1") {
			t.Errorf("c.Has(\"key1\") should be false after deletion")
		}
	})

	t.Run("DeleteByPrefix", func(t *testing.T) {
		c.Set("prefix_key1", "value1", "1m")
		c.Set("prefix_key2", "value2", "1m")
		c.Set("other_key", "value3", "1m")

		assert.Nil(t, c.DeleteByPrefix("prefix_"))
		_, err := c.Get("prefix_key1")
		assert.Equal(t, types.ErrCacheMiss, err)
		_, err = c.Get("prefix_key2")
		assert.Equal(t, types.ErrCacheMiss, err)

		v, err := c.Get("other_key")
		assert.Nil(t, err)
		assert.Equal(t, "value3", v)
	})

	t.Run("SetWithInvalidTTL", func(t *testing.T) {
		c := NewMemoryCache(time.Minute)
		err := c.Set("key_invalid_ttl", "value", "invalid-duration-string")
		assert.NotNil(t, err)
		_, err = c.Get("key_invalid_ttl")
		assert.Equal(t, types.ErrCacheMiss, err) // Should not be set
	})

}

func TestMemoryCache_GC_Lifecycle(t *testing.T) {
	c := NewMemoryCache(50 * time.Millisecond) // Use a short GC interval for testing

	t.Run("GCNotStartedWithoutExpirableItems", func(t *testing.T) {
		c.Set("key_no_expire", "value_no_expire", "") // No TTL, should not start GC
		c.mu.RLock()
		tickerRunning := c.ticker != nil
		c.mu.RUnlock()
		if tickerRunning {
			t.Errorf("GC should not be running without expirable items")
		}
	})

	t.Run("GCStartsWhenExpirableItemAdded", func(t *testing.T) {
		c.Set("key_expire_1", "value_expire_1", "100ms") // Expirable item
		// GC should start automatically due to the Set method's logic
		time.Sleep(60 * time.Millisecond) // Give GC a chance to start
		c.mu.RLock()
		tickerRunning := c.ticker != nil
		c.mu.RUnlock()
		if !tickerRunning {
			t.Errorf("GC should be running after adding an expirable item")
		}
	})

	t.Run("GCStopsWhenAllExpirableItemsGone", func(t *testing.T) {
		// Wait for key_expire_1 to expire and be collected
		time.Sleep(150 * time.Millisecond) // key_expire_1 (100ms) + gcInterval (50ms)

		c.mu.RLock()
		item1Exists := c.items["key_expire_1"].expiration > 0 && time.Now().UnixNano() < c.items["key_expire_1"].expiration
		tickerRunningAfterExpiry := c.ticker != nil
		c.mu.RUnlock()

		if item1Exists {
			t.Errorf("key_expire_1 should have expired and been collected")
		}
		// GC should stop because no expirable items are left (key_no_expire is non-expirable)
		if tickerRunningAfterExpiry {
			t.Errorf("GC should stop when no expirable items remain")
		}
	})

	t.Run("GCRestartsWhenNewExpirableItemAdded", func(t *testing.T) {
		c.Set("key_expire_2", "value_expire_2", "100ms") // Add another expirable item
		// GC should restart
		time.Sleep(60 * time.Millisecond) // Give GC a chance to start
		c.mu.RLock()
		tickerRunning := c.ticker != nil
		c.mu.RUnlock()
		if !tickerRunning {
			t.Errorf("GC should restart after adding a new expirable item")
		}
		c.StopGC() // Clean up GC for this test case
	})

	t.Run("GCStopsAfterStopGCCalled", func(t *testing.T) {
		cache := NewMemoryCache(50 * time.Millisecond)
		cache.Set("key_temp_expire", "value", "100ms") // Starts GC
		time.Sleep(60 * time.Millisecond)              // Ensure GC is running
		cache.mu.RLock()
		initialTickerState := cache.ticker != nil
		cache.mu.RUnlock()
		if !initialTickerState {
			t.Errorf("GC should be running initially")
		}

		cache.StopGC()
		time.Sleep(60 * time.Millisecond) // Allow time for GC to fully stop

		cache.mu.RLock()
		finalTickerState := cache.ticker != nil
		cache.mu.RUnlock()
		if finalTickerState {
			t.Errorf("GC should be stopped after StopGC() is called")
		}
	})

}

func TestMemoryCache_GetByPrefix(t *testing.T) {
	c := NewMemoryCache(time.Second)

	t.Run("EmptyPrefix", func(t *testing.T) {
		c.Set("key1", "value1", "1m")
		c.Set("key2", "value2", "1m")
		result := c.GetByPrefix("")
		assert.Equal(t, 2, len(result))
		assert.Equal(t, "value1", result["key1"])
		assert.Equal(t, "value2", result["key2"])
	})

	t.Run("FullMatchPrefix", func(t *testing.T) {
		c.Set("prefix_key1", "value1", "1m")
		c.Set("prefix_key2", "value2", "1m")
		c.Set("other_key", "value3", "1m")
		result := c.GetByPrefix("prefix_")
		assert.Equal(t, 2, len(result))
		assert.Equal(t, "value1", result["prefix_key1"])
		assert.Equal(t, "value2", result["prefix_key2"])
	})

	t.Run("PartialMatchPrefix", func(t *testing.T) {
		c.Set("prefix:sub1", "value1", "1m")
		c.Set("prefix:sub2", "value2", "1m")
		c.Set("other_key", "value3", "1m")
		result := c.GetByPrefix("prefix:")
		assert.Equal(t, 2, len(result))
		assert.Equal(t, "value1", result["prefix:sub1"])
		assert.Equal(t, "value2", result["prefix:sub2"])
	})

	t.Run("ExpiredItems", func(t *testing.T) {
		c.Set("prefix3_key1", "value1", "1s")
		time.Sleep(2 * time.Second)
		result := c.GetByPrefix("prefix3_")
		assert.Equal(t, 0, len(result))
	})
}

func TestNamespaceCache(t *testing.T) {
	// Create underlying caches and namespace caches
	baseCache := NewMemoryCache(time.Minute * 5)
	namespace := "test:"
	cache := NewNamespaceCache(baseCache, namespace)

	// Test Set and Get
	t.Run("SetAndGet", func(t *testing.T) {
		err := cache.Set("key1", "value1", "1m")
		assert.Nil(t, err)

		value, err := cache.Get("key1")
		assert.Nil(t, err)
		assert.Equal(t, "value1", value)

		// Verify that the underlying cache key is correctly prefixed
		baseValue, err := baseCache.Get(namespace + "key1")
		assert.Nil(t, err)
		assert.Equal(t, "value1", baseValue)
	})

	// Test Has
	t.Run("Has", func(t *testing.T) {
		if !cache.Has("key1") {
			t.Errorf("cache.Has(\"key1\") should be true")
		}
		if cache.Has("nonexistent") {
			t.Errorf("cache.Has(\"nonexistent\") should be false")
		}
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		err := cache.Delete("key1")
		assert.Nil(t, err)
		_, err = cache.Get("key1")
		assert.Equal(t, types.ErrCacheMiss, err)
		if cache.Has("key1") {
			t.Errorf("cache.Has(\"key1\") should be false after deletion")
		}
	})

	// Test DeleteByPrefix
	t.Run("DeleteByPrefix", func(t *testing.T) {
		// Add multiple keys with prefixes
		cache.Set("key2", "value2", "1m")
		cache.Set("key3", "value3", "1m")

		// Delete all keys with prefixes
		err := cache.DeleteByPrefix("")
		assert.Nil(t, err)

		// Verify that all keys have been deleted
		_, err = cache.Get("key2")
		assert.Equal(t, types.ErrCacheMiss, err)
		_, err = cache.Get("key3")
		assert.Equal(t, types.ErrCacheMiss, err)
		if cache.Has("key2") {
			t.Errorf("cache.Has(\"key2\") should be false after DeleteByPrefix")
		}
		if cache.Has("key3") {
			t.Errorf("cache.Has(\"key3\") should be false after DeleteByPrefix")
		}
	})

	// Test custom prefix deletion
	t.Run("DeleteWithCustomPrefix", func(t *testing.T) {
		cache.Set("sub:key4", "value4", "1m")
		cache.Set("sub:key5", "value5", "1m")

		// Delete keys with specific prefixes
		err := cache.DeleteByPrefix("sub:")
		assert.Nil(t, err)

		_, err = cache.Get("sub:key4")
		assert.Equal(t, types.ErrCacheMiss, err)
		_, err = cache.Get("sub:key5")
		assert.Equal(t, types.ErrCacheMiss, err)
	})

	// Test whether the key returned by GetByPrefix has correctly extracted the namespace prefix
	t.Run("GetByPrefixKeyFormat", func(t *testing.T) {
		cache.Set("prefix1", "value1", "1m")
		cache.Set("prefix2", "value2", "1m")
		cache.Set("prefix3", "value3", "1m")

		result := cache.GetByPrefix("")
		assert.Equal(t, 3, len(result))

		// Verify that the returned key does not contain the namespace prefix
		for k := range result {
			if len(k) >= len(namespace) && k[:len(namespace)] == namespace {
				t.Errorf("GetByPrefix returned key contains namespace prefix: %s", k)
			}
		}

		// Test with prefix query
		result = cache.GetByPrefix("pre")
		assert.Equal(t, 3, len(result))
		for k := range result {
			if !strings.HasPrefix(k, "pre") {
				t.Errorf("GetByPrefix returned key does not match prefix: %s", k)
			}
		}
	})

}
