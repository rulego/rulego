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

	"github.com/rulego/rulego/test/assert"
)

func TestMemoryCache(t *testing.T) {
	c := NewMemoryCache(time.Minute)

	t.Run("SetAndGet", func(t *testing.T) {
		err := c.Set("key1", "value1", "1m")
		assert.Equal(t, "value1", c.Get("key1"))
		assert.Nil(t, err)

		// 测试过期时间
		err = c.Set("key2", "value2", "1s")
		assert.Nil(t, err)
		time.Sleep(2 * time.Second)
		assert.Nil(t, c.Get("key2"))
	})

	t.Run("Has", func(t *testing.T) {
		c.Set("key1", "value1", "1m")
		if !c.Has("key1") {
			t.Errorf("c.Has(\"key1\") should be true")
		}
		if c.Has("nonexistent") {
			t.Errorf("c.Has(\"nonexistent\") should be false")
		}

		// 测试过期后Has返回false
		c.Set("key2", "value2", "1s")
		time.Sleep(2 * time.Second)
		if c.Has("key2") {
			t.Errorf("c.Has(\"key2\") should be false after expiration")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		c.Set("key1", "value1", "1m")
		assert.Nil(t, c.Delete("key1"))
		assert.Nil(t, c.Get("key1"))
		if c.Has("key1") {
			t.Errorf("c.Has(\"key1\") should be false after deletion")
		}
	})

	t.Run("DeleteByPrefix", func(t *testing.T) {
		c.Set("prefix_key1", "value1", "1m")
		c.Set("prefix_key2", "value2", "1m")
		c.Set("other_key", "value3", "1m")

		assert.Nil(t, c.DeleteByPrefix("prefix_"))
		assert.Nil(t, c.Get("prefix_key1"))
		assert.Nil(t, c.Get("prefix_key2"))
		assert.Equal(t, "value3", c.Get("other_key"))
	})

	t.Run("SetWithInvalidTTL", func(t *testing.T) {
		c := NewMemoryCache(time.Minute)
		err := c.Set("key_invalid_ttl", "value", "invalid-duration-string")
		assert.NotNil(t, err)
		assert.Nil(t, c.Get("key_invalid_ttl")) // Should not be set
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
	// 创建底层缓存和命名空间缓存
	baseCache := NewMemoryCache(time.Minute * 5)
	namespace := "test:"
	cache := NewNamespaceCache(baseCache, namespace)

	// 测试Set和Get
	t.Run("SetAndGet", func(t *testing.T) {
		err := cache.Set("key1", "value1", "1m")
		assert.Nil(t, err)

		value := cache.Get("key1")
		assert.Equal(t, "value1", value)

		// 验证底层缓存key是否正确添加前缀
		baseValue := baseCache.Get(namespace + "key1")
		assert.Equal(t, "value1", baseValue)
	})

	// 测试Has
	t.Run("Has", func(t *testing.T) {
		if !cache.Has("key1") {
			t.Errorf("cache.Has(\"key1\") should be true")
		}
		if cache.Has("nonexistent") {
			t.Errorf("cache.Has(\"nonexistent\") should be false")
		}
	})

	// 测试Delete
	t.Run("Delete", func(t *testing.T) {
		err := cache.Delete("key1")
		assert.Nil(t, err)
		assert.Nil(t, cache.Get("key1"))
		if cache.Has("key1") {
			t.Errorf("cache.Has(\"key1\") should be false after deletion")
		}
	})

	// 测试DeleteByPrefix
	t.Run("DeleteByPrefix", func(t *testing.T) {
		// 添加多个带前缀的key
		cache.Set("key2", "value2", "1m")
		cache.Set("key3", "value3", "1m")

		// 删除所有带前缀的key
		err := cache.DeleteByPrefix("")
		assert.Nil(t, err)

		// 验证所有key已被删除
		assert.Nil(t, cache.Get("key2"))
		assert.Nil(t, cache.Get("key3"))
		if cache.Has("key2") {
			t.Errorf("cache.Has(\"key2\") should be false after DeleteByPrefix")
		}
		if cache.Has("key3") {
			t.Errorf("cache.Has(\"key3\") should be false after DeleteByPrefix")
		}
	})

	// 测试自定义前缀删除
	t.Run("DeleteWithCustomPrefix", func(t *testing.T) {
		cache.Set("sub:key4", "value4", "1m")
		cache.Set("sub:key5", "value5", "1m")

		// 删除特定前缀的key
		err := cache.DeleteByPrefix("sub:")
		assert.Nil(t, err)

		assert.Nil(t, cache.Get("sub:key4"))
		assert.Nil(t, cache.Get("sub:key5"))
	})

	// 测试GetByPrefix返回的key是否已正确截取命名空间前缀
	t.Run("GetByPrefixKeyFormat", func(t *testing.T) {
		cache.Set("prefix1", "value1", "1m")
		cache.Set("prefix2", "value2", "1m")
		cache.Set("prefix3", "value3", "1m")

		result := cache.GetByPrefix("")
		assert.Equal(t, 3, len(result))

		// 验证返回的key不包含命名空间前缀
		for k := range result {
			if len(k) >= len(namespace) && k[:len(namespace)] == namespace {
				t.Errorf("GetByPrefix returned key contains namespace prefix: %s", k)
			}
		}

		// 测试带前缀查询
		result = cache.GetByPrefix("pre")
		assert.Equal(t, 3, len(result))
		for k := range result {
			if !strings.HasPrefix(k, "pre") {
				t.Errorf("GetByPrefix returned key does not match prefix: %s", k)
			}
		}
	})

}
