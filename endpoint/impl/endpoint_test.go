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
	"sync/atomic"
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
		assert.Equal(t, 2, len(testEp.Interceptors))
		testEp.DoProcess(context.WithValue(context.TODO(), "baseAdd", "baseValue"), router, exchange)
		//测试from process中断
		var firstDone int32
		var secondDone int32
		testEp = &testEndpoint{}
		testEp.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return true
		})
		router.GetFrom().Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			atomic.StoreInt32(&firstDone, 1)
			return false
		}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			atomic.StoreInt32(&secondDone, 1)
			return false
		})
		testEp.DoProcess(context.WithValue(context.TODO(), "baseAdd", "baseValue"), router, exchange)
		time.Sleep(time.Millisecond * 100)
		assert.True(t, atomic.LoadInt32(&firstDone) == 1)
		assert.False(t, atomic.LoadInt32(&secondDone) == 1)

		//测试to process中断
		atomic.StoreInt32(&firstDone, 0)
		atomic.StoreInt32(&secondDone, 0)
		testEp = &testEndpoint{}
		testEp.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return true
		})
		router = NewRouter(endpoint.RouterOptions.WithRuleConfig(config), endpoint.RouterOptions.WithRuleGo(engine.DefaultPool)).From(from).
			Process(transformFunc).
			To("chain:${chainId}").
			Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
				atomic.StoreInt32(&firstDone, 1)
				return false
			}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			atomic.StoreInt32(&secondDone, 1)
			return false
		}).End()

		testEp.DoProcess(context.Background(), router, exchange)
		time.Sleep(time.Millisecond * 100)
		assert.True(t, atomic.LoadInt32(&firstDone) == 1)
		assert.False(t, atomic.LoadInt32(&secondDone) == 1)
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
		var end int32
		router := NewRouter(endpoint.RouterOptions.WithRuleConfig(config), endpoint.RouterOptions.WithRuleGo(engine.DefaultPool)).From(from).Process(transformFunc).Process(processFunc).ToComponent(func() types.Node {
			node := &transform.JsTransformNode{}
			_ = node.Init(config, configuration)
			return node
		}()).Wait().Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			toProcessFunc(router, exchange)
			atomic.StoreInt32(&end, 1)
			return true
		}).End()
		//执行路由
		executeRouterTest(router, exchange)
		assert.True(t, atomic.LoadInt32(&end) == 1)
	})
	t.Run("ExecuteComponent", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		var end int32
		router := NewRouter()
		router.From(from).
			Transform(transformFunc).
			Process(processFunc).
			To("component:jsTransform", configuration).
			Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
				return true
			}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			toProcessFunc(router, exchange)
			atomic.StoreInt32(&end, 1)
			return true
		}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return false
		})
		//执行路由
		executeRouterTest(router, exchange)
		time.Sleep(time.Millisecond * 200)
		assert.True(t, atomic.LoadInt32(&end) == 1)
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
		var end int32
		router := NewRouter()
		router.From(from).Transform(transformFunc).Process(processFunc).To("component:jsTransform", configuration).Wait().Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			toProcessFunc(router, exchange)
			atomic.StoreInt32(&end, 1)
			return true
		})
		//执行路由
		executeRouterTest(router, exchange)
		//同步
		assert.True(t, atomic.LoadInt32(&end) == 1)
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
		var end int32
		router2 := NewRouter()
		router2.From(from).Transform(transformFunc).Process(processFunc).To(toDefault).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			assert.Nil(t, exchange.Out.GetError())
			atomic.StoreInt32(&end, 1)
			return true
		})
		//执行路由
		executeRouterTest(router2, exchange)
		time.Sleep(time.Millisecond * 100)
		assert.True(t, atomic.LoadInt32(&end) == 1)
	})

	t.Run("ExecuteChainErr", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		errChain := strings.Replace(string(buf), "\"jsScript\": \"return msg=='aa';\"", "\"jsScript\": \"return a;\"", -1)

		//注册规则链
		_, err = engine.New("errChainId", []byte(errChain), engine.WithConfig(config))

		var end int32
		router2 := NewRouter()
		router2.From(from).Transform(transformFunc).Process(processFunc).To("chain:errChainId").
			Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
				assert.NotNil(t, exchange.Out.GetError())
				atomic.StoreInt32(&end, 1)
				return true
			})
		//执行路由
		executeRouterTest(router2, exchange)
		time.Sleep(time.Millisecond * 100)
		assert.True(t, atomic.LoadInt32(&end) == 1)
	})

	t.Run("ExecuteChainFromBroker", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		router2 := NewRouter()
		var done int32
		router2.From(from).Transform(transformFunc).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return false
		}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			atomic.StoreInt32(&done, 1)
			return true
		}).To("nothing:aa").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return true
		})
		//执行路由
		executeRouterTest(router2, exchange)
		time.Sleep(time.Millisecond * 100)
		assert.False(t, atomic.LoadInt32(&done) == 1)
	})
	t.Run("ExecuteChainToBroker", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		router2 := NewRouter()
		var done int32
		router2.From(from).Transform(transformFunc).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return true
		}).To(toAa).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return false
		}).Wait().Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			atomic.StoreInt32(&done, 1)
			return true
		})
		//执行路由
		executeRouterTest(router2, exchange)
		time.Sleep(time.Millisecond * 100)
		assert.False(t, atomic.LoadInt32(&done) == 1)
	})

	t.Run("ExecuteChainAndWait", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		var end int32
		router2 := NewRouter()
		router2.From(from).Transform(transformFunc).Process(processFunc).To(toDefault).Wait().
			Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
				assert.Nil(t, exchange.Out.GetError())
				atomic.StoreInt32(&end, 1)
				return true
			}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			return false
		})
		//执行路由
		executeRouterTest(router2, exchange)
		//同步
		assert.True(t, atomic.LoadInt32(&end) == 1)
	})

	t.Run("ExecuteChainVar", func(t *testing.T) {
		exchange := &endpoint.Exchange{
			In:  &testRequestMessage{body: []byte("{\"productName\":\"lala\"}")},
			Out: &testResponseMessage{}}
		router2 := NewRouter()
		router2.From(from).Process(transformFunc).To("chain:${chainId}").Wait().Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			assert.Equal(t, "chainId=aa not found error: rule chain not found", exchange.Out.GetError().Error())
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
	meta    *types.Metadata
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

// AddHeader adds a header value for testing header mutation pass-through.
func (r *testResponseMessage) AddHeader(key, value string) {
	r.Headers().Add(key, value)
}

// SetHeader sets a header value for testing header mutation pass-through.
func (r *testResponseMessage) SetHeader(key, value string) {
	r.Headers().Set(key, value)
}

// DelHeader removes a header value for testing header mutation pass-through.
func (r *testResponseMessage) DelHeader(key string) {
	r.Headers().Del(key)
}

// GetMetadata returns metadata for testing HeaderModifier pass-through.
func (r *testResponseMessage) GetMetadata() *types.Metadata {
	if r.meta == nil {
		r.meta = types.NewMetadata()
	}
	return r.meta
}

// TestScopedMessageHeaderModifierPassThrough verifies ScopedMessage preserves HeaderModifier behavior.
func TestScopedMessageHeaderModifierPassThrough(t *testing.T) {
	out := &testResponseMessage{}
	scoped := &ScopedMessage{
		Message: out,
	}

	modifier, ok := any(scoped).(endpoint.HeaderModifier)
	assert.True(t, ok)

	modifier.SetHeader("Content-Type", "text/event-stream")
	modifier.AddHeader("X-Test", "a")
	modifier.AddHeader("X-Test", "b")

	assert.Equal(t, "text/event-stream", out.Headers().Get("Content-Type"))
	assert.Equal(t, "a", out.Headers().Values("X-Test")[0])
	assert.Equal(t, "b", out.Headers().Values("X-Test")[1])

	meta := modifier.GetMetadata()
	meta.PutValue("stream", "true")
	assert.Equal(t, "true", out.GetMetadata().GetValue("stream"))

	modifier.DelHeader("Content-Type")
	assert.Equal(t, "", out.Headers().Get("Content-Type"))
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

// To 缺 ":path" 后缀时应报错而非切片越界 panic。
func TestToMissingPath(t *testing.T) {
	router := NewRouter().From("test/missing-path").To("chain").End()
	if router.Err() == nil {
		t.Error("To without executor path suffix should set router error")
	}
	router2 := NewRouter().From("test/missing-path").To("chain:abc").End()
	if router2.Err() != nil {
		t.Errorf("To with path should not error: %v", router2.Err())
	}
}

// From/To accessors: configuration, wait flag, opts and variable resolution.
func TestFromToAccessors(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	router := NewRouter(endpoint.RouterOptions.WithRuleConfig(config)).
		From("/from", types.Configuration{"k": "v"}).
		To("chain:${chainId}").
		Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool { return true }).
		End()

	from := router.GetFrom()
	assert.Equal(t, "v", from.GetConfiguration()["k"])

	// GetTo returns nil before To is configured.
	empty := NewRouter().From("/x").End()
	assert.Nil(t, empty.GetFrom().GetTo())

	to := from.GetTo().(*To)
	assert.True(t, to.HasVars)
	assert.Equal(t, "${chainId}", to.ToString())
	assert.Equal(t, "aa", to.ToStringByDict(map[string]string{"chainId": "aa"}))
	// Without vars ToStringByDict returns the path unchanged.
	plain := NewRouter().From("/x").To("chain:abc").End()
	assert.Equal(t, "abc", plain.GetFrom().GetTo().(*To).ToStringByDict(map[string]string{"chainId": "zz"}))

	assert.False(t, to.IsWait())
	to.SetWait(true)
	assert.True(t, to.IsWait())
	to.Wait()
	assert.True(t, to.IsWait())

	opt := types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {})
	assert.Equal(t, 0, len(to.GetOpts()))
	to.SetOpts(opt)
	assert.Equal(t, 1, len(to.GetOpts()))
}

// Router accessors: id, definition, params and dynamic rule engine pool.
func TestRouterAccessors(t *testing.T) {
	// Same defaults as NewRouter: DefaultPool for the rule engine.
	router := &Router{RuleGo: engine.DefaultPool}

	// No From configured yet.
	assert.Equal(t, "", router.FromToString())
	assert.Nil(t, router.GetFrom())

	router.From("/f")
	assert.Equal(t, "/f", router.FromToString())

	router.SetId("rid")
	assert.Equal(t, "rid", router.GetId())

	assert.Nil(t, router.Definition())
	def := &types.RouterDsl{Id: "rid"}
	router.SetDefinition(def)
	assert.Equal(t, def, router.Definition())

	assert.Equal(t, 0, len(router.GetParams()))
	router.SetParams("GET", 1)
	params := router.GetParams()
	assert.Equal(t, "GET", params[0])
	assert.Equal(t, 1, params[1])

	// GetRuleGo prefers the dynamic pool function when set.
	otherPool := engine.NewPool()
	assert.Equal(t, engine.DefaultPool, router.GetRuleGo(nil))
	router.SetRuleEnginePoolFunc(func(exchange *endpoint.Exchange) types.RuleEnginePool {
		return otherPool
	})
	assert.Equal(t, otherPool, router.GetRuleGo(nil))
	router.SetRuleEnginePoolFunc(nil)
	router.SetRuleEnginePool(otherPool)
	assert.Equal(t, otherPool, router.GetRuleGo(nil))
}

// BaseEndpoint utilities: event callback, log helpers, router id fallback,
// destroy reset and rule chain definition extraction.
func TestBaseEndpointUtilities(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	testEp := &testEndpoint{}

	// Log helpers are safe with a nil logger and forward to a configured one.
	testEp.Debugf("debug %s", "msg")
	testEp.Infof("info %s", "msg")
	testEp.Warnf("warn %s", "msg")
	testEp.Errorf("error %s", "msg")
	testEp.Logger = config.Logger
	testEp.Debugf("debug %s", "msg")
	testEp.Infof("info %s", "msg")
	testEp.Warnf("warn %s", "msg")
	testEp.Errorf("error %s", "msg")

	var fired string
	testEp.SetOnEvent(func(event string, params ...interface{}) {
		fired = event
	})
	testEp.OnEvent("evt")
	assert.Equal(t, "evt", fired)

	// Empty router id falls back to the from path.
	r1 := NewRouter().From("/a/b").End()
	assert.Equal(t, "/a/b", testEp.CheckAndSetRouterId(r1))
	r2 := NewRouter().SetId("keep").From("/a/b").End()
	assert.Equal(t, "keep", testEp.CheckAndSetRouterId(r2))

	// HasRouter reflects RouterStorage contents.
	router := NewRouter().SetId("in-store").From("/x").End()
	testEp.RouterStorage = map[string]endpoint.Router{"in-store": router}
	assert.True(t, testEp.HasRouter("in-store"))
	assert.False(t, testEp.HasRouter("absent"))

	// Destroy resets interceptors and router storage.
	testEp.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool { return true })
	testEp.BaseEndpoint.Destroy()
	assert.Equal(t, 0, len(testEp.Interceptors))
	assert.Equal(t, 0, len(testEp.RouterStorage))

	// GetRuleChainDefinition extracts only *types.RuleChain values.
	chainDef := &types.RuleChain{}
	configuration := types.Configuration{
		types.NodeConfigurationKeyRuleChainDefinition: chainDef,
		"wrongType": "not a chain",
	}
	assert.Equal(t, chainDef, testEp.GetRuleChainDefinition(configuration))
	assert.Nil(t, testEp.GetRuleChainDefinition(types.Configuration{
		types.NodeConfigurationKeyRuleChainDefinition: "wrong type",
	}))
	assert.Nil(t, testEp.GetRuleChainDefinition(types.Configuration{}))
}

