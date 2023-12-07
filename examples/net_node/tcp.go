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

// 初始化规则引擎实例和配置
func init() {
	config := rulego.NewConfig()
	var err error
	ruleEngine, err = rulego.New("rule01", []byte(chainJsonFile1), rulego.WithConfig(config))
	if err != nil {
		panic(err)
	}
}

// 测试tcp节点
func main() {

	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")

	msg1 := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")

	ruleEngine.OnMsg(msg1)

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
	  "type": "net",
	  "name": "推送数据",
	  "configuration": {
		"protocol": "tcp",
		"server": "127.0.0.1:8888"
	  }
	 }
      
    ],
    "connections": []
    
  }
}
`
