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

// Package schedule provides a scheduled task endpoint implementation for the RuleGo framework.
// It enables creating scheduled tasks that execute rule chains or components at specified times
// using cron expressions, providing time-based automation capabilities.
//
// Package schedule 为 RuleGo 框架提供定时任务端点实现。
// 它支持创建定时任务，使用 cron 表达式在指定时间执行规则链或组件，
// 提供基于时间的自动化功能。
//
// Key Features / 主要特性：
//
// • Cron Expression Support: Full cron syntax with second-level precision  Cron 表达式支持：完整的 cron 语法，具有秒级精度
// • Rule Chain Integration: Direct integration with RuleGo rule chains  规则链集成：与 RuleGo 规则链直接集成
// • Multiple Schedules: Support for multiple concurrent scheduled tasks  多重调度：支持多个并发定时任务
// • Dynamic Management: Runtime addition and removal of scheduled tasks  动态管理：运行时添加和删除定时任务
// • Built-in Expressions: Predefined expressions for common scheduling patterns  内置表达式：常见调度模式的预定义表达式
//
// Architecture / 架构：
//
// The Schedule endpoint follows a time-driven execution model:
// Schedule 端点遵循时间驱动的执行模型：
//
// 1. Cron Engine: Manages time-based task scheduling  Cron 引擎：管理基于时间的任务调度
// 2. Task Execution: Triggers rule chains at scheduled times  任务执行：在预定时间触发规则链
// 3. Message Generation: Creates RuleMsg for scheduled executions  消息生成：为定时执行创建 RuleMsg
// 4. Rule Processing: Executes business logic through rule chains  规则处理：通过规则链执行业务逻辑
//
// Cron Expression Format / Cron 表达式格式：
//
// The router's 'from' field supports the following cron expression format:
// 路由器的 'from' 字段支持以下 cron 表达式格式：
//
// Field name   | Mandatory? | Allowed values  | Allowed special characters
// 字段名称     | 是否必需?  | 允许的值        | 允许的特殊字符
// ----------   | ---------- | --------------  | --------------------------
// Seconds      | Yes        | 0-59            | * / , -
// 秒           | 是         | 0-59            | * / , -
// Minutes      | Yes        | 0-59            | * / , -
// 分钟         | 是         | 0-59            | * / , -
// Hours        | Yes        | 0-23            | * / , -
// 小时         | 是         | 0-23            | * / , -
// Day of month | Yes        | 1-31            | * / , - ?
// 月中的日     | 是         | 1-31            | * / , - ?
// Month        | Yes        | 1-12 or JAN-DEC | * / , -
// 月份         | 是         | 1-12 或 JAN-DEC | * / , -
// Day of week  | Yes        | 0-6 or SUN-SAT  | * / , - ?
// 周中的日     | 是         | 0-6 或 SUN-SAT  | * / , - ?
//
// Built-in Special Expressions / 内置特殊表达式：
//
// Entry                  | Description                                | Equivalent To
// 条目                   | 描述                                       | 等价于
// -----                  | -----------                                | -------------
// @yearly (or @annually) | Run once a year, midnight, Jan. 1st        | 0 0 0 1 1 *
// @yearly (或 @annually) | 每年运行一次，1月1日午夜                   | 0 0 0 1 1 *
// @monthly               | Run once a month, midnight, first of month | 0 0 0 1 * *
// @monthly               | 每月运行一次，每月1日午夜                  | 0 0 0 1 * *
// @weekly                | Run once a week, midnight between Sat/Sun  | 0 0 0 * * 0
// @weekly                | 每周运行一次，周六/周日之间的午夜          | 0 0 0 * * 0
// @daily (or @midnight)  | Run once a day, midnight                   | 0 0 0 * * *
// @daily (或 @midnight)  | 每天运行一次，午夜                         | 0 0 0 * * *
// @hourly                | Run once an hour, beginning of hour        | 0 0 * * * *
// @hourly                | 每小时运行一次，整点开始                   | 0 0 * * * *
//
// Initialization Methods / 初始化方法：
//
// The Schedule endpoint supports two initialization approaches:
// Schedule 端点支持两种初始化方法：
//
// 1. Registry-based Initialization / 基于注册表的初始化：
//
//	import "github.com/rulego/rulego/endpoint"
//
//	// Create endpoint through registry
//	// 通过注册表创建端点
//	endpoint, err := endpoint.Registry.New(schedule.Type, ruleConfig, types.Configuration{})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Add scheduled tasks
//	// 添加定时任务
//	router1 := endpoint.NewRouter().From("0 * * * * *").To("chain:minuteTask")
//	endpoint.AddRouter(router1)
//
//	router2 := endpoint.NewRouter().From("0 30 2 * * *").To("chain:dailyBackup")
//	endpoint.AddRouter(router2)
//
//	endpoint.Start()
//
// 2. Dynamic DSL Initialization / 动态 DSL 初始化：
//
//	dslConfig := `{
//	  "id": "schedule-endpoint",
//	  "type": "endpoint/schedule",
//	  "name": "Task Scheduler",
//	  "configuration": {},
//	  "routers": [
//	    {
//	      "id": "minute-task",
//	      "from": {
//	        "path": "0 * * * * *"
//	      },
//	      "to": {
//	        "path": "chain:minuteTask"
//	      }
//	    },
//	    {
//	      "id": "daily-backup",
//	      "from": {
//	        "path": "0 30 2 * * *"
//	      },
//	      "to": {
//	        "path": "chain:dailyBackup"
//	      }
//	    },
//	    {
//	      "id": "monitoring",
//	      "from": {
//	        "path": "*/15 * * * * *"
//	      },
//	      "to": {
//	        "path": "chain:monitoring"
//	      }
//	    }
//	  ]
//	}`
//
//	// Create endpoint from DSL
//	// 从 DSL 创建端点
//	endpoint, err := endpoint.NewFromDsl([]byte(dslConfig))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	endpoint.Start()
//
// Cron Expression Examples / Cron 表达式示例：
//
//	// Every minute
//	// 每分钟
//	router := endpoint.NewRouter().From("0 * * * * *").To("chain:minuteTask")
//
//	// Every day at 2:30 AM
//	// 每天凌晨 2:30
//	router := endpoint.NewRouter().From("0 30 2 * * *").To("chain:dailyBackup")
//
//	// Every 15 seconds
//	// 每 15 秒
//	router := endpoint.NewRouter().From("*/15 * * * * *").To("chain:monitoring")
//
//	// Business hours (9 AM to 5 PM, weekdays)
//	// 工作时间（工作日上午 9 点到下午 5 点）
//	router := endpoint.NewRouter().From("0 0 9-17 * * 1-5").To("chain:businessHours")
package schedule

