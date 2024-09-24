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
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/str"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	shareKey       = "shareKey"
	shareValue     = "shareValue"
	addShareKey    = "addShareKey"
	addShareValue  = "addShareValue"
	testdataFolder = "../testdata/rule/"
)
var ruleChainFile = `{
          "ruleChain": {
            "id": "test01",
            "name": "testRuleChain01",
            "debugMode": true,
            "root": true
          },
          "metadata": {
            "firstNodeIndex": 0,
            "nodes": [
              {
                "id": "s1",
                "additionalInfo": {
                  "description": "",
                  "layoutX": 0,
                  "layoutY": 0
                },
                "type": "jsFilter",
                "name": "过滤",
                "debugMode": true,
                "configuration": {
                  "jsScript": "return msg.temperature>10;"
                }
              },
              {
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
                  "jsScript": "msgType='TEST_MSG_TYPE';var msg2={};\n  msg2['aa']=66\n return {'msg':msg,'metadata':metadata,'msgType':msgType};"
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
        }`

var updateRuleChainFile = `
	{
	  "ruleChain": {
		"id":"test01",
		"name": "updateRuleChainFile"
	  },
	  "metadata": {
		"nodes": [
		  {
			"id":"s1",
			"type": "jsFilter",
			"name": "过滤",
			"debugMode": true,
			"configuration": {
			  "jsScript": "return msg.temperature>10;"
			}
		  },
		  {
			"id":"s3",
			"type": "jsTransform",
			"name": "转换2",
			"debugMode": true,
			"configuration": {
			  "jsScript": "metadata['productType']='product02';msgType='TEST_MSG_TYPE';var msg2={};\n  msg2['aa']=77\n return {'msg':msg,'metadata':metadata,'msgType':msgType};"
			}
		  },
		  {
			"id":"s4",
			"type": "jsTransform",
			"name": "转换4",
			"debugMode": true,
			"configuration": {
			  "jsScript": "metadata['name']='productName'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
			}
		  }
		],
		"connections": [
		  {
			"fromId": "s1",
			"toId": "s3",
			"type": "True"
		  },
		  {
			"fromId": "s3",
			"toId": "s4",
			"type": "Success"
		  }
		]
	  }
	}
`

// 修改metadata和msg 节点
var modifyMetadataAndMsgNode = `
	  {
			"id":"s2",
			"type": "jsTransform",
			"name": "转换",
			"debugMode": true,
			"configuration": {
			  "jsScript": "metadata['test']='test02';\n metadata['index']=50;\n msgType='TEST_MSG_TYPE_MODIFY';\n  msg['aa']=66;\n return {'msg':msg,'metadata':metadata,'msgType':msgType};"
			}
		  }
`

// 加载文件
func loadFile(filePath string) []byte {
	buf, err := os.ReadFile(testdataFolder + filePath)
	if err != nil {
		return nil
	} else {
		return buf
	}
}

func testRuleEngine(t *testing.T, ruleChainFile string, modifyNodeId, modifyNodeFile string) {
	config := NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		//config.Logger.Printf("flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
		if flowType == types.Out && nodeId == modifyNodeId && modifyNodeId != "" {
			indexStr := msg.Metadata.GetValue("index")
			testStr := msg.Metadata.GetValue("test")
			assert.Equal(t, "50", indexStr)
			assert.Equal(t, "test02", testStr)
			assert.Equal(t, "TEST_MSG_TYPE_MODIFY", msg.Type)
		} else {
			assert.Equal(t, "{\"temperature\":35}", msg.Data)
		}
	}
	ruleEngine, err := New("rule01", []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del("rule01")

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TELEMETRY_MSG", types.JSON, metaData, "{\"temperature\":35}")
	maxTimes := 1
	for j := 0; j < maxTimes; j++ {
		if modifyNodeId != "" {
			//modify the node
			_ = ruleEngine.ReloadChild(modifyNodeId, []byte(modifyNodeFile))
		}
		ruleEngine.OnMsg(msg)
	}
	time.Sleep(time.Second)
}

func TestRuleChain(t *testing.T) {
	testRuleEngine(t, ruleChainFile, "", "")
}

func TestRuleChainChangeMetadataAndMsg(t *testing.T) {
	testRuleEngine(t, ruleChainFile, "s2", modifyMetadataAndMsgNode)
}

// test reload rule chain
func TestReloadRuleChain(t *testing.T) {
	config1 := NewConfig()
	config1DebugDone := false
	config1.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		//config1.Logger.Printf("before reload : flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
		if flowType == types.Out && nodeId == "s2" {
			productType := msg.Metadata.GetValue("productType")
			assert.Equal(t, "test01", productType)
		}
		config1DebugDone = true
	}

	chainId := str.RandomStr(10)

	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config1))
	assert.Nil(t, err)
	defer Del(chainId)

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TELEMETRY_MSG", types.JSON, metaData, "{\"temperature\":35}")

	ruleEngine.OnMsg(msg)

	time.Sleep(time.Millisecond * 200)

	assert.True(t, config1DebugDone)

	//config1.Logger.Printf("reload rule chain......")
	config2 := NewConfig()
	config2DebugDone := false
	config2.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		//config2.Logger.Printf("before after : flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
		if flowType == types.Out && nodeId == "s3" {
			productType := msg.Metadata.GetValue("productType")
			assert.Equal(t, "product02", productType)
		}
		config2DebugDone = true
	}
	//更新规则链
	err = ruleEngine.ReloadSelf([]byte(updateRuleChainFile), WithConfig(config2))
	assert.Nil(t, err)

	ruleEngine.OnMsg(msg)
	time.Sleep(time.Millisecond * 200)
	assert.True(t, config2DebugDone)
}

