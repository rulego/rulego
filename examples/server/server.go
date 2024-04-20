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
	"examples/server/event"
	"flag"
	"fmt"
	"github.com/rulego/rulego"
	luaEngine "github.com/rulego/rulego-components/pkg/lua_engine"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/mqtt"
	"github.com/rulego/rulego/endpoint"
	endpointMqtt "github.com/rulego/rulego/endpoint/mqtt"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/utils/fs"
	"github.com/rulego/rulego/utils/json"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// base HTTP paths.
	apiVersion  = "v1"
	apiBasePath = "/api/" + apiVersion + "/"

	//获取组件配置列表 POST /api/v1/msg/:chainId/:msgType
	msgPath = apiBasePath + "msg/:chainId/:msgType"
	//获取规则链描述文件 GET /api/v1/rule/:chainId
	//保存或者修改规则链描述文件 POST /api/v1/rule/:chainId
	rulePath = apiBasePath + "rule/:chainId"
	//获取组件配置列表 /api/v1/components
	componentsPath = apiBasePath + "components"
	//获取规则链节点调试数据列表 /api/v1/event/debug?chainId=xx%nodeId=yy
	eventPath = apiBasePath + "event/debug"
	version   = "1.0.0"
)

var (
	port    int
	logfile string
	ver     bool
	//规则链存储路径，弃用，请使用rules
	ruleFile string
	//规则链存储路径
	rules string
	//预加载js文件路径
	js string
	//plugins
	plugins          string
	debugToLog       bool
	subscribeTopics  string
	mqttClientConfig = mqtt.Config{}
	mqttAvailable    bool
	//mqtt 数据交给哪个规则链处理，默认是default
	mqtt2ChainId string
	logger       *log.Logger
	//基于内存的节点调试数据管理器
	//如果需要查询历史数据，请把调试日志数据存放数据库等可以持久化载体
	ruleChainDebugData *event.RuleChainDebugData
	//ruleGo 配置
	config types.Config
)

func init() {
	//弃用
	flag.StringVar(&ruleFile, "rule_file", "", "规则链文件存储路径，弃用。请使用-rules")
	flag.StringVar(&rules, "rules", "./rules/", "规则链文件存储路径")
	flag.StringVar(&js, "js", "./js/", "预加载js文件路径")
	flag.StringVar(&plugins, "plugins", "./plugins/", "组件插件路径")

	flag.IntVar(&port, "port", 9090, "http端口")
	flag.BoolVar(&debugToLog, "debug", true, "是否把节点调试日志打印到日志文件")

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
	flag.StringVar(&mqtt2ChainId, "chain_id", "chain:default", "订阅数据处理规则链Id")

}

func main() {

	flag.Parse()

	if ver {
		fmt.Printf("RuleGo Server v%s", version)
		os.Exit(0)
	}
	//基于内存的节点调试数据管理器
	ruleChainDebugData = event.NewRuleChainDebugData(40)
	//初始化日志
	logger = initLogger()

	if ruleFile != "" {
		rules = ruleFile
	}

	log.Println("rules:", rules)
	log.Println("js:", js)
	log.Println("plugins:", plugins)
	log.Println("mqtt2ChainId:", mqtt2ChainId)
	//创建文件夹
	_ = fs.CreateDirs(rules)
	_ = fs.CreateDirs(js)
	_ = fs.CreateDirs(plugins)
	//初始化规则链文件夹
	initRuleGo(logger, rules)

	if mqttAvailable && mqttClientConfig.Server != "" {
		//开启mqtt订阅服务接收端点
		mqttServe(logger)
	}
	strPort := ":" + strconv.Itoa(port)
	//开启http服务接收端点
	restServe(logger, strPort)

}

// 初始化日志记录器
func initLogger() *log.Logger {
	if logfile == "" {
		return log.New(os.Stdout, "", log.LstdFlags)
	} else {
		f, err := os.OpenFile(logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			panic(err)
		}
		return log.New(f, "", log.LstdFlags)
	}
}

