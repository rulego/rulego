/*
 * Copyright 2024 The RuleGo Authors.
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

package engine

import (
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/str"
	"testing"
)

// TestCache 测试缓存
func TestCache(t *testing.T) {
	config := NewConfig()
	ruleEngine, err := New(str.RandomStr(10), []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(ruleEngine.Id())
	rootCtx := ruleEngine.RootRuleContext().(*DefaultRuleContext)
	rootCtxCopy := NewRuleContext(rootCtx.GetContext(), rootCtx.config, rootCtx.ruleChainCtx, rootCtx.from, rootCtx.self, rootCtx.pool, rootCtx.onEnd, DefaultPool)

	globalCache := rootCtxCopy.GlobalCache()
	assert.NotNil(t, globalCache)
	chainCache := rootCtxCopy.ChainCache()
	assert.NotNil(t, globalCache)

	t.Run("GlobalCache", func(t *testing.T) {
		t.Run("SetAndGet", func(t *testing.T) {
			err := globalCache.Set("key1", "value1", "1m")
			assert.Nil(t, err)
			assert.Equal(t, "value1", globalCache.Get("key1"))
		})

		t.Run("Delete", func(t *testing.T) {
			globalCache.Set("key2", "value2", "1m")
			assert.Nil(t, globalCache.Delete("key2"))
			assert.Nil(t, globalCache.Get("key2"))
		})
	})

	t.Run("ChainCache", func(t *testing.T) {
		t.Run("Isolation", func(t *testing.T) {
			chainCache.Set("key1", "value1", "1m")
			assert.Equal(t, "value1", chainCache.Get("key1"))
		})

		t.Run("Has", func(t *testing.T) {
			chainCache.Set("key2", "value2", "1m")
			assert.True(t, chainCache.Has("key2"))
			assert.False(t, chainCache.Has("key3"))
		})
	})

	t.Run("SameKeyIsolation", func(t *testing.T) {
		// 设置相同key到两个缓存
		globalCache.Set("same_key", "global_value", "1m")
		chainCache.Set("same_key", "chain_value", "1m")

		// 验证两个缓存的值互不影响
		assert.Equal(t, "global_value", globalCache.Get("same_key"))
		assert.Equal(t, "chain_value", chainCache.Get("same_key"))

		// 删除一个缓存的值，另一个不受影响
		globalCache.Delete("same_key")
		assert.Nil(t, globalCache.Get("same_key"))
		assert.Equal(t, "chain_value", chainCache.Get("same_key"))
	})
}