// 测试子规则链
func TestSubRuleChain(t *testing.T) {
	//start := time.Now()
	var completed int32
	maxTimes := 1
	var group sync.WaitGroup
	group.Add(maxTimes * 2)
	subChainDone := false
	config := NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if chainId == "sub_chain_01" {
			subChainDone = true
		}
		//config.Logger.Printf("chainId=%s,flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", chainId, flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
	}

	ruleFile := loadFile("./chain_has_sub_chain_node.json")
	subRuleFile := loadFile("./sub_chain.json")
	//初始化子规则链实例
	_, err := New("sub_chain_01", subRuleFile, WithConfig(config))

	chainId := str.RandomStr(10)

	//初始化主规则链实例
	ruleEngine, err := New(chainId, ruleFile, WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	for i := 0; i < maxTimes; i++ {
		metaData := types.NewMetadata()
		metaData.PutValue("productType", "productType01")
		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "aa")

		//处理消息并得到处理结果
		ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {

			atomic.AddInt32(&completed, 1)
			group.Done()
			if msg.Type == "TEST_MSG_TYPE1" {
				//root chain end
				assert.Equal(t, msg.Data, "{\"aa\":11}")
				v := msg.Metadata.GetValue("test")
				assert.Equal(t, v, "Modified by root chain")
			} else {
				//sub chain end
				assert.Equal(t, true, strings.Contains(msg.Data, `"data":"{\"bb\":22}"`))
				v := msg.Metadata.GetValue("test")
				assert.Equal(t, v, "Modified by sub chain")
			}
		}))

	}
	group.Wait()
	assert.Equal(t, int32(maxTimes*2), completed)
	time.Sleep(time.Millisecond * 200)
	assert.True(t, subChainDone)
	//fmt.Printf("use times:%s \n", time.Since(start))
}

// 测试规则链debug模式
func TestRuleChainDebugMode(t *testing.T) {
	config := NewConfig()
	var inTimes int
	var outTimes int
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if flowType == types.In {
			inTimes++
		}
		if flowType == types.Out {
			outTimes++
		}
	}
	chainId := str.RandomStr(10)
	ruleFile := loadFile("./sub_chain.json")
	ruleEngine, err := New(chainId, ruleFile, WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "productType01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "aa")
	//处理消息并得到处理结果
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Millisecond * 200)

	assert.Equal(t, 2, inTimes)
	assert.Equal(t, 2, outTimes)

	// close s1 node debug mode
	nodeCtx, ok := ruleEngine.RootRuleChainCtx().GetNodeById(types.RuleNodeId{Id: "sub_s1"})
	assert.True(t, ok)
	ruleNodeCtx, ok := nodeCtx.(*RuleNodeCtx)
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
	nodeCtx, ok = ruleEngine.RootRuleChainCtx().GetNodeById(types.RuleNodeId{Id: "sub_s2"})
	assert.True(t, ok)
	ruleNodeCtx, ok = nodeCtx.(*RuleNodeCtx)
	assert.True(t, ok)
	ruleNodeCtx.SelfDefinition.DebugMode = false

	inTimes = 0
	outTimes = 0
	//处理消息并得到处理结果
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Millisecond * 200)

	assert.Equal(t, 0, inTimes)
	assert.Equal(t, 0, outTimes)
}

func TestNotDebugModel(t *testing.T) {
	//start := time.Now()
	config := NewConfig()
	debugDone := false
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		debugDone = true
	}
	// closed debug mode
	ruleEngine, err := New(str.RandomStr(10), loadFile("./not_debug_mode_chain.json"), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":41}")
	var wg sync.WaitGroup
	wg.Add(1)
	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		wg.Done()
		//已经被 s2 节点修改消息类型
		assert.Equal(t, "TEST_MSG_TYPE2", msg.Type)
		assert.Nil(t, err)
	}))

	wg.Wait()

	assert.False(t, debugDone)

	// open debug mode
	debugEnableRuleChain := strings.Replace(string(loadFile("./not_debug_mode_chain.json")), "\"debugMode\": false", "\"debugMode\": true", -1)
	err = ruleEngine.ReloadSelf([]byte(debugEnableRuleChain))
	assert.Nil(t, err)

	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
	}))
	time.Sleep(time.Millisecond * 200)
	assert.True(t, debugDone)
}

