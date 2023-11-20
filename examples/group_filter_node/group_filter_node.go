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
	"strings"
	"time"
)

var ruleEngine *rulego.RuleEngine

//初始化规则引擎实例和配置
func init() {
	config := rulego.NewConfig()
	var err error
	ruleEngine, err = rulego.New("rule01", []byte(chainJsonFile1), rulego.WithConfig(config))
	if err != nil {
		panic(err)
	}
}

func main() {

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	msg1 := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")

	fmt.Println("第一次执行，allMatches=false：")
	ruleEngine.OnMsg(msg1)
	time.Sleep(time.Second * 1)

	fmt.Println("第二次执行，allMatches=true：")
	//更新规则链，groupFilter必须所有节点都满足True,才走True链
	_ = ruleEngine.ReloadSelf([]byte(strings.Replace(chainJsonFile1, `"allMatches":false`, `"allMatches":true`, -1)))

	ruleEngine.OnMsg(msg1)
	time.Sleep(time.Second * 1)

	fmt.Println("第三次次执行，allMatches=true：")
	msg2 := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":51,\"humidity\":90}")
	ruleEngine.OnMsg(msg2)
	time.Sleep(time.Second * 1)
}

//注意：规则链从第三个节点开始触发。firstNodeIndex=2
var chainJsonFile1 = `
{
  "ruleChain": {
	"id":"rule01",
    "name": "测试规则链",
    "root": true
  },
  "metadata": {
	"firstNodeIndex":2,
    "nodes": [
       {
       "id": "s1",
       "type": "jsFilter",
       "name": "过滤1",
       "debugMode": true,
       "configuration": {
         "jsScript": "return msg.temperature > 50;"
       }
     },
	{
       "id": "s2",
       "type": "jsFilter",
       "name": "过滤2",
       "debugMode": true,
       "configuration": {
         "jsScript": "return msg.humidity > 80;"
       }
     },
	{
       "id": "group1",
       "type": "groupFilter",
       "name": "过滤组",
       "debugMode": true,
       "configuration": {
		"allMatches":false,
         "nodeIds": "s1,s2"
       }
     },
	{
	   "id": "s3",
	   "type": "log",
	   "name": "记录日志",
	   "debugMode": false,
	   "configuration": {
		 "jsScript": "return 'call this node for True relation';"
	   }
	 },
	{
	   "id": "s4",
	   "type": "log",
	   "name": "记录日志",
	   "debugMode": false,
	   "configuration": {
		 "jsScript": "return 'call this node for False relation';"
	   }
	}
    ],
    "connections": [
     {
        "fromId": "group1",
        "toId": "s3",
        "type": "True"
      },
     {
        "fromId": "group1",
        "toId": "s4",
        "type": "False"
      }
    ]
  }
}
`
