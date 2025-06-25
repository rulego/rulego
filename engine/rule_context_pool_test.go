/*
 * Copyright 2023 The RuleGo Authors.
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
	"context"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

// TestContextObserverPool 测试ContextObserver对象池的基本功能
func TestContextObserverPool(t *testing.T) {
	// 测试从池中获取对象
	observer1 := globalObserverPool.Get()
	assert.NotNil(t, observer1)
	assert.Equal(t, 16, observer1.segmentCount) // 验证初始化正确

	// 模拟使用observer
	observer1.addInMsg("node1", "from1", types.NewMsg(0, "test", types.JSON, types.NewMetadata(), "data"), "")

	// 测试回收到池中
	globalObserverPool.Put(observer1)

	// 重新获取，应该是重用的对象
	observer2 := globalObserverPool.Get()
	assert.NotNil(t, observer2)

	// 验证对象被重置了
	msgs := observer2.getInMsgList("node1")
	assert.Equal(t, 0, len(msgs)) // Reset应该清空了数据
}

// TestContextObserverReset 测试ContextObserver的Reset方法
func TestContextObserverReset(t *testing.T) {
	observer := NewContextObserver()

	// 添加一些测试数据
	observer.addInMsg("node1", "from1", types.NewMsg(0, "test", types.JSON, types.NewMetadata(), "data1"), "")
	observer.addInMsg("node2", "from2", types.NewMsg(0, "test", types.JSON, types.NewMetadata(), "data2"), "")
	observer.executedNode("node1")

	// 注册一个回调事件
	observer.registerNodeDoneEvent("node1", []string{"from1"}, func([]types.WrapperMsg) {})

	// 验证数据存在
	msgs1 := observer.getInMsgList("node1")
	assert.Equal(t, 1, len(msgs1))
	msgs2 := observer.getInMsgList("node2")
	assert.Equal(t, 1, len(msgs2))

	// 重置
	observer.Reset()

	// 验证数据被清空
	msgs1After := observer.getInMsgList("node1")
	assert.Equal(t, 0, len(msgs1After))
	msgs2After := observer.getInMsgList("node2")
	assert.Equal(t, 0, len(msgs2After))
}

// TestRuleContextObserverOwnership 测试RuleContext中observer的拥有权机制
func TestRuleContextObserverOwnership(t *testing.T) {
	config := types.NewConfig()

	// 创建root context，observer对象预先创建，但内部字段延迟初始化
	rootCtx := NewRuleContext(context.Background(), config, nil, nil, nil, nil, nil, nil)
	assert.True(t, rootCtx.ownsObserver) // 现在预先拥有observer对象
	assert.NotNil(t, rootCtx.observer)   // observer对象预先创建

	// 创建子context，应该共享observer
	childCtx := rootCtx.NewNextNodeRuleContext(nil)
	assert.False(t, childCtx.ownsObserver)               // 子context不拥有observer
	assert.NotNil(t, childCtx.observer)                  // 但有observer对象
	assert.Equal(t, rootCtx.observer, childCtx.observer) // 同一个observer实例

	// 验证observer的内部字段已预先初始化（新的简单设计）
	assert.Equal(t, 16, len(rootCtx.observer.segments)) // segments已预先初始化

	// 测试TellCollect功能正常
	rootCtx.TellCollect(types.NewMsg(0, "test", types.JSON, types.NewMetadata(), "data"), func([]types.WrapperMsg) {})

	// segments仍然是16个
	assert.Equal(t, 16, len(rootCtx.observer.segments))

	// 子context使用同一个observer，内部字段已经初始化
	childCtx.TellCollect(types.NewMsg(0, "test", types.JSON, types.NewMetadata(), "child-data"), func([]types.WrapperMsg) {})
	assert.Equal(t, rootCtx.observer, childCtx.observer) // 仍然是同一个实例

	// 创建新的子context，应该直接继承已有的observer
	child2Ctx := rootCtx.NewNextNodeRuleContext(nil)
	assert.False(t, child2Ctx.ownsObserver)
	assert.Equal(t, rootCtx.observer, child2Ctx.observer) // 直接继承
}

// TestSubRuleChainObserverPool 测试子规则链场景下的ContextObserver对象池行为
func TestSubRuleChainObserverPool(t *testing.T) {
	config := types.NewConfig()

	// 创建主规则链上下文，observer预先创建
	mainChainCtx := NewRuleContext(context.Background(), config, nil, nil, nil, nil, nil, nil)
	assert.True(t, mainChainCtx.ownsObserver) // 拥有observer
	assert.NotNil(t, mainChainCtx.observer)   // observer已创建

	// 模拟子规则链调用 - 也会预先创建observer
	subChainCtx := NewRuleContext(context.Background(), config, nil, nil, nil, nil, nil, nil)
	assert.True(t, subChainCtx.ownsObserver) // 子规则链也拥有自己的observer
	assert.NotNil(t, subChainCtx.observer)   // observer已创建

	// 验证各自有独立的observer（这是正确的，因为不同的规则链应该有独立的join状态）
	// 使用指针比较避免reflect.DeepEqual导致的数据竞争
	if mainChainCtx.observer == subChainCtx.observer {
		t.Errorf("Expected different observer instances, got the same: %p", mainChainCtx.observer)
	}

	// 模拟子规则链完成，应该回收observer
	subChainCtx.childDone()

	// 验证主规则链的observer仍然有效
	assert.NotNil(t, mainChainCtx.observer)

	// 清理主规则链
	mainChainCtx.childDone()
}

// TestTellNodeObserverIsolation 测试TellNode创建的context的observer隔离
func TestTellNodeObserverIsolation(t *testing.T) {
	config := types.NewConfig()

	// 创建主context，observer预先创建
	mainCtx := NewRuleContext(context.Background(), config, nil, nil, nil, nil, nil, nil)
	assert.True(t, mainCtx.ownsObserver)
	assert.NotNil(t, mainCtx.observer)

	// 模拟TellNode调用 - 也会创建新的RuleContext
	tellNodeCtx := NewRuleContext(context.Background(), config, nil, nil, nil, nil, nil, nil)
	assert.True(t, tellNodeCtx.ownsObserver)
	assert.NotNil(t, tellNodeCtx.observer)

	// 验证各自有独立的observer
	// 使用指针比较避免reflect.DeepEqual导致的数据竞争
	if mainCtx.observer == tellNodeCtx.observer {
		t.Errorf("Expected different observer instances, got the same: %p", mainCtx.observer)
	}

	// 验证各自的ownsObserver状态是正确的
	assert.True(t, mainCtx.ownsObserver, "主context应该拥有observer")
	assert.True(t, tellNodeCtx.ownsObserver, "TellNode创建的context应该拥有observer")

	// 测试回收机制
	tellNodeCtx.childDone()
	mainCtx.childDone()

	// 验证回收后，原context的observer引用仍然有效
	assert.NotNil(t, mainCtx.observer)
	assert.NotNil(t, tellNodeCtx.observer)
}

// TestContextObserverPoolConcurrency 测试对象池的并发安全性
func TestContextObserverPoolConcurrency(t *testing.T) {
	const numGoroutines = 100
	const numOperations = 10

	results := make(chan *ContextObserver, numGoroutines*numOperations)

	// 启动多个goroutine并发获取和归还observer
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numOperations; j++ {
				observer := globalObserverPool.Get()
				assert.NotNil(t, observer)

				// 模拟使用
				observer.addInMsg("test", "from", types.NewMsg(0, "test", types.JSON, types.NewMetadata(), "data"), "")

				results <- observer

				// 归还到池中
				globalObserverPool.Put(observer)
			}
		}()
	}

	// 收集结果，验证没有nil对象
	for i := 0; i < numGoroutines*numOperations; i++ {
		observer := <-results
		assert.NotNil(t, observer)
	}
}
