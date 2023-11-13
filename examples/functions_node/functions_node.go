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
	"github.com/rulego/rulego/components/action"
	"time"
)

var ruleEngine *rulego.RuleEngine

//初始化自定义函数、规则引擎实例和配置
func init() {
	action.Functions.Register("add", func(ctx types.RuleContext, msg types.RuleMsg) {
		ctx.TellSuccess(msg)
	})
	action.Functions.Register("filterMsg", func(ctx types.RuleContext, msg types.RuleMsg) {
		if msg.Type == "TEST_MSG_TYPE1" {
			ctx.TellNext(msg, types.True)
		} else {
			ctx.TellNext(msg, types.False)
		}

	})
	action.Functions.Register("handleMsg", func(ctx types.RuleContext, msg types.RuleMsg) {
		msg.Data = "{\"handleMsgAdd\":\"handleMsgAddValue\"}"
		ctx.TellSuccess(msg)
	})

	config := rulego.NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		config.Logger.Printf("flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
	}
	var err error
	ruleEngine, err = rulego.New("rule01", []byte(chainJsonFile), rulego.WithConfig(config))
	if err != nil {
		panic(err)
	}
}

//测试使用占位符替换配置
func main() {

	//元数据
	metaData := types.NewMetadata()
	metaData.PutValue("postUrl", "http://127.0.0.1:8080/api/msg")

	//处理数据
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")
	ruleEngine.OnMsgWithOptions(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		//得到规则链处理结果
		fmt.Println("执行结果", msg, err)
	}))

	time.Sleep(time.Second * 5)
}

var chainJsonFile = `
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
        "type": "functions",
        "name": "调用函数1",
		"debugMode": true,
        "configuration": {
          "functionName": "add"
        }
      },
     {
        "id": "s2",
        "type": "functions",
        "name": "调用函数2",
		"debugMode": true,
        "configuration": {
          "functionName": "filterMsg"
        }
      },
      {
        "id": "s3",
        "type": "functions",
        "name": "调用函数3",
		"debugMode": true,
        "configuration": {
			"functionName": "handleMsg"
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
        "fromId": "s2",
        "toId": "s3",
        "type": "True"
      }
    ]
  }
}
`
