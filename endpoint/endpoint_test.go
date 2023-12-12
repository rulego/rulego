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

package endpoint

import (
	"context"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/transform"
	"github.com/rulego/rulego/test/assert"
	"net/textproto"
	"os"
	"testing"
)

func TestEndpoint(t *testing.T) {
	buf, err := os.ReadFile("../testdata/chain_call_rest_api.json")
	if err != nil {
		t.Fatal(err)
	}
	config := rulego.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = rulego.New("default", buf, rulego.WithConfig(config))

	exchange := &Exchange{
		In:  &RequestMessage{body: []byte("{\"productName\":\"lala\"}")},
		Out: &ResponseMessage{}}

	t.Run("ExecutorFactory", func(t *testing.T) {
		_, ok := DefaultExecutorFactory.New("chain")
		assert.True(t, ok)
		_, ok = DefaultExecutorFactory.New("component")
		assert.True(t, ok)
	})

	t.Run("ExecuteToComponent", func(t *testing.T) {
		end := false
		router := NewRouter().From("aa").Process(func(router *Router, exchange *Exchange) bool {
			exchange.In.GetMsg().Metadata.PutValue("addValue", "addValueFromProcess")
			return true
		}).Process(func(router *Router, exchange *Exchange) bool {
			assert.Equal(t, "addValueFromProcess", exchange.In.GetMsg().Metadata.GetValue("addValue"))
			return true
		}).ToComponent(func() types.Node {
			//定义日志组件，处理数据
			var configuration = make(types.Configuration)
			configuration["jsScript"] = `
			metadata['name']='test01';
			msg['test']='addFromJs'; 
			return {'msg':msg,'metadata':metadata,'msgType':msgType};
		   `
			node := &transform.JsTransformNode{}
			_ = node.Init(config, configuration)
			return node
		}()).Wait().Process(func(router *Router, exchange *Exchange) bool {
			assert.Equal(t, "{\"productName\":\"lala\",\"test\":\"addFromJs\"}", exchange.Out.GetMsg().Data)
			assert.Equal(t, "addValueFromProcess", exchange.In.GetMsg().Metadata.GetValue("addValue"))
			assert.Equal(t, "test01", exchange.In.GetMsg().Metadata.GetValue("name"))
			end = true
			return true
		}).End()
		//执行路由
		executeRouterTest(router, exchange)
		assert.True(t, end)
	})
	t.Run("ExecuteComponent", func(t *testing.T) {
		end := false
		router := NewRouter()
		router.From("aa").Transform(func(router *Router, exchange *Exchange) bool {
			exchange.In.GetMsg().Metadata.PutValue("addValue", "addValueFromProcess")
			return true
		}).Process(func(router *Router, exchange *Exchange) bool {
			assert.Equal(t, "addValueFromProcess", exchange.In.GetMsg().Metadata.GetValue("addValue"))
			return true
		}).To("component:jsTransform", types.Configuration{
			"jsScript": `
			metadata['name']='test01';
			msg['test']='addFromJs'; 
			return {'msg':msg,'metadata':metadata,'msgType':msgType};
		`,
		}).Process(func(router *Router, exchange *Exchange) bool {
			assert.Equal(t, "{\"productName\":\"lala\",\"test\":\"addFromJs\"}", exchange.Out.GetMsg().Data)
			assert.Equal(t, "addValueFromProcess", exchange.In.GetMsg().Metadata.GetValue("addValue"))
			assert.Equal(t, "test01", exchange.In.GetMsg().Metadata.GetValue("name"))
			end = true
			return true
		})
		//执行路由
		executeRouterTest(router, exchange)
		//异步
		assert.False(t, end)
	})

	t.Run("ExecuteComponentAndWait", func(t *testing.T) {
		end := false
		router := NewRouter()
		router.From("aa").Process(func(router *Router, exchange *Exchange) bool {
			exchange.In.GetMsg().Metadata.PutValue("addValue", "addValueFromProcess")
			return true
		}).Process(func(router *Router, exchange *Exchange) bool {
			assert.Equal(t, "addValueFromProcess", exchange.In.GetMsg().Metadata.GetValue("addValue"))
			return true
		}).To("component:jsTransform", types.Configuration{
			"jsScript": `
			metadata['name']='test01';
			msg['test']='addFromJs'; 
			return {'msg':msg,'metadata':metadata,'msgType':msgType};
		`,
		}).Wait().Process(func(router *Router, exchange *Exchange) bool {
			assert.Equal(t, "{\"productName\":\"lala\",\"test\":\"addFromJs\"}", exchange.Out.GetMsg().Data)
			assert.Equal(t, "addValueFromProcess", exchange.In.GetMsg().Metadata.GetValue("addValue"))
			assert.Equal(t, "test01", exchange.In.GetMsg().Metadata.GetValue("name"))
			end = true
			return true
		})
		//执行路由
		executeRouterTest(router, exchange)
		//同步
		assert.True(t, end)
	})

	t.Run("ExecuteChain", func(t *testing.T) {
		end := false
		router2 := NewRouter()
		router2.From("aa").Process(func(router *Router, exchange *Exchange) bool {
			exchange.In.GetMsg().Metadata.PutValue("addValue", "addValueFromProcess")
			return true
		}).Process(func(router *Router, exchange *Exchange) bool {
			assert.Equal(t, "addValueFromProcess", exchange.In.GetMsg().Metadata.GetValue("addValue"))
			return true
		}).To("chain:default").Process(func(router *Router, exchange *Exchange) bool {
			assert.NotNil(t, exchange.Out.GetError())
			end = true
			return true
		})
		//执行路由
		executeRouterTest(router2, exchange)
		//同步
		assert.False(t, end)
	})

	t.Run("ExecuteChainAndWait", func(t *testing.T) {
		end := false
		router2 := NewRouter()
		router2.From("aa").Process(func(router *Router, exchange *Exchange) bool {
			exchange.In.GetMsg().Metadata.PutValue("addValue", "addValueFromProcess")
			return true
		}).Process(func(router *Router, exchange *Exchange) bool {
			assert.Equal(t, "addValueFromProcess", exchange.In.GetMsg().Metadata.GetValue("addValue"))
			return true
		}).To("chain:default").Wait().Process(func(router *Router, exchange *Exchange) bool {
			assert.NotNil(t, exchange.Out.GetError())
			end = true
			return true
		})
		//执行路由
		executeRouterTest(router2, exchange)
		//同步
		assert.True(t, end)
	})

	t.Run("ExecuteChainVar", func(t *testing.T) {
		router2 := NewRouter()
		router2.From("aa").Process(func(router *Router, exchange *Exchange) bool {
			exchange.In.GetMsg().Metadata.PutValue("chainId", "aa")
			return true
		}).To("chain:${chainId}").Wait().Process(func(router *Router, exchange *Exchange) bool {
			assert.Equal(t, "chainId=aa not found error", exchange.Out.GetError().Error())
			return true
		})
		//执行路由
		executeRouterTest(router2, exchange)
	})

}
func executeRouterTest(router *Router, exchange *Exchange) {
	//执行from端逻辑
	if fromFlow := router.GetFrom(); fromFlow != nil {
		if !fromFlow.ExecuteProcess(router, exchange) {
			return
		}
	}
	//执行to端逻辑
	if router.GetFrom() != nil && router.GetFrom().GetTo() != nil {
		router.GetFrom().GetTo().Execute(context.TODO(), exchange)
	}
}

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

func (r *RequestMessage) From() string {
	return ""
}

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
	body    []byte
	msg     *types.RuleMsg
	headers textproto.MIMEHeader
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

func (r *ResponseMessage) From() string {
	return ""
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

}

func (r *ResponseMessage) SetError(err error) {
	r.err = err
}

func (r *ResponseMessage) GetError() error {
	return r.err
}
