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
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

// TestRuleGo 测试加载规则链文件夹
func TestRuleGo(t *testing.T) {

	//注册自定义组件
	_ = Registry.Register(&test.UpperNode{})
	_ = Registry.Register(&test.TimeNode{})

	err := Load("./api/")
	_, err = New("aa", []byte(ruleChainFile))
	assert.Nil(t, err)
	_, err = New("aa", []byte(ruleChainFile))
	assert.Nil(t, err)

	myRuleGo := NewPool()
	config := NewConfig()
	var chainHasSubChainNodeDone int32 = 0
	var chainMsgTypeSwitchDone int32 = 0
	config.OnDebug = func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if ruleChainId == "chain_has_sub_chain_node" {
			atomic.StoreInt32(&chainHasSubChainNodeDone, 1)
		}
		if ruleChainId == "chain_msg_type_switch" {
			atomic.StoreInt32(&chainMsgTypeSwitchDone, 1)
		}
	}
	err = myRuleGo.Load("../testdata/aa.txt", WithConfig(config))
	assert.NotNil(t, err)
	err = myRuleGo.Load("../testdata/aa", WithConfig(config))
	assert.NotNil(t, err)

	err = myRuleGo.Load("../testdata/rule/*.json", WithConfig(config))
	assert.Nil(t, err)

	var i = 0
	myRuleGo.Range(func(key, value any) bool {
		i++
		return true
	})
	assert.True(t, i > 0)
	i = 0
	Range(func(key, value any) bool {
		i++
		return true
	})
	assert.True(t, i > 0)
	_, ok := myRuleGo.Get("chain_call_rest_api")
	assert.Equal(t, true, ok)

	_, ok = myRuleGo.Get("chain_has_sub_chain_node")
	assert.Equal(t, true, ok)

	_, ok = myRuleGo.Get("chain_msg_type_switch")
	assert.Equal(t, true, ok)

	_, ok = myRuleGo.Get("not_debug_mode_chain")
	assert.Equal(t, true, ok)

	_, ok = myRuleGo.Get("sub_chain")
	assert.Equal(t, true, ok)

	_, ok = myRuleGo.Get("test_context_chain")
	assert.Equal(t, true, ok)

	_, ok = myRuleGo.Get("aa")
	assert.Equal(t, false, ok)

	myRuleGo.Del("sub_chain")

	_, ok = myRuleGo.Get("sub_chain")
	assert.Equal(t, false, ok)

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")

	myRuleGo.OnMsg(msg)

	time.Sleep(time.Millisecond * 500)

	assert.True(t, atomic.LoadInt32(&chainHasSubChainNodeDone) == 1)
	assert.True(t, atomic.LoadInt32(&chainMsgTypeSwitchDone) == 1)

	ruleEngine, _ := myRuleGo.Get("test_context_chain")
	ruleEngine.Stop(context.Background())

	ruleEngine.OnMsg(msg)

	time.Sleep(time.Millisecond * 200)

	myRuleGo.Reload()

	myRuleGo.Stop()
	_, ok = myRuleGo.Get("test_context_chain")
	assert.Equal(t, false, ok)

}

// aliasChainDefTpl 用 test/upper 节点构造一条可解析的最小链，ruleChain.id 由 %s 注入。
// 专用于别名机制测试（不依赖业务组件注册）。
const aliasChainDefTpl = `{
  "ruleChain": {"id": "%s", "name": "alias test", "root": true, "debugMode": false},
  "metadata": {"firstNodeIndex": 0, "nodes": [
    {"id": "n1", "type": "test/upper", "name": "upper", "configuration": {}}
  ], "connections": []}
}`

