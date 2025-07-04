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

// Package impl provides the core implementation of the endpoint module.
// It includes structures and methods for handling endpoints, routers,
// and message processing in the RuleGo framework.
//
// Package impl 提供端点模块的核心实现。
// 它包括在 RuleGo 框架中处理端点、路由器和消息处理的结构和方法。
//
// Core Components / 核心组件：
//
// • From: Represents the source of incoming data with processing capabilities  表示传入数据的源头，具有处理能力
// • To: Represents the destination for processed data with target execution  表示处理后数据的目标，具有目标执行能力
// • Router: Manages the routing of messages between From and To  管理 From 和 To 之间的消息路由
// • BaseEndpoint: Base implementation for endpoint functionality  端点功能的基础实现
// • Executors: Different execution strategies for processing messages  处理消息的不同执行策略
//
// Message Flow / 消息流：
//
// The implementation follows a pipeline pattern where messages flow through:
// 实现遵循管道模式，消息流经：
//
// 1. From: Input processing and transformation  输入处理和转换
// 2. To: Target execution (rule chains or components)  目标执行（规则链或组件）
// 3. Process: Post-processing and response handling  后处理和响应处理
//
// Configuration / 配置：
//
// The implementation supports flexible configuration through:
// 实现通过以下方式支持灵活配置：
//
// • Dynamic path variables using ${var} syntax  使用 ${var} 语法的动态路径变量
// • Multiple processing interceptors  多个处理拦截器
// • Different executor types (chain, component)  不同的执行器类型（链、组件）
// • Synchronous and asynchronous execution modes  同步和异步执行模式
package impl

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/utils/str"
)

const (
	pathKey = "_path"
	//分割标志如：{chainId}:{nodeId}  Split flag like: {chainId}:{nodeId}
	pathSplitFlag = ":"
)

var _ endpoint.From = (*From)(nil)

// From represents the input source of a router with processing capabilities.
// It handles incoming data transformation, processing, and routing to target destinations.
//
// From 表示具有处理能力的路由器输入源。
// 它处理传入的数据转换、处理和路由到目标目的地。
//
// Architecture / 架构：
// • Configuration Management: Stores source-specific configuration  配置管理：存储源特定配置
// • Process Pipeline: Chain of processing functions for data transformation  处理管道：数据转换的处理函数链
// • Router Integration: Back-reference to the parent router  路由器集成：对父路由器的反向引用
// • Target Binding: Connection to the destination (To) endpoint  目标绑定：连接到目标（To）端点
//
// Processing Flow / 处理流程：
// 1. Receive input data  接收输入数据
// 2. Execute processing pipeline  执行处理管道
// 3. Route to target destination  路由到目标目的地
// 4. Handle responses if needed  如果需要处理响应
type From struct {
	//Config 配置  Configuration for the From endpoint  From 端点的配置
	Config types.Configuration
	//Router router指针  Pointer to the parent router  父路由器的指针
	Router *Router
	//来源路径  Source path pattern for input matching  用于输入匹配的源路径模式
	From string
	//消息处理拦截器  Message processing interceptors  消息处理拦截器
	processList []endpoint.Process
	//流转目标路径，例如"chain:{chainId}"，则是交给规则引擎处理数据  Target flow path, e.g., "chain:{chainId}" for rule engine processing  流转目标路径
	to *To
}

// ToString returns the string representation of the From path.
// This is used for identification and logging purposes.
//
// ToString 返回 From 路径的字符串表示。
// 用于标识和日志记录目的。
func (f *From) ToString() string {
	return f.From
}

// Transform adds a transformation processor to the From processing pipeline.
// Transformations are applied to incoming data before routing to the destination.
//
// Transform 向 From 处理管道添加转换处理器。
// 转换在路由到目标之前应用于传入数据。
//
// Parameters / 参数：
// • transform: The transformation function to apply  要应用的转换函数
//
// Returns / 返回值：
// • endpoint.From: Returns self for method chaining  返回自身以支持方法链
func (f *From) Transform(transform endpoint.Process) endpoint.From {
	f.processList = append(f.processList, transform)
	return f
}

// Process adds a processing function to the From processing pipeline.
// Processing functions can modify, validate, or filter incoming data.
//
// Process 向 From 处理管道添加处理函数。
// 处理函数可以修改、验证或过滤传入数据。
//
// Parameters / 参数：
// • process: The processing function to add  要添加的处理函数
//
// Returns / 返回值：
// • endpoint.From: Returns self for method chaining  返回自身以支持方法链
func (f *From) Process(process endpoint.Process) endpoint.From {
	f.processList = append(f.processList, process)
	return f
}

// GetProcessList returns the list of processing functions in the pipeline.
// Used internally for execution and inspection purposes.
//
// GetProcessList 返回管道中的处理函数列表。
// 用于内部执行和检查目的。
func (f *From) GetProcessList() []endpoint.Process {
	return f.processList
}

// ExecuteProcess executes all processing functions in the pipeline sequentially.
// If any processing function returns false, the pipeline stops and returns false.
//
// ExecuteProcess 按顺序执行管道中的所有处理函数。
// 如果任何处理函数返回 false，管道停止并返回 false。
//
// Parameters / 参数：
// • router: The router context  路由器上下文
// • exchange: The message exchange containing input/output data  包含输入/输出数据的消息交换
//
// Returns / 返回值：
// • bool: true if all processing succeeded, false otherwise  如果所有处理成功返回 true，否则返回 false
func (f *From) ExecuteProcess(router endpoint.Router, exchange *endpoint.Exchange) bool {
	result := true
	for _, process := range f.GetProcessList() {
		if !process(router, exchange) {
			result = false
			break
		}
	}
	return result
}

