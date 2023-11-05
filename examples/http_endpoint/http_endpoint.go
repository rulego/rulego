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
	"net/http"
	"os"
)

//处理http路由
func main() {

	config := rulego.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, err := rulego.New("default", []byte(defaultChain1), rulego.WithConfig(config))
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}
	_, err = rulego.New("default2", []byte(defaultChain2), rulego.WithConfig(config))
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}
	//启动http接收服务
	restEndpoint := &rest.Rest{Config: rest.Config{Server: ":9090"}, RuleConfig: config}
	//添加全局拦截器
	restEndpoint.AddInterceptors(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		userId := exchange.In.Headers().Get("userId")
		if userId == "blacklist" {
			//不允许访问
			return false
		}
		//权限校验逻辑
		return true
	})
	//路由1
	router1 := endpoint.NewRouter().From("/api/v1/hello/:name").Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		//处理请求
		request, ok := exchange.In.(*rest.RequestMessage)
		if ok {
			if request.Request().Method != http.MethodGet {
				//响应错误
				exchange.Out.SetStatusCode(http.StatusMethodNotAllowed)
				//不执行后续动作
				return false
			} else {
				//响应请求
				exchange.Out.Headers().Set("Content-Type", "application/json")
				exchange.Out.SetBody([]byte(exchange.In.From() + "\n"))
				exchange.Out.SetBody([]byte("s1 process" + "\n"))
				name := request.GetMsg().Metadata.GetValue("name")
				if name == "break" {
					//不执行后续动作
					return false
				} else {
					return true
				}

			}
		} else {
			exchange.Out.Headers().Set("Content-Type", "application/json")
			exchange.Out.SetBody([]byte(exchange.In.From()))
			exchange.Out.SetBody([]byte("s1 process" + "\n"))
			return true
		}

	}).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("s2 process" + "\n"))
		return true
	}).End()

	//注册路由,Get 方法
	restEndpoint.GET(router1)

	//路由2 采用配置方式调用规则链
	router2 := endpoint.NewRouter().From("/api/v1/msg2Chain1/:msgType").To("chain:default").End()

	//路由3 采用配置方式调用规则链,to路径带变量
	router3 := endpoint.NewRouter().From("/api/v1/msg2Chain2/:msgType").
		Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
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
		}).To("chain:${userId}").End()

	//路由4 采用配置方式调用规则链,to路径带变量，并异步响应
	router4 := endpoint.NewRouter().From("/api/v1/msg2Chain3/:msgType").
		Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
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
	}).To("chain:${userId}").End()

	//路由5 采用配置方式调用规则链,同步等待规则链执行结果，并同步响应客户端
	router5 := endpoint.NewRouter().From("/api/v1/msg2Chain4/:chainId").
		To("chain:${chainId}").
		//必须增加Wait，异步转同步，http才能正常响应，如果不响应同步响应，不要加这一句，会影响吞吐量
		Wait().
		Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
			err := exchange.Out.GetError()
			if err != nil {
				//错误
				exchange.Out.SetStatusCode(400)
				exchange.Out.SetBody([]byte(exchange.Out.GetError().Error()))
			} else {
				//把处理结果响应给客户端，http endpoint 必须增加 Wait()，否则无法正常响应
				outMsg := exchange.Out.GetMsg()
				exchange.Out.Headers().Set("Content-Type", "application/json")
				exchange.Out.SetBody([]byte(outMsg.Data))
			}

			return true
		}).End()

	//路由6 直接调用node组件方式
	router6 := endpoint.NewRouter().From("/api/v1/msgToComponent1/:msgType").
		Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
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

	//路由7 采用配置方式调用node组件
	router7 := endpoint.NewRouter().From("/api/v1/msgToComponent2/:msgType").
		Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
			msg := exchange.In.GetMsg()
			//获取消息类型
			msg.Type = msg.Metadata.GetValue("msgType")
			return true
		}).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		//响应给客户端
		//exchange.Out.SetBody([]byte("ok"))
		return true
	}).To("component:log", types.Configuration{"jsScript": `
		return 'log::Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);
        `}).
		Wait().
		Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
			//把处理结果，同步响应给前端，http endpoint 必须增加 Wait()，否则无法正常响应
			outMsg := exchange.Out.GetMsg()
			exchange.Out.Headers().Set("Content-Type", "application/json")
			exchange.Out.SetBody([]byte(outMsg.Data))
			return true
		}).End()

	//注册路由，POST方式
	restEndpoint.POST(router2, router3, router4, router5, router6, router7)
	//并启动服务
	_ = restEndpoint.Start()
}

var defaultChain1 = `
{
  "ruleChain": {
    "name": "测试规则链",
	"id":"default"
  },
  "metadata": {
    "nodes": [
       {
        "id": "s1",
        "type": "jsTransform",
        "name": "转换",
        "debugMode": true,
        "configuration": {
          "jsScript": "metadata['name']='defaultTest02';\n metadata['index']=11;\n msg['addField']='defaultAddValue2'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
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
    ]
  }
}
`

var defaultChain2 = `
{
  "ruleChain": {
    "name": "测试规则链",
    "id":"default2"
  },
  "metadata": {
    "nodes": [
       {
        "id": "s1",
        "type": "jsTransform",
        "name": "转换",
        "debugMode": true,
        "configuration": {
          "jsScript": "metadata['name']='default2Test02';\n metadata['index']=22;\n msg['addField']='default2AddValue2'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
        }
      }
    ],
    "connections": [
    ]
  }
}
`