// stagedCancelCtx reports itself cancelled only once Done() has been called
// cancelAt times, so tests can pin which cancellation gate in DoProcess fires.
type stagedCancelCtx struct {
	calls    int
	cancelAt int
	done     chan struct{}
}

func (c *stagedCancelCtx) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *stagedCancelCtx) Done() <-chan struct{} {
	c.calls++
	if c.calls >= c.cancelAt {
		return c.done
	}
	return nil
}
func (c *stagedCancelCtx) Err() error {
	if c.calls >= c.cancelAt {
		return context.Canceled
	}
	return nil
}
func (c *stagedCancelCtx) Value(key interface{}) interface{} { return nil }

// DoProcess aborts at each cancellation gate with a distinct error message.
func TestDoProcessCancellationGates(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	newExchange := func() *endpoint.Exchange {
		return &endpoint.Exchange{In: &testRequestMessage{body: []byte("{}")}, Out: &testResponseMessage{}}
	}

	// Cancelled base context aborts before anything runs.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	router := NewRouter(endpoint.RouterOptions.WithRuleConfig(config)).From("/x").To("chain:default").End()
	exchange := newExchange()
	(&testEndpoint{}).DoProcess(ctx, router, exchange)
	assert.NotNil(t, exchange.Out.GetError())
	assert.True(t, strings.Contains(exchange.Out.GetError().Error(), "processing cancelled"))

	cases := []struct {
		name     string
		cancelAt int
		wantMsg  string
	}{
		// Done() call order: interceptor gate, before-From gate, before-To gate.
		{"duringInterceptor", 1, "cancelled during interceptor"},
		{"beforeFrom", 1, "cancelled before From"},
		{"beforeTo", 2, "cancelled before To"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The done channel must be closed so select sees it as cancelled.
			done := make(chan struct{})
			close(done)
			var staged context.Context = &stagedCancelCtx{cancelAt: tc.cancelAt, done: done}
			router := NewRouter(
				endpoint.RouterOptions.WithRuleConfig(config),
				endpoint.RouterOptions.WithContextFunc(func(ctx context.Context, exchange *endpoint.Exchange) context.Context {
					return staged
				})).
				From("/x").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
				return true
			}).To("chain:default").End()
			if tc.name == "beforeFrom" || tc.name == "beforeTo" {
				// No global interceptors: the first Done() call is the before-From gate.
				exchange := newExchange()
				(&testEndpoint{}).DoProcess(context.Background(), router, exchange)
				assert.NotNil(t, exchange.Out.GetError())
				assert.True(t, strings.Contains(exchange.Out.GetError().Error(), tc.wantMsg))
				return
			}
			testEp := &testEndpoint{}
			testEp.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
				return true
			})
			exchange := newExchange()
			testEp.DoProcess(context.Background(), router, exchange)
			assert.NotNil(t, exchange.Out.GetError())
			assert.True(t, strings.Contains(exchange.Out.GetError().Error(), tc.wantMsg))
		})
	}
}