// To creates and configures the destination endpoint for message routing.
// The destination format follows the pattern: {executorType}:{path}
//
// To 创建并配置消息路由的目标端点。
// 目标格式遵循模式：{executorType}:{path}
//
// Parameters / 参数：
// • to: Target path string in format "executorType:path"  格式为 "executorType:path" 的目标路径字符串
//   - "chain:{chainId}": Route to a rule chain  路由到规则链
//   - "chain:{chainId}:{nodeId}": Route to specific node in rule chain  路由到规则链中的特定节点
//   - "component:{nodeType}": Route to a registered component  路由到注册的组件
//
// • configs: Optional configuration parameters for the destination  目标的可选配置参数
//
// Returns / 返回值：
// • endpoint.To: The configured destination endpoint  配置的目标端点
//
// Executor Types / 执行器类型：
// • chain: Rule chain executor for processing with rule engines  规则链执行器，用于规则引擎处理
// • component: Component executor for individual node processing  组件执行器，用于单个节点处理
//
// Variable Support / 变量支持：
// The path can contain variables like "${userId}" that will be resolved at runtime
// 路径可以包含如 "${userId}" 的变量，将在运行时解析
func (f *From) To(to string, configs ...types.Configuration) endpoint.To {
	var toConfig = make(types.Configuration)
	for _, item := range configs {
		for k, v := range item {
			toConfig[k] = v
		}
	}
	f.to = &To{Router: f.Router, To: to, Config: toConfig}

	//路径中是否有变量，如：chain:${userId}  Check if path contains variables like: chain:${userId}
	if strings.Contains(to, "${") && strings.Contains(to, "}") {
		f.to.HasVars = true
	}

	//获取To执行器类型  Get To executor type
	executorType := strings.Split(to, pathSplitFlag)[0]

	//获取To执行器  Get To executor
	if executor, ok := DefaultExecutorFactory.New(executorType); ok {
		if f.to.HasVars && !executor.IsPathSupportVar() {
			f.Router.err = fmt.Errorf("executor=%s, path not support variables", executorType)
			return f.to
		}
		f.to.ToPath = strings.TrimSpace(to[len(executorType)+1:])
		toConfig[pathKey] = f.to.ToPath
		//初始化组件  Initialize component
		err := executor.Init(f.Router.Config, toConfig)
		if err != nil {
			f.Router.err = err
			return f.to
		}
		f.to.executor = executor
	} else {
		f.to.executor = &ChainExecutor{}
		f.to.ToPath = to
	}
	return f.to
}

// GetTo returns the configured destination endpoint.
// Returns nil if no destination has been configured.
//
// GetTo 返回配置的目标端点。
// 如果没有配置目标则返回 nil。
func (f *From) GetTo() endpoint.To {
	if f.to == nil {
		return nil
	}
	return f.to
}

// ToComponent creates a destination that routes directly to a specific component node.
// This bypasses the executor factory and uses the component directly.
//
// ToComponent 创建直接路由到特定组件节点的目标。
// 这绕过执行器工厂并直接使用组件。
//
// Parameters / 参数：
// • node: The component node to route to  要路由到的组件节点
//
// Returns / 返回值：
// • endpoint.To: The configured component destination  配置的组件目标
func (f *From) ToComponent(node types.Node) endpoint.To {
	component := &ComponentExecutor{component: node, config: f.Router.Config}
	f.to = &To{Router: f.Router, To: node.Type(), ToPath: node.Type()}
	f.to.executor = component
	return f.to
}

// End completes the From configuration and returns the parent router.
// Used for method chaining to continue router configuration.
//
// End 完成 From 配置并返回父路由器。
// 用于方法链以继续路由器配置。
func (f *From) End() endpoint.Router {
	return f.Router
}

// To represents the destination endpoint for message processing.
// It handles the execution of target logic and post-processing of results.
//
// To 表示消息处理的目标端点。
// 它处理目标逻辑的执行和结果的后处理。
//
// Architecture / 架构：
// • Variable Resolution: Supports dynamic path variables  变量解析：支持动态路径变量
// • Executor Integration: Pluggable execution strategies  执行器集成：可插拔的执行策略
// • Process Pipeline: Post-processing functions for results  处理管道：结果的后处理函数
// • Synchronous Support: Optional blocking execution for responses  同步支持：响应的可选阻塞执行
//
// Execution Modes / 执行模式：
// • Asynchronous: Fire-and-forget message processing  异步：发送即忘的消息处理
// • Synchronous: Wait for execution completion and results  同步：等待执行完成和结果
type To struct {
	//toPath是否有占位符变量  Whether toPath has placeholder variables  toPath 是否有占位符变量
	HasVars bool
	//Config to组件配置  Configuration for To component  To 组件配置
	Config types.Configuration
	//Router router指针  Pointer to parent router  父路由器指针
	Router *Router
	//流转目标路径，例如"chain:{chainId}"，则是交给规则引擎处理数据  Target flow path, e.g., "chain:{chainId}" for rule engine processing  流转目标路径
	To string
	//去掉to执行器标记的路径  Path with executor type prefix removed  去掉 to 执行器标记的路径
	ToPath string
	//消息处理拦截器  Message processing interceptors  消息处理拦截器
	processList []endpoint.Process
	//目标处理器，默认是规则链处理  Target processor, defaults to rule chain processing  目标处理器
	executor endpoint.Executor
	//等待规则链/组件执行结束，并恢复到父进程，同步得到规则链结果。  Wait for rule chain/component execution completion and return to parent process, synchronously get rule chain results  等待执行结束
	//用于需要等待规则链执行结果，并且要保留父进程的场景，否则不需要设置该字段。例如：http的响应。  Used for scenarios requiring rule chain execution results while preserving parent process, e.g., HTTP responses  用于需要等待结果的场景
	wait bool
	//规则上下文配置，如果配置了	`types.WithOnEnd` 需要接管`ChainExecutor`结果响应逻辑  Rule context configuration, requires handling ChainExecutor result response logic if `types.WithOnEnd` is configured  规则上下文配置
	opts []types.RuleContextOption
}