// 初始化规则链池
func initRuleGo(logger *log.Logger, ruleFolder string) {

	config = rulego.NewConfig(types.WithDefaultPool())
	//加载lua第三方库
	config.Properties.PutValue(luaEngine.LoadLuaLibs, "true")
	//初始化RuleGo日志
	config.Logger = logger
	//调试模式回调信息
	//debugMode=true 的节点才会记录调试日志
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		var errStr = ""
		if err != nil {
			errStr = err.Error()
		}
		//把日志记录到内存管理器，用于界面显示
		ruleChainDebugData.Add(chainId, nodeId, event.DebugData{
			Ts: time.Now().UnixMilli(),
			//节点ID
			NodeId: nodeId,
			//流向OUT/IN
			FlowType: flowType,
			//消息
			Msg: msg,
			//关系
			RelationType: relationType,
			//Err 错误
			Err: errStr,
		})
		//记录到日志文件
		if debugToLog {
			config.Logger.Printf("flowType=%s,nodeId=%s,msgType=%s,data=%s,metaData=%s,relationType=%s,err=%s", flowType, nodeId, msg.Type, msg.Data, msg.Metadata, relationType, err)
		}
	}

	//加载js
	err := loadJs(js)
	if err != nil {
		logger.Fatal("parser js file error:", err)
	}
	//加载组件插件
	err = loadPlugins(plugins)
	if err != nil {
		logger.Fatal("parser plugin file error:", err)
	}
	//加载规则链
	err = loadRules(rules)
	if err != nil {
		logger.Fatal("parser rule file error:", err)
	}
}

// 加载js
func loadJs(folderPath string) error {
	//创建文件夹
	_ = fs.CreateDirs(folderPath)
	//遍历所有文件
	paths, err := fs.GetFilePaths(folderPath + "/*.js")
	if err != nil {
		return err
	}
	for _, file := range paths {
		if b := fs.LoadFile(file); b != nil {
			config.RegisterUdf(path.Base(file), types.Script{
				Type:    types.Js,
				Content: string(b),
			})
		}
	}
	return nil
}

// 加载组件插件
func loadPlugins(folderPath string) error {
	//创建文件夹
	_ = fs.CreateDirs(folderPath)
	//遍历所有文件
	paths, err := fs.GetFilePaths(folderPath + "/*.so")
	if err != nil {
		return err
	}
	for _, file := range paths {
		if err := rulego.Registry.RegisterPlugin(path.Base(file), file); err != nil {
			logger.Printf("load plugin=%s error=%s", file, err.Error())
		}
	}
	return nil

}

// 加载规则链
func loadRules(folderPath string) error {
	//创建文件夹
	_ = fs.CreateDirs(folderPath)
	//遍历所有文件
	err := rulego.Load(folderPath, rulego.WithConfig(config))
	if err != nil {
		logger.Fatal("parser rule file error:", err)
	}
	return err
}

// mqtt 订阅服务
func mqttServe(logger *log.Logger) {
	//mqtt 订阅服务 接收端点
	mqttEndpoint, err := endpoint.New(endpointMqtt.Type, config, mqttClientConfig)
	if err != nil {
		logger.Fatal(err)
	}
	for _, topic := range strings.Split(subscribeTopics, ",") {
		router := endpoint.NewRouter().From(topic).To(mqtt2ChainId).End()
		_, _ = mqttEndpoint.AddRouter(router)
	}
	if err := mqttEndpoint.Start(); err != nil {
		logger.Fatal(err)
	}
}

