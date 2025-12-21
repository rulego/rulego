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

// FunctionDef 函数定义
// FunctionDef defines the structure for a registered function, including metadata and implementation.
type FunctionDef struct {
	// Name 函数名称
	// Name is the unique identifier for the function.
	Name string `json:"name"`
	// Label 函数显示名/标签
	// Label is the display name or label for the function.
	Label string `json:"label"`
	// Desc 函数描述
	// Desc provides a description of what the function does.
	Desc string `json:"desc"`
	// F 函数实现
	// F is the actual function implementation to be executed.
	F func(ctx types.RuleContext, msg types.RuleMsg) `json:"-"`
}

// FunctionsRegistry 线程安全的自定义处理函数注册表
// FunctionsRegistry is a thread-safe registry for custom processing functions.
//
// 函数签名 - Function signature:
//   - func(ctx types.RuleContext, msg types.RuleMsg)
//   - 函数必须调用ctx.TellSuccess/TellNext/TellFailure进行路由 - Functions must call ctx.Tell* methods for routing
type FunctionsRegistry struct {
	// functions 存储函数名到定义的映射
	// functions stores the mapping from function names to their definitions
	functions map[string]FunctionDef
	// functionNames 存储函数名列表，用于保持注册顺序
	// functionNames stores the list of function names to maintain registration order
	functionNames []string
	sync.RWMutex
}

// Register 注册函数到注册表
// Register adds a new function to the registry with the specified name.
// params[0] label
// params[1] desc
func (x *FunctionsRegistry) Register(functionName string, f func(ctx types.RuleContext, msg types.RuleMsg), params ...string) {
	def := FunctionDef{
		Name: functionName,
		F:    f,
	}
	if len(params) > 0 {
		def.Label = params[0]
	}
	if len(params) > 1 {
		def.Desc = params[1]
	}
	x.RegisterDef(def)
}

// RegisterDef 注册函数定义到注册表
// RegisterDef adds a new function definition to the registry.
func (x *FunctionsRegistry) RegisterDef(def FunctionDef) {
	x.Lock()
	defer x.Unlock()
	if x.functions == nil {
		x.functions = make(map[string]FunctionDef)
		x.functionNames = make([]string, 0)
	}
	if _, ok := x.functions[def.Name]; !ok {
		x.functionNames = append(x.functionNames, def.Name)
	}
	x.functions[def.Name] = def
}

// UnRegister 从注册表移除函数
// UnRegister removes a function from the registry by name.
func (x *FunctionsRegistry) UnRegister(functionName string) {
	x.Lock()
	defer x.Unlock()
	if x.functions != nil {
		if _, ok := x.functions[functionName]; ok {
			delete(x.functions, functionName)
			// remove from slice
			for i, name := range x.functionNames {
				if name == functionName {
					x.functionNames = append(x.functionNames[:i], x.functionNames[i+1:]...)
					break
				}
			}
		}
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
	if ok {
		return f.F, true
	}
	return nil, false
}

// List 返回所有已注册的函数定义列表
// List returns a list of all registered function definitions.
func (x *FunctionsRegistry) List() []FunctionDef {
	x.RLock()
	defer x.RUnlock()
	var defs = make([]FunctionDef, 0, len(x.functions))
	for _, name := range x.functionNames {
		if v, ok := x.functions[name]; ok {
			defs = append(defs, v)
		}
	}
	return defs
}

// Names 返回所有已注册的函数名称列表
// Names returns a list of all registered function names.
func (x *FunctionsRegistry) Names() []string {
	x.RLock()
	defer x.RUnlock()
	var keys = make([]string, len(x.functionNames))
	copy(keys, x.functionNames)
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
	FunctionName string `json:"functionName"`
	// Param 函数入参，支持变量替换。如果空，则使用消息负荷作为参数
	// Param specifies the input parameter for the function.
	// Supports dynamic resolution using placeholder variables:
	//   - ${metadata.key}: Retrieves value from message metadata
	//   - ${msg.key}: Retrieves value from message payload
	// If empty, the message payload is used as the parameter.
	Param string `json:"param"`
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
	// paramTemplate 参数模板
	// paramTemplate template for resolving function parameters
	paramTemplate el.Template
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
	// 初始化参数模板
	// Initialize parameter template
	if x.Config.Param != "" {
		x.paramTemplate, err = el.NewTemplate(x.Config.Param)
		if err != nil {
			return fmt.Errorf("failed to create param template: %w", err)
		}
	}
	return nil
}

// OnMsg 处理消息，调用指定的函数
// OnMsg processes incoming messages by invoking the specified function from the registry.
func (x *FunctionsNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	funcName := x.getFunctionName(ctx, msg)
	if f, ok := Functions.Get(funcName); ok {
		// Handle parameter
		if x.paramTemplate != nil {
			evn := base.NodeUtils.GetEvnAndMetadata(ctx, msg)
			param := x.paramTemplate.ExecuteAsString(evn)
			msg.SetData(param)
		}
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