// A ContextFunc returning nil must fall back to context.Background().
func TestDoProcessContextFuncNilResult(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	router := NewRouter(
		endpoint.RouterOptions.WithRuleConfig(config),
		endpoint.RouterOptions.WithContextFunc(func(ctx context.Context, exchange *endpoint.Exchange) context.Context {
			return nil
		})).
		From("/x").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		return true
	}).End()
	exchange := &endpoint.Exchange{In: &testRequestMessage{body: []byte("{}")}, Out: &testResponseMessage{}}
	(&testEndpoint{}).DoProcess(context.Background(), router, exchange)
	assert.Nil(t, exchange.Out.GetError())
}

// ScopedMessage keeps msg scope-local while delegating IO to the wrapped message.
func TestScopedMessageDelegation(t *testing.T) {
	out := &testResponseMessage{body: []byte("orig")}
	local := types.NewMsg(0, "from", types.TEXT, types.NewMetadata(), "scoped-data")
	scoped := &ScopedMessage{Message: out, msg: &local}

	assert.Equal(t, "scoped-data", scoped.GetMsg().GetData())
	next := types.NewMsg(0, "from2", types.TEXT, types.NewMetadata(), "next")
	scoped.SetMsg(&next)
	assert.Equal(t, "next", scoped.GetMsg().GetData())
	assert.Nil(t, out.GetMsg()) // underlying message untouched

	assert.Equal(t, "orig", string(scoped.Body()))
	scoped.SetBody([]byte("written"))
	assert.Equal(t, "written", string(out.Body()))
	assert.NotNil(t, scoped.Headers())
	assert.Equal(t, "", scoped.From())
	assert.Equal(t, "", scoped.GetParam("k"))
	scoped.SetStatusCode(200)
	assert.Nil(t, scoped.GetError())
	scoped.SetError(fmt.Errorf("boom"))
	assert.Equal(t, "boom", scoped.GetError().Error())
	// testResponseMessage implements HeaderModifier.
	assert.NotNil(t, scoped.GetMetadata())
	// testResponseMessage has no Response/Flush support: both degrade to no-ops.
	assert.Nil(t, scoped.Response())
	scoped.Flush()
}

