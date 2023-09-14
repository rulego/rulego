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

//根据不同消息类型，路由到不同节点处理
func main() {

	config := rulego.NewConfig()
	ruleEngine, err := rulego.New("rule01", []byte(chainJsonFile), rulego.WithConfig(config))
	if err != nil {
		panic(err)
	}
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	//TEST_MSG_TYPE1 找到2条chains
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsg(msg)

	//TEST_MSG_TYPE2 找到1条chain
	msg = types.NewMsg(0, "TEST_MSG_TYPE2", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsg(msg)

	//TEST_MSG_TYPE3 找到0条chain
	msg = types.NewMsg(0, "TEST_MSG_TYPE3", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsg(msg)

	time.Sleep(time.Second * 3)
}

var chainJsonFile = `
{
  "ruleChain": {
	"id":"rule01",
    "name": "测试规则链-msgTypeSwitch",
    "root": true
  },
  "metadata": {
    "nodes": [
      {
        "id": "s1",
        "type": "msgTypeSwitch",
        "name": "过滤",
        "debugMode": true
      },
      {
        "id": "s2",
        "type": "log",
        "name": "记录日志1",
        "debugMode": true,
        "configuration": {
          "jsScript": "return 'handle msgType='+ msgType+':s2';"
        }
      },
      {
        "id": "s3",
        "type": "log",
        "name": "记录日志2",
        "debugMode": true,
        "configuration": {
          "jsScript": "return 'handle msgType='+ msgType+':s3';"
        }
      },
      {
        "id": "s4",
        "type": "log",
        "name": "记录日志3",
        "debugMode": true,
        "configuration": {
          "jsScript": "return 'handle msgType='+ msgType+':s4';"
        }
      }
    ],
    "connections": [
      {
        "fromId": "s1",
        "toId": "s2",
        "type": "TEST_MSG_TYPE1"
      },
      {
        "fromId": "s1",
        "toId": "s3",
        "type": "TEST_MSG_TYPE1"
      },
      {
        "fromId": "s1",
        "toId": "s4",
        "type": "TEST_MSG_TYPE2"
      }
    ],
    "ruleChainConnections": null
  }
}
`