// ToStringByDict resolves variables in the path using the provided dictionary and returns the final string.
// This enables dynamic routing based on message metadata or other runtime values.
//
// ToStringByDict 使用提供的字典解析路径中的变量并返回最终字符串。
// 这使得基于消息元数据或其他运行时值的动态路由成为可能。
//
// Parameters / 参数：
// • dict: Variable dictionary for path resolution  用于路径解析的变量字典
//
// Returns / 返回值：
// • string: Resolved path with variables substituted  替换变量后的解析路径
func (t *To) ToStringByDict(dict map[string]string) string {
	if t.HasVars {
		return str.SprintfDict(t.ToPath, dict)
	}
	return t.ToPath
}

// ToString returns the string representation of the To path.
// Used for identification and logging purposes.
//
// ToString 返回 To 路径的字符串表示。
// 用于标识和日志记录目的。
func (t *To) ToString() string {
	return t.ToPath
}

// Execute executes the To endpoint logic using the configured executor.
// This is the main entry point for processing messages at the destination.
//
// Execute 使用配置的执行器执行 To 端点逻辑。
// 这是在目标处理消息的主要入口点。
//
// Parameters / 参数：
// • ctx: Execution context  执行上下文
// • exchange: Message exchange containing input/output data  包含输入/输出数据的消息交换
func (t *To) Execute(ctx context.Context, exchange *endpoint.Exchange) {
	if t.executor != nil {
		t.executor.Execute(ctx, t.Router, exchange)
	}
}

// Transform adds a transformation processor to the To post-processing pipeline.
// These transformations are applied to results after To logic execution.
// If the rule chain has multiple end points, this will be executed multiple times.
//
// Transform 向 To 后处理管道添加转换处理器。
// 这些转换在 To 逻辑执行后应用于结果。
// 如果规则链有多个结束点，则会执行多次。
func (t *To) Transform(transform endpoint.Process) endpoint.To {
	t.processList = append(t.processList, transform)
	return t
}

// Process adds a processing function to the To post-processing pipeline.
// These processors handle results after To logic execution.
// If the rule chain has multiple end points, this will be executed multiple times.
//
// Process 向 To 后处理管道添加处理函数。
// 这些处理器在 To 逻辑执行后处理结果。
// 如果规则链有多个结束点，则会执行多次。
func (t *To) Process(process endpoint.Process) endpoint.To {
	t.processList = append(t.processList, process)
	return t
}

// Wait enables synchronous execution mode for the To endpoint.
// When enabled, the execution waits for rule chain/component completion and returns to the parent process.
// This is used for scenarios requiring rule chain execution results while preserving the parent process.
// Example use case: HTTP response handling.
//
// Wait 为 To 端点启用同步执行模式。
// 启用时，执行等待规则链/组件完成并返回到父进程。
// 用于需要规则链执行结果并且要保留父进程的场景。
// 使用示例：HTTP 响应处理。
func (t *To) Wait() endpoint.To {
	t.wait = true
	return t
}

// SetWait configures the synchronous execution mode.
//
// SetWait 配置同步执行模式。
func (t *To) SetWait(wait bool) endpoint.To {
	t.wait = wait
	return t
}

// IsWait returns whether synchronous execution mode is enabled.
//
// IsWait 返回是否启用了同步执行模式。
func (t *To) IsWait() bool {
	return t.wait
}

// SetOpts configures rule context options for the To execution.
// These options are passed to the rule engine when executing rule chains.
//
// SetOpts 为 To 执行配置规则上下文选项。
// 这些选项在执行规则链时传递给规则引擎。
func (t *To) SetOpts(opts ...types.RuleContextOption) endpoint.To {
	t.opts = opts
	return t
}

// GetOpts returns the configured rule context options.
//
// GetOpts 返回配置的规则上下文选项。
func (t *To) GetOpts() []types.RuleContextOption {
	return t.opts
}

// GetProcessList returns the list of post-processing functions.
// Used internally for execution and inspection purposes.
//
// GetProcessList 返回后处理函数列表。
// 用于内部执行和检查目的。
func (t *To) GetProcessList() []endpoint.Process {
	return t.processList
}

// End completes the To configuration and returns the parent router.
// Used for method chaining to continue router configuration.
//
// End 完成 To 配置并返回父路由器。
// 用于方法链以继续路由器配置。
func (t *To) End() endpoint.Router {
	return t.Router
}

