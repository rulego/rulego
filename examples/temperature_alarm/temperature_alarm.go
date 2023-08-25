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
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"time"
)

//处理规则链，如果温度大于50，则温度异常调用api推送告警，否则记录日志
func main() {
	//创建rule config
	config := rulego.NewConfig()
	//初始化规则引擎实例
	ruleEngine, err := rulego.New("rule01", []byte(chainJsonFile), rulego.WithConfig(config))
	if err != nil {
		panic(err)
	}

	//消息1 温度正常，没大于50度
	//消息元数据
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "productType01")
	//创建消息体
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":35}")
	//处理消息
	ruleEngine.OnMsg(msg)

	//消息2 温度异常，没大于50度
	msg = types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":65}")
	//处理消息
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Second * 40)
}

var chainJsonFile = `
{
  "ruleChain": {
    "name": "rule01",
    "root": true,
    "debugMode": false
  },
  "metadata": {
    "nodes": [
      {
        "id": "s1",
        "type": "jsFilter",
        "name": "过滤",
        "debugMode": true,
        "configuration": {
          "jsScript": "return msg.temperature>50;"
        }
      },
      {
        "id": "s2",
        "type": "restApiCall",
        "name": "推送告警",
        "debugMode": true,
        "configuration": {
          "restEndpointUrlPattern": "http://192.168.136.26:9099/api/msg/err",
          "requestMethod": "POST",
          "maxParallelRequestsCount": 200
        }
      },
      {
        "id": "s3",
        "type": "log",
        "name": "温度正常，记录日志",
        "debugMode": true,
        "configuration": {
          "jsScript": "return '温度正常。\\n Incoming message:\\n' + JSON.stringify(msg) + '\\nIncoming metadata:\\n' + JSON.stringify(metadata);"
        }
      },
      {
        "id": "s4",
        "type": "log",
        "name": "温度异常，记录推送告警结果",
        "debugMode": true,
        "configuration": {
          "jsScript": "return '温度异常，记录推送异常日志。\\n Incoming message:\\n' + JSON.stringify(msg) + '\\nIncoming metadata:\\n' + JSON.stringify(metadata);"
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
        "toId": "s3",
        "type": "False"
      },
      {
        "fromId": "s2",
        "toId": "s4",
        "type": "Failure"
      }
    ],
    "ruleChainConnections": null
  }
}
`
