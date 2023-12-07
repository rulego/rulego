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

// js处理后，并调用http服务对数据进行增加处理，并得到响应结果，后继续处理http响应的body数据
// 如果http请求失败记录日志
func main() {

	config := rulego.NewConfig()

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	//js处理后，并调用http 服务对数据进行处理，并得到响应结果，后继续处理
	ruleEngine, err := rulego.New("rule01", []byte(chainJsonFile), rulego.WithConfig(config))
	if err != nil {
		panic(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")

	ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
		fmt.Println("msg处理结果=====")
		//得到规则链处理结果
		fmt.Println(msg, err)
	}))

	time.Sleep(time.Second * 30)
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
        "type": "jsTransform",
        "name": "转换",
        "configuration": {
          "jsScript": "metadata['name']='test02';\n metadata['index']=22;\n msg['addField']='addValue2'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
        }
      },
      {
        "id": "s2",
        "type": "restApiCall",
        "name": "调用restApi增强数据",
        "configuration": {
          "restEndpointUrlPattern": "http://192.168.136.26:9099/api/msgHandle",
          "requestMethod": "POST",
          "maxParallelRequestsCount": 200
        }
      },
		{
        "id": "s3",
        "type": "jsTransform",
        "name": "继续转换http响应数据",
        "configuration": {
          "jsScript": "msg['addField2']='addValue22'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
        }
      },
      {
        "id": "s4",
        "type": "log",
        "name": "请求错误记录日志",
        "configuration": {
          "jsScript": "return '请求出错\\n Incoming message:\\n' + JSON.stringify(msg) + '\\nIncoming metadata:\\n' + JSON.stringify(metadata);"
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
        "type": "Success"
      },
		{
        "fromId": "s2",
        "toId": "s4",
        "type": "Failure"
      }
    ]
  }
}
`
