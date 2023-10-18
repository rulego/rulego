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
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/utils/json"
)

//演示获取所有组件配置表单列表接口
//GET http:{ip}:9090/api/v1/components

func main() {

	config := rulego.NewConfig(types.WithDefaultPool())
	//启动http接收服务
	restEndpoint := &rest.Rest{Config: rest.Config{Server: ":9090"}, RuleConfig: config}
	//添加全局拦截器
	restEndpoint.AddInterceptors(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.Headers().Set("Content-Type", "application/json")
		exchange.Out.Headers().Set("Access-Control-Allow-Origin", "*")
		userId := exchange.In.Headers().Get("userId")
		if userId == "blacklist" {
			//不允许访问
			return false
		}
		//权限校验逻辑
		return true
	})
	//路由1
	router1 := endpoint.NewRouter().From("/api/v1/components").Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {

		//响应组件配置表单列表
		list, err := json.Marshal(rulego.Registry.GetComponentForms().Values())
		if err != nil {
			exchange.Out.SetStatusCode(400)
			exchange.Out.SetBody([]byte(err.Error()))
		} else {
			exchange.Out.SetBody(list)
		}
		return true
	}).End()

	//注册路由，POST方式
	restEndpoint.GET(router1)
	//并启动服务
	_ = restEndpoint.Start()
}