// 测试获取节点
func TestGetNodeId(t *testing.T) {
	parser := JsonParser{}
	def, _ := parser.DecodeRuleChain([]byte(ruleChainFile))
	ctx, err := InitRuleChainCtx(NewConfig(), nil, &def)
	assert.Nil(t, err)
	nodeCtx, ok := ctx.GetNodeById(types.RuleNodeId{Id: "s1", Type: types.NODE})
	assert.True(t, ok)

	nodeCtx, ok = ctx.GetNodeById(types.RuleNodeId{Id: "s1", Type: types.CHAIN})
	assert.False(t, ok)
	nodeCtx, ok = ctx.GetNodeById(types.RuleNodeId{Id: "node5", Type: types.NODE})
	assert.False(t, ok)
	_ = nodeCtx
}

// 测试callRestApi
func TestCallRestApi(t *testing.T) {
	//start := time.Now()
	maxTimes := 1
	var group sync.WaitGroup
	group.Add(maxTimes)

	//wp, _ := ants.NewPool(math.MaxInt32)
	//使用协程池
	config := NewConfig(types.WithDefaultPool())
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if err != nil {
			config.Logger.Printf("flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
		}
	}
	ruleFile := loadFile("./chain_call_rest_api.json")
	ruleEngine, err := New(str.RandomStr(10), []byte(ruleFile), WithConfig(config))
	defer Stop()

	for i := 0; i < maxTimes; i++ {
		if err == nil {
			metaData := types.NewMetadata()
			metaData.PutValue("productType", "productType01")
			msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"aa\":\"aaaaaaaaaaaaaa\"}")
			ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
				group.Done()
			}))

		}
	}
	group.Wait()
	time.Sleep(time.Millisecond * 200)
	//fmt.Printf("total massages:%d,use times:%s \n", maxTimes, time.Since(start))
}

// 测试消息路由
func TestMsgTypeSwitch(t *testing.T) {
	var wg sync.WaitGroup

	config := NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		wg.Done()
	}
	ruleEngine, err := New(str.RandomStr(10), loadFile("./chain_msg_type_switch.json"), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	//TEST_MSG_TYPE1 找到2条chains,4个nodes
	wg.Add(6)
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsg(msg)
	wg.Wait()

	//TEST_MSG_TYPE2 找到1条chain,2个nodes
	wg.Add(4)
	msg = types.NewMsg(0, "TEST_MSG_TYPE2", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, "s4", msg.Type)
		v := msg.Metadata.GetValue("addFrom")
		assert.Equal(t, "s4", v)
	}))
	wg.Wait()

	//TEST_MSG_TYPE3 找到1 other chain,4个node
	wg.Add(4)
	msg = types.NewMsg(0, "TEST_MSG_TYPE3", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, "TEST_MSG_TYPE3", msg.Type)
		v := msg.Metadata.GetValue("addFrom")
		assert.Equal(t, "Other", v)
	}))
	wg.Wait()
}

func TestWithContext(t *testing.T) {
	//注册自定义组件
	_ = Registry.Register(&test.UpperNode{})
	_ = Registry.Register(&test.TimeNode{})

	//start := time.Now()
	config := NewConfig()

	_, err := New("test_context_chain", loadFile("./test_context_chain.json"), WithConfig(config))
	if err != nil {
		t.Error(err)
	}
	ruleEngine, err := New(str.RandomStr(10), loadFile("./test_context.json"), WithConfig(config))
	if err != nil {
		t.Error(err)
	}

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":41}")
	var maxTimes = 1000
	var wg sync.WaitGroup
	wg.Add(maxTimes)
	for j := 0; j < maxTimes; j++ {
		go func() {
			index := j
			ruleEngine.OnMsg(msg, types.WithContext(context.WithValue(context.Background(), shareKey, shareValue+strconv.Itoa(index))), types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
				v1 := msg.Metadata.GetValue(shareKey)
				assert.Equal(t, shareValue+strconv.Itoa(index), v1)

				assert.Equal(t, "TEST_MSG_TYPE", msg.Type)

				v2 := msg.Metadata.GetValue(addShareKey)
				assert.Equal(t, addShareValue, v2)
				assert.Nil(t, err)
				wg.Done()
			}))
		}()

	}
	wg.Wait()
	//fmt.Printf("total massages:%d,use times:%s \n", maxTimes, time.Since(start))
}