import (
	"context"
	"errors"
	"fmt"
	"net/textproto"
	"strconv"
	"sync"

	"github.com/gofrs/uuid/v5"
	"github.com/robfig/cron/v3"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/utils/runtime"
)

// Type defines the component type identifier for the Schedule endpoint.
// This identifier is used for component registration and DSL configuration.
// Type 定义 Schedule 端点的组件类型标识符。
// 此标识符用于组件注册和 DSL 配置。
const Type = types.EndpointTypePrefix + "schedule"

// Endpoint is an alias for Schedule to provide consistent naming with other endpoints.
// This allows users to reference the component using the standard Endpoint name.
// Endpoint 是 Schedule 的别名，提供与其他端点一致的命名。
// 这允许用户使用标准的 Endpoint 名称引用组件。
type Endpoint = Schedule

// RequestMessage represents a scheduled task execution request in the RuleGo processing pipeline.
// Unlike other endpoints that receive external messages, the Schedule endpoint generates
// internal messages when scheduled tasks are triggered.
//
// RequestMessage 表示 RuleGo 处理管道中的定时任务执行请求。
// 与接收外部消息的其他端点不同，Schedule 端点在定时任务触发时生成内部消息。
//
// Key Features / 主要特性：
// • Time-Triggered Generation: Messages are generated based on cron schedules  基于时间触发的生成：根据 cron 调度生成消息
// • Minimal Payload: Contains minimal data as the trigger is time-based  最小载荷：由于触发器基于时间，包含最少数据
// • Metadata Integration: Seamlessly integrates with RuleGo's metadata system  元数据集成：与 RuleGo 元数据系统无缝集成
// • JSON Data Type: Uses JSON format for consistent processing  JSON 数据类型：使用 JSON 格式进行一致处理
//
// Message Content / 消息内容：
// The message body is typically empty as the trigger event is the schedule itself.
// Additional context can be provided through metadata or rule chain configuration.
// 消息体通常为空，因为触发事件就是调度本身。
// 可以通过元数据或规则链配置提供额外的上下文。
type RequestMessage struct {
	//HTTP 风格的头部映射，存储调度特定信息  HTTP-style headers map storing schedule-specific information  头部映射
	headers textproto.MIMEHeader
	//消息体数据，通常为空  Message body data, typically empty  消息体数据
	body []byte
	//转换后的规则消息，缓存以避免重复转换  Converted rule message, cached to avoid re-conversion  转换后的规则消息
	msg *types.RuleMsg
	//处理过程中的错误信息  Error information during processing  处理错误信息
	err error
}