// Router provides message routing abstraction for different input sources.
// It manages the flow of messages from input endpoints (From), through transformation/processing,
// to target destinations (To) such as rule chains or components.
//
// Router 为不同输入源提供消息路由抽象。
// 它管理消息从输入端点（From）通过转换/处理到目标目的地（To）（如规则链或组件）的流程。
//
// Architecture / 架构：
// • Fluent API: Chain method calls for intuitive configuration  流式 API：链式方法调用便于直观配置
// • Context Management: Handles execution context and lifecycle  上下文管理：处理执行上下文和生命周期
// • Pool Integration: Manages rule engine pool access  池集成：管理规则引擎池访问
// • State Management: Tracks router state and configuration  状态管理：跟踪路由器状态和配置
//
// Usage Patterns / 使用模式：
//
// HTTP Endpoint Examples / HTTP 端点示例：
//
//	router.From("/api/v1/msg/").Transform().To("chain:xx")
//	router.From("/api/v1/msg/").Transform().Process().To("chain:xx")
//	router.From("/api/v1/msg/").Transform().Process().To("component:nodeType")
//	router.From("/api/v1/msg/").Transform().Process()
//
// MQTT Endpoint Examples / MQTT 端点示例：
//
//	router.From("#").Transform().Process().To("chain:xx")
//	router.From("device/+/msg").Transform().Process().To("chain:xx")
//
// Configuration / 配置：
// • Dynamic Rule Engine Pool: Support for runtime pool selection  动态规则引擎池：支持运行时池选择
// • Context Customization: Custom context creation for each exchange  上下文自定义：为每个交换创建自定义上下文
// • Error Handling: Centralized error tracking and reporting  错误处理：集中错误跟踪和报告
// • State Control: Enable/disable routing dynamically  状态控制：动态启用/禁用路由
type Router struct {
	//创建上下文回调函数  Context creation callback function  创建上下文回调函数
	ContextFunc func(ctx context.Context, exchange *endpoint.Exchange) context.Context
	//Config ruleEngine Config  Rule engine configuration  规则引擎配置
	Config types.Config
	//路由器唯一标识符  Router unique identifier  路由器唯一标识符
	id string
	//输入端点配置  Input endpoint configuration  输入端点配置
	from *From
	//规则链池，默认使用engine.DefaultPool  Rule chain pool, defaults to engine.DefaultPool  规则链池
	RuleGo types.RuleEnginePool
	//动态获取规则链池函数  Function for dynamic rule chain pool retrieval  动态获取规则链池函数
	ruleGoFunc func(exchange *endpoint.Exchange) types.RuleEnginePool
	//是否不可用 1:不可用;0:可以  Disable state: 1=disabled, 0=enabled  禁用状态
	disable uint32
	//路由定义，如果没设置会返回nil  Router definition, returns nil if not set  路由定义
	def *types.RouterDsl
	//配置参数  Configuration parameters  配置参数
	params []interface{}
	//记录初始化的错误  Records initialization errors  记录初始化错误
	err error
}

// RouterOption is a type alias for router configuration options.
// It enables functional options pattern for router customization.
//
// RouterOption 是路由器配置选项的类型别名。
// 它启用函数选项模式用于路由器自定义。
type RouterOption = endpoint.RouterOption

// NewRouter creates a new router instance with optional configuration.
// The router is initialized with default settings and can be customized using options.
//
// NewRouter 使用可选配置创建新的路由器实例。
// 路由器使用默认设置初始化，可以使用选项进行自定义。
//
// Parameters / 参数：
// • opts: Optional configuration functions  可选的配置函数
//
// Returns / 返回值：
// • endpoint.Router: Configured router instance  配置的路由器实例
//
// Default Configuration / 默认配置：
// • RuleGo: Uses engine.DefaultPool for rule engine access  使用 engine.DefaultPool 进行规则引擎访问
// • Config: Uses engine.NewConfig() for basic configuration  使用 engine.NewConfig() 进行基本配置
//
// Usage Example / 使用示例：
//
//	router := NewRouter(
//	    RouterOptions.WithConfig(customConfig),
//	    RouterOptions.WithRuleEnginePool(customPool),
//	)
func NewRouter(opts ...RouterOption) endpoint.Router {
	router := &Router{RuleGo: engine.DefaultPool, Config: engine.NewConfig()}
	// 设置选项值  Apply option values  设置选项值
	for _, opt := range opts {
		_ = opt(router)
	}
	return router
}

func (r *Router) SetConfig(config types.Config) {
	r.Config = config
}

func (r *Router) SetRuleEnginePool(pool types.RuleEnginePool) {
	r.RuleGo = pool
}

func (r *Router) SetRuleEnginePoolFunc(f func(exchange *endpoint.Exchange) types.RuleEnginePool) {
	r.ruleGoFunc = f
}

func (r *Router) SetContextFunc(f func(ctx context.Context, exchange *endpoint.Exchange) context.Context) {
	r.ContextFunc = f
}

func (r *Router) GetContextFunc() func(ctx context.Context, exchange *endpoint.Exchange) context.Context {
	return r.ContextFunc
}

func (r *Router) SetDefinition(def *types.RouterDsl) {
	r.def = def
}

