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
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/maps"
)

// Functions 全局函数注册表，用于注册和查找自定义处理函数
// Functions is the global registry for custom functions that can be called by FunctionsNode.
var Functions = &FunctionsRegistry{}

// init 注册FunctionsNode组件
// init registers the FunctionsNode component with the default registry.
func init() {
	Registry.Add(&FunctionsNode{})
}

// FunctionsRegistry 线程安全的自定义处理函数注册表
// FunctionsRegistry is a thread-safe registry for custom processing functions.
//
// 函数签名 - Function signature:
//   - func(ctx types.RuleContext, msg types.RuleMsg)
//   - 函数必须调用ctx.TellSuccess/TellNext/TellFailure进行路由 - Functions must call ctx.Tell* methods for routing
type FunctionsRegistry struct {
	// functions 存储函数名到实现的映射
	// functions stores the mapping from function names to their implementations
	functions map[string]func(ctx types.RuleContext, msg types.RuleMsg)
	sync.RWMutex
}

// Register 注册函数到注册表
// Register adds a new function to the registry with the specified name.
func (x *FunctionsRegistry) Register(functionName string, f func(ctx types.RuleContext, msg types.RuleMsg)) {
	x.Lock()
	defer x.Unlock()
	if x.functions == nil {
		x.functions = make(map[string]func(ctx types.RuleContext, msg types.RuleMsg))
	}
	x.functions[functionName] = f
}

// UnRegister 从注册表移除函数
// UnRegister removes a function from the registry by name.
func (x *FunctionsRegistry) UnRegister(functionName string) {
	x.Lock()
	defer x.Unlock()
	if x.functions != nil {
		delete(x.functions, functionName)
	}
}

// Get 从注册表获取函数
// Get retrieves a function from the registry by name.
func (x *FunctionsRegistry) Get(functionName string) (func(ctx types.RuleContext, msg types.RuleMsg), bool) {
	x.RLock()
	defer x.RUnlock()
	if x.functions == nil {
		return nil, false
	}
	f, ok := x.functions[functionName]
	return f, ok
}

// Names 返回所有已注册的函数名称列表
// Names returns a list of all registered function names.
func (x *FunctionsRegistry) Names() []string {
	x.RLock()
	defer x.RUnlock()
	var keys = make([]string, 0, len(x.functions))
	for k := range x.functions {
		keys = append(keys, k)
	}
	return keys
}

// FunctionsNodeConfiguration FunctionsNode配置结构
// FunctionsNodeConfiguration defines the configuration structure for the FunctionsNode component.
type FunctionsNodeConfiguration struct {
	// FunctionName 要调用的函数名称，支持变量替换
	// FunctionName specifies the name of the function to call from the registry.
	// Supports dynamic resolution using placeholder variables:
	//   - ${metadata.key}: Retrieves function name from message metadata
	//   - ${msg.key}: Retrieves function name from message payload
	FunctionName string
}

// FunctionsNode 通过函数名调用已注册自定义函数的动作组件
// FunctionsNode is an action component that invokes registered custom functions by name.
//
// 核心算法：
// Core Algorithm:
// 1. 解析函数名（静态或动态变量替换）- Resolve function name (static or dynamic variable substitution)
// 2. 从全局注册表查找函数 - Look up function in global registry
// 3. 调用函数并由函数处理路由 - Invoke function and let function handle routing
//
// 函数名解析 - Function name resolution:
//   - 静态名称：直接使用 - Static names: used directly
//   - 动态名称：支持${metadata.key}、${msg.key}和${nodeId.metadata.key}变量替换 - Dynamic names: support variable substitution including cross-node access
type FunctionsNode struct {
	// Config 节点配置
	// Config holds the node configuration including function name specification
	Config FunctionsNodeConfiguration

	// functionNameTemplate 函数名模板，用于解析动态函数名
	// functionNameTemplate template for resolving dynamic function names
	functionNameTemplate el.Template
}

// Type 返回组件类型
// Type returns the component type identifier.
func (x *FunctionsNode) Type() string {
	return "functions"
}

// New 创建新实例
// New creates a new instance.
func (x *FunctionsNode) New() types.Node {
	return &FunctionsNode{Config: FunctionsNodeConfiguration{
		FunctionName: "test",
	}}
}

// Init 初始化组件
// Init initializes the component.
func (x *FunctionsNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}

	// 初始化函数名模板
	// Initialize function name template
	x.functionNameTemplate, err = el.NewTemplate(x.Config.FunctionName)
	if err != nil {
		return fmt.Errorf("failed to create function name template: %w", err)
	}
	return nil
}

// OnMsg 处理消息，调用指定的函数
// OnMsg processes incoming messages by invoking the specified function from the registry.
func (x *FunctionsNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	funcName := x.getFunctionName(ctx, msg)
	if f, ok := Functions.Get(funcName); ok {
		// 调用函数
		f(ctx, msg)
	} else {
		ctx.TellFailure(msg, fmt.Errorf("can not found the function=%s", funcName))
	}
}

// Destroy 清理资源
// Destroy cleans up resources.
func (x *FunctionsNode) Destroy() {
	// 无资源需要清理
	// No resources to clean up
}

// getFunctionName 解析函数名称，处理静态和动态情况，支持跨节点取值
// getFunctionName resolves the function name, handling both static and dynamic cases with cross-node access support.
func (x *FunctionsNode) getFunctionName(ctx types.RuleContext, msg types.RuleMsg) string {
	if x.functionNameTemplate != nil {
		// Execute template
		return x.functionNameTemplate.ExecuteAsString(base.NodeUtils.GetEvnAndMetadata(ctx, msg))
	}
	return x.Config.FunctionName
}
