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
	"log"
	"time"
)

// Hot-update rule chains do not require starting the service, taking effect immediately
// Hot update to a node in the rule chain does not require starting the service; it takes effect immediately
func main() {

	config := rulego.NewConfig()

	//Create MSG metadata
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	//Create a rule engine instance
	ruleEngine, err := rulego.New("rule01", []byte(chainJsonFile1), rulego.WithConfig(config))
	if err != nil {
		log.Fatal(err)
	}

	//Create MSG
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")

	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		fmt.Println("处理结果=====")
		//Obtain the result of the rule chain processing
		fmt.Println(msg, err)
	}))

	time.Sleep(time.Second)

	//Update the s1 node
	_ = ruleEngine.ReloadChild("s1", []byte(s1Node))

	//Re-executed
	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		fmt.Println("更新s1节点后，处理结果=====")
		//Obtain the result of the rule chain processing
		fmt.Println(msg, err)
	}))

	time.Sleep(time.Second)

	//Update the rule chain
	_ = ruleEngine.ReloadSelf([]byte(chainJsonFile2), rulego.WithConfig(config))
	//Re-executed
	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		fmt.Println("更新规则链后，处理结果=====")
		//Obtain the result of the rule chain processing
		//Because the pushed url:http://192.168.136.26:9099/api/msg is an invalid URL, it will return a timeout error
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
    ]
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
    ]
  }
}
`
