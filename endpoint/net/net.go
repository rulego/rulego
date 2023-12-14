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

package net

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/maps"
	"log"
	"net"
	"net/textproto"
	"os"
	"regexp"
	"time"
)

const (
	// Type 组件类型
	Type = "net"
	// RemoteAddrKey 远程地址键
	RemoteAddrKey = "remoteAddr"
	// PingData 心跳数据
	PingData = "ping"
)

// 注册组件
func init() {
	_ = endpoint.Registry.Register(&Endpoint{})
}

// RequestMessage 请求消息
type RequestMessage struct {
	headers textproto.MIMEHeader
	conn    net.Conn
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
	if r.conn != nil {
		r.headers.Set(RemoteAddrKey, r.conn.RemoteAddr().String())
	}
	return r.headers
}

// From 返回客户端Addr
func (r RequestMessage) From() string {
	if r.conn == nil {
		return ""
	}
	r.conn.RemoteAddr().Network()
	return r.conn.RemoteAddr().String()
}

func (r *RequestMessage) GetParam(key string) string {
	return ""
}

func (r *RequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}

func (r *RequestMessage) GetMsg() *types.RuleMsg {
	if r.msg == nil {
		dataType := types.TEXT
		ruleMsg := types.NewMsg(0, r.From(), dataType, types.NewMetadata(), string(r.Body()))
		r.msg = &ruleMsg
	}
	return r.msg
}

// SetStatusCode 不提供设置响应状态码
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

func (r *RequestMessage) Conn() net.Conn {
	return r.conn
}

// ResponseMessage 响应消息
type ResponseMessage struct {
	headers textproto.MIMEHeader
	conn    net.Conn
	log     func(format string, v ...interface{})
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
	if r.conn != nil {
		r.headers.Set(RemoteAddrKey, r.conn.RemoteAddr().String())
	}
	return r.headers
}

func (r *ResponseMessage) From() string {
	if r.conn == nil {
		return ""
	}
	return r.conn.RemoteAddr().String()
}

func (r *ResponseMessage) GetParam(key string) string {
	return ""
}

func (r *ResponseMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}
func (r *ResponseMessage) GetMsg() *types.RuleMsg {
	return r.msg
}

func (r *ResponseMessage) SetStatusCode(statusCode int) {
}

func (r *ResponseMessage) SetBody(body []byte) {
	r.body = body
	if r.conn == nil {
		log.Println("write err: conn is nil")
		return
	}
	_, err := r.conn.Write(body)
	if err != nil {
		log.Println("write err:", err)
	}
}

func (r *ResponseMessage) SetError(err error) {
	r.err = err
}

func (r *ResponseMessage) GetError() error {
	return r.err
}

// Config endpoint组件的配置
type Config struct {
	// 通信协议，可以是tcp、udp、ip4:1、ip6:ipv6-icmp、ip6:58、unix、unixgram，以及net包支持的协议类型。默认tcp协议
	Protocol string
	// 服务器的地址，格式为host:port
	Server string
	// 读取超时，用于设置读取数据的超时时间，单位为秒，可以为0表示不设置超时
	ReadTimeout int
}

// RegexpRouter 正则表达式路由
type RegexpRouter struct {
	//路由
	router *endpoint.Router
	//正则表达式
	regexp *regexp.Regexp
}

// Endpoint net endpoint组件
// 支持通过正则表达式把匹配的消息路由到指定路由
type Endpoint struct {
	// 嵌入endpoint.BaseEndpoint，继承其方法
	endpoint.BaseEndpoint
	// 配置
	Config Config
	// rulego配置
	RuleConfig types.Config
	// 服务器监听器对象
	listener net.Listener
	// 路由映射表
	routers map[string]*RegexpRouter
}

// Type 组件类型
func (ep *Endpoint) Type() string {
	return Type
}

func (ep *Endpoint) New() types.Node {
	return &Endpoint{
		Config: Config{Protocol: "tcp", ReadTimeout: 60},
	}
}

// Init 初始化
func (ep *Endpoint) Init(ruleConfig types.Config, configuration types.Configuration) error {
	// 将配置转换为EndpointConfiguration结构体
	err := maps.Map2Struct(configuration, &ep.Config)
	if ep.Config.Protocol == "" {
		ep.Config.Protocol = "tcp"
	}
	ep.RuleConfig = ruleConfig
	return err
}

// Destroy 销毁
func (ep *Endpoint) Destroy() {
	_ = ep.Close()
}

func (ep *Endpoint) Close() error {
	if nil != ep.listener {
		err := ep.listener.Close()
		ep.listener = nil
		return err
	}
	return nil
}

func (ep *Endpoint) Id() string {
	return ep.Config.Server
}

