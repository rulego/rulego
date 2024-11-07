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
	"context"
	"fmt"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"log"
	"time"
)

var (
	shareKey      = "shareKey"
	shareValue    = "shareValue"
	addShareKey   = "addShareKey"
	addShareValue = "addShareValue"
)

// 演示自定义组件
func main() {

	//注册自定义组件
	rulego.Registry.Register(&UpperNode{})
	rulego.Registry.Register(&TimeNode{})

	config := rulego.NewConfig()
	ruleEngine, err := rulego.New("rule01", []byte(chainJsonFile), rulego.WithConfig(config))
	if err != nil {
		log.Fatal(err)
	}
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":41}")

	ruleEngine.OnMsg(msg, types.WithContext(context.WithValue(context.Background(), shareKey, shareValue)), types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		fmt.Println(msg)
	}))
	time.Sleep(time.Second * 1)
}

var chainJsonFile = `
{
  "ruleChain": {
	"id":"rule01",
    "name": "测试数据共享规则链"
  },
  "metadata": {
    "nodes": [
      {
        "id": "s1",
        "type": "test/upper",
        "name": "test upper",
        "debugMode": false
      },
      {
        "id":"s2",
        "type": "test/time",
        "name": "add time",
        "debugMode": false
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
