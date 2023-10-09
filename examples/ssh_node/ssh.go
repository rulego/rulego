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

//测试ssh执行命令或者shell脚本
//脚本返回结果，会通过msg返回给下一个节点
//测试前，请配置正确的ssh登录信息
//count.sh 内容
//#!/bin/sh
//echo "The first argument is $1"
func main() {

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	msg1 := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")

	ruleEngine.OnMsgWithOptions(msg1)

	time.Sleep(time.Second * 1)
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
        "type": "ssh",
        "name": "执行shell1",
        "configuration": {
			"host": "192.168.1.1",
			"port": 22,
			"username": "root",
			"password": "aaaaaa",
			"cmd": "ls /root"
        }
       },
      {
        "id": "s2",
        "type": "log",
        "name": "打印脚本返回结果",
        "configuration": {
           "jsScript": "return '执行shell1结果：\\n'+msg+'\\n';"
        }
      },  {
        "id": "s3",
        "type": "ssh",
        "name": "执行shell2",
        "configuration": {
			"host": "192.168.1.1",
			"port": 22,
			"username": "root",
			"password": "aaaaaa",
			"cmd": "sh /root/count.sh lala"
        }
       },
      {
        "id": "s4",
        "type": "log",
        "name": "打印脚本返回结果",
        "configuration": {
          "jsScript": "return '执行shell2结果：\\n'+msg+'\\n';"
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
      },
    {
        "fromId": "s2",
        "toId": "s3",
        "type": "Success"
      },
	{
        "fromId": "s3",
        "toId": "s4",
        "type": "Success"
      },
     {
        "fromId": "s3",
        "toId": "s4",
        "type": "Failure"
      }
    ]
  }
}
`
