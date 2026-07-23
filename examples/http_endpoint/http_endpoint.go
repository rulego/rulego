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
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// Handle HTTP routing
func main() {

	config := rulego.NewConfig(types.WithDefaultPool())
	//Register the rule chain
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
	//Create an HTTP endpoint service
	restEndpoint, err := endpoint.Registry.New(rest.Type, config, rest.Config{
		Server: ":9090",
	})

	//Added a global interceptor
	restEndpoint.AddInterceptors(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		userId := exchange.In.Headers().Get("userId")
		if userId == "blacklist" {
			//Access is not permitted
			return false
		}
		//Permission validation logic
		return true
	})
	//Route 1
	router1 := endpoint.NewRouter().From("/api/v1/hello/:name").Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		//Processing requests
		request, ok := exchange.In.(*rest.RequestMessage)
		if ok {
			if request.Request().Method != http.MethodGet {
				//Response errors
				exchange.Out.SetStatusCode(http.StatusMethodNotAllowed)
				//Do not perform subsequent actions
				return false
			} else {
				//Responding to requests
				exchange.Out.Headers().Set("Content-Type", "application/json")
				exchange.Out.SetBody([]byte(exchange.In.From() + "\n"))
				exchange.Out.SetBody([]byte("s1 process" + "\n"))
				name := request.GetMsg().Metadata.GetValue("name")
				if name == "break" {
					//Do not perform subsequent actions
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

	}).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		exchange.Out.SetBody([]byte("s2 process" + "\n"))
		return true
	}).End()

	//Route 2 calls the rule chain using configuration methods
	router2 := endpoint.NewRouter().From("/api/v1/msg2Chain1/:msgType").To("chain:default").End()

	//Route 3 calls the rule chain using configuration mode, with the to path with variables
	router3 := endpoint.NewRouter().From("/api/v1/msg2Chain2/:msgType").
		Transform(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
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
		}).To("chain:${userId}").End()

	//Route 4 calls the rule chain in a configuration manner, with a to-path variable and asynchronous response
	router4 := endpoint.NewRouter().From("/api/v1/msg2Chain3/:msgType").
		Transform(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
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
	}).To("chain:${userId}").End()

	//Route 5 calls the rule chain in a configuration manner, synchronously waits for the execution result of the rule chain, and responds to the client in sync
	router5 := endpoint.NewRouter().From("/api/v1/msg2Chain4/:chainId").
		To("chain:${chainId}").
		//You must add Wait and switch from asynchronous to synchronous for HTTP to respond properly. If it doesn't respond synchronously, don't add this phrase, as it will affect throughput
		Wait().
		Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
			err := exchange.Out.GetError()
			if err != nil {
				//Wrong
				exchange.Out.SetStatusCode(400)
				exchange.Out.SetBody([]byte(exchange.Out.GetError().Error()))
			} else {
				//Deliver the processing result to the client; the HTTP endpoint must add Wait(), otherwise it cannot respond properly
				outMsg := exchange.Out.GetMsg()
				exchange.Out.Headers().Set("Content-Type", "application/json")
				exchange.Out.SetBody([]byte(outMsg.GetData()))
			}

			return true
		}).End()

	//Route 6: Direct call to node components
	router6 := endpoint.NewRouter().From("/api/v1/msgToComponent1/:msgType").
		Transform(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
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

	//Route 7 calls node components using configuration methods
	router7 := endpoint.NewRouter().From("/api/v1/msgToComponent2/:msgType").
		Transform(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
			msg := exchange.In.GetMsg()
			//Get message types
			msg.Type = msg.Metadata.GetValue("msgType")
			return true
		}).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		//Respond to the client
		//exchange.Out.SetBody([]byte("ok"))
		return true
	}).To("component:log", types.Configuration{"jsScript": `
		return 'log::Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);
        `}).
		Wait().
		Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
			//Synchronize the processing results to the front end; the HTTP endpoint must add Wait(), otherwise it cannot respond properly
			outMsg := exchange.Out.GetMsg()
			exchange.Out.Headers().Set("Content-Type", "application/json")
			exchange.Out.SetBody([]byte(outMsg.GetData()))
			return true
		}).End()

	//Register a route and get a method
	_, _ = restEndpoint.AddRouter(router1, "GET")
	//Register routing and POST methods
	_, _ = restEndpoint.AddRouter(router2, "POST")
	_, _ = restEndpoint.AddRouter(router3, "POST")
	_, _ = restEndpoint.AddRouter(router4, "POST")
	_, _ = restEndpoint.AddRouter(router5, "POST")
	_, _ = restEndpoint.AddRouter(router6, "POST")
	_, _ = restEndpoint.AddRouter(router7, "POST")

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
          "jsScript": "msg=msg||{};metadata['name']='defaultTest02';\n metadata['index']=11;\n msg['addField']='defaultAddValue2'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
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
