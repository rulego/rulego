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

package impl

import (
	"context"
	"fmt"
	"net/textproto"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/components/transform"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test/assert"
)

func TestEndpoint(t *testing.T) {
	buf, err := os.ReadFile("../../testdata/rule/sub_chain.json")
	if err != nil {
		t.Fatal(err)
	}
	config := engine.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = engine.New("default", buf, engine.WithConfig(config))

	var from = "aa"
	var toAa = "chain:aa"
	var toDefault = "chain:default"
	var transformFunc = func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.In.GetMsg().Metadata.PutValue("addValue", "addValueFromProcess")
		exchange.In.GetMsg().Metadata.PutValue("chainId", "aa")
		return true
	}
	var processFunc = func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		assert.Equal(t, "addValueFromProcess", exchange.In.GetMsg().Metadata.GetValue("addValue"))
		return true
	}
	var toProcessFunc = func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		assert.Equal(t, "{\"productName\":\"lala\",\"test\":\"addFromJs\"}", exchange.Out.GetMsg().GetData())
		assert.Equal(t, "addValueFromProcess", exchange.In.GetMsg().Metadata.GetValue("addValue"))
		assert.Equal(t, "test01", exchange.In.GetMsg().Metadata.GetValue("name"))
		return true
	}
	jsScript := `
			metadata['name']='test01';
			msg['test']='addFromJs'; 
			return {'msg':msg,'metadata':metadata,'msgType':msgType};
	`
	configuration := types.Configuration{
		"jsScript": jsScript,
	}

	t.Run("ExecutorFactory", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		executor, ok := DefaultExecutorFactory.New("chain")
		assert.True(t, ok)
		assert.True(t, executor.IsPathSupportVar())

		router := Router{}
		executor.Execute(context.TODO(), &router, exchange)

		executor, ok = DefaultExecutorFactory.New("component")
		assert.True(t, ok)
		assert.False(t, executor.IsPathSupportVar())
		err = executor.Init(config, types.Configuration{pathKey: "log"})
		assert.Nil(t, err)
		executor.Execute(context.TODO(), &router, exchange)

		//not nodeType
		err = executor.Init(config, nil)
		assert.Equal(t, "nodeType can't empty", err.Error())

		_, ok = DefaultExecutorFactory.New("nothing")
		assert.False(t, ok)

	})

	//测试新建路由
	t.Run("NewRouter", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}

		router := NewRouter(endpoint.RouterOptions.WithRuleConfig(config), endpoint.RouterOptions.WithRuleGo(engine.DefaultPool)).
			From(from, configuration).End()
		assert.NotNil(t, router)

		router = NewRouter(endpoint.RouterOptions.WithRuleConfig(config), endpoint.RouterOptions.WithRuleGo(engine.DefaultPool))
		assert.Equal(t, "", router.FromToString())

		router = NewRouter(endpoint.RouterOptions.WithRuleConfig(config), endpoint.RouterOptions.WithRuleGo(engine.DefaultPool)).
			From(from, configuration).
			Process(transformFunc).
			To(toDefault).
			Process(processFunc).End()
		assert.Equal(t, from, router.FromToString())
		assert.Equal(t, from, router.GetFrom().ToString())
		assert.Equal(t, "default", router.GetFrom().GetTo().ToString())
		assert.Equal(t, "default", router.GetFrom().GetTo().ToStringByDict(map[string]string{
			"chainId": "default",
		}))
		assert.Equal(t, 1, len(router.GetFrom().GetProcessList()))
		assert.Equal(t, 1, len(router.GetFrom().GetTo().GetProcessList()))

		router.Disable(true)
		assert.True(t, router.IsDisable())

		router.Disable(false)
		assert.False(t, router.IsDisable())

		router = NewRouter(endpoint.RouterOptions.WithRuleConfig(config), endpoint.RouterOptions.WithContextFunc(func(ctx context.Context, exchange *endpoint.Exchange) context.Context {
			return context.WithValue(ctx, "addValue", "default")
		}), endpoint.RouterOptions.WithRuleGo(engine.DefaultPool)).From(from).
			Process(transformFunc).
			Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
				assert.Equal(t, "default", exchange.Context.Value("addValue"))
				assert.Equal(t, "baseValue", exchange.Context.Value("baseAdd"))
				return true
			}).
			To("chain:${chainId}").
			Process(processFunc).
			Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
				return true
			}).End()
		assert.Equal(t, from, router.GetFrom().ToString())
		assert.Equal(t, "${chainId}", router.GetFrom().GetTo().ToString())
		assert.Equal(t, "default", router.GetFrom().GetTo().ToStringByDict(map[string]string{
			"chainId": "default",
		}))
		assert.Equal(t, 2, len(router.GetFrom().GetProcessList()))
		assert.Equal(t, 2, len(router.GetFrom().GetTo().GetProcessList()))
		testEp := &testEndpoint{}
		testEp.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return true
		}, func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return false
		})
		assert.Equal(t, 2, len(testEp.interceptors))
		testEp.DoProcess(context.WithValue(context.TODO(), "baseAdd", "baseValue"), router, exchange)
		//测试from process中断
		var firstDone = false
		var secondDone = false
		testEp = &testEndpoint{}
		testEp.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return true
		})
		router.GetFrom().Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			firstDone = true
			return false
		}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			secondDone = true
			return false
		})
		testEp.DoProcess(context.WithValue(context.TODO(), "baseAdd", "baseValue"), router, exchange)
		time.Sleep(time.Millisecond * 100)
		assert.True(t, firstDone)
		assert.False(t, secondDone)

		//测试to process中断
		firstDone = false
		secondDone = false
		testEp = &testEndpoint{}
		testEp.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return true
		})
		router = NewRouter(endpoint.RouterOptions.WithRuleConfig(config), endpoint.RouterOptions.WithRuleGo(engine.DefaultPool)).From(from).
			Process(transformFunc).
			To("chain:${chainId}").
			Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
				firstDone = true
				return false
			}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			secondDone = true
			return false
		}).End()

		testEp.DoProcess(context.Background(), router, exchange)
		time.Sleep(time.Millisecond * 100)
		assert.True(t, firstDone)
		assert.False(t, secondDone)
	})

	t.Run("EndpointOnMsg", func(t *testing.T) {
		defer func() {
			if caught := recover(); caught != nil {
				assert.Equal(t, "not support this method", fmt.Sprintf("%s", caught))
			}
		}()
		testEp := &testEndpoint{}
		testEp.OnMsg(nil, types.RuleMsg{})
	})

	t.Run("ExecuteToComponent", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		end := false
		router := NewRouter(endpoint.RouterOptions.WithRuleConfig(config), endpoint.RouterOptions.WithRuleGo(engine.DefaultPool)).From(from).Process(transformFunc).Process(processFunc).ToComponent(func() types.Node {
			node := &transform.JsTransformNode{}
			_ = node.Init(config, configuration)
			return node
		}()).Wait().Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			toProcessFunc(router, exchange)
			end = true
			return true
		}).End()
		//执行路由
		executeRouterTest(router, exchange)
		assert.True(t, end)
	})
	t.Run("ExecuteComponent", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		end := false
		router := NewRouter()
		router.From(from).
			Transform(transformFunc).
			Process(processFunc).
			To("component:jsTransform", configuration).
			Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
				return true
			}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			toProcessFunc(router, exchange)
			end = true
			return true
		}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return false
		})
		//执行路由
		executeRouterTest(router, exchange)
		//异步
		assert.False(t, end)
	})

	t.Run("ExecuteComponentVar", func(t *testing.T) {
		router := NewRouter().From(from).To("component:${componentType}", configuration).End()
		assert.Equal(t, "executor=component, path not support variables", router.Err().Error())
	})

	//测试组件不存在
	t.Run("ExecuteComponentNotFount", func(t *testing.T) {
		router := NewRouter().From(from).To("component:aa", configuration).End()
		assert.Equal(t, "component not found. componentType=aa", router.Err().Error())
	})

	t.Run("ExecuteComponentAndWait", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		end := false
		router := NewRouter()
		router.From(from).Transform(transformFunc).Process(processFunc).To("component:jsTransform", configuration).Wait().Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			toProcessFunc(router, exchange)
			end = true
			return true
		})
		//执行路由
		executeRouterTest(router, exchange)
		//同步
		assert.True(t, end)
	})

	t.Run("ExecuteComponentErr", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		router := NewRouter().From(from).
			To("component:jsTransform", types.Configuration{
				"jsScript": "return a",
			}).
			Wait().
			Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
				assert.NotNil(t, exchange.Out.GetError())
				return true
			}).End()

		executeRouterTest(router, exchange)
	})

	t.Run("ExecuteChain", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		end := false
		router2 := NewRouter()
		router2.From(from).Transform(transformFunc).Process(processFunc).To(toDefault).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			assert.Nil(t, exchange.Out.GetError())
			end = true
			return true
		})
		//执行路由
		executeRouterTest(router2, exchange)
		//异步
		assert.False(t, end)
		time.Sleep(time.Millisecond * 200)
	})

	t.Run("ExecuteChainErr", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		errChain := strings.Replace(string(buf), "\"jsScript\": \"return msg=='aa';\"", "\"jsScript\": \"return a;\"", -1)

		//注册规则链
		_, err = engine.New("errChainId", []byte(errChain), engine.WithConfig(config))

		end := false
		router2 := NewRouter()
		router2.From(from).Transform(transformFunc).Process(processFunc).To("chain:errChainId").
			Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
				assert.NotNil(t, exchange.Out.GetError())
				end = true
				return true
			})
		//执行路由
		executeRouterTest(router2, exchange)
		//异步
		assert.False(t, end)
		time.Sleep(time.Millisecond * 100)
	})

	t.Run("ExecuteChainFromBroker", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		router2 := NewRouter()
		var done = false
		router2.From(from).Transform(transformFunc).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return false
		}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			done = true
			return true
		}).To("nothing:aa").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return true
		})
		//执行路由
		executeRouterTest(router2, exchange)
		//异步
		assert.False(t, done)
		time.Sleep(time.Millisecond * 100)
	})
	t.Run("ExecuteChainToBroker", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		router2 := NewRouter()
		var done = false
		router2.From(from).Transform(transformFunc).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return true
		}).To(toAa).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return false
		}).Wait().Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			done = true
			return true
		})
		//执行路由
		executeRouterTest(router2, exchange)
		//同步
		assert.False(t, done)
	})

	t.Run("ExecuteChainAndWait", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		end := false
		router2 := NewRouter()
		router2.From(from).Transform(transformFunc).Process(processFunc).To(toDefault).Wait().
			Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
				assert.Nil(t, exchange.Out.GetError())
				end = true
				return true
			}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return false
		})
		//执行路由
		executeRouterTest(router2, exchange)
		//同步
		assert.True(t, end)
	})

	t.Run("ExecuteChainVar", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		router2 := NewRouter()
		router2.From(from).Process(transformFunc).To("chain:${chainId}").Wait().Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			assert.Equal(t, "chainId=aa not found error", exchange.Out.GetError().Error())
			return true
		})
		//执行路由
		executeRouterTest(router2, exchange)
	})

	t.Run("DoProcessContextIsNil", func(t *testing.T) {
		defer func() {
			if caught := recover(); caught != nil {
				assert.Equal(t, "ContextFunc returned nil", fmt.Sprintf("%s", caught))
			}
		}()
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		router := NewRouter(endpoint.RouterOptions.WithRuleConfig(config), endpoint.RouterOptions.WithContextFunc(func(ctx context.Context, exchange *endpoint.Exchange) context.Context {
			return ctx
		}), endpoint.RouterOptions.WithRuleGo(engine.DefaultPool)).From(from).
			Process(transformFunc).
			To("chain:${chainId}").
			Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
				return false
			}).End()
		testEp := &testEndpoint{}
		testEp.DoProcess(nil, router, exchange)
	})

}