func TestSpecifyID(t *testing.T) {
	config := NewConfig()
	ruleEngine, err := New("", []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	assert.Equal(t, "test01", ruleEngine.Id())
	_, ok := Get("test01")
	assert.Equal(t, true, ok)

	chainId := str.RandomStr(10)

	ruleEngine, err = New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	assert.Equal(t, chainId, ruleEngine.Id())
	ruleEngine, ok = Get(chainId)
	assert.Equal(t, true, ok)
}

// TestOnMsgAndWait 测试同步执行规则链
func TestOnMsgAndWait(t *testing.T) {
	var wg sync.WaitGroup

	config := NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		wg.Done()
	}
	ruleEngine, err := New(str.RandomStr(10), loadFile("./test_wait.json"), WithConfig(config))
	if err != nil {
		t.Error(err)
	}
	_, err = New("sub_chain_02", loadFile("./sub_chain.json"), WithConfig(config))
	if err != nil {
		t.Error(err)
	}
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	//TEST_MSG_TYPE1 找到2条chains,5个nodes
	wg.Add(10)
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")
	var count int32
	ruleEngine.OnMsgAndWait(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		atomic.AddInt32(&count, 1)
	}))
	assert.Equal(t, int32(2), count)
	wg.Wait()

	//TEST_MSG_TYPE2 找到1条chain,2个nodes
	wg.Add(4)
	count = 0
	msg = types.NewMsg(0, "TEST_MSG_TYPE2", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsgAndWait(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		atomic.AddInt32(&count, 1)
	}))

	assert.Equal(t, int32(1), count)
	wg.Wait()

	//TEST_MSG_TYPE3 找到0条chain,1个node
	wg.Add(2)
	count = 0
	msg = types.NewMsg(0, "TEST_MSG_TYPE3", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsgAndWait(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		atomic.AddInt32(&count, 1)
	}))
	assert.Equal(t, int32(1), count)
	wg.Wait()
}

// 测试functions节点，并发修改metadata
func TestFunctionsNode(t *testing.T) {
	action.Functions.Register("modifyMetadata", func(ctx types.RuleContext, msg types.RuleMsg) {
		msg.Metadata.PutValue("aa", "aa")
		msg.Metadata.PutValue("bb", "bb")
		ctx.TellSuccess(msg)
	})

	config := NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if flowType == types.Out {
			assert.Equal(t, "aa", msg.Metadata.GetValue("aa"))
			assert.Equal(t, "bb", msg.Metadata.GetValue("bb"))
		}
	}
	ruleEngine, err := New(str.RandomStr(10), loadFile("./test_functions_node.json"), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")
	var i = 0
	for i < 10 {
		ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		}))
		i++
	}

	time.Sleep(time.Second)
}

func TestFunctionsNodeRelationTypeEmpty(t *testing.T) {
	action.Functions.Register("tellNextRelationTypeEmpty", func(ctx types.RuleContext, msg types.RuleMsg) {
		msg.Metadata.PutValue("aa", "aa")
		msg.Metadata.PutValue("bb", "bb")
		ctx.TellNext(msg)
	})

	config := NewConfig()
	ruleEngine, err := New(str.RandomStr(10), loadFile("./test_functions_node2.json"), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")
	var wg sync.WaitGroup
	wg.Add(1)
	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		assert.Equal(t, "aa", msg.Metadata.GetValue("aa"))
		assert.Equal(t, "bb", msg.Metadata.GetValue("bb"))
		wg.Done()
	}))
	wg.Wait()
}

func TestExecuteNode(t *testing.T) {
	config := NewConfig()
	var err error
	chainId := "executeNode_rule01"
	b := loadFile("./chain_call_rest_api.json")
	ruleEngine, err := New(chainId, b, WithConfig(config))

	chainId2 := "executeNode_rule02"
	chainJson2 := strings.Replace(string(b), "test_group_filter_node", chainId2, -1)
	_, err = New(chainId2, []byte(chainJson2), WithConfig(config))

	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	msg1 := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")

	ruleEngine.OnMsg(msg1, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, "true", msg.Metadata.GetValue("result"))
	}))

	time.Sleep(time.Millisecond * 200)

	chainJsonFile1 := string(loadFile("./test_group_filter_node.json"))
	newChainJsonFile1 := strings.Replace(chainJsonFile1, `"allMatches": false`, `"allMatches": true`, -1)
	newChainJsonFile1 = strings.Replace(newChainJsonFile1, "test_group_filter_node", chainId, -1)
	//更新规则链，groupFilter必须所有节点都满足True,才走True链
	_ = ruleEngine.ReloadSelf([]byte(newChainJsonFile1))

	ruleEngine.OnMsg(msg1, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, "false", msg.Metadata.GetValue("result"))
	}))
	var wg sync.WaitGroup
	wg.Add(4)

	msg2 := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":51,\"humidity\":90}")
	ruleEngine.OnMsg(msg2, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, "true", msg.Metadata.GetValue("result"))
		ctx.TellNode(context.Background(), "aa", msg, true, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.NotNil(t, err)
			assert.Equal(t, types.Failure, relationType)

			wg.Done()
		}, nil)

		ctx.TellChainNode(context.Background(), chainId, "s1", msg, true, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.True, relationType)
			wg.Done()
		}, nil)

		ctx.TellChainNode(context.Background(), "notfound", "s2", msg, true, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.NotNil(t, err)
			assert.Equal(t, "ruleChain id=notfound not found", err.Error())
			assert.Equal(t, types.Failure, relationType)
			wg.Done()
		}, nil)

		ctx.TellChainNode(context.Background(), chainId2, "s2", msg, true, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
			wg.Done()
		}, nil)
		//ctx.TellChainNode(context.Background(), chainId2, "s2", msg, false, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		//	assert.Equal(t, types.Failure, relationType)
		//	assert.Equal(t, "Computing the full value of call results is not supported", err.Error())
		//	wg.Done()
		//}, nil)
	}))

	wg.Wait()
}

