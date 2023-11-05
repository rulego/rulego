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

package rulego

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	"strings"
	"testing"
)

var rootRuleChain = `
	{
	  "ruleChain": {
		"name": "测试规则链",
		"root": true,
		"debugMode": false
	  },
	  "metadata": {
		"nodes": [
		  {
			"Id":"s1",
			"type": "jsFilter",
			"name": "过滤",
			"debugMode": true,
			"configuration": {
			  "jsScript": "return msg!='aa';"
			}
		  },
          {
			"Id":"s2",
			"type": "jsTransform",
			"name": "转换",
			"debugMode": true,
			"configuration": {
			  "jsScript": "msgType='TEST_MSG_TYPE2';var msg2={};\n  msg2['aa']=66\n return {'msg':msg,'metadata':metadata,'msgType':msgType};"
			}
		  }
		],
		"connections": [
		  {
			"fromId": "s1",
			"toId": "s2",
			"type": "True"
		  }
		],
		"ruleChainConnections": [
 			{
			"fromId": "s1",
			"toId": "subChain01",
			"type": "True"
		  }
		]
	  }
	}
`
var subRuleChain = `
	{
	  "ruleChain": {
		"name": "测试子规则链",
		"debugMode": false
	  },
	  "metadata": {
		"nodes": [
		  {
			"Id":"s1",
			"type": "jsFilter",
			"name": "过滤",
			"debugMode": true,
			"configuration": {
			  "jsScript": "return msg!='aa';"
			}
		  },
			{"Id":"s2",
			"type": "jsTransform",
			"name": "转换",
			"debugMode": true,
			"configuration": {
			  "jsScript": "msgType='TEST_MSG_TYPE2';var msg2={};\n  msg2['aa']=66\n return {'msg':msg,'metadata':metadata,'msgType':msgType};"
			}
		  }
		],
		"connections": [
		  {
			"fromId": "s1",
			"toId": "s2",
			"type": "True"
		  }
 
		]
	  }
	}
`
var s1NodeFile = `
  {
			"Id":"s1",
			"type": "jsFilter",
			"name": "过滤-更改",
			"debugMode": true,
			"configuration": {
			  "jsScript": "return msg!='bb';"
			}
		  }
`

//TestEngine 测试规则引擎
func TestEngine(t *testing.T) {
	config := NewConfig()
	//初始化子规则链
	subRuleEngine, err := New("subChain01", []byte(subRuleChain), WithConfig(config))
	//初始化根规则链
	ruleEngine, err := New("rule01", []byte(rootRuleChain), WithConfig(config))
	if err != nil {
		t.Errorf("%v", err)
	}
	assert.True(t, ruleEngine.Initialized())

	//获取节点
	s1NodeId := types.RuleNodeId{Id: "s1"}
	s1Node, ok := ruleEngine.rootRuleChainCtx.nodes[s1NodeId]
	assert.True(t, ok)
	s1RuleNodeCtx, ok := s1Node.(*RuleNodeCtx)
	assert.True(t, ok)
	assert.Equal(t, "过滤", s1RuleNodeCtx.SelfDefinition.Name)
	assert.Equal(t, "return msg!='aa';", s1RuleNodeCtx.SelfDefinition.Configuration["jsScript"])

	//获取子规则链
	subChain01Id := types.RuleNodeId{Id: "subChain01", Type: types.CHAIN}
	subChain01Node, ok := ruleEngine.rootRuleChainCtx.GetNodeById(subChain01Id)
	assert.True(t, ok)
	subChain01NodeCtx, ok := subChain01Node.(*RuleChainCtx)
	assert.True(t, ok)
	assert.Equal(t, "测试子规则链", subChain01NodeCtx.SelfDefinition.RuleChain.Name)
	assert.Equal(t, subChain01NodeCtx, subRuleEngine.rootRuleChainCtx)

	//修改根规则链节点
	_ = ruleEngine.ReloadChild(s1NodeId.Id, []byte(s1NodeFile))
	s1Node, ok = ruleEngine.rootRuleChainCtx.nodes[s1NodeId]
	assert.True(t, ok)
	s1RuleNodeCtx, ok = s1Node.(*RuleNodeCtx)
	assert.True(t, ok)
	assert.Equal(t, "过滤-更改", s1RuleNodeCtx.SelfDefinition.Name)
	assert.Equal(t, "return msg!='bb';", s1RuleNodeCtx.SelfDefinition.Configuration["jsScript"])

	//修改子规则链
	_ = subRuleEngine.ReloadSelf([]byte(strings.Replace(subRuleChain, "测试子规则链", "测试子规则链-更改", -1)))

	subChain01Node, ok = ruleEngine.rootRuleChainCtx.GetNodeById(types.RuleNodeId{Id: "subChain01", Type: types.CHAIN})
	assert.True(t, ok)
	subChain01NodeCtx, ok = subChain01Node.(*RuleChainCtx)
	assert.True(t, ok)
	assert.Equal(t, "测试子规则链-更改", subChain01NodeCtx.SelfDefinition.RuleChain.Name)

	//获取规则引擎实例
	ruleEngineNew, ok := Get("rule01")
	assert.True(t, ok)
	assert.Equal(t, ruleEngine, ruleEngineNew)
	//删除对应规则引擎实例
	Del("rule01")
	_, ok = Get("rule01")
	assert.False(t, ok)
	assert.False(t, ruleEngine.Initialized())
}