func executeRouterTest(router endpoint.Router, exchange *endpoint.Exchange) {
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

// testRequestMessage 请求消息
type testRequestMessage struct {
	headers textproto.MIMEHeader
	body    []byte
	msg     *types.RuleMsg
	err     error
}

func (r *testRequestMessage) Body() []byte {
	return r.body
}
func (r *testRequestMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	return r.headers
}

func (r *testRequestMessage) From() string {
	return ""
}

func (r *testRequestMessage) GetParam(key string) string {
	return ""
}

func (r *testRequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}

func (r *testRequestMessage) GetMsg() *types.RuleMsg {
	if r.msg == nil {
		ruleMsg := types.NewMsg(0, r.From(), types.JSON, types.NewMetadata(), string(r.Body()))
		r.msg = &ruleMsg
	}
	return r.msg
}

func (r *testRequestMessage) SetStatusCode(statusCode int) {
}

func (r *testRequestMessage) SetBody(body []byte) {
	r.body = body
}

func (r *testRequestMessage) SetError(err error) {
	r.err = err
}

func (r *testRequestMessage) GetError() error {
	return r.err
}

// testResponseMessage 响应消息
type testResponseMessage struct {
	body    []byte
	msg     *types.RuleMsg
	headers textproto.MIMEHeader
	err     error
}

func (r *testResponseMessage) Body() []byte {
	return r.body
}

func (r *testResponseMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	return r.headers
}

func (r *testResponseMessage) From() string {
	return ""
}

func (r *testResponseMessage) GetParam(key string) string {
	return ""
}

func (r *testResponseMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}
func (r *testResponseMessage) GetMsg() *types.RuleMsg {
	return r.msg
}

func (r *testResponseMessage) SetStatusCode(statusCode int) {
}

func (r *testResponseMessage) SetBody(body []byte) {
	r.body = body

}

func (r *testResponseMessage) SetError(err error) {
	r.err = err
}

func (r *testResponseMessage) GetError() error {
	return r.err
}

// 测试endpoint
type testEndpoint struct {
	BaseEndpoint
	configuration types.Configuration
}

// Type 组件类型
func (test *testEndpoint) Type() string {
	return "test"
}

func (test *testEndpoint) New() types.Node {
	return &testEndpoint{}
}

// Init 初始化
func (test *testEndpoint) Init(ruleConfig types.Config, configuration types.Configuration) error {
	test.configuration = configuration
	return nil
}

// Destroy 销毁
func (test *testEndpoint) Destroy() {
	_ = test.Close()
}

func (test *testEndpoint) Close() error {
	return nil
}

func (test *testEndpoint) Id() string {
	return "id"
}

func (test *testEndpoint) AddRouter(router endpoint.Router, params ...interface{}) (string, error) {
	//返回任务ID，用于清除任务
	return "1", nil
}

func (test *testEndpoint) RemoveRouter(routeId string, params ...interface{}) error {
	return nil
}

func (test *testEndpoint) Start() error {
	return nil
}
