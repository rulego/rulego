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
	"log"
	"time"
)

// 测试数据库操作组件dbClient
// dbClient支持对数据库的增、删、修改、查
// dbClient组件查询的数据可以继续使用其它组件对数据进行处理，
// 例如：使用`jsTransform`组件对数据进行处理、把从数据库查询数据通过`restApiCall`组件和其他系统集成
func main() {

	config := rulego.NewConfig()
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		config.Logger.Printf("flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.GetData(), msg.GetMetadata().Values(), relationType, err)
	}

	metaData := types.NewMetadata()
	metaData.PutValue("id", "1")
	metaData.PutValue("age", "18")
	metaData.PutValue("name", "test01")
	metaData.PutValue("updateAge", "21")

	//加载规则链
	ruleEngine, err := rulego.New("rule01", []byte(chainJsonFile), rulego.WithConfig(config))
	if err != nil {
		log.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41}")

	ruleEngine.OnMsg(msg)

	time.Sleep(time.Second * 2)
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
        "type": "dbClient",
        "name": "插入1条记录",
		"debugMode":true,
        "configuration": {
			"driverName":"mysql",
			"dsn":"root:root@tcp(127.0.0.1:3306)/test",
			"poolSize":5,
			"sql":"insert into users (id,name, age) values (?,?,?)",
			"params":["${metadata.id}", "${metadata.name}", "${metadata.age}"]
        }
      },
     {
        "id": "s2",
        "type": "dbClient",
        "name": "查询1条记录",
		"debugMode":true,
        "configuration": {
			"driverName":"mysql",
			"dsn":"root:root@tcp(127.0.0.1:3306)/test",
			"sql":"select * from users where id = ?",
			"params":["${metadata.id}"],
			"getOne":true
        }
      },
	  {
        "id": "s3",
        "type": "dbClient",
        "name": "查询多条记录，参数不使用占位符",
		"debugMode":true,
        "configuration": {
			"driverName":"mysql",
			"dsn":"root:root@tcp(127.0.0.1:3306)/test",
			"sql":"select * from users where age >= 18"
        }
      },
	  {
        "id": "s4",
        "type": "dbClient",
        "name": "更新记录，参数使用占位符",
		"debugMode":true,
        "configuration": {
			"driverName":"mysql",
			"dsn":"root:root@tcp(127.0.0.1:3306)/test",
			"sql":"update users set age = ? where id = ?",
			"params":["${metadata.updateAge}","${metadata.id}"]
        }
      },
	  {
        "id": "s5",
        "type": "dbClient",
        "name": "删除记录",
		"debugMode":true,
        "configuration": {
			"driverName":"mysql",
			"dsn":"root:root@tcp(127.0.0.1:3306)/test",
			"sql":"delete from users"
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
		"fromId": "s3",
		"toId": "s4",
		"type": "Success"
	  },
	{
		"fromId": "s4",
		"toId": "s5",
		"type": "Success"
	  }
    ]
  }
}
`
