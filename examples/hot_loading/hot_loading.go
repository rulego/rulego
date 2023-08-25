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

package main

import (
	"fmt"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"time"
)

//热更新规则链，不需要启动服务，立刻生效
//热更新规则链某个节点，不需要启动服务，立刻生效
func main() {

	config := rulego.NewConfig()

	//创建msg元数据
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	//创建规则引擎实例
	ruleEngine, err := rulego.New("rule01", []byte(chainJsonFile1), rulego.WithConfig(config))
	if err != nil {
		panic(err)
	}

	//创建msg
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")

	ruleEngine.OnMsgWithOptions(msg, types.WithEndFunc(func(msg types.RuleMsg, err error) {
		fmt.Println("处理结果=====")
		//得到规则链处理结果
		fmt.Println(msg, err)
	}))

	time.Sleep(time.Second)

	//更新s1节点
	_ = ruleEngine.ReloadChild(types.EmptyRuleNodeId, types.RuleNodeId{Id: "s1"}, []byte(s1Node))

	//重新执行
	ruleEngine.OnMsgWithOptions(msg, types.WithEndFunc(func(msg types.RuleMsg, err error) {
		fmt.Println("更新s1节点后，处理结果=====")
		//得到规则链处理结果
		fmt.Println(msg, err)
	}))

	time.Sleep(time.Second)

	//更新规则链
	_ = ruleEngine.ReloadSelf([]byte(chainJsonFile2), rulego.WithConfig(config))
	//重新执行
	ruleEngine.OnMsgWithOptions(msg, types.WithEndFunc(func(msg types.RuleMsg, err error) {
		fmt.Println("更新规则链后，处理结果=====")
		//得到规则链处理结果
		//因为推送的url:http://192.168.136.26:9099/api/msg 是无效url，所以会返回超时错误
		fmt.Println(msg, err)
	}))

	time.Sleep(time.Second * 30)
}

var s1Node = `
	{
        "id": "s1",
        "type": "jsTransform",
        "name": "转换",
        "debugMode": true,
        "configuration": {
          "jsScript": "metadata['name']='updateTest01';\n metadata['index']=33;\n msg['addField']='updateAddValue1'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
        }
      }
`
var chainJsonFile1 = `
{
  "ruleChain": {
    "name": "测试规则链",
    "root": false,
    "debugMode": false
  },
  "metadata": {
    "nodes": [
       {
        "id": "s1",
        "type": "jsTransform",
        "name": "转换",
        "debugMode": true,
        "configuration": {
          "jsScript": "metadata['name']='test01';\n metadata['index']=11;\n msg['addField']='addValue1'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
        }
      }
    ],
    "connections": [
    ],
    "ruleChainConnections": null
  }
}
`

var chainJsonFile2 = `
{
  "ruleChain": {
    "name": "测试规则链",
    "root": false,
    "debugMode": false
  },
  "metadata": {
    "nodes": [
       {
        "id": "s1",
        "type": "jsTransform",
        "name": "转换",
        "debugMode": true,
        "configuration": {
          "jsScript": "metadata['name']='test02';\n metadata['index']=22;\n msg['addField']='addValue2'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
        }
      },
      {
        "id": "s2",
        "type": "restApiCall",
        "name": "推送数据",
        "debugMode": true,
        "configuration": {
          "restEndpointUrlPattern": "http://192.168.136.26:9099/api/msg",
          "requestMethod": "POST",
          "maxParallelRequestsCount": 200
        }
      }
    ],
    "connections": [
      {
        "fromId": "s1",
        "toId": "s2",
        "type": "Success"
      }
    ],
    "ruleChainConnections": null
  }
}
`
