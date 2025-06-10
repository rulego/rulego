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
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	mqttEndpoint "github.com/rulego/rulego/endpoint/mqtt"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/utils/mqtt"
	"log"
)

// 使用相同路由逻辑处理http和mqtt数据
// HTTP POST http://127.0.0.1:9090/api/v1/msg
// MQTT pub topic /api/v1/msg
func main() {
	config := rulego.NewConfig(types.WithDefaultPool())
	//定义路由 采用配置方式调用node组件
	router := endpoint.NewRouter().From("/api/v1/msg").Transform(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		return true
	}).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		return true
	}).To("component:log", types.Configuration{"jsScript": `
		return 'log::Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata)+'\n msgType='+msgType;
        `}).End()

	//创建mqtt endpoint服务
	_mqttEndpoint, err := endpoint.Registry.New(mqttEndpoint.Type, config, mqtt.Config{
		Server: "127.0.0.1:1883",
	})
	if err != nil {
		//退出程序
		log.Fatal(err)
	}
	//添加全局拦截器
	_mqttEndpoint.AddInterceptors(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		//权限校验逻辑
		return true
	})
	//注册路由并启动服务
	_, _ = _mqttEndpoint.AddRouter(router)

	err = _mqttEndpoint.Start()
	if err != nil {
		log.Fatal(err)
	}
	//创建http接收服务
	_restEndpoint, err := endpoint.Registry.New(rest.Type, config, mqtt.Config{
		Server: ":9090",
	})
	//添加全局拦截器
	_restEndpoint.AddInterceptors(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		userId := exchange.In.Headers().Get("userId")
		if userId == "blacklist" {
			//不允许访问
			return false
		}
		//权限校验逻辑
		return true
	})

	//注册路由，POST方式
	_, _ = _restEndpoint.AddRouter(router, "POST")
	//启动http服务
	err = _restEndpoint.Start()
	if err != nil {
		log.Fatal(err)
	}
}