// Definition 返回路由定义，如果没设置会返回nil
func (r *Router) Definition() *types.RouterDsl {
	return r.def
}

func (r *Router) SetId(id string) endpoint.Router {
	r.id = id
	return r
}

func (r *Router) GetId() string {
	return r.id
}
func (r *Router) FromToString() string {
	if r.from == nil {
		return ""
	} else {
		return r.from.ToString()
	}
}

func (r *Router) From(from string, configs ...types.Configuration) endpoint.From {
	var fromConfig = make(types.Configuration)
	for _, item := range configs {
		for k, v := range item {
			fromConfig[k] = v
		}
	}
	r.from = &From{Router: r, From: from, Config: fromConfig}

	return r.from
}

func (r *Router) GetFrom() endpoint.From {
	if r.from == nil {
		return nil
	}
	return r.from
}

func (r *Router) GetRuleGo(exchange *endpoint.Exchange) types.RuleEnginePool {
	if r.ruleGoFunc != nil {
		return r.ruleGoFunc(exchange)
	} else {
		return r.RuleGo
	}
}

// Disable 设置状态 true:不可用，false:可以
func (r *Router) Disable(disable bool) endpoint.Router {
	if disable {
		atomic.StoreUint32(&r.disable, 1)
	} else {
		atomic.StoreUint32(&r.disable, 0)
	}
	return r
}

// IsDisable 是否是不可用状态 true:不可用，false:可以
func (r *Router) IsDisable() bool {
	return atomic.LoadUint32(&r.disable) == 1
}

func (r *Router) SetParams(args ...interface{}) {
	r.params = args
}

func (r *Router) GetParams() []interface{} {
	return r.params
}
func (r *Router) Err() error {
	return r.err
}

// BaseEndpoint provides the fundamental implementation for all endpoint types.
// It implements common functionality including global interceptors, router management,
// and thread-safe operations for endpoint lifecycle management.
//
// BaseEndpoint 为所有端点类型提供基础实现。
// 它实现了通用功能，包括全局拦截器、路由器管理和端点生命周期管理的线程安全操作。
//
// Architecture / 架构：
// • Router Management: Thread-safe storage and retrieval of routers  路由器管理：路由器的线程安全存储和检索
// • Global Interceptors: Cross-cutting concerns for all message processing  全局拦截器：所有消息处理的横切关注点
// • Event Handling: Callback mechanism for endpoint lifecycle events  事件处理：端点生命周期事件的回调机制
// • Thread Safety: RWMutex protection for concurrent access  线程安全：RWMutex 保护并发访问
//
// Interceptor Pipeline / 拦截器管道：
// The BaseEndpoint processes messages through the following pipeline:
// BaseEndpoint 通过以下管道处理消息：
//
// 1. Global interceptors execution  全局拦截器执行
// 2. From endpoint processing  From 端点处理
// 3. To endpoint execution  To 端点执行
// 4. Post-processing and response handling  后处理和响应处理
//
// Concurrency / 并发性：
// • Safe for concurrent router addition/removal  安全的并发路由器添加/删除
// • Lock-free interceptor access using exported field  使用导出字段的无锁拦截器访问
// • Context creation and lifecycle management  上下文创建和生命周期管理
type BaseEndpoint struct {
	//endpoint 路由存储器  Router storage for the endpoint  端点的路由器存储
	RouterStorage map[string]endpoint.Router
	//事件回调处理器  Event callback handler  事件回调处理器
	OnEvent endpoint.OnEvent
	//全局拦截器 - 导出字段，允许直接访问以避免锁竞争  Global interceptors - exported field for direct access to avoid lock contention  全局拦截器
	Interceptors []endpoint.Process
	sync.RWMutex
}

func (e *BaseEndpoint) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	panic("not support this method")
}

func (e *BaseEndpoint) SetOnEvent(onEvent endpoint.OnEvent) {
	e.OnEvent = onEvent
}

// AddInterceptors adds global interceptors to the endpoint processing pipeline.
// These interceptors are executed for all incoming messages before routing logic.
// Interceptors are applied in the order they are added.
//
// AddInterceptors 向端点处理管道添加全局拦截器。
// 这些拦截器在路由逻辑之前对所有传入消息执行。
// 拦截器按添加顺序应用。
//
// Parameters / 参数：
// • interceptors: Processing functions to add to the global pipeline  要添加到全局管道的处理函数
//
// Thread Safety / 线程安全：
// This method is not thread-safe and should be called during initialization.
// 此方法不是线程安全的，应在初始化期间调用。
func (e *BaseEndpoint) AddInterceptors(interceptors ...endpoint.Process) {
	e.Interceptors = append(e.Interceptors, interceptors...)
}

