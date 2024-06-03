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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"testing"
	"time"
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
	chainHasSubChainNodeDone := false
	chainMsgTypeSwitchDone := false
	config.OnDebug = func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if ruleChainId == "chain_has_sub_chain_node" {
			chainHasSubChainNodeDone = true
		}
		if ruleChainId == "chain_msg_type_switch" {
			chainMsgTypeSwitchDone = true
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

	assert.True(t, chainHasSubChainNodeDone)
	assert.True(t, chainMsgTypeSwitchDone)

	ruleEngine, _ := myRuleGo.Get("test_context_chain")
	ruleEngine.Stop()

	ruleEngine.OnMsg(msg)

	time.Sleep(time.Millisecond * 200)

	myRuleGo.Reload()
	
	myRuleGo.Stop()
	_, ok = myRuleGo.Get("test_context_chain")
	assert.Equal(t, false, ok)

}
