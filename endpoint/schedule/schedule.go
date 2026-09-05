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

// Package schedule provides a scheduled task endpoint that triggers rule chains
// at times given by cron expressions.
//
// Package schedule 提供定时任务端点，按 cron 表达式在指定时间触发规则链。
//
// The router 'from' field must be a full 6-field cron expression with seconds
// (second minute hour day-of-month month day-of-week), e.g. `0 0 9 * * *` runs
// daily at 09:00. Predefined descriptors such as @hourly and @daily are also
// supported.
// 路由 from 字段必须是含秒位的 6 字段 cron 表达式（秒 分 时 日 月 周），
// 如 `0 0 9 * * *` 表示每天 09:00；也支持 @hourly、@daily 等预定义描述符。
//
//	router := endpoint.NewRouter().From("0 0 9 * * *").To("chain:dailyTask")
package schedule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/textproto"
	"strconv"
	"sync"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/robfig/cron/v3"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/utils/runtime"
	"github.com/rulego/rulego/utils/str"
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

// RequestMessage is generated on each scheduled tick. The body is empty unless
// the router provides it through params.
// RequestMessage 表示每次定时触发生成的消息；消息体默认为空，可经路由 params 指定。
type RequestMessage struct {
	//HTTP 风格的头部映射，存储调度特定信息  HTTP-style headers map storing schedule-specific information  头部映射
	headers textproto.MIMEHeader
	//消息体数据，通常为空  Message body data, typically empty  消息体数据
	body []byte
	//转换后的规则消息，缓存以避免重复转换  Converted rule message, cached to avoid re-conversion  转换后的规则消息
	msg *types.RuleMsg
	//处理过程中的错误信息  Error information during processing  处理错误信息
	err error
	//消息类型
	msgType types.DataType
	//元数据，用于传递额外的上下文信息  Metadata for passing additional context information  元数据
	metadata map[string]string
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
		dataType := types.JSON
		if r.msgType != "" {
			dataType = r.msgType
		}
		metadata := types.NewMetadata()
		// 如果有传入的 metadata，设置到消息中
		if r.metadata != nil {
			for k, v := range r.metadata {
				metadata.PutValue(k, v)
			}
		}
		ruleMsg := types.NewMsg(0, r.From(), dataType, metadata, string(r.Body()))
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

// Schedule is the cron-based scheduled task endpoint. Each router is one
// schedule: AddRouter registers it, RemoveRouter stops it, and Destroy stops
// the whole cron engine. When a Locker is configured, a scheduled slot runs at
// most once across replicas; see newOnceJob.
// Schedule 是基于 cron 的定时任务端点，每条路由即一个定时任务：
// AddRouter 注册、RemoveRouter 停止、Destroy 停止整个 cron 引擎。
// 配置 Locker 时同一计划槽位在副本间至多执行一次（见 newOnceJob）。
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

// Category returns the component category
func (schedule *Schedule) Category() string {
	return "endpoint"
}

// Def returns the component definition. Schedule requires a 6-field cron expression in from.path.
func (schedule *Schedule) Def() types.ComponentForm {
	return types.ComponentForm{
		Desc: "Scheduled task endpoint that triggers rule chains at specified times. from.path must be a full 6-field cron expression (Seconds Minutes Hours Day-of-month Month Day-of-week), e.g. `*/5 * * * * *` runs every 5 seconds. A bare `*` is NOT a valid cron expression.",
		RouterForm: &types.RouterForm{
			From: &types.RouterFormField{
				Path: types.ComponentFormField{
					Name:     "path",
					Type:     "string",
					Label:    "Cron",
					Desc:     "6-field cron expression, e.g. */5 * * * * *",
					Required: true,
				},
			},
			Params: &types.ComponentFormField{
				Name:     "params",
				Type:     "array",
				Desc:     "message body + data type emitted on each scheduled tick (no external input), e.g. [\"{}\",\"JSON\"]; do not leave null",
				Required: true,
			},
		},
	}
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
	if len(params) > 0 {
		router.SetParams(params...)
	}
	if schedule.cron == nil {
		schedule.cron = cron.New(cron.WithSeconds())
	}
	//获取cron表达式
	from := router.GetFrom().ToString()
	spec, err := cronParser.Parse(from)
	if err != nil {
		return "", err
	}
	job := func() {
		schedule.handler(router)
	}
	if schedule.RuleConfig.Locker != nil {
		job = schedule.newOnceJob(router, spec)
	}
	//添加任务
	id, err := schedule.cron.AddFunc(from, job)
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

// cronParser 与 cron 引擎一致的 6 字段秒级解析规则。
var cronParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// newOnceJob 包装定时回调：同一计划槽位在共享 Locker 的副本间只执行一次，
// 槽位重锚到触发时刻，推进与执行分离，长任务不阻塞下一拍。
func (schedule *Schedule) newOnceJob(router endpoint.Router, spec cron.Schedule) func() {
	routerId := router.GetId()
	if routerId == "" {
		routerId = router.GetFrom().ToString()
	}
	guard := types.NewOnceGuard(schedule.RuleConfig, "schedule:"+schedule.RuleConfig.Owner+":"+routerId)
	var mu sync.Mutex
	next := spec.Next(time.Now())
	return func() {
		now := time.Now()
		mu.Lock()
		slot, nextSlot := advanceSlot(spec, next, now)
		next = nextSlot
		mu.Unlock()
		if !guard.Allow(context.Background(), strconv.FormatInt(slot.Unix(), 10)) {
			return
		}
		schedule.handler(router)
	}
}

// advanceSlot 推进到不晚于 now 的最近计划时刻，落后的槽位直接跳过。
func advanceSlot(spec cron.Schedule, from, now time.Time) (slot, next time.Time) {
	slot = from
	for {
		n := spec.Next(slot)
		if n.After(now) {
			return slot, n
		}
		slot = n
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
	var body []byte
	var msgType = types.JSON
	var metadata map[string]string
	params := router.GetParams()
	if len(params) > 0 {
		if params[0] != nil {
			body = []byte(str.ToString(params[0]))
		}
	}
	if len(params) > 1 {
		if params[1] != nil {
			switch v := params[1].(type) {
			case types.DataType:
				msgType = v
			case string:
				msgType = types.DataType(v)
			default:
				msgType = types.DataType(str.ToString(params[1]))
			}
		}
	}
	// 处理第三个参数作为 metadata
	if len(params) > 2 && params[2] != nil {
		switch v := params[2].(type) {
		case map[string]string:
			metadata = v
		case map[string]interface{}:
			metadata = make(map[string]string)
			for key, val := range v {
				metadata[key] = str.ToString(val)
			}
		case string:
			if v != "" {
				// 尝试解析为 JSON
				var m map[string]string
				if err := json.Unmarshal([]byte(v), &m); err == nil {
					metadata = m
				}
			}
		}
	}
	// 触发来源记录为 schedule 端点组件名
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata[types.KeyTriggerSource] = Type
	exchange := &endpoint.Exchange{
		In:  &RequestMessage{body: body, msgType: msgType, metadata: metadata},
		Out: &ResponseMessage{}}

	schedule.DoProcess(context.Background(), router, exchange)
}
