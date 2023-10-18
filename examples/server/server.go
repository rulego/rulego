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
	"github.com/rulego/rulego/utils/fs"
	"github.com/rulego/rulego/utils/json"
	"log"
	"net/http"
	"path/filepath"

	//_ "net/http/pprof"
	"os"
	"strconv"
	"strings"
)

const (
	// base HTTP paths.
	apiVersion  = "v1"
	apiBasePath = "/api/" + apiVersion + "/"

	msgPath        = apiBasePath + "msg/:chainId/:msgType"
	rulePath       = apiBasePath + "rule/:chainId"
	componentsPath = apiBasePath + "components"

	version = "1.0.0"
)

var (
	port     int
	logfile  string
	ver      bool
	ruleFile string

	subscribeTopics  string
	mqttClientConfig = mqtt.Config{}
	mqttAvailable    bool
	logger           *log.Logger
)

func init() {
	flag.StringVar(&ruleFile, "rule_file", "", "规则链文件夹路径")
	flag.IntVar(&port, "port", 9090, "http端口")

	flag.StringVar(&logfile, "log_file", "", "日志文件路径.")
	flag.BoolVar(&ver, "version", false, "打印版本")

	//以下是mqtt 订阅配置的参数
	flag.BoolVar(&mqttAvailable, "mqtt", false, "是否开启mqtt订阅")
	flag.StringVar(&mqttClientConfig.Server, "server", "127.0.0.1:1883", "mqtt broker服务地址")
	flag.StringVar(&mqttClientConfig.Username, "username", "", "mqtt客户端用户名")
	flag.StringVar(&mqttClientConfig.Password, "password", "", "mqtt客户端密码")
	flag.DurationVar(&mqttClientConfig.MaxReconnectInterval, "max_reconnect_interval", 100000*100000*60, "max_reconnect_interval of reconnect the mqtt broker.")
	flag.BoolVar(&mqttClientConfig.CleanSession, "clean-session", false, "cleanSession.")
	flag.StringVar(&mqttClientConfig.ClientID, "client_id", "", "client_id of the client.")
	flag.StringVar(&mqttClientConfig.CAFile, "ca_file", "", "ca_file of the client.")
	flag.StringVar(&mqttClientConfig.CertFile, "cert_file", "", "cert_file of the client.")
	flag.StringVar(&mqttClientConfig.CertKeyFile, "key_file", "", "key_file of the client.")
	flag.StringVar(&subscribeTopics, "topics", "#", "订阅的主题")

}

func main() {

	flag.Parse()

	if ver {
		fmt.Printf("RuleGo Server v%s", version)
		os.Exit(0)
	}

	//初始化日志
	logger = initLogger()

	if ruleFile == "" {
		ruleFile = "./rules/"
	} else {
		//初始化规则链文件夹
		initRuleGo(logger, ruleFile)
	}

	if mqttAvailable && mqttClientConfig.Server != "" {
		//开启mqtt订阅服务接收端点
		mqttServe(logger)
	}
	strPort := ":" + strconv.Itoa(port)
	//开启http服务接收端点
	restServe(logger, strPort)

}

//初始化日志记录器
func initLogger() *log.Logger {
	if logfile == "" {
		return log.New(os.Stdout, "", log.LstdFlags)
	} else {
		f, err := os.OpenFile(logfile, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}
		return log.New(f, "", log.LstdFlags)
	}
}

