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
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

// TestRuleGo tests the rule chain folder
func TestRuleGo(t *testing.T) {

	//Register custom components
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

// aliasChainDefTpl constructs a parsable minimum chain using the test/upper node, ruleChain.id injected by %s.
// Dedicated to testing the alias mechanism (does not rely on business component registration).
const aliasChainDefTpl = `{
  "ruleChain": {"id": "%s", "name": "alias test", "root": true, "debugMode": false},
  "metadata": {"firstNodeIndex": 0, "nodes": [
    {"id": "n1", "type": "test/upper", "name": "upper", "configuration": {}}
  ], "connections": []}
}`

// TestPoolAlias verifies alias addressing: When the id of NewRuleEngine(id, def) overrides def.ruleChain.id,
// ruleChain.id should be recorded as an alias so that Pool.Get(ruleChain.id) can also parse into the same engine.
// Scenario corresponds to rulego-bpm: chains register as processDef.ID (unique), subProcess addresses using ruleChain.id.
func TestPoolAlias(t *testing.T) {
	_ = Registry.Register(&test.UpperNode{}) // Register test/upper nodes (idempotent)

	pool := NewPool()
	const (
		primaryID   = "bpm-process-def-uuid-123" // External IDs (such as BPM processDef.ID) guarantee uniqueness
		ruleChainID = "leave_approval_v2"        // def.ruleChain.id (human-readable, subflow targetId)
	)
	def := []byte(fmt.Sprintf(aliasChainDefTpl, ruleChainID))

	// Register with an external ID: id overrides ruleChain.id, ruleChain.id is recorded as an alias
	e, err := pool.New(primaryID, def)
	assert.Nil(t, err)
	assert.Equal(t, primaryID, e.Id())
	re := e.(*RuleEngine)
	assert.Equal(t, []string{ruleChainID}, re.Aliases(), "ruleChain.id 应作为别名保留")

	// The main ID is verifiable
	byPrimary, ok := pool.Get(primaryID)
	assert.True(t, ok)
	// Alias: ruleChain.id Queryable (Key to Subflow Addressing)
	byAlias, ok := pool.Get(ruleChainID)
	assert.True(t, ok)
	// Same engine instance (interface values equal⇔ holding the same *RuleEngine pointer)
	assert.True(t, byPrimary == byAlias)
	// A key that doesn't exist still returns false
	_, ok = pool.Get("not-exist")
	assert.False(t, ok)

	// Delete by alias: Both primary keys and aliases should be cleaned
	pool.Del(ruleChainID)
	_, ok = pool.Get(primaryID)
	assert.False(t, ok, "按别名删除后主键应失效")
	_, ok = pool.Get(ruleChainID)
	assert.False(t, ok, "按别名删除后别名应失效")
}

// TestPoolAlias_NoOverride Verification: When New(id="", def) (not overridden), id=ruleChain.id, no aliases.
// Maintains backward compatibility: The behavior of unspecified external IDs remains consistent with the previous changes.
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

// TestPoolAlias_Stop Verify Pool.Stop synchronized cleanup alias: After stopping the entire pool,
// Alias should no longer be resolved to a stopped engine.
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

	// Aliases before discontinuation are available
	_, ok := pool.Get(ruleChainID)
	assert.True(t, ok)

	pool.Stop()

	// After stopping, both the primary key and aliases become invalid
	_, ok = pool.Get(primaryID)
	assert.False(t, ok, "Stop 后主键应失效")
	_, ok = pool.Get(ruleChainID)
	assert.False(t, ok, "Stop 后别名应被清理")
}
