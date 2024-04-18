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

package test

import (
	"context"
	"errors"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	reflect2 "github.com/rulego/rulego/utils/reflect"
	"net/textproto"
	"reflect"
	"strings"
	"time"

	"testing"
)

var (
	shareKey       = "shareKey"
	shareValue     = "shareValue"
	addShareKey    = "addShareKey"
	addShareValue  = "addShareValue"
	testdataFolder = "./testdata/"
	contentType    = "Content-Type"
	content        = "application/json"
)

// CreateAndInitNode 创建并初始化一个节点实例
func CreateAndInitNode(targetNodeType string, initConfig types.Configuration, registry *types.SafeComponentSlice) (types.Node, error) {
	var nodeFactory types.Node
	for _, component := range registry.Components() {
		if component.Type() == targetNodeType {
			nodeFactory = component
		}
	}
	node := nodeFactory.New()

	err := node.Init(types.NewConfig(), initConfig)
	return node, err
}

// NodeNew 测试创建节点实例
func NodeNew(t *testing.T, targetNodeType string, targetNode types.Node, defaultConfig types.Configuration, registry *types.SafeComponentSlice) {
	var nodeFactory types.Node
	for _, component := range registry.Components() {
		if component.Type() == targetNode.Type() {
			nodeFactory = component
		}
	}
	assert.NotNil(t, nodeFactory)

	assert.Equal(t, targetNodeType, nodeFactory.Type())

	node := nodeFactory.New()

	assert.True(t, reflect.ValueOf(node).Kind() == reflect.ValueOf(targetNode).Kind())

	componentForm := reflect2.GetComponentForm(node)
	var count = 0
	for k, v := range defaultConfig {
		for _, field := range componentForm.Fields {
			if field.Name == k {
				count++
				assert.Equal(t, field.DefaultValue, v)
				break
			}
		}
	}
	assert.Equal(t, len(defaultConfig), count)

}

// NodeInit 测试初始化
func NodeInit(t *testing.T, targetNodeType string, initConfig types.Configuration, expected types.Configuration, registry *types.SafeComponentSlice) {
	node, err := CreateAndInitNode(targetNodeType, initConfig, registry)
	assert.Nil(t, err)
	componentForm := reflect2.GetComponentForm(node)
	var count = 0
	for k, v := range expected {
		for _, field := range componentForm.Fields {
			if field.Name == k {
				count++
				assert.Equal(t, field.DefaultValue, v)
				break
			}
		}
	}

	assert.Equal(t, len(expected), count)
}

type NodeAndCallback struct {
	Node          types.Node
	MsgList       []Msg
	ChildrenNodes map[string]types.Node
	Callback      func(msg types.RuleMsg, relationType string, err error)
}

type Msg struct {
	MetaData types.Metadata
	DataType types.DataType
	MsgType  string
	Data     string
	//发之后暂停间隔
	AfterSleep time.Duration
}

// NodeOnMsg 发送消息
func NodeOnMsg(t *testing.T, node types.Node, msgList []Msg, callback func(msg types.RuleMsg, relationType string, err error)) {
	NodeOnMsgWithChildren(t, node, msgList, nil, callback)
}

// NodeOnMsgWithChildren 发送消息
func NodeOnMsgWithChildren(t *testing.T, node types.Node, msgList []Msg, childrenNodes map[string]types.Node, callback func(msg types.RuleMsg, relationType string, err error)) {

	//defer node.Destroy()

	ctx := NewRuleContextFull(types.NewConfig(), node, childrenNodes, callback)
	for _, item := range msgList {
		dataType := types.JSON
		if item.DataType != "" {
			dataType = item.DataType
		}
		types.NewMsg(time.Now().UnixMilli(), item.MsgType, dataType, item.MetaData, item.Data)
		msg := ctx.NewMsg(item.MsgType, item.MetaData, item.Data)
		go node.OnMsg(ctx, msg)
		if item.AfterSleep > 0 {
			time.Sleep(item.AfterSleep)
		}
	}
}

// EndpointMessage 测试endpoint请求、响应消息
func EndpointMessage(t *testing.T, m interface{}) {
	message, ok := m.(interface {
		//Body message body
		Body() []byte
		Headers() textproto.MIMEHeader
		From() string
		//GetParam http.Request#FormValue
		GetParam(key string) string
		//SetMsg set RuleMsg
		SetMsg(msg *types.RuleMsg)
		//GetMsg 把接收数据转换成 RuleMsg
		GetMsg() *types.RuleMsg
		//SetStatusCode 响应 code
		SetStatusCode(statusCode int)
		//SetBody 响应 body
		SetBody(body []byte)
		//SetError 设置错误
		SetError(err error)
		//GetError 获取错误
		GetError() error
	})
	assert.True(t, ok)
	if message.Headers() != nil {
		message.Headers().Set(contentType, content)
		assert.Equal(t, content, message.Headers().Get(contentType))
	}

	message.SetBody([]byte("123"))
	assert.Equal(t, "123", string(message.Body()))
	assert.Equal(t, "", message.From())
	assert.Equal(t, "", message.GetParam("aa"))
	if message.GetMsg() != nil {
		assert.Equal(t, "123", message.GetMsg().Data)
	}

	msg := types.NewMsg(int64(1), "aa", types.TEXT, nil, "123")
	message.SetMsg(&msg)
	assert.Equal(t, "aa", message.GetMsg().Type)

	message.SetStatusCode(200)
	message.SetError(errors.New("error"))
	assert.Equal(t, "error", message.GetError().Error())
}

// UpperNode A plugin that converts the message data to uppercase
type UpperNode struct{}

func (n *UpperNode) Type() string {
	return "test/upper"
}
func (n *UpperNode) New() types.Node {
	return &UpperNode{}
}
func (n *UpperNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	// Do some initialization work
	return nil
}

func (n *UpperNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	msg.Data = strings.ToUpper(msg.Data)
	v := ctx.GetContext().Value(shareKey)
	if v != nil {
		msg.Metadata.PutValue(shareKey, v.(string))
	}
	//增加新的共享数据
	modifyCtx := context.WithValue(ctx.GetContext(), addShareKey, addShareValue)
	ctx.SetContext(modifyCtx)
	// Send the modified message to the next node
	ctx.TellSuccess(msg)
}

func (n *UpperNode) Destroy() {
	// Do some cleanup work
}

// TimeNode A plugin that adds a timestamp to the message metadata
type TimeNode struct{}

func (n *TimeNode) Type() string {
	return "test/time"
}

func (n *TimeNode) New() types.Node {
	return &TimeNode{}
}

func (n *TimeNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	// Do some initialization work
	return nil
}

func (n *TimeNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	msg.Metadata.PutValue("timestamp", time.Now().Format(time.RFC3339))
	v1 := ctx.GetContext().Value(shareKey)
	if v1 != nil {
		msg.Metadata.PutValue(shareKey, v1.(string))
	}
	v2 := ctx.GetContext().Value(addShareKey)
	if v2 != nil {
		msg.Metadata.PutValue(addShareKey, v2.(string))
	}
	// Send the modified message to the next node
	ctx.TellSuccess(msg)
}

func (n *TimeNode) Destroy() {
	// Do some cleanup work
}