func TestBatchOnMsgAndWait(t *testing.T) {
	config := NewConfig()
	ruleEngine, err := New(str.RandomStr(10), []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	var maxTimes = 100000
	var wg sync.WaitGroup
	wg.Add(maxTimes)
	for i := 0; i < maxTimes; i++ {
		metaData := types.NewMetadata()
		metaData.PutValue("productType", "test01")
		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":35}")
		ruleEngine.OnMsgAndWait(msg, types.WithOnAllNodeCompleted(func() {
		}), types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
			wg.Done()
		}))
	}
	wg.Wait()

}

// TestBatchOnMsgAndWait  测试同步处理消息，有多个end
func TestBatchOnMsgAndWaitMultipleOnEnd(t *testing.T) {
	config := NewConfig()
	ruleEngine, err := New(str.RandomStr(10), loadFile("./chain_msg_type_switch.json"), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")

	var maxTimes = 100
	for i := 0; i < maxTimes; i++ {
		var count = int32(0)
		ruleEngine.OnMsgAndWait(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
			atomic.AddInt32(&count, 1)
		}))
		assert.Equal(t, int32(2), count)
	}
	time.Sleep(time.Millisecond * 100)
}

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

// TestEngine 测试规则引擎
func TestEngine(t *testing.T) {
	config := NewConfig()
	_, err := New("subChain01", []byte{}, WithConfig(config))
	assert.NotNil(t, err)
	//初始化子规则链
	subRuleEngine, err := New("subChain01", loadFile("./sub_chain.json"), WithConfig(config))
	//初始化根规则链
	ruleEngine, err := New("testEngine", []byte(ruleChainFile), WithConfig(config))
	if err != nil {
		t.Errorf("%v", err)
	}
	assert.True(t, ruleEngine.Initialized())

	assert.Equal(t, strings.Replace(ruleChainFile, " ", "", -1), strings.Replace(string(ruleEngine.DSL()), " ", "", -1))

	//获取节点
	s1NodeId := types.RuleNodeId{Id: "s1"}
	ruleEngine.RootRuleChainCtx()
	s1Node, ok := ruleEngine.RootRuleChainCtx().GetNodeById(s1NodeId)
	assert.True(t, ok)

	nodeDsl := ruleEngine.NodeDSL(types.RuleNodeId{}, s1NodeId)

	assert.Equal(t, strings.Replace(` {
                "id": "s1",
                "additionalInfo": {
                  "description": "",
                  "layoutX": 0,
                  "layoutY": 0
                },
                "type": "jsFilter",
                "name": "过滤",
                "debugMode": true,
                "configuration": {
                  "jsScript": "return msg.temperature>10;"
                }
              }`, " ", "", -1), strings.Replace(string(nodeDsl), " ", "", -1))

	s1RuleNodeCtx, ok := s1Node.(*RuleNodeCtx)
	assert.True(t, ok)
	assert.Equal(t, "过滤", s1RuleNodeCtx.SelfDefinition.Name)
	assert.Equal(t, "return msg.temperature>10;", s1RuleNodeCtx.SelfDefinition.Configuration["jsScript"])

	//获取子规则链
	subChain01Id := types.RuleNodeId{Id: "subChain01", Type: types.CHAIN}
	subChain01Node, ok := ruleEngine.RootRuleChainCtx().GetNodeById(subChain01Id)
	assert.True(t, ok)
	subChain01NodeCtx, ok := subChain01Node.(*RuleChainCtx)
	assert.True(t, ok)
	assert.Equal(t, "测试子规则链", subChain01NodeCtx.SelfDefinition.RuleChain.Name)
	assert.Equal(t, subChain01NodeCtx, subRuleEngine.RootRuleChainCtx())

	//修改根规则链节点
	_ = ruleEngine.ReloadChild(s1NodeId.Id, []byte(s1NodeFile))
	s1Node, ok = ruleEngine.RootRuleChainCtx().GetNodeById(s1NodeId)
	assert.True(t, ok)
	s1RuleNodeCtx, ok = s1Node.(*RuleNodeCtx)
	assert.True(t, ok)
	assert.Equal(t, "过滤-更改", s1RuleNodeCtx.SelfDefinition.Name)
	assert.Equal(t, "return msg!='bb';", s1RuleNodeCtx.SelfDefinition.Configuration["jsScript"])

	subRuleChain := string(loadFile("./sub_chain.json"))
	//修改子规则链
	_ = subRuleEngine.ReloadSelf([]byte(strings.Replace(subRuleChain, "测试子规则链", "测试子规则链-更改", -1)))

	subChain01Node, ok = ruleEngine.RootRuleChainCtx().GetNodeById(types.RuleNodeId{Id: "subChain01", Type: types.CHAIN})
	assert.True(t, ok)
	subChain01NodeCtx, ok = subChain01Node.(*RuleChainCtx)
	assert.True(t, ok)
	assert.Equal(t, "测试子规则链-更改", subChain01NodeCtx.SelfDefinition.RuleChain.Name)

	//获取规则引擎实例
	ruleEngineNew, ok := Get("testEngine")
	assert.True(t, ok)
	assert.Equal(t, ruleEngine, ruleEngineNew)

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")

	var onAllNodeCompleted = false
	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		newMsg := ctx.NewMsg("TEST_MSG_TYPE2", types.NewMetadata(), "test")
		assert.Equal(t, "test", newMsg.Data)
		assert.Equal(t, types.JSON, newMsg.DataType)
		assert.Equal(t, "TEST_MSG_TYPE2", newMsg.Type)
	}), types.WithOnAllNodeCompleted(func() {
		onAllNodeCompleted = true
	}))
	time.Sleep(time.Millisecond * 100)
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {

	}))
	ruleEngine.OnMsg(msg)

	time.Sleep(time.Millisecond * 200)
	assert.True(t, onAllNodeCompleted)

	//删除对应规则引擎实例
	Del("testEngine")
	_, ok = Get("testEngine")
	assert.False(t, ok)
	assert.False(t, ruleEngine.Initialized())

}