// DoProcess executes the complete message processing pipeline for the endpoint.
// This is the main entry point for processing incoming messages through the endpoint system.
// The processing follows a specific order: context creation, global interceptors,
// From endpoint processing, and finally To endpoint execution.
//
// DoProcess 执行端点的完整消息处理管道。
// 这是通过端点系统处理传入消息的主要入口点。
// 处理遵循特定顺序：上下文创建、全局拦截器、From 端点处理，最后是 To 端点执行。
//
// Parameters / 参数：
// • baseCtx: Base context for the processing  处理的基础上下文
// • router: Router configuration for message routing  消息路由的路由器配置
// • exchange: Message exchange containing input and output data  包含输入和输出数据的消息交换
//
// Processing Pipeline / 处理管道：
// 1. Create execution context using router's context function  使用路由器的上下文函数创建执行上下文
// 2. Execute global interceptors in sequence  按顺序执行全局拦截器
// 3. Execute From endpoint processing pipeline  执行 From 端点处理管道
// 4. Execute To endpoint target logic  执行 To 端点目标逻辑
//
// Early Termination / 提前终止：
// If any interceptor or From processing returns false, the pipeline terminates early
// and subsequent steps are not executed.
// 如果任何拦截器或 From 处理返回 false，管道提前终止，后续步骤不会执行。
//
// Context Cancellation / 上下文取消：
// The method checks for context cancellation at key points to support graceful shutdown.
// 该方法在关键点检查上下文取消以支持优雅停机。
//
// Thread Safety / 线程安全：
// This method creates a thread-safe copy of interceptors to avoid race conditions
// during concurrent interceptor modifications.
// 此方法创建拦截器的线程安全副本，以避免并发拦截器修改期间的竞态条件。
func (e *BaseEndpoint) DoProcess(baseCtx context.Context, router endpoint.Router, exchange *endpoint.Exchange) {
	// Check if context is already cancelled before starting processing
	// 在开始处理前检查上下文是否已被取消
	if baseCtx != nil {
		select {
		case <-baseCtx.Done():
			// Context cancelled, set error and return early
			// 上下文已取消，设置错误并提前返回
			exchange.Out.SetError(fmt.Errorf("processing cancelled: %w", baseCtx.Err()))
			return
		default:
		}
	}

	//创建上下文  Create context  创建上下文
	ctx := e.createContext(baseCtx, router, exchange)

	// 线程安全地获取拦截器副本  Thread-safely get interceptor copy  线程安全地获取拦截器副本
	e.RLock()
	interceptors := make([]endpoint.Process, len(e.Interceptors))
	copy(interceptors, e.Interceptors)
	e.RUnlock()

	for _, item := range interceptors {
		// Check for context cancellation before each interceptor
		// 在每个拦截器前检查上下文取消
		if ctx != nil {
			select {
			case <-ctx.Done():
				exchange.Out.SetError(fmt.Errorf("processing cancelled during interceptor: %w", ctx.Err()))
				return
			default:
			}
		}

		//执行全局拦截器  Execute global interceptors  执行全局拦截器
		if !item(router, exchange) {
			return
		}
	}

	// Check for context cancellation before From processing
	// 在 From 处理前检查上下文取消
	if ctx != nil {
		select {
		case <-ctx.Done():
			exchange.Out.SetError(fmt.Errorf("processing cancelled before From: %w", ctx.Err()))
			return
		default:
		}
	}

	//执行from端逻辑  Execute From endpoint logic  执行 From 端逻辑
	if fromFlow := router.GetFrom(); fromFlow != nil {
		if !fromFlow.ExecuteProcess(router, exchange) {
			return
		}
	}

	// Check for context cancellation before To processing
	// 在 To 处理前检查上下文取消
	if ctx != nil {
		select {
		case <-ctx.Done():
			exchange.Out.SetError(fmt.Errorf("processing cancelled before To: %w", ctx.Err()))
			return
		default:
		}
	}

	//执行to端逻辑  Execute To endpoint logic  执行 To 端逻辑
	if router.GetFrom() != nil && router.GetFrom().GetTo() != nil {
		router.GetFrom().GetTo().Execute(ctx, exchange)
	}
}

func (e *BaseEndpoint) createContext(baseCtx context.Context, router endpoint.Router, exchange *endpoint.Exchange) context.Context {
	if router.GetContextFunc() != nil {
		if ctx := router.GetContextFunc()(baseCtx, exchange); ctx == nil {
			return context.Background()
		} else {
			exchange.Context = ctx
			return ctx
		}
	} else if baseCtx != nil {
		return baseCtx
	} else {
		return context.Background()
	}

}

func (e *BaseEndpoint) CheckAndSetRouterId(router endpoint.Router) string {
	if router.GetId() == "" {
		router.SetId(router.FromToString())
	}
	return router.GetId()
}

func (e *BaseEndpoint) Destroy() {
	e.Lock()
	defer e.Unlock()
	e.Interceptors = nil
	// Create a new map instead of clearing the existing one to avoid race conditions
	e.RouterStorage = make(map[string]endpoint.Router)
}

func (e *BaseEndpoint) GetRuleChainDefinition(configuration types.Configuration) *types.RuleChain {
	if v, ok := configuration[types.NodeConfigurationKeyRuleChainDefinition]; ok {
		if ruleNode, ok := v.(*types.RuleChain); ok {
			return ruleNode
		}
	}
	return nil
}

func (e *BaseEndpoint) HasRouter(id string) bool {
	e.RLock()
	defer e.RUnlock()
	_, ok := e.RouterStorage[id]
	return ok
}