// Header mutation on a wrapped message without HeaderModifier falls back to
// the raw header map; GetMetadata then returns nil.
func TestScopedMessageHeaderFallback(t *testing.T) {
	plain := &testRequestMessage{}
	scoped := &ScopedMessage{Message: plain}

	scoped.AddHeader("X-Add", "a")
	scoped.SetHeader("X-Set", "b")
	assert.Equal(t, "a", plain.Headers().Get("X-Add"))
	assert.Equal(t, "b", plain.Headers().Get("X-Set"))
	scoped.DelHeader("X-Add")
	assert.Equal(t, "", plain.Headers().Get("X-Add"))
	assert.Nil(t, scoped.GetMetadata())
}

// Executors tolerate routers without a configured From/To.
func TestExecutorNilFromAndTo(t *testing.T) {
	exchange := &endpoint.Exchange{
		In:  &testRequestMessage{body: []byte("{}")},
		Out: &testResponseMessage{},
	}
	config := engine.NewConfig(types.WithDefaultPool())

	chainExecutor, _ := DefaultExecutorFactory.New("chain")
	routerNoTo := NewRouter(endpoint.RouterOptions.WithRuleConfig(config)).From("/x").End()
	chainExecutor.Execute(context.TODO(), routerNoTo, exchange)
	assert.Nil(t, exchange.Out.GetError())

	componentExecutor, _ := DefaultExecutorFactory.New("component")
	routerNoFrom := NewRouter(endpoint.RouterOptions.WithRuleConfig(config))
	componentExecutor.Execute(context.TODO(), routerNoFrom, exchange)
	assert.Nil(t, exchange.Out.GetError())
}