func TestRuleContext(t *testing.T) {
	config := NewConfig(types.WithDefaultPool())
	config.OnEnd = func(msg types.RuleMsg, err error) {

	}
	ruleEngine, _ := New("TestRuleContext", []byte(ruleChainFile), WithConfig(config))
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")

	t.Run("hasOnEnd", func(t *testing.T) {
		ctx := NewRuleContext(context.Background(), config, ruleEngine.RootRuleChainCtx().(*RuleChainCtx), nil, nil, nil, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {

		}, nil)
		assert.Nil(t, ctx.From())
		assert.True(t, reflect.DeepEqual(ctx.Config().EndpointEnabled, config.EndpointEnabled))
		ctx.SetRuleChainPool(DefaultPool)
		assert.Equal(t, ctx.ruleChainPool, DefaultPool)

		//ctx.SetAllCompletedFunc(func() {
		//
		//})
		assert.NotNil(t, ctx.GetEndFunc())

		ruleEngine.OnMsg(msg)
		err := ruleEngine.ReloadChild("s1", []byte(""))
		assert.NotNil(t, err)
		err = ruleEngine.ReloadChild("", []byte("{"))
		assert.NotNil(t, err)

		ruleEngine.Stop()

		err = ruleEngine.ReloadChild("", []byte("{"))
		assert.Equal(t, "unexpected end of JSON input", err.Error())
		time.Sleep(time.Millisecond * 100)
	})
	t.Run("notEnd", func(t *testing.T) {
		ctx := NewRuleContext(context.Background(), config, ruleEngine.RootRuleChainCtx().(*RuleChainCtx), nil, nil, nil, nil, nil)
		ctx.DoOnEnd(msg, nil, types.Success)
	})
	t.Run("doOnEnd", func(t *testing.T) {
		ctx := NewRuleContext(context.Background(), config, ruleEngine.RootRuleChainCtx().(*RuleChainCtx), nil, nil, nil, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
		}, nil)
		ctx.DoOnEnd(msg, nil, types.Success)
	})
	t.Run("notSelf", func(t *testing.T) {
		ctx := NewRuleContext(context.Background(), config, ruleEngine.RootRuleChainCtx().(*RuleChainCtx), nil, nil, nil, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, "", relationType)
		}, nil)
		ctx.tellSelf(msg, nil, types.Success)
	})
	t.Run("notRuleChainCtx", func(t *testing.T) {
		ctx := NewRuleContext(context.Background(), config, nil, nil, nil, nil, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, "", relationType)
		}, nil)
		_, ok := ctx.getNextNodes(types.Success)
		assert.False(t, ok)
	})

	t.Run("tellSelf", func(t *testing.T) {
		selfDefinition := types.RuleNode{
			Id:            "s1",
			Type:          "log",
			Configuration: map[string]interface{}{"Add": "add"},
		}
		nodeCtx, _ := InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		ruleEngine2, _ := New("TestRuleContextTellSelf", []byte(ruleChainFile), WithConfig(config))

		ctx := NewRuleContext(context.Background(), config, ruleEngine2.RootRuleChainCtx().(*RuleChainCtx), nil, nodeCtx, nil, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			//assert.Equal(t, "", relationType)
		}, nil)

		ctx.TellSelf(msg, 1000)
		ctx.tellSelf(msg, nil, types.Success)
	})
	t.Run("WithStartNode", func(t *testing.T) {
		var count = int32(0)
		ruleEngine.OnMsg(msg, types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(4), count)
		atomic.StoreInt32(&count, 0)

		ruleEngine.OnMsgAndWait(msg, types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(4), count)
		atomic.StoreInt32(&count, 0)

		ruleEngine.OnMsg(msg, types.WithStartNode("s2"), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(2), count)
		atomic.StoreInt32(&count, 0)

		ruleEngine.OnMsg(msg, types.WithStartNode("notFound"), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}), types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, fmt.Errorf("SetExecuteNode node id=%s not found", "notFound").Error(), err.Error())
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(0), count)
	})
	t.Run("WithTellNext", func(t *testing.T) {
		var count = int32(0)
		ruleEngine.OnMsg(msg, types.WithTellNext("s1", types.True), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(3), count)
		atomic.StoreInt32(&count, 0)

		ruleEngine.OnMsg(msg, types.WithTellNext("s2", types.Success), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(1), count)
		atomic.StoreInt32(&count, 0)

		ruleEngine.OnMsgAndWait(msg, types.WithTellNext("s2", types.Success), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(1), count)
		atomic.StoreInt32(&count, 0)

		ruleEngine.OnMsg(msg, types.WithStartNode("notFound"), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}), types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, fmt.Errorf("SetExecuteNode node id=%s not found", "notFound").Error(), err.Error())
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(0), count)
	})
}

