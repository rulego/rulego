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

package types

import (
	"github.com/rulego/rulego/api/pool"
	"math"
	"time"
)

var OnDebug func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)

// Config 规则引擎配置
type Config struct {
	//OnDebug 节点调试信息回调函数，只有节点debugMode=true才会调用
	//ruleChainId 规则链ID
	//flowType IN/OUT,流入(IN)该组件或者流出(OUT)该组件事件类型
	//nodeId 节点ID
	//msg 当前msg
	//relationType 如果flowType=IN，则代表上一个节点和该节点的连接关系，例如(True/False);如果flowType=OUT，则代表该节点和下一个节点的连接关系，例如(True/False)
	//err 错误信息
	OnDebug func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)
	//Deprecated
	//使用types.WithEndFunc方式代替
	//OnEnd 规则链执行完成回调函数，如果有多个结束点，则执行多次
	OnEnd func(msg RuleMsg, err error)
	//ScriptMaxExecutionTime 脚本执行超时时间，默认2000毫秒
	ScriptMaxExecutionTime time.Duration
	//Pool 协程池接口
	//如果不配置，则使用 go func 方式
	//默认使用`pool.WorkerPool`。兼容ants协程池，可以使用ants协程池实现
	//例如：
	//	pool, _ := ants.NewPool(math.MaxInt32)
	//	config := rulego.NewConfig(types.WithPool(pool))
	Pool Pool
	//ComponentsRegistry 组件库
	//默认使用`rulego.Registry`
	ComponentsRegistry ComponentRegistry
	//规则链解析接口，默认使用：`rulego.JsonParser`
	Parser Parser
	//Logger 日志记录接口，默认使用：`DefaultLogger()`
	Logger Logger
	//Properties 全局属性，key-value形式
	//规则链节点配置可以通过${global.propertyKey}方式替换Properties值
	//节点初始化时候替换，只替换一次
	Properties Metadata
	//Udf 注册自定义Golang函数和原生脚本，js等脚本引擎运行时可以调用
	//不同脚本类型函数名可以重复
	Udf map[string]interface{}
	// SecretKey AES-256 32长度密钥，用于解密规则链`Secrets`配置
	SecretKey string
	//规则链DSL，endpoint模块是否可用
	EndpointEnabled bool
}

// RegisterUdf 注册自定义函数
// 不同脚本类型函数名可以重复
func (c *Config) RegisterUdf(name string, value interface{}) {
	if c.Udf == nil {
		c.Udf = make(map[string]interface{})
	}
	if c, ok := value.(Script); ok {
		//解决不同类型脚本的函数名冲突
		name = c.Type + ScriptFuncSeparator + name
	}
	c.Udf[name] = value
}

func NewConfig(opts ...Option) Config {
	// Create a new Config with default values.
	c := &Config{
		ScriptMaxExecutionTime: time.Millisecond * 2000,
		Logger:                 DefaultLogger(),
		Properties:             NewMetadata(),
		EndpointEnabled:        true,
	}

	// Apply the options to the Config.
	for _, opt := range opts {
		_ = opt(c)
	}
	return *c
}

func DefaultPool() Pool {
	wp := &pool.WorkerPool{MaxWorkersCount: math.MaxInt32}
	wp.Start()
	return wp
}
