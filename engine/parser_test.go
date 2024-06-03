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
	"context"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
	"strings"
	"testing"
)

func TestParser(t *testing.T) {
	jsonParser := JsonParser{}
	config := NewConfig()
	chainNode, err := jsonParser.DecodeRuleChain(config, []byte(ruleChainFile))
	assert.Nil(t, err)
	ruleChainCtx, ok := chainNode.(*RuleChainCtx)
	assert.True(t, ok)
	assert.True(t, ruleChainCtx.IsDebugMode())
	assert.Equal(t, "ruleChain", chainNode.Type())

	ruleChainCtx.Init(config, types.Configuration{})
	ctx := NewRuleContext(context.Background(), config, ruleChainCtx, nil, nil, nil, nil, nil)
	ruleChainCtx.OnMsg(ctx, types.RuleMsg{})
	//超过范围
	_, ok = ruleChainCtx.GetNodeByIndex(5)
	assert.False(t, ok)
	//错误
	_, err = jsonParser.DecodeRuleChain(config, []byte("{"))
	assert.NotNil(t, err)

	//找不到组件的规则链测试
	notFoundComponent := strings.Replace(ruleChainFile, "\"type\": \"jsFilter\"", "\"type\": \"noFound\"", -1)
	_, err = jsonParser.DecodeRuleChain(config, []byte(notFoundComponent))
	assert.NotNil(t, err)

	chainNodeJson, err := jsonParser.EncodeRuleChain(ruleChainCtx.SelfDefinition)
	assert.Equal(t, strings.Replace(ruleChainFile, " ", "", -1), strings.Replace(string(chainNodeJson), " ", "", -1))

	node, err := jsonParser.DecodeRuleNode(config, []byte(modifyMetadataAndMsgNode), nil)
	assert.Nil(t, err)
	nodeCtx, ok := node.(*RuleNodeCtx)
	assert.True(t, ok)
	assert.True(t, nodeCtx.IsDebugMode())

	_, err = jsonParser.DecodeRuleNode(config, []byte("{"), nil)
	assert.NotNil(t, err)

	notFoundComponent = strings.Replace(modifyMetadataAndMsgNode, "\"type\": \"jsTransform\"", "\"type\": \"noFound\"", -1)
	_, err = jsonParser.DecodeRuleNode(config, []byte(notFoundComponent), nil)
	assert.NotNil(t, err)

	_, ok = node.(*RuleNodeCtx).GetNodeById(types.RuleNodeId{Id: "s2"})
	assert.False(t, ok)

	nodeJson, err := jsonParser.EncodeRuleNode(nodeCtx.SelfDefinition)

	var targetMap map[string]interface{}
	err = json.Unmarshal(nodeJson, &targetMap)
	var expectMap map[string]interface{}
	err = json.Unmarshal([]byte(`{
	  "id": "s2",
	  "additionalInfo": {
		"description": "",
		"layoutX": 0,
		"layoutY": 0
	  },
	  "type": "jsTransform",
	  "name": "转换",
	  "debugMode": true,
	  "configuration": {
		"jsScript": "metadata['test']='test02';\n metadata['index']=50;\n msgType='TEST_MSG_TYPE_MODIFY';\n  msg['aa']=66;\n return {'msg':msg,'metadata':metadata,'msgType':msgType};"
	  }
	}`), &expectMap)
	assert.Equal(t, expectMap, targetMap)

	str := strings.Replace(ruleChainFile, "connections", "ruleChainConnections", -1)
	chainNode, err = jsonParser.DecodeRuleChain(config, []byte(str))
	assert.Nil(t, err)
	assert.True(t, len(chainNode.(*RuleChainCtx).SelfDefinition.Metadata.RuleChainConnections) > 0)
}
