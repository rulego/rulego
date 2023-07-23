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

package testcases

import (
	"fmt"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/str"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var ruleChainFile = `
	{
	  "ruleChain": {
		"name": "testRuleChain01",
		"root": true,
		"debugMode": false
	  },
	  "metadata": {
		"nodes": [
		  {
			"id":"s1",
			"type": "jsFilter",
			"name": "过滤",
			"debugMode": true,
			"configuration": {
			  "jsScript": "var msg2=JSON.parse(msg);return msg2.temperature>10;"
			}
		  },
		  {
			"id":"s2",
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
		"ruleChainConnections": null
	  }
	}
`

//修改metadata和msg 节点
var modifyMetadataAndMsgNode = `
	  {
			"id":"s2",
			"type": "jsTransform",
			"name": "转换",
			"debugMode": true,
			"configuration": {
			  "jsScript": "metadata['test']='test02';\n metadata['index']=50;\n msgType='TEST_MSG_TYPE2';\n var msg2=JSON.parse(msg);\n msg2['aa']=66;\n return {'msg':msg2,'metadata':metadata,'msgType':msgType};"
			}
		  }
`

//加载文件
func loadFile(filePath string) []byte {
	buf, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	} else {
		return buf
	}
}

func testRuleEngine(t *testing.T, ruleChainFile string, modifyNodeId, modifyNodeFile string) {
	config := rulego.NewConfig()
	config.OnDebug = func(flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		config.Logger.Printf("flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
		if flowType == types.Out && nodeId == modifyNodeId && modifyNodeId != "" {
			indexStr, _ := msg.Metadata.GetValue("index")
			testStr, _ := msg.Metadata.GetValue("test")
			assert.Equal(t, "50", indexStr)
			assert.Equal(t, "test02", testStr)
			assert.Equal(t, "TEST_MSG_TYPE2", msg.Type)
		} else {
			assert.Equal(t, "{\"temperature\":35}", msg.Data)
		}
	}
	config.OnEnd = func(msg types.RuleMsg, err error) {
		config.Logger.Printf("OnEnd data=%s,metaData=%s,err=%s", msg.Data, msg.Metadata, err)
	}
	ruleEngine, err := rulego.New(str.RandomStr(10), []byte(ruleChainFile), rulego.WithConfig(config))
	assert.Nil(t, err)
	defer rulego.Del("rule01")

	if modifyNodeId != "" {
		//modify the node
		ruleEngine.ReloadChild(types.EmptyRuleNodeId, types.RuleNodeId{Id: modifyNodeId}, []byte(modifyNodeFile))
	}

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TELEMETRY_MSG", types.JSON, metaData, "{\"temperature\":35}")
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Second)
}

func TestRuleChain(t *testing.T) {
	testRuleEngine(t, ruleChainFile, "", "")
}

func TestRuleChainChangeMetadataAndMsg(t *testing.T) {
	testRuleEngine(t, ruleChainFile, "s2", modifyMetadataAndMsgNode)
}

//测试子规则链
func TestSubRuleChain(t *testing.T) {
	start := time.Now()
	var completed int32
	maxTimes := 1
	var group sync.WaitGroup
	group.Add(maxTimes * 2)
	config := rulego.NewConfig()
	config.OnDebug = func(flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		config.Logger.Printf("flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
	}
	config.OnEnd = func(msg types.RuleMsg, err error) {
		group.Done()
		atomic.AddInt32(&completed, 1)
	}

	ruleFile := loadFile("./chain_has_sub_chain.json")
	subRuleFile := loadFile("./sub_chain.json")
	ruleEngine, err := rulego.New("rule01", ruleFile, rulego.WithConfig(config), rulego.WithAddSubChain("sub_chain_01", subRuleFile))
	assert.Nil(t, err)
	defer rulego.Del("rule01")

	for i := 0; i < maxTimes; i++ {
		metaData := types.NewMetadata()
		metaData.PutValue("productType", "productType01")
		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "aa")

		//处理消息并得到处理结果
		ruleEngine.OnMsgWithEndFunc(msg, func(msg types.RuleMsg, err error) {
			if msg.Type == "TEST_MSG_TYPE1" {
				//root chain end
				assert.Equal(t, msg.Data, "{\"aa\":11}")
				v, _ := msg.Metadata.GetValue("test")
				assert.Equal(t, v, "test01")
			} else {
				//sub chain end
				assert.Equal(t, msg.Data, "{\"bb\":22}")
				v, _ := msg.Metadata.GetValue("test")
				assert.Equal(t, v, "test02")
			}
		})

	}
	group.Wait()
	assert.Equal(t, int32(maxTimes*2), completed)
	fmt.Printf("use times:%s", time.Since(start))
}

//测试规则链debug模式
func TestRuleChainDebugMode(t *testing.T) {
	config := rulego.NewConfig()
	var inTimes int
	var outTimes int
	config.OnDebug = func(flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if flowType == types.In {
			inTimes++
		}
		if flowType == types.Out {
			outTimes++
		}
	}
	config.OnEnd = func(msg types.RuleMsg, err error) {
	}

	ruleFile := loadFile("./sub_chain.json")
	ruleEngine, err := rulego.New("rule01", ruleFile, rulego.WithConfig(config))
	assert.Nil(t, err)
	defer rulego.Del("rule01")

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "productType01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "aa")
	//处理消息并得到处理结果
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Second)

	assert.Equal(t, 2, inTimes)
	assert.Equal(t, 2, outTimes)

	// close s1 node debug mode
	nodeCtx, ok := ruleEngine.RootRuleChainCtx().GetNodeById(types.RuleNodeId{Id: "s1"})
	assert.True(t, ok)
	ruleNodeCtx, ok := nodeCtx.(*rulego.RuleNodeCtx)
	assert.True(t, ok)
	ruleNodeCtx.SelfDefinition.DebugMode = false

	inTimes = 0
	outTimes = 0
	//处理消息并得到处理结果
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Second)

	assert.Equal(t, 1, inTimes)
	assert.Equal(t, 1, outTimes)

	// close s1 node debug mode
	nodeCtx, ok = ruleEngine.RootRuleChainCtx().GetNodeById(types.RuleNodeId{Id: "s2"})
	assert.True(t, ok)
	ruleNodeCtx, ok = nodeCtx.(*rulego.RuleNodeCtx)
	assert.True(t, ok)
	ruleNodeCtx.SelfDefinition.DebugMode = false

	inTimes = 0
	outTimes = 0
	//处理消息并得到处理结果
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Second)

	assert.Equal(t, 0, inTimes)
	assert.Equal(t, 0, outTimes)
}
func TestNotDebugModel(t *testing.T) {
	//var group sync.WaitGroup
	//group.Add(1)
	config := rulego.NewConfig()
	config.OnEnd = func(msg types.RuleMsg, err error) {
		assert.Equal(t, "TEST_MSG_TYPE2", msg.Type)
	}
	ruleEngine, err := rulego.New(str.RandomStr(10), loadFile("./not_debug_mode_chain.json"), rulego.WithConfig(config))
	if err != nil {
		t.Error(err)
	}
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Second)
}

//测试获取节点
func TestGetNodeId(t *testing.T) {
	def, _ := rulego.ParserRuleChain([]byte(ruleChainFile))
	ctx, err := rulego.InitRuleChainCtx(rulego.NewConfig(), def)
	if err != nil {
		t.Errorf("err=%s", err)
	}
	nodeCtx, ok := ctx.GetNodeById(types.RuleNodeId{Id: "s1", Type: types.NODE})
	assert.True(t, ok)

	nodeCtx, ok = ctx.GetNodeById(types.RuleNodeId{Id: "s1", Type: types.CHAIN})
	assert.False(t, ok)
	nodeCtx, ok = ctx.GetNodeById(types.RuleNodeId{Id: "node5", Type: types.NODE})
	assert.False(t, ok)
	_ = nodeCtx

}

//测试callRestApi
func TestCallRestApi(t *testing.T) {

	start := time.Now()
	maxTimes := 100000
	var group sync.WaitGroup
	group.Add(maxTimes)

	//wp, _ := ants.NewPool(math.MaxInt32)
	//使用协程池
	config := rulego.NewConfig(types.WithDefaultPool())
	config.OnDebug = func(flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if err != nil {
			config.Logger.Printf("flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
		}
	}
	config.OnEnd = func(msg types.RuleMsg, err error) {
		group.Done()
		//config.Logger.Printf("completed num %d", completed)
	}

	ruleFile := loadFile("./chain_call_rest_api.json")
	ruleEngine, err := rulego.New(str.RandomStr(10), []byte(ruleFile), rulego.WithConfig(config))
	defer rulego.Stop()

	for i := 0; i < maxTimes; i++ {
		if err == nil {
			metaData := types.NewMetadata()
			metaData.PutValue("productType", "productType01")
			msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"aa\":\"aaaaaaaaaaaaaa\"}")
			ruleEngine.OnMsg(msg)

		}
	}
	group.Wait()
	fmt.Printf("total massages:%d,use times:%s", maxTimes, time.Since(start))
}