func (r *RequestMessage) Body() []byte {
	return r.body
}

func (r *RequestMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	return r.headers
}

// From 不提供获取来源
func (r *RequestMessage) From() string {
	return ""
}

// GetParam 不提供获取参数
func (r *RequestMessage) GetParam(key string) string {
	return ""
}

func (r *RequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}

func (r *RequestMessage) GetMsg() *types.RuleMsg {
	if r.msg == nil {
		ruleMsg := types.NewMsg(0, r.From(), types.JSON, types.NewMetadata(), string(r.Body()))
		r.msg = &ruleMsg
	}
	return r.msg
}

// SetStatusCode 不提供设置状态码
func (r *RequestMessage) SetStatusCode(statusCode int) {
}

func (r *RequestMessage) SetBody(body []byte) {
	r.body = body
}

func (r *RequestMessage) SetError(err error) {
	r.err = err
}

func (r *RequestMessage) GetError() error {
	return r.err
}

// ResponseMessage 响应消息
type ResponseMessage struct {
	headers textproto.MIMEHeader
	body    []byte
	msg     *types.RuleMsg
	err     error
	mu      sync.RWMutex
}

func (r *ResponseMessage) Body() []byte {
	return r.body
}

func (r *ResponseMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	return r.headers
}

// From 不提供获取来源
func (r *ResponseMessage) From() string {
	return ""
}

// GetParam 不提供获取参数
func (r *ResponseMessage) GetParam(key string) string {
	return ""
}

func (r *ResponseMessage) SetMsg(msg *types.RuleMsg) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msg = msg
}
func (r *ResponseMessage) GetMsg() *types.RuleMsg {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.msg
}

// SetStatusCode 不提供设置状态码
func (r *ResponseMessage) SetStatusCode(statusCode int) {
}

func (r *ResponseMessage) SetBody(body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.body = body
}

func (r *ResponseMessage) SetError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

func (r *ResponseMessage) GetError() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.err
}

// Schedule represents a scheduled task endpoint implementation for the RuleGo framework.
// It provides time-based automation capabilities by executing rule chains or components
// at specified intervals using cron expressions.
//
// Schedule 表示 RuleGo 框架的定时任务端点实现。
// 它通过使用 cron 表达式在指定间隔执行规则链或组件来提供基于时间的自动化功能。
//
// Architecture / 架构：
//
// The Schedule endpoint uses a cron-based scheduling system:
// Schedule 端点使用基于 cron 的调度系统：
//
// 1. Cron Engine: Manages all scheduled tasks and timing  Cron 引擎：管理所有定时任务和时间
// 2. Task Registry: Stores and manages active scheduled tasks  任务注册表：存储和管理活动的定时任务
// 3. Execution Engine: Triggers rule chain execution at scheduled times  执行引擎：在预定时间触发规则链执行
// 4. Lifecycle Management: Handles task creation, modification, and cleanup  生命周期管理：处理任务创建、修改和清理
//
// Key Features / 主要特性：
//
// • Precise Scheduling: Second-level precision with full cron syntax  精确调度：秒级精度和完整 cron 语法
// • Multiple Tasks: Support for unlimited concurrent scheduled tasks  多任务：支持无限并发定时任务
// • Dynamic Management: Runtime task addition and removal  动态管理：运行时任务添加和删除
// • Unique Identification: Each endpoint instance has a unique identifier  唯一标识：每个端点实例都有唯一标识符
// • Automatic Cleanup: Proper resource cleanup on endpoint destruction  自动清理：端点销毁时的适当资源清理
//
// Task Management / 任务管理：
//
// Tasks are managed through the following lifecycle:
// 任务通过以下生命周期进行管理：
//
// 1. Creation: AddRouter() creates and registers a new scheduled task  创建：AddRouter() 创建并注册新的定时任务
// 2. Execution: Tasks execute automatically based on cron schedule  执行：任务根据 cron 调度自动执行
// 3. Removal: RemoveRouter() stops and removes scheduled tasks  删除：RemoveRouter() 停止并删除定时任务
// 4. Cleanup: Destroy() cleans up all resources and stops the cron engine  清理：Destroy() 清理所有资源并停止 cron 引擎
//
// Cron Engine Configuration / Cron 引擎配置：
//
// The cron engine is configured with second-level precision:
// cron 引擎配置为秒级精度：
//
// • WithSeconds(): Enables second-level scheduling precision  启用秒级调度精度
// • Thread-safe: Multiple goroutines can safely interact with tasks  线程安全：多个协程可以安全地与任务交互
// • Efficient: Optimized for minimal resource overhead  高效：优化以减少资源开销
//
// Error Handling / 错误处理：
//
// • Invalid cron expressions are detected during task registration  无效的 cron 表达式在任务注册期间被检测
// • Task execution errors are isolated and don't affect other tasks  任务执行错误被隔离且不影响其他任务
// • Comprehensive logging for debugging and monitoring  全面的日志记录用于调试和监控
//
// Performance Considerations / 性能考虑：
//
// • Lightweight cron engine with minimal memory footprint  轻量级 cron 引擎，内存占用最小
// • Efficient task scheduling algorithms  高效的任务调度算法
// • Non-blocking task execution  非阻塞任务执行
// • Automatic garbage collection of completed tasks  已完成任务的自动垃圾收集
type Schedule struct {
	// id is a unique identifier for this Schedule endpoint instance
	// id 是此 Schedule 端点实例的唯一标识符
	id string

	// BaseEndpoint provides common endpoint functionality
	// BaseEndpoint 提供通用端点功能
	impl.BaseEndpoint

	// RuleConfig provides access to the rule engine configuration
	// RuleConfig 提供对规则引擎配置的访问
	RuleConfig types.Config

	// cron is the underlying cron engine instance that manages all scheduled tasks
	// cron 是管理所有定时任务的底层 cron 引擎实例
	cron *cron.Cron
}

