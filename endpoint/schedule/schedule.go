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

// Package schedule 用于启动定时任务
//路由from支持以下cron表达式

//Field name   | Mandatory? | Allowed values  | Allowed special characters
//----------   | ---------- | --------------  | --------------------------
//Seconds      | Yes        | 0-59            | * / , -
//Minutes      | Yes        | 0-59            | * / , -
//Hours        | Yes        | 0-23            | * / , -
//Day of month | Yes        | 1-31            | * / , - ?
//Month        | Yes        | 1-12 or JAN-DEC | * / , -
//Day of week  | Yes        | 0-6 or SUN-SAT  | * / , - ?

//内置一些特殊表达式：
//Entry                  | Description                                | Equivalent To
//-----                  | -----------                                | -------------
//@yearly (or @annually) | Run once a year, midnight, Jan. 1st        | 0 0 0 1 1 *
//@monthly               | Run once a month, midnight, first of month | 0 0 0 1 * *
//@weekly                | Run once a week, midnight between Sat/Sun  | 0 0 0 * * 0
//@daily (or @midnight)  | Run once a day, midnight                   | 0 0 0 * * *
//@hourly                | Run once an hour, beginning of hour        | 0 0 * * * *

package schedule

import (
	"context"
	"errors"
	"fmt"
	"github.com/gofrs/uuid/v5"
	"github.com/robfig/cron/v3"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"net/textproto"
	"strconv"
)

// Type 组件类型
const Type = types.EndpointTypePrefix + "schedule"

// Endpoint 别名
type Endpoint = Schedule

// RequestMessage http请求消息
type RequestMessage struct {
	headers textproto.MIMEHeader
	body    []byte
	msg     *types.RuleMsg
	err     error
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
	r.msg = msg
}
func (r *ResponseMessage) GetMsg() *types.RuleMsg {
	return r.msg
}

// SetStatusCode 不提供设置状态码
func (r *ResponseMessage) SetStatusCode(statusCode int) {
}

func (r *ResponseMessage) SetBody(body []byte) {
	r.body = body
}

func (r *ResponseMessage) SetError(err error) {
	r.err = err
}

func (r *ResponseMessage) GetError() error {
	return r.err
}

// Schedule 定时任务端点
type Schedule struct {
	id string
	impl.BaseEndpoint
	RuleConfig types.Config
	cron       *cron.Cron
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
			schedule.Printf("schedule handler err :%v", e)
		}
	}()
	exchange := &endpoint.Exchange{
		In:  &RequestMessage{},
		Out: &ResponseMessage{}}

	schedule.DoProcess(context.Background(), router, exchange)
}