func (ep *Endpoint) AddRouter(router *endpoint.Router, params ...interface{}) (string, error) {
	if router == nil {
		return "", errors.New("router can not nil")
	} else {
		expr := router.GetFrom().From
		//允许空expr，表示匹配所有
		var regexpV *regexp.Regexp
		if expr != "" {
			//编译表达式
			if re, err := regexp.Compile(expr); err != nil {
				return "", err
			} else {
				regexpV = re
			}
		}
		ep.Lock()
		defer ep.Unlock()
		if ep.routers == nil {
			ep.routers = make(map[string]*RegexpRouter)
		}
		if _, ok := ep.routers[expr]; ok {
			return expr, fmt.Errorf("duplicate router %s", expr)
		} else {
			ep.routers[expr] = &RegexpRouter{
				router: router,
				regexp: regexpV,
			}

			return expr, nil
		}

	}
}

func (ep *Endpoint) RemoveRouter(routerId string, params ...interface{}) error {
	ep.Lock()
	defer ep.Unlock()
	if ep.routers != nil {
		delete(ep.routers, routerId)
	}
	return nil
}

func (ep *Endpoint) Start() error {
	var err error
	// 根据配置的协议和地址，创建一个服务器监听器
	ep.listener, err = net.Listen(ep.Config.Protocol, ep.Config.Server)
	if err != nil {
		return err
	}
	// 打印服务器启动的信息
	ep.Printf("starting server on :%s", ep.Config.Server)
	// 循环接受客户端的连接请求
	for {
		// 从监听器中获取一个客户端连接，返回连接对象和错误信息
		conn, err := ep.listener.Accept()
		if err != nil {
			if opError, ok := err.(*net.OpError); ok && opError.Err == net.ErrClosed {
				ep.Printf("net endpoint stop")
				return endpoint.EndpointStopErr
			} else {
				ep.Printf("accept:", err)
				continue
			}
		}
		// 打印客户端连接的信息
		ep.Printf("new connection from:", conn.RemoteAddr().String())
		// 启动一个协端处理客户端连接
		go ep.handler(conn)
	}
}

func (ep *Endpoint) Printf(format string, v ...interface{}) {
	if ep.RuleConfig.Logger != nil {
		ep.RuleConfig.Logger.Printf(format, v...)
	}
}

func (ep *Endpoint) handler(conn net.Conn) {
	h := ClientHandler{
		endpoint: ep,
		conn:     conn,
	}
	h.handler()
}

type ClientHandler struct {
	endpoint *Endpoint
	// 客户端连接对象
	conn net.Conn
	// 创建一个读取超时定时器，用于设置读取数据的超时时间，可以为0表示不设置超时
	readTimeoutTimer *time.Timer
}

func (x *ClientHandler) handler() {
	defer func() {
		_ = x.conn.Close()
		//捕捉异常
		if e := recover(); e != nil {
			x.endpoint.Printf("net handler err :%v\n %s", e, assert.CallerInfo())
		}
	}()
	readTimeoutDuration := time.Duration(x.endpoint.Config.ReadTimeout+5) * time.Second
	//读超时，断开连接
	x.readTimeoutTimer = time.AfterFunc(readTimeoutDuration, func() {
		if x.endpoint.Config.ReadTimeout > 0 {
			x.onDisconnect()
		}
	})
	// 创建一个缓冲读取器，用于读取客户端发送的数据
	reader := bufio.NewReader(x.conn)
	// 循环读取客户端发送的数据
	for {
		// 设置读取超时
		if x.endpoint.Config.ReadTimeout > 0 {
			err := x.conn.SetReadDeadline(time.Now().Add(readTimeoutDuration))
			if err != nil {
				x.onDisconnect()
				break
			}
		}

		// 读取一行数据，直到遇到\n或者\t\n为止
		data, _, err := reader.ReadLine()

		if err != nil && err.Error() != os.ErrDeadlineExceeded.Error() {
			if e, ok := err.(*net.OpError); ok {
				if e.Err != os.ErrDeadlineExceeded {
					x.onDisconnect()
					break
				} else {
					continue
				}
			} else {
				x.onDisconnect()
				break
			}
		}
		//重置读超时定时器
		if x.endpoint.Config.ReadTimeout > 0 {
			x.readTimeoutTimer.Reset(readTimeoutDuration)
		}
		if string(data) == PingData {
			continue
		}
		// 创建一个交换对象，用于存储输入和输出的消息
		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				conn: x.conn,
				body: data,
			},
			Out: &ResponseMessage{
				log: func(format string, v ...interface{}) {
					x.endpoint.Printf(format, v...)
				},
				conn: x.conn,
			}}

		msg := exchange.In.GetMsg()
		// 把客户端连接的地址放到msg元数据中
		msg.Metadata.PutValue(RemoteAddrKey, x.conn.RemoteAddr().String())

		// 匹配符合的路由，处理消息
		for _, v := range x.endpoint.routers {
			if v.regexp == nil || v.regexp.Match(data) {
				x.endpoint.DoProcess(v.router, exchange)
			}
		}
	}

}

func (x *ClientHandler) onDisconnect() {
	if x.conn != nil {
		_ = x.conn.Close()
	}
	if x.readTimeoutTimer != nil {
		x.readTimeoutTimer.Stop()
	}
	x.endpoint.Printf("onDisconnect:" + x.conn.RemoteAddr().String())
}