func TestOnDebug(t *testing.T) {
	var onDebugConfigWg sync.WaitGroup
	onDebugConfigWg.Add(8)
	config := NewConfig(types.WithDefaultPool())
	config.OnDebug = func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if nodeId == "s1" && flowType == types.Out {
			assert.Equal(t, types.True, relationType)
		}
		if nodeId == "s2" && flowType == types.Out {
			assert.Equal(t, types.Success, relationType)
		}
		onDebugConfigWg.Done()
	}
	ruleEngine, _ := New("testOnDebug", []byte(ruleChainFile), WithConfig(config))
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")

	t.Run("hasOnDebug", func(t *testing.T) {
		var snapshotWg sync.WaitGroup
		snapshotWg.Add(2)
		var onDebugWg sync.WaitGroup
		onDebugWg.Add(8)
		ruleEngine.OnMsg(msg, types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			assert.Equal(t, "testOnDebug", ctx.RuleChain().GetNodeId().Id)
			assert.Equal(t, "s1", ctx.GetSelfId())
			assert.Equal(t, 2, len(snapshot.Logs))
			for _, item := range snapshot.Logs {
				if item.Id == "s1" {
					assert.Equal(t, types.True, item.RelationType)
				}
				if item.Id == "s2" {
					assert.Equal(t, types.Success, item.RelationType)
				}
			}
			snapshotWg.Done()
		}), types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
			assert.Equal(t, "testOnDebug", ctx.RuleChain().GetNodeId().Id)
			if nodeRunLog.Id == "s1" {
				assert.Equal(t, "s1", ctx.GetSelfId())
				assert.Equal(t, types.True, nodeRunLog.RelationType)
			}
			if nodeRunLog.Id == "s2" {
				assert.Equal(t, "s2", ctx.GetSelfId())
				assert.Equal(t, types.Success, nodeRunLog.RelationType)
			}
		}), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			if nodeId == "s1" && flowType == types.Out {
				assert.Equal(t, types.True, relationType)
			}
			if nodeId == "s2" && flowType == types.Out {
				assert.Equal(t, types.Success, relationType)
			}
			onDebugWg.Done()
		}))

		ruleEngine.OnMsg(msg, types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			assert.Equal(t, "testOnDebug", ctx.RuleChain().GetNodeId().Id)
			assert.Equal(t, "s1", ctx.GetSelfId())
			assert.Equal(t, 2, len(snapshot.Logs))
			for _, item := range snapshot.Logs {
				if item.Id == "s1" {
					assert.Equal(t, types.True, item.RelationType)
				}
				if item.Id == "s2" {
					assert.Equal(t, types.Success, item.RelationType)
				}
			}
			snapshotWg.Done()
		}), types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
			assert.Equal(t, "testOnDebug", ctx.RuleChain().GetNodeId().Id)
			if nodeRunLog.Id == "s1" {
				assert.Equal(t, types.True, nodeRunLog.RelationType)
			}
			if nodeRunLog.Id == "s2" {
				assert.Equal(t, "s2", ctx.GetSelfId())
				assert.Equal(t, types.Success, nodeRunLog.RelationType)
			}
		}), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			if nodeId == "s1" && flowType == types.Out {
				assert.Equal(t, types.True, relationType)
			}
			if nodeId == "s2" && flowType == types.Out {
				assert.Equal(t, types.Success, relationType)
			}
			onDebugWg.Done()
		}))
		snapshotWg.Wait()
		onDebugWg.Wait()
		onDebugConfigWg.Wait()
	})
}

func TestReload(t *testing.T) {
	var ruleChainFile = `{
          "ruleChain": {
            "id": "testReload",
            "name": "testRuleChain01"
          },
          "metadata": {
            "firstNodeIndex": 0,
            "nodes": [
              {
                "id": "s1",
                "type": "jsFilter",
                "name": "过滤",
                "debugMode": true,
                "configuration": {
                  "jsScript": "${global.js}"
                }
              }
            ]
          }
        }`

	config := NewConfig(types.WithDefaultPool())
	config.Properties.PutValue("js", "return msg.temperature>10;")
	ruleEngine, err := New("testReload", []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")
	ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.True, relationType)
	}))

	config.Properties.PutValue("js", "return msg.temperature>70;")
	//刷新配置
	_ = ruleEngine.Reload(WithConfig(config))
	ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.False, relationType)
	}))
}

