/*
 * Copyright 2024 The RuleGo Authors.
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
	"errors"
	"net/textproto"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

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

	msg := types.NewMsg(int64(1), "aa", types.TEXT, types.NewMetadata(), "123")
	message.SetMsg(&msg)
	assert.Equal(t, "aa", message.GetMsg().Type)

	message.SetStatusCode(200)
	message.SetError(errors.New("error"))
	assert.Equal(t, "error", message.GetError().Error())
}