// ExecutorFactory is a registry and factory for To endpoint executors.
// It manages different types of executors that handle the final destination logic
// for message processing in the endpoint system.
//
// ExecutorFactory 是 To 端点执行器的注册表和工厂。
// 它管理不同类型的执行器，处理端点系统中消息处理的最终目标逻辑。
//
// Architecture / 架构：
// • Registration: Thread-safe executor type registration  注册：线程安全的执行器类型注册
// • Factory Pattern: Creates new executor instances on demand  工厂模式：按需创建新的执行器实例
// • Type Safety: Maps executor names to their implementations  类型安全：将执行器名称映射到其实现
//
// Built-in Executors / 内置执行器：
// • "chain": Rule chain executor for routing to rule engines  规则链执行器，用于路由到规则引擎
// • "component": Component executor for direct node processing  组件执行器，用于直接节点处理
//
// Thread Safety / 线程安全：
// All operations are protected by RWMutex for concurrent access safety.
// 所有操作都受 RWMutex 保护，确保并发访问安全。
type ExecutorFactory struct {
	sync.RWMutex
	//执行器注册表，映射名称到执行器实现  Executor registry mapping names to implementations  执行器注册表
	executors map[string]endpoint.Executor
}

// Register adds a new executor type to the factory.
// The executor serves as a prototype for creating new instances.
//
// Register 向工厂添加新的执行器类型。
// 执行器作为创建新实例的原型。
//
// Parameters / 参数：
// • name: Unique identifier for the executor type  执行器类型的唯一标识符
// • executor: Prototype executor implementation  原型执行器实现
//
// Thread Safety / 线程安全：
// This method is thread-safe and can be called concurrently.
// 此方法是线程安全的，可以并发调用。
func (r *ExecutorFactory) Register(name string, executor endpoint.Executor) {
	r.Lock()
	defer r.Unlock()
	if r.executors == nil {
		r.executors = make(map[string]endpoint.Executor)
	}
	r.executors[name] = executor
}

// New creates a new executor instance by type name.
// Returns a new instance of the registered executor or false if not found.
//
// New 根据类型名称创建新的执行器实例。
// 返回注册执行器的新实例，如果未找到则返回 false。
//
// Parameters / 参数：
// • name: The executor type name to create  要创建的执行器类型名称
//
// Returns / 返回值：
// • endpoint.Executor: New executor instance  新的执行器实例
// • bool: True if executor type was found  如果找到执行器类型则为 true
//
// Thread Safety / 线程安全：
// This method is thread-safe and uses read lock for optimal performance.
// 此方法是线程安全的，使用读锁以获得最佳性能。
func (r *ExecutorFactory) New(name string) (endpoint.Executor, bool) {
	r.RLock()
	defer r.RUnlock()
	h, ok := r.executors[name]
	if ok {
		return h.New(), true
	} else {
		return nil, false
	}
}

// ChainExecutor is an executor implementation that routes messages to rule chains.
// It handles the integration between endpoint routing and the RuleGo rule engine,
// supporting both synchronous and asynchronous execution modes.
//
// ChainExecutor 是将消息路由到规则链的执行器实现。
// 它处理端点路由与 RuleGo 规则引擎之间的集成，支持同步和异步执行模式。
//
// Features / 功能：
// • Dynamic Path Resolution: Supports variable substitution in chain paths  动态路径解析：支持链路径中的变量替换
// • Multi-mode Execution: Synchronous and asynchronous processing  多模式执行：同步和异步处理
// • Node Targeting: Can route to specific nodes within a rule chain  节点定位：可以路由到规则链中的特定节点
// • Error Handling: Comprehensive error reporting and callback integration  错误处理：全面的错误报告和回调集成
//
// Path Format / 路径格式：
// • "chainId": Route to rule chain root  路由到规则链根
// • "chainId:nodeId": Route to specific node in chain  路由到链中的特定节点
//
// Variable Support / 变量支持：
// Paths can contain variables like "${userId}" that are resolved from message metadata.
// 路径可以包含如 "${userId}" 的变量，从消息元数据中解析。
type ChainExecutor struct {
}

func (ce *ChainExecutor) New() endpoint.Executor {

	return &ChainExecutor{}
}

// IsPathSupportVar to路径允许带变量
func (ce *ChainExecutor) IsPathSupportVar() bool {
	return true
}

func (ce *ChainExecutor) Init(_ types.Config, _ types.Configuration) error {
	return nil
}

func (ce *ChainExecutor) Execute(ctx context.Context, router endpoint.Router, exchange *endpoint.Exchange) {
	fromFlow := router.GetFrom()
	if fromFlow == nil {
		return
	}
	inMsg := exchange.In.GetMsg()
	if toFlow := fromFlow.GetTo(); toFlow != nil && inMsg != nil {
		toChainId := toFlow.ToStringByDict(inMsg.Metadata.GetReadOnlyValues())
		tos := strings.Split(toChainId, pathSplitFlag)
		toChainId = tos[0]
		//查找规则链，并执行
		if ruleEngine, ok := router.GetRuleGo(exchange).Get(toChainId); ok {
			opts := toFlow.GetOpts()
			//监听结束回调函数
			endFunc := types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				if err != nil {
					exchange.Out.SetError(err)
				} else {
					exchange.Out.SetMsg(&msg)
				}

				for _, process := range toFlow.GetProcessList() {
					if !process(router, exchange) {
						break
					}
				}
			})
			opts = append(opts, types.WithContext(ctx))
			if len(tos) > 1 {
				opts = append(opts, types.WithStartNode(tos[1]))
			}
			opts = append(opts, endFunc)

			if toFlow.IsWait() {
				//同步
				ruleEngine.OnMsgAndWait(*inMsg, opts...)
			} else {
				//异步
				ruleEngine.OnMsg(*inMsg, opts...)
			}
		} else {
			//找不到规则链返回错误
			for _, process := range toFlow.GetProcessList() {
				exchange.Out.SetError(fmt.Errorf("chainId=%s not found error", toChainId))
				if !process(router, exchange) {
					break
				}
			}
		}

	}
}

