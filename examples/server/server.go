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
	"flag"
	"fmt"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/mqtt"
	"github.com/rulego/rulego/endpoint"
	endpointMqtt "github.com/rulego/rulego/endpoint/mqtt"
	endpointRest "github.com/rulego/rulego/endpoint/rest"
	"log"
	"net/http"
	//_ "net/http/pprof"
	"os"
	"strconv"
	"strings"
)

const (
	// base HTTP paths.
	apiVersion  = "v1"
	apiBasePath = "/api/" + apiVersion + "/"

	// path to rule. /msg/{tenant_id}/{msg_type}
	msgPath = apiBasePath + "msg/:msgType"
	// /rule/{tenant_id}/{rule_id}
	rulePath     = apiBasePath + "rule/"
	ruleNodePath = apiBasePath + "rule/:nodeId"
	// server version.
	version = "1.0.0"
)

var (
	httpAvailable bool
	port          int
	logfile       string
	ver           bool
	ruleFile      string
	ruleEngine    *rulego.RuleEngine

	subscribeTopics  string
	mqttClientConfig = mqtt.Config{}
	mqttAvailable    bool
)

func init() {
	flag.BoolVar(&mqttAvailable, "mqtt", true, "mqtt client available.")
	flag.StringVar(&mqttClientConfig.Server, "server", "127.0.0.1:1883", "Server of the mqtt broker.")
	flag.StringVar(&mqttClientConfig.Username, "username", "", "username of the mqtt client.")
	flag.StringVar(&mqttClientConfig.Password, "password", "", "Password of the mqtt client.")
	flag.DurationVar(&mqttClientConfig.MaxReconnectInterval, "max_reconnect_interval", 100000*100000*60, "max_reconnect_interval of reconnect the mqtt broker.")
	flag.BoolVar(&mqttClientConfig.CleanSession, "clean-session", false, "cleanSession.")
	flag.StringVar(&mqttClientConfig.ClientID, "client_id", "", "client_id of the client.")
	flag.StringVar(&mqttClientConfig.CAFile, "ca_file", "", "ca_file of the client.")
	flag.StringVar(&mqttClientConfig.CertFile, "cert_file", "", "cert_file of the client.")
	flag.StringVar(&mqttClientConfig.CertKeyFile, "key_file", "", "key_file of the client.")
	flag.StringVar(&subscribeTopics, "topics", "#", "subscribe the topics .")

	flag.StringVar(&ruleFile, "rule_file", "", "Location of the rule_file.")

	flag.IntVar(&port, "port", 9090, "The port to listen on.")
	flag.StringVar(&logfile, "log_file", "", "Location of the log_file.")
	flag.BoolVar(&ver, "version", false, "Print server version.")

}

func main() {

	flag.Parse()

	if ver {
		fmt.Printf("RuleGo Server v%s", version)
		os.Exit(0)
	}

	var logger *log.Logger

	if logfile == "" {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	} else {
		f, err := os.OpenFile(logfile, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}
		logger = log.New(f, "", log.LstdFlags)
	}

	if ruleFile == "" {
		fmt.Println("not the root rule file,set the flag of rule_file")
		os.Exit(0)
	} else {
		buf, err := os.ReadFile(ruleFile)
		if err != nil {
			logger.Fatal("parser rule file error:", err)
		}
		config := rulego.NewConfig(types.WithDefaultPool())
		//调试模式回调信息
		//debugMode=true 的节点会打印
		config.OnDebug = func(flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			config.Logger.Printf("flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
		}
		ruleEngine, err = rulego.New("default", buf, rulego.WithConfig(config))
		if err != nil {
			logger.Fatal("parser rule file error:", err)
		}
	}

	if mqttClientConfig.Server != "" {
		//mqtt 订阅服务 接收端点
		mqttEndpoint := &endpointMqtt.Mqtt{
			Config: mqttClientConfig,
		}
		for _, topic := range strings.Split(subscribeTopics, ",") {
			router := endpoint.NewRouter().From(topic).To("chain:default").End()
			mqttEndpoint.AddRouter(router)
		}
		if err := mqttEndpoint.Start(); err != nil {
			logger.Fatal(err)
		}
	}
	strPort := ":" + strconv.Itoa(port)
	restServe(logger, strPort)

}

//rest服务 接收端点
func restServe(logger *log.Logger, addr string) {
	logger.Print("server initialised.")
	restEndpoint := &endpointRest.Rest{
		Config: endpointRest.Config{Server: addr},
	}
	//处理请求，并转发到规则引擎
	restEndpoint.POST(endpoint.NewRouter().From(msgPath).Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		//获取消息类型
		msg.Type = msg.Metadata.GetValue("msgType")

		//用户ID
		userId := exchange.In.Headers().Get("userId")
		if userId == "" {
			userId = "default"
		}
		msg.Metadata.PutValue("userId", userId)
		return true
	}).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetStatusCode(http.StatusOK)
		return true
	}).To("chain:${userId}").End())

	//获取根规则链DSL
	restEndpoint.GET(endpoint.NewRouter().From(rulePath).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		getDsl("", exchange)
		return true
	}).End())
	//获取某个节点DSL
	restEndpoint.GET(endpoint.NewRouter().From(ruleNodePath).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		nodeId := msg.Metadata.GetValue("nodeId")
		getDsl(nodeId, exchange)
		return true
	}).End())

	//修改根规则链DSL
	restEndpoint.PUT(endpoint.NewRouter().From(rulePath).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		reloadDsl("", exchange)
		return true
	}).End())
	//修改某个节点DSL
	restEndpoint.PUT(endpoint.NewRouter().From(ruleNodePath).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		nodeId := msg.Metadata.GetValue("nodeId")
		reloadDsl(nodeId, exchange)
		return true
	}).End())

	//注册路由
	_ = restEndpoint.Start()
}

//获取DSL
func getDsl(nodeId string, exchange *endpoint.Exchange) {
	var def []byte
	if nodeId == "" {
		def = ruleEngine.DSL()
	} else {
		def = ruleEngine.NodeDSL(types.EmptyRuleNodeId, types.RuleNodeId{Id: nodeId, Type: types.NODE})
		if def == nil {
			def = ruleEngine.NodeDSL(types.EmptyRuleNodeId, types.RuleNodeId{Id: nodeId, Type: types.CHAIN})
		}
	}
	exchange.Out.SetBody(def)
}

//更新DSL
func reloadDsl(nodeId string, exchange *endpoint.Exchange) {
	var err error
	if nodeId == "" {
		err = ruleEngine.ReloadSelf(exchange.In.Body())
	} else {
		err = ruleEngine.ReloadChild(types.RuleNodeId{}, types.RuleNodeId{Id: nodeId, Type: types.NODE}, exchange.In.Body())
	}
	if err != nil {
		log.Print(err)
		exchange.Out.SetStatusCode(http.StatusInternalServerError)
	} else {
		exchange.Out.SetStatusCode(http.StatusCreated)
	}
}
