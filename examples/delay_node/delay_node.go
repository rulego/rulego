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
	"sync"
	"time"
)

var ruleEngine *rulego.RuleEngine

//初始化规则引擎实例和配置
func init() {
	config := rulego.NewConfig()
	//config.OnDebug = func(chainId,flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
	//config.Logger.Printf("flowType=%s,nodeId=%s,msgId=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Id, msg.Data, msg.Metadata, relationType, err)
	//}
	var err error
	ruleEngine, err = rulego.New("rule01", []byte(chainJsonFile1), rulego.WithConfig(config))
	if err != nil {
		panic(err)
	}
}

//测试延迟组件
func main() {
	//var i = 0
	//for i < 20 {

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	msg1 := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")

	fmt.Println("msg1 id=" + msg1.Id)
	var wg sync.WaitGroup
	wg.Add(1)
	start := time.Now()
	//第1条,走Success链
	ruleEngine.OnMsg(msg1, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		fmt.Println("用时:" + time.Since(start).String())
		useTime := time.Since(start)
		if useTime < time.Second {
			panic("延迟组件不生效")
		}
		wg.Done()
	}))

	time.Sleep(time.Millisecond * 200)
	msg2 := types.NewMsg(0, "TEST_MSG_TYPE2", types.JSON, metaData, "{\"temperature\":42}")

	//fmt.Println("msg2 id=" + msg2.Id)
	//第2条，因为队列已经满，走Failure链
	ruleEngine.OnMsg(msg2)

	wg.Wait()
	//	i++
	//}
	//time.Sleep(time.Second * 1)
}

var chainJsonFile1 = `
{
  "ruleChain": {
	"id":"rule01",
    "name": "测试规则链",
    "root": true
  },
  "metadata": {
    "nodes": [
       {
        "id": "s1",
        "type": "delay",
        "name": "执行延迟组件",
        "configuration": {
			"periodInSeconds": 1,
			"maxPendingMsgs": 1
        }
       },
      {
        "id": "s2",
        "type": "log",
        "name": "打印结果",
		"debugMode": true,
        "configuration": {
           "jsScript": "return '日志打印：\\n'+JSON.stringify(msg)+'\\n'+msgType+'\\n';"
        }
      }
    ],
    "connections": [
     {
        "fromId": "s1",
        "toId": "s2",
        "type": "Success"
      },
     {
        "fromId": "s1",
        "toId": "s2",
        "type": "Failure"
      }
    ]
  }
}
`
