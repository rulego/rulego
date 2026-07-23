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
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/utils/json"
	"os"
	"os/signal"
	"syscall"
)

//Demonstration of obtaining all component configuration form list interfaces
//GET http:{ip}:9090/api/v1/components

func main() {

	config := rulego.NewConfig(types.WithDefaultPool())
	//Start the HTTP reception service
	restEndpoint := &rest.Rest{Config: rest.Config{Server: ":9090"}, RuleConfig: config}
	//Added a global interceptor
	restEndpoint.AddInterceptors(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		exchange.Out.Headers().Set("Content-Type", "application/json")
		exchange.Out.Headers().Set("Access-Control-Allow-Origin", "*")
		userId := exchange.In.Headers().Get("userId")
		if userId == "blacklist" {
			//Access is not permitted
			return false
		}
		//Permission validation logic
		return true
	})
	//Route 1
	router1 := endpoint.NewRouter().From("/api/v1/components").Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {

		//The response component configures the list of forms
		list, err := json.Marshal(rulego.Registry.GetComponentForms().Values())
		if err != nil {
			exchange.Out.SetStatusCode(400)
			exchange.Out.SetBody([]byte(err.Error()))
		} else {
			exchange.Out.SetBody(list)
		}
		return true
	}).End()

	//Register routing and POST methods
	restEndpoint.GET(router1)
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
