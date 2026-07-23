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
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/utils/json"
	"os"
	"os/signal"
	"syscall"
)

// Develop web applications using routers
func main() {
	config := rulego.NewConfig(types.WithDefaultPool())
	//Register the rule chain
	_, _ = rulego.New("default", []byte(chainJsonFile), rulego.WithConfig(config))

	//Start the HTTP reception service
	restEndpoint := &rest.Rest{Config: rest.Config{Server: ":9090"}, RuleConfig: config}
	//Added a global interceptor
	restEndpoint.AddInterceptors(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		//Simulated authentication
		userId := exchange.In.Headers().Get("userId")
		if userId == "blacklist" {
			//Access is not permitted
			return false
		}
		//Permission validation logic
		return true
	})
	//Route 1
	router1 := endpoint.NewRouter().From("/api/v1/user/:id").Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		id := exchange.In.GetMsg().Metadata.GetValue("id")
		//Simulated query database
		user := struct {
			Id   string
			Name string
		}{Id: id, Name: "test"}
		body, _ := json.Marshal(user)
		//Response results
		exchange.Out.SetBody(body)
		return true
	}).End()

	//Register a route and get a method
	restEndpoint.GET(router1)

	//Route 2 calls the rule chain using configuration methods
	router2 := endpoint.NewRouter().From("/api/v1/userEvent").To("chain:default").End()

	//Route 3 calls the rule chain using configuration mode, with the to path with variables
	router3 := endpoint.NewRouter().From("/api/v1/msg2Chain2/:msgType").Transform(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		//Get message types
		msg.Type = msg.Metadata.GetValue("msgType")

		//Obtain the user ID from the header
		userId := exchange.In.Headers().Get("userId")
		if userId == "" {
			userId = "default"
		}
		//Store userId in the msg metadata
		msg.Metadata.PutValue("userId", userId)
		return true
	}).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		//Respond to the client
		exchange.Out.Headers().Set("Content-Type", "application/json")
		exchange.Out.SetBody([]byte("ok"))
		return true
	}).To("chain:${userId}").Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		outMsg := exchange.Out.GetMsg()
		fmt.Println("规则链处理后结果：", outMsg)
		return true
	}).End()

	//Routing 4: Direct call to node components
	router4 := endpoint.NewRouter().From("/api/v1/msgToComponent1/:msgType").Transform(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		//Get message types
		msg.Type = msg.Metadata.GetValue("msgType")
		return true
	}).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		//Respond to the client
		exchange.Out.Headers().Set("Content-Type", "application/json")
		exchange.Out.SetBody([]byte("ok"))
		return true
	}).ToComponent(func() types.Node {
		//Define log components and process data
		var configuration = make(types.Configuration)
		configuration["jsScript"] = `
		return 'log::Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);
        `
		logNode := &action.LogNode{}
		_ = logNode.Init(config, configuration)
		return logNode
	}()).End()

	//Route 5 calls node components using configuration methods
	router5 := endpoint.NewRouter().From("/api/v1/msgToComponent2/:msgType").Transform(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		//Get message types
		msg.Type = msg.Metadata.GetValue("msgType")
		return true
	}).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		//Respond to the client
		exchange.Out.Headers().Set("Content-Type", "application/json")
		exchange.Out.SetBody([]byte("ok"))
		return true
	}).To("component:log", types.Configuration{"jsScript": `
		return 'log::Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);
        `}).End()

	//Register routing and POST methods
	restEndpoint.POST(router2, router3, router4, router5)
	//And launch the service
	_ = restEndpoint.Start()

	sigs := make(chan os.Signal, 1)
	// Monitor system signals, including interrupt and termination signals
	signal.Notify(sigs, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigs:
		if restEndpoint != nil {
			restEndpoint.Destroy()
		}
		os.Exit(0)
	}
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
    ]
  }
}
`
