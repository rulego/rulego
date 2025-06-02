//go:build !windows && test_plugin

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
	"github.com/rulego/rulego/test/assert"
	string2 "github.com/rulego/rulego/utils/str"
	"sync"
	"testing"
)

var testPluginRuleFile = `
	{
	  "ruleChain": {
		"name": "测试规则链",
		"root": true
	  },
	  "metadata": {
		"nodes": [
		  {
			"id":"s1",
			"type": "myPlugins/upper",
			"name": "转大写",
			"debugMode": true
		  },
		  {
			"id":"s2",
			"type": "myPlugins/time",
			"name": "增加时间",
			"debugMode": true
		  }
		],
		"connections": [
		  {
			"fromId": "s1",
			"toId": "s2",
			"type": "Success"
		  }
		]
	  }
	}
`
var testPluginFile = "../plugin.so"

func TestPlugin(t *testing.T) {
	_ = Registry.Unregister("test")
	err := Registry.RegisterPlugin("test", testPluginFile)
	if err != nil {
		t.Fatal(err)
	}
	err = Registry.RegisterPlugin("test", testPluginFile)
	assert.NotNil(t, err)

	err = Registry.Unregister("test")

	err = Registry.RegisterPlugin("test", testPluginFile)
	assert.Nil(t, err)

	err = Registry.RegisterPlugin("test_not_found", "not found")
	assert.NotNil(t, err)

	maxTimes := 1
	var group sync.WaitGroup
	group.Add(maxTimes)
	config := NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		//config.Logger.Printf("flowType=%s,nodeId=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Data, msg.Metadata, relationType, err)
	}

	ruleEngine, err := New(string2.RandomStr(10), []byte(testPluginRuleFile), WithConfig(config))
	defer ruleEngine.Stop()
	for i := 0; i < maxTimes; i++ {
		if err == nil {
			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("productType", "test01")
			msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "aa")
			//time.Sleep(time.Millisecond * 50)
			ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
				assert.Equal(t, "AA", msg.GetData())
				v := msg.Metadata.GetValue("timestamp")
				assert.True(t, v != "")
				group.Done()
				//config.Logger.Printf("OnEnd data=%s,metaData=%s,err=%s", msg.Data, msg.Metadata, err)
			}))
		}
	}
	group.Wait()
}