// New 创建一个新的Schedule Endpoint 实例
func New(ruleConfig types.Config) *Schedule {
	uuId, _ := uuid.NewV4()
	return &Schedule{RuleConfig: ruleConfig, cron: cron.New(cron.WithSeconds()), id: uuId.String()}
}

// Type 组件类型
func (schedule *Schedule) Type() string {
	return Type
}

func (schedule *Schedule) New() types.Node {
	uuId, _ := uuid.NewV4()
	return &Schedule{cron: cron.New(cron.WithSeconds()), id: uuId.String()}
}

// Init 初始化
func (schedule *Schedule) Init(ruleConfig types.Config, configuration types.Configuration) error {
	schedule.RuleConfig = ruleConfig
	return nil
}

// Destroy 销毁
func (schedule *Schedule) Destroy() {
	_ = schedule.Close()
}

func (schedule *Schedule) Close() error {
	if schedule.cron != nil {
		schedule.cron.Stop()
		schedule.cron = nil
	}
	schedule.BaseEndpoint.Destroy()
	return nil
}

func (schedule *Schedule) Id() string {
	return schedule.id
}

func (schedule *Schedule) AddRouter(router endpoint.Router, params ...interface{}) (string, error) {
	if router == nil {
		return "", errors.New("router can not nil")
	}
	if router.GetFrom() == nil {
		return "", errors.New("from can not nil")
	}
	if schedule.cron == nil {
		schedule.cron = cron.New(cron.WithSeconds())
	}
	//获取cron表达式
	from := router.GetFrom().ToString()
	//添加任务
	id, err := schedule.cron.AddFunc(from, func() {
		schedule.handler(router)
	})
	idStr := strconv.Itoa(int(id))
	router.SetId(idStr)
	//返回任务ID，用于清除任务
	return idStr, err
}

func (schedule *Schedule) RemoveRouter(routeId string, params ...interface{}) error {
	entryID, err := strconv.Atoi(routeId)
	if err != nil {
		return fmt.Errorf("%s it is an illegal routing id", routeId)
	}
	if schedule.cron != nil {
		schedule.cron.Remove(cron.EntryID(entryID))
	}
	return nil
}

func (schedule *Schedule) Start() error {
	if schedule.cron == nil {
		return errors.New("cron has not been initialized yet")
	}
	schedule.cron.Start()
	return nil
}

func (schedule *Schedule) Printf(format string, v ...interface{}) {
	if schedule.RuleConfig.Logger != nil {
		schedule.RuleConfig.Logger.Printf(format, v...)
	}
}

// 处理定时任务
func (schedule *Schedule) handler(router endpoint.Router) {
	defer func() {
		//捕捉异常
		if e := recover(); e != nil {
			schedule.Printf("schedule endpoint handler err :\n%v", runtime.Stack())
		}
	}()
	exchange := &endpoint.Exchange{
		In:  &RequestMessage{},
		Out: &ResponseMessage{}}

	schedule.DoProcess(context.Background(), router, exchange)
}