//初始化规则链池
func initRuleGo(logger *log.Logger, ruleFolder string) {

	config := rulego.NewConfig(types.WithDefaultPool())
	//调试模式回调信息
	//debugMode=true 的节点会打印
	config.OnDebug = func(flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		config.Logger.Printf("flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
	}

	err := rulego.Load(ruleFolder, rulego.WithConfig(config))

	if err != nil {
		logger.Fatal("parser rule file error:", err)
	}
}

//mqtt 订阅服务
func mqttServe(logger *log.Logger) {
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

//rest服务 接收端点
func restServe(logger *log.Logger, addr string) {
	logger.Println("rest serve initialised.addr=" + addr)
	restEndpoint := &endpointRest.Rest{
		Config: endpointRest.Config{Server: addr},
	}
	//添加全局拦截器
	restEndpoint.AddInterceptors(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.Headers().Set("Content-Type", "application/json")
		exchange.Out.Headers().Set("Access-Control-Allow-Origin", "*")
		return true
	})
	//设置跨域
	restEndpoint.GlobalOPTIONS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Access-Control-Request-Method") != "" {
			// 设置 CORS 相关的响应头
			header := w.Header()
			header.Set("Access-Control-Allow-Methods", r.Header.Get("Allow"))
			header.Set("Access-Control-Allow-Headers", "*")
			header.Set("Access-Control-Allow-Origin", "*")
		}
		// 返回 204 状态码
		w.WriteHeader(http.StatusNoContent)
	}))
	//创建获取所有组件列表路由
	restEndpoint.GET(createComponentsRouter())
	//获取规则链DSL
	restEndpoint.GET(createGetDslRouter())
	//新增/修改规则链DSL
	restEndpoint.POST(createSaveDslRouter())
	//处理请求，并转发到规则引擎
	restEndpoint.POST(createPostMsgRouter())

	//注册路由
	_ = restEndpoint.Start()
}

//处理请求，并转发到规则引擎
func createPostMsgRouter() *endpoint.Router {
	return endpoint.NewRouter().From(msgPath).Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		//获取消息类型
		msg.Type = msg.Metadata.GetValue("msgType")
		//交由哪个规则链ID机芯处理
		chainId := msg.Metadata.GetValue("chainId")
		msg.Metadata.PutValue("chainId", chainId)
		return true
	}).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetStatusCode(http.StatusOK)
		return true
	}).To("chain:${chainId}").End()
}

//创建获取指定规则链路由
func createGetDslRouter() *endpoint.Router {
	return endpoint.NewRouter().From(rulePath).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue("chainId")
		nodeId := msg.Metadata.GetValue("nodeId")

		getDsl(chainId, nodeId, exchange)
		return true
	}).End()
}

//创建保存/更新指定规则链路由
func createSaveDslRouter() *endpoint.Router {
	return endpoint.NewRouter().From(rulePath).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue("chainId")
		nodeId := msg.Metadata.GetValue("nodeId")

		saveDsl(chainId, nodeId, exchange)
		return true
	}).End()
}

//创建获取组件列表路由
func createComponentsRouter() *endpoint.Router {
	//路由1
	return endpoint.NewRouter().From(componentsPath).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
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
}

//获取DSL
func getDsl(chainId, nodeId string, exchange *endpoint.Exchange) {
	var def []byte
	if chainId != "" {
		ruleEngine, ok := rulego.Get(chainId)
		if ok {
			if nodeId == "" {
				def = ruleEngine.DSL()
			} else {
				def = ruleEngine.NodeDSL(types.EmptyRuleNodeId, types.RuleNodeId{Id: nodeId, Type: types.NODE})
				if def == nil {
					def = ruleEngine.NodeDSL(types.EmptyRuleNodeId, types.RuleNodeId{Id: nodeId, Type: types.CHAIN})
				}
			}
		}

	}
	exchange.Out.SetBody(def)
}

//保存或者更新DSL
func saveDsl(chainId, nodeId string, exchange *endpoint.Exchange) {
	var err error
	if chainId != "" {
		ruleEngine, ok := rulego.Get(chainId)
		if ok {
			if nodeId == "" {
				err = ruleEngine.ReloadSelf(exchange.In.Body())
			} else {
				err = ruleEngine.ReloadChild(nodeId, exchange.In.Body())
			}
		} else {
			body := exchange.In.Body()
			//保存到文件
			dir, _ := filepath.Split(ruleFile)
			v, _ := json.Format(body)
			//保存规则链到文件
			err = fs.SaveFile(dir+chainId+".json", v)
			if err == nil {
				_, err = rulego.New(chainId, body)
			}

		}
	}

	if err != nil {
		logger.Println(err)
		exchange.Out.SetStatusCode(http.StatusInternalServerError)
	} else {
		exchange.Out.SetStatusCode(http.StatusCreated)
	}
}