func TestUseVars(t *testing.T) {
	var ruleChainFile = `{
          "ruleChain": {
            "id": "testReload",
            "name": "testRuleChain01",
			"configuration": {
				"vars":{
					"js":"return msg.temperature>10;"
				}
			}
          },
          "metadata": {
            "firstNodeIndex": 0,
            "nodes": [
              {
                "id": "s1",
                "type": "jsFilter",
                "name": "过滤",
                "debugMode": true,
                "configuration": {
                  "jsScript": "${vars.js}"
                }
              }
            ]
          }
        }`

	config := NewConfig(types.WithDefaultPool())
	config.Properties.PutValue("js", "return msg.temperature>10;")
	ruleEngine, err := New("testUseVars", []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")
	ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.True, relationType)
	}))
	ruleChainFile = strings.Replace(ruleChainFile, "msg.temperature>10;", "msg.temperature>70;", 1)
	//刷新配置
	_ = ruleEngine.ReloadSelf([]byte(ruleChainFile), WithConfig(config))
	ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.False, relationType)
	}))
	go func() {
		var i = 0
		for i < 100 {
			ruleEngine.RootRuleChainCtx().Definition().RuleChain.Configuration[types.Vars] = map[string]string{"js": "return msg.temperature>30;"}
			_ = ruleEngine.Reload(WithConfig(config))
			i++
		}
	}()
	time.Sleep(time.Millisecond * 100)
	var i = 0
	for i < 200 {
		msg = types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":21,\"humidity\":90}")
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.False, relationType)
		}))
		msg = types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.True, relationType)
		}))
		i++
	}
	time.Sleep(time.Millisecond * 200)
}

func TestNoNodes(t *testing.T) {
	var ruleChainFile = `{
          "ruleChain": {
            "id": "testNoNodes",
            "name": "testRuleChain01"
          }
        }`
	var wg sync.WaitGroup
	wg.Add(1)
	config := NewConfig(types.WithDefaultPool())
	config.Properties.PutValue("js", "return msg.temperature>10;")
	ruleEngine, err := New("testNoNodes", []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")
	ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Failure, relationType)
		assert.Equal(t, "the rule chain has no nodes", err.Error())
	}), types.WithOnAllNodeCompleted(func() {
		wg.Done()
	}))
	time.Sleep(time.Millisecond * 100)
	wg.Wait()
}

func TestDoOnEnd(t *testing.T) {
	var ruleChainFile = `{
          "ruleChain": {
            "id": "testDoOnEnd",
            "name": "TestDoOnEnd"
          },
          "metadata": {
            "nodes": [
              {
                "id": "s1",
                "type": "functions",
                "name": "结束函数",
                "debugMode": true,
                "configuration": {
                  "functionName": "doEnd"
                }
              },
              {
                "id": "s2",
                "type": "log",
                "name": "记录日志",
                "debugMode": true,
                "configuration": {
                  "jsScript": "return 'Incoming';"
                }
              }
            ],
            "connections": [
              {
                "fromId": "s1",
                "toId": "s2",
                "type": "True"
              },
 				{
                "fromId": "s1",
                "toId": "s2",
                "type": "False"
              }
            ]
          }
        }`

	//测试函数
	action.Functions.Register("doEnd", func(ctx types.RuleContext, msg types.RuleMsg) {
		if msg.Metadata.GetValue("productType") == "test01" {
			ctx.TellNext(msg, types.True)
		} else {
			//中断执行规则链
			ctx.DoOnEnd(msg, nil, types.False)
		}
	})
	count := int32(0)
	config := NewConfig(types.WithDefaultPool())
	ruleEngine, err := New("testDoOnEnd", []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"body\":{\"sms\":[\"aa\"]}}")
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Success, relationType)
	}), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		atomic.AddInt32(&count, 1)
	}))
	time.Sleep(time.Millisecond * 100)
	assert.Equal(t, int32(4), count)
	count = int32(0)
	metaData.PutValue("productType", "test02")
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.False, relationType)
	}), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		atomic.AddInt32(&count, 1)
	}))
	time.Sleep(time.Millisecond * 100)
	assert.Equal(t, int32(1), count)
}

func TestJoinNode(t *testing.T) {
	var ruleChainFile = loadFile("test_join_node.json")
	var wg sync.WaitGroup
	wg.Add(1)
	config := NewConfig(types.WithDefaultPool())
	ruleEngine, err := New("testJoinNode", ruleChainFile, WithConfig(config))
	assert.Nil(t, err)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")
	ruleEngine.OnMsgAndWait(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		var result []map[string]interface{}
		json.Unmarshal([]byte(msg.Data), &result)
		assert.Equal(t, types.Success, relationType)
		assert.Equal(t, 2, len(result))
		assert.Equal(t, "node_c", result[0]["nodeId"])
		assert.Equal(t, "node_b", result[1]["nodeId"])
	}), types.WithOnAllNodeCompleted(func() {
		wg.Done()
	}))
	time.Sleep(time.Millisecond * 100)
	wg.Wait()
}