// rest服务 接收端点
func restServe(logger *log.Logger, addr string) {
	logger.Println("rest serve initialised.addr=" + addr)
	restEndpoint := &rest.Endpoint{
		Config: rest.Config{Server: addr},
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
			header.Set("Access-Control-Allow-Methods", "*")
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
	//获取节点调试数据
	restEndpoint.GET(createGetDebugDataRouter())

	//执行规则链,并得到规则链处理结果
	restEndpoint.POST(executeRuleRouter(apiBasePath + "rule/:chainId/execute/:msgType"))
	//处理数据上报请求，并转发到规则引擎，不等待规则引擎处理结果
	restEndpoint.POST(createPostMsgRouter(apiBasePath + "rule/:chainId/notify/:msgType"))
	//处理数据上报请求，并转发到规则引擎
	restEndpoint.POST(createPostMsgRouter(msgPath))
	//注册路由
	if err := restEndpoint.Start(); err != nil {
		logger.Fatal(err)
	}
}

// 处理请求，并转发到规则引擎
func createPostMsgRouter(url string) *endpoint.Router {
	return endpoint.NewRouter().From(url).Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		//获取消息类型
		msg.Type = msg.Metadata.GetValue("msgType")
		return true
	}).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetStatusCode(http.StatusOK)
		return true
	}).To("chain:${chainId}").End()
}

// 执行规则链,并得到规则链处理结果
func executeRuleRouter(url string) *endpoint.Router {
	return endpoint.NewRouter().From(url).Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		//获取消息类型
		msg.Type = msg.Metadata.GetValue("msgType")
		return true
	}).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetStatusCode(http.StatusOK)
		return true
	}).To("chain:${chainId}").Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
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
	}).Wait().End()
}

// 创建获取指定规则链路由
func createGetDslRouter() *endpoint.Router {
	return endpoint.NewRouter().From(rulePath).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue("chainId")
		nodeId := msg.Metadata.GetValue("nodeId")

		getDsl(chainId, nodeId, exchange)
		return true
	}).End()
}

// 创建保存/更新指定规则链路由
func createSaveDslRouter() *endpoint.Router {
	return endpoint.NewRouter().From(rulePath).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue("chainId")
		nodeId := msg.Metadata.GetValue("nodeId")

		saveDsl(chainId, nodeId, exchange)
		return true
	}).End()
}

// 创建获取组件列表路由
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

// 创建获取节点调试数据路由
func createGetDebugDataRouter() *endpoint.Router {
	//路由1
	return endpoint.NewRouter().From(eventPath).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue("chainId")
		nodeId := msg.Metadata.GetValue("nodeId")
		page := ruleChainDebugData.GetToPage(chainId, nodeId)
		if v, err := json.Marshal(page); err != nil {
			exchange.Out.SetStatusCode(500)
			exchange.Out.SetBody([]byte(err.Error()))
		} else {
			exchange.Out.SetBody(v)
		}
		return true
	}).End()
}

// 获取DSL
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
			exchange.Out.SetBody(def)
		} else {
			exchange.Out.SetStatusCode(404)
			//exchange.Out.SetBody([]byte("not found"))
		}

	}

}

// 保存或者更新DSL
func saveDsl(chainId, nodeId string, exchange *endpoint.Exchange) {
	var err error
	if chainId != "" {
		body := exchange.In.Body()
		ruleEngine, ok := rulego.Get(chainId)
		if ok {
			if nodeId == "" {
				err = ruleEngine.ReloadSelf(exchange.In.Body())
			} else {
				err = ruleEngine.ReloadChild(nodeId, exchange.In.Body())
			}
		} else {
			_, err = rulego.New(chainId, body, rulego.WithConfig(config))
		}
		//保存到文件
		dir := filepath.Dir(rules)
		v, _ := json.Format(body)
		//保存规则链到文件
		err = fs.SaveFile(filepath.Join(dir, chainId+".json"), v)
	}

	if err != nil {
		logger.Println(err)
		exchange.Out.SetStatusCode(http.StatusInternalServerError)
	} else {
		exchange.Out.SetStatusCode(http.StatusOK)
	}
}
