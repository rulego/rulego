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
)

// mockEndpointMessage implements the anonymous interface EndpointMessage expects.
type mockEndpointMessage struct {
	headers    textproto.MIMEHeader
	msg        *types.RuleMsg
	body       []byte
	statusCode int
	err        error
}

func (m *mockEndpointMessage) Body() []byte                  { return m.body }
func (m *mockEndpointMessage) Headers() textproto.MIMEHeader { return m.headers }
func (m *mockEndpointMessage) From() string                  { return "" }
func (m *mockEndpointMessage) GetParam(string) string        { return "" }
func (m *mockEndpointMessage) SetMsg(msg *types.RuleMsg)     { m.msg = msg }
func (m *mockEndpointMessage) GetMsg() *types.RuleMsg        { return m.msg }
func (m *mockEndpointMessage) SetStatusCode(code int)        { m.statusCode = code }
func (m *mockEndpointMessage) SetBody(body []byte)           { m.body = body }
func (m *mockEndpointMessage) SetError(err error)            { m.err = err }
func (m *mockEndpointMessage) GetError() error               { return m.err }

// msgCarryingMessage returns a pre-populated RuleMsg to cover the GetMsg data check.
type msgCarryingMessage struct {
	mockEndpointMessage
}

func TestEndpointMessage(t *testing.T) {
	// with headers and a pre-set msg
	withMsg := &msgCarryingMessage{}
	withMsg.headers = textproto.MIMEHeader{}
	preSet := types.NewMsg(0, "pre", types.TEXT, types.NewMetadata(), "123")
	withMsg.msg = &preSet
	withMsg.body = []byte("123")
	withMsg.err = errors.New("error")
	EndpointMessage(t, withMsg)

	// nil headers and nil msg: only the nil-safe paths run
	EndpointMessage(t, &mockEndpointMessage{err: errors.New("error")})
}
