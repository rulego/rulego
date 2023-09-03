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
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/utils/json"
)

//使用router开发web应用
func main() {

	config := rulego.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = rulego.New("default", []byte(chainJsonFile), rulego.WithConfig(config))

	//启动http接收服务
	restEndpoint := &rest.Rest{Config: rest.Config{Server: ":9090"}}
	//添加全局拦截器
	restEndpoint.AddInterceptors(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		//模拟鉴权
		userId := exchange.In.Headers().Get("userId")
		if userId == "blacklist" {
			//不允许访问
			return false
		}
		//权限校验逻辑
		return true
	})
	//路由1
	router1 := endpoint.NewRouter().From("/api/v1/user/:id").Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		id := exchange.In.GetMsg().Metadata.GetValue("id")
		//模拟查询数据库
		user := struct {
			Id   string
			Name string
		}{Id: id, Name: "test"}
		body, _ := json.Marshal(user)
		//响应结果
		exchange.Out.SetBody(body)
		return true
	}).End()

	//注册路由,Get 方法
	restEndpoint.GET(router1)

	//路由2 采用配置方式调用规则链
	router2 := endpoint.NewRouter().From("/api/v1/userEvent").To("chain:default").End()

	//路由3 采用配置方式调用规则链,to路径带变量
	router3 := endpoint.NewRouter().From("/api/v1/msg2Chain2/:msgType").Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		//获取消息类型
		msg.Type = msg.Metadata.GetValue("msgType")

		//从header获取用户ID
		userId := exchange.In.Headers().Get("userId")
		if userId == "" {
			userId = "default"
		}
		//把userId存放在msg元数据
		msg.Metadata.PutValue("userId", userId)
		return true
	}).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		//响应给客户端
		exchange.Out.Headers().Set("Content-Type", "application/json")
		exchange.Out.SetBody([]byte("ok"))
		return true
	}).To("chain:${userId}").Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		outMsg := exchange.Out.GetMsg()
		fmt.Println("规则链处理后结果：", outMsg)
		return true
	}).End()

	//路由4 直接调用node组件方式
	router4 := endpoint.NewRouter().From("/api/v1/msgToComponent1/:msgType").Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		//获取消息类型
		msg.Type = msg.Metadata.GetValue("msgType")
		return true
	}).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		//响应给客户端
		exchange.Out.Headers().Set("Content-Type", "application/json")
		exchange.Out.SetBody([]byte("ok"))
		return true
	}).ToComponent(func() types.Node {
		//定义日志组件，处理数据
		var configuration = make(types.Configuration)
		configuration["jsScript"] = `
		return 'log::Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);
        `
		logNode := &action.LogNode{}
		_ = logNode.Init(config, configuration)
		return logNode
	}()).End()

	//路由5 采用配置方式调用node组件
	router5 := endpoint.NewRouter().From("/api/v1/msgToComponent2/:msgType").Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		//获取消息类型
		msg.Type = msg.Metadata.GetValue("msgType")
		return true
	}).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		//响应给客户端
		exchange.Out.Headers().Set("Content-Type", "application/json")
		exchange.Out.SetBody([]byte("ok"))
		return true
	}).To("component:log", types.Configuration{"jsScript": `
		return 'log::Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);
        `}).End()

	//注册路由，POST方式
	restEndpoint.POST(router2, router3, router4, router5)
	//并启动服务
	_ = restEndpoint.Start()
}

var chainJsonFile = `
{
  "ruleChain": {
    "name": "测试规则链",
    "root": false,
    "debugMode": false
  },
  "metadata": {
    "nodes": [
       {
        "id": "s1",
        "type": "jsTransform",
        "name": "转换",
        "debugMode": true,
        "configuration": {
          "jsScript": "metadata['name']='test02';\n metadata['index']=22;\n msg['addField']='addValue2'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
        }
      },
      {
        "id": "s2",
        "type": "restApiCall",
        "name": "推送数据",
        "debugMode": true,
        "configuration": {
          "restEndpointUrlPattern": "http://192.168.136.26:9099/api/msg",
          "requestMethod": "POST",
          "maxParallelRequestsCount": 200
        }
      }
    ],
    "connections": [
      {
        "fromId": "s1",
        "toId": "s2",
        "type": "Success"
      }
    ],
    "ruleChainConnections": null
  }
}
`