// ComponentExecutor is an executor implementation that routes messages directly to individual components.
// It provides a way to execute single node components without the overhead of a full rule chain,
// suitable for simple processing scenarios or component testing.
//
// ComponentExecutor 是将消息直接路由到单个组件的执行器实现。
// 它提供了一种执行单个节点组件的方式，无需完整规则链的开销，适用于简单处理场景或组件测试。
//
// Architecture / 架构：
// • Direct Execution: Bypasses rule chain infrastructure for performance  直接执行：绕过规则链基础设施以提高性能
// • Component Integration: Works with any registered component type  组件集成：适用于任何注册的组件类型
// • Context Management: Creates minimal rule context for component execution  上下文管理：为组件执行创建最小规则上下文
// • Synchronous Support: Optional blocking execution for responses  同步支持：响应的可选阻塞执行
//
// Use Cases / 使用场景：
// • Simple transformations without complex routing  无复杂路由的简单转换
// • Component testing and validation  组件测试和验证
// • High-performance single-step processing  高性能单步处理
// • Microservice-style component execution  微服务风格的组件执行
//
// Limitations / 限制：
// • No variable path support (IsPathSupportVar returns false)  不支持变量路径
// • Single component execution only  仅单组件执行
// • Limited rule context features compared to full chains  与完整链相比，规则上下文功能有限
type ComponentExecutor struct {
	//要执行的组件实例  Component instance to execute  要执行的组件实例
	component types.Node
	//规则引擎配置  Rule engine configuration  规则引擎配置
	config types.Config
}

func (ce *ComponentExecutor) New() endpoint.Executor {
	return &ComponentExecutor{}
}

// IsPathSupportVar to路径不允许带变量
func (ce *ComponentExecutor) IsPathSupportVar() bool {
	return false
}

func (ce *ComponentExecutor) Init(config types.Config, configuration types.Configuration) error {
	ce.config = config
	if configuration == nil {
		return fmt.Errorf("nodeType can't empty")
	}
	var nodeType = ""
	if v, ok := configuration[pathKey]; ok {
		nodeType = str.ToString(v)
	}
	node, err := config.ComponentsRegistry.NewNode(nodeType)
	if err == nil {
		ce.component = node
		err = ce.component.Init(config, configuration)
	}
	return err
}

func (ce *ComponentExecutor) Execute(ctx context.Context, router endpoint.Router, exchange *endpoint.Exchange) {
	if ce.component != nil {
		fromFlow := router.GetFrom()
		if fromFlow == nil {
			return
		}

		inMsg := exchange.In.GetMsg()
		if toFlow := fromFlow.GetTo(); toFlow != nil && inMsg != nil {
			//初始化的空上下文
			ruleCtx := engine.NewRuleContext(ctx, ce.config, nil, nil, nil, ce.config.Pool, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				if err != nil {
					exchange.Out.SetError(err)
				} else {
					exchange.Out.SetMsg(&msg)
				}
				for _, process := range toFlow.GetProcessList() {
					if !process(router, exchange) {
						break
					}
				}
			}, engine.DefaultPool)

			if toFlow.IsWait() {
				c := make(chan struct{})
				ruleCtx.SetOnAllNodeCompleted(func() {
					close(c)
				})
				//执行组件逻辑
				ce.component.OnMsg(ruleCtx, *inMsg)
				//等待执行结束
				<-c
			} else {
				//执行组件逻辑
				ce.component.OnMsg(ruleCtx, *inMsg)
			}
		}
	}
}

// DefaultExecutorFactory is the global factory instance for To endpoint executors.
// It provides a centralized registry for all executor types used in the endpoint system.
// The factory is pre-configured with built-in executor types during package initialization.
//
// DefaultExecutorFactory 是 To 端点执行器的全局工厂实例。
// 它为端点系统中使用的所有执行器类型提供集中注册表。
// 工厂在包初始化期间预配置了内置执行器类型。
//
// Built-in Executors / 内置执行器：
// • "chain": Routes messages to rule chains with full rule engine features  将消息路由到具有完整规则引擎功能的规则链
// • "component": Routes messages to individual components for direct processing  将消息路由到单个组件进行直接处理
//
// Extension / 扩展：
// Custom executor types can be registered using DefaultExecutorFactory.Register()
// 可以使用 DefaultExecutorFactory.Register() 注册自定义执行器类型
var DefaultExecutorFactory = new(ExecutorFactory)

// init registers the default executor types with the DefaultExecutorFactory.
// This initialization ensures that the basic executor types are available
// for use throughout the endpoint system.
//
// init 向 DefaultExecutorFactory 注册默认执行器类型。
// 此初始化确保基本执行器类型可在整个端点系统中使用。
//
// Registered Types / 注册类型：
// • "chain": ChainExecutor for rule chain integration  用于规则链集成的 ChainExecutor
// • "component": ComponentExecutor for direct component execution  用于直接组件执行的 ComponentExecutor
func init() {
	DefaultExecutorFactory.Register("chain", &ChainExecutor{})
	DefaultExecutorFactory.Register("component", &ComponentExecutor{})
}