// TestPoolAlias 验证别名寻址：当 NewRuleEngine(id, def) 的 id 覆盖 def.ruleChain.id 时，
// ruleChain.id 应被记为别名，使 Pool.Get(ruleChain.id) 也能解析到同一引擎。
// 场景对应 rulego-bpm：链以 processDef.ID（唯一）注册，subProcess 用 ruleChain.id 寻址。
func TestPoolAlias(t *testing.T) {
	_ = Registry.Register(&test.UpperNode{}) // 注册 test/upper 节点（幂等）

	pool := NewPool()
	const (
		primaryID    = "bpm-process-def-uuid-123" // 外部 id（如 BPM processDef.ID），保证唯一
		ruleChainID  = "leave_approval_v2"        // def.ruleChain.id（人类可读，子流程 targetId）
	)
	def := []byte(fmt.Sprintf(aliasChainDefTpl, ruleChainID))

	// 以外部 id 注册：id 覆盖 ruleChain.id，ruleChain.id 记为别名
	e, err := pool.New(primaryID, def)
	assert.Nil(t, err)
	assert.Equal(t, primaryID, e.Id())
	re := e.(*RuleEngine)
	assert.Equal(t, []string{ruleChainID}, re.Aliases(), "ruleChain.id 应作为别名保留")

	// 主 id 可查
	byPrimary, ok := pool.Get(primaryID)
	assert.True(t, ok)
	// 别名 ruleChain.id 可查（子流程寻址的关键）
	byAlias, ok := pool.Get(ruleChainID)
	assert.True(t, ok)
	// 同一引擎实例（接口值相等 ⇔ 持有同一 *RuleEngine 指针）
	assert.True(t, byPrimary == byAlias)
	// 不存在的 key 仍返回 false
	_, ok = pool.Get("not-exist")
	assert.False(t, ok)

	// 按别名删除：主键与别名都应清理
	pool.Del(ruleChainID)
	_, ok = pool.Get(primaryID)
	assert.False(t, ok, "按别名删除后主键应失效")
	_, ok = pool.Get(ruleChainID)
	assert.False(t, ok, "按别名删除后别名应失效")
}

// TestPoolAlias_NoOverride 验证：当 New(id="", def)（不覆盖）时，id=ruleChain.id，无别名。
// 保持向后兼容：未指定外部 id 的行为与改动前一致。
func TestPoolAlias_NoOverride(t *testing.T) {
	_ = Registry.Register(&test.UpperNode{})
	pool := NewPool()
	const ruleChainID = "plain_chain"
	def := []byte(fmt.Sprintf(aliasChainDefTpl, ruleChainID))

	e, err := pool.New("", def)
	assert.Nil(t, err)
	assert.Equal(t, ruleChainID, e.Id())
	re := e.(*RuleEngine)
	assert.True(t, len(re.Aliases()) == 0, "未覆盖 id 时不应产生别名")

	_, ok := pool.Get(ruleChainID)
	assert.True(t, ok)
}

// TestPoolAlias_Stop 验证 Pool.Stop 同步清理别名：停止整个池后，
// 别名不应再解析到已停止的引擎。
func TestPoolAlias_Stop(t *testing.T) {
	_ = Registry.Register(&test.UpperNode{})
	pool := NewPool()
	const (
		primaryID   = "stop-primary"
		ruleChainID = "stop_alias_chain"
	)
	def := []byte(fmt.Sprintf(aliasChainDefTpl, ruleChainID))

	e, err := pool.New(primaryID, def)
	assert.Nil(t, err)
	assert.Equal(t, []string{ruleChainID}, e.(*RuleEngine).Aliases())

	// 停止前别名可查
	_, ok := pool.Get(ruleChainID)
	assert.True(t, ok)

	pool.Stop()

	// 停止后主键与别名都失效
	_, ok = pool.Get(primaryID)
	assert.False(t, ok, "Stop 后主键应失效")
	_, ok = pool.Get(ruleChainID)
	assert.False(t, ok, "Stop 后别名应被清理")
}

// 并发 New 同 id：check-then-create 窗口内会双建引擎，后者覆盖前者导致前者泄漏。
// LoadOrStore 后必须返回同一实例、池中唯一、OnNew 恰好触发一次。
func TestPoolNewConcurrentSameId(t *testing.T) {
	_ = Registry.Register(&test.UpperNode{})
	pool := NewPool()
	var onNewCount int32
	pool.SetCallbacks(types.Callbacks{
		OnNew: func(chainId string, dsl []byte) {
			atomic.AddInt32(&onNewCount, 1)
		},
	})

	const (
		workers   = 20
		chainID   = "concurrent-same-id"
		chainName = "concurrent_same_id"
	)
	def := []byte(fmt.Sprintf(aliasChainDefTpl, chainName))

	engines := make([]types.RuleEngine, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e, err := pool.New(chainID, def)
			if err != nil {
				t.Error(err)
				return
			}
			engines[i] = e
		}(i)
	}
	wg.Wait()

	for i := 0; i < workers; i++ {
		if engines[i] == nil {
			t.Fatalf("worker %d got nil engine", i)
		}
		if engines[i] != engines[0] {
			t.Fatalf("worker %d got different engine instance", i)
		}
	}
	got, ok := pool.Get(chainID)
	assert.True(t, ok)
	assert.True(t, got == engines[0], "pool entry must be the returned instance")
	assert.Equal(t, int32(1), atomic.LoadInt32(&onNewCount), "OnNew must fire exactly once")
}
