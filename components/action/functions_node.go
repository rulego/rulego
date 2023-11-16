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

package action

//规则链节点配置示例：
//{
//        "id": "s2",
//        "type": "functions",
//        "name": "函数调用",
//        "debugMode": false,
//        "configuration": {
//          "functionName": "test"
//        }
//  }
import (
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"sync"
)

// Functions 自定义函数注册器
var Functions = &FunctionsRegistry{}

// 注册节点
func init() {
	Registry.Add(&FunctionsNode{})
}

// FunctionsRegistry 函数注册器
type FunctionsRegistry struct {
	//函数列表
	//ctx 上下文
	//msg 上一节点传入的msg
	//函数处理成功，必须使用以下方法通知规则引擎已成功处理：
	//ctx.TellSuccess(msg RuleMsg) //通知规则引擎处理当前消息处理成功，并把消息通过`Success`关系发送到下一个节点
	//ctx.TellNext(msg RuleMsg, relationTypes ...string)//使用指定的relationTypes，发送消息到下一个节点
	//如果消息处理失败，函数实现必须调用tellFailure方法：
	//ctx.TellFailure(msg RuleMsg, err error)//通知规则引擎处理当前消息处理失败，并把消息通过`Failure`关系发送到下一个节点
	functions map[string]func(ctx types.RuleContext, msg types.RuleMsg)
	sync.RWMutex
}

// Register 注册函数
func (x *FunctionsRegistry) Register(functionName string, f func(ctx types.RuleContext, msg types.RuleMsg)) {
	x.Lock()
	defer x.Unlock()
	if x.functions == nil {
		x.functions = make(map[string]func(ctx types.RuleContext, msg types.RuleMsg))
	}
	x.functions[functionName] = f
}

// UnRegister 删除函数
func (x *FunctionsRegistry) UnRegister(functionName string) {
	x.Lock()
	defer x.Unlock()
	if x.functions != nil {
		delete(x.functions, functionName)
	}
}

// Get 获取函数
func (x *FunctionsRegistry) Get(functionName string) (func(ctx types.RuleContext, msg types.RuleMsg), bool) {
	x.RLock()
	defer x.RUnlock()
	if x.functions == nil {
		return nil, false
	}
	f, ok := x.functions[functionName]
	return f, ok
}

// FunctionsNodeConfiguration 节点配置
type FunctionsNodeConfiguration struct {
	//FunctionName 调用的函数名称
	FunctionName string
}

// FunctionsNode 通过方法名调用注册在Functions的处理函数
// 如果没找到函数，则TellFailure
type FunctionsNode struct {
	//节点配置
	Config FunctionsNodeConfiguration
}

// Type 组件类型
func (x *FunctionsNode) Type() string {
	return "functions"
}

func (x *FunctionsNode) New() types.Node {
	return &FunctionsNode{Config: FunctionsNodeConfiguration{
		FunctionName: "test",
	}}
}

// Init 初始化
func (x *FunctionsNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return maps.Map2Struct(configuration, &x.Config)
}

// OnMsg 处理消息
func (x *FunctionsNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	if f, ok := Functions.Get(x.Config.FunctionName); ok {
		//调用函数
		f(ctx, msg)
	} else {
		ctx.TellFailure(msg, fmt.Errorf("can not found the function=%s", x.Config.FunctionName))
	}
}

// Destroy 销毁
func (x *FunctionsNode) Destroy() {
}
