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

package endpoint

import (
	"context"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

// mockOptionsSetter 记录 RouterOption 会调用的 setter，其余由内嵌零值兜底。
type mockOptionsSetter struct {
	OptionsSetter
	config types.Config
	pool   types.RuleEnginePool
	poolFn func(exchange *Exchange) types.RuleEnginePool
	ctxFn  func(ctx context.Context, exchange *Exchange) context.Context
	def    *types.RouterDsl
}

func (m *mockOptionsSetter) SetConfig(config types.Config) { m.config = config }
func (m *mockOptionsSetter) SetRuleEnginePool(pool types.RuleEnginePool) {
	m.pool = pool
}
func (m *mockOptionsSetter) SetRuleEnginePoolFunc(f func(exchange *Exchange) types.RuleEnginePool) {
	m.poolFn = f
}
func (m *mockOptionsSetter) SetContextFunc(f func(ctx context.Context, exchange *Exchange) context.Context) {
	m.ctxFn = f
}
func (m *mockOptionsSetter) SetDefinition(dsl *types.RouterDsl) { m.def = dsl }

type stubEnginePool struct{ types.RuleEnginePool }

// TestRouterOptions 各 RouterOption 对 OptionsSetter 的副作用
func TestRouterOptions(t *testing.T) {
	pool := &stubEnginePool{}
	poolFn := func(exchange *Exchange) types.RuleEnginePool { return nil }
	ctxFn := func(ctx context.Context, exchange *Exchange) context.Context { return ctx }
	def := &types.RouterDsl{Id: "r1"}
	config := types.NewConfig()

	m := &mockOptionsSetter{}
	assert.Nil(t, RouterOptions.WithRuleGoFunc(poolFn)(m))
	assert.Nil(t, RouterOptions.WithRuleGo(pool)(m))
	assert.Nil(t, RouterOptions.WithRuleConfig(config)(m))
	assert.Nil(t, RouterOptions.WithContextFunc(ctxFn)(m))
	assert.Nil(t, RouterOptions.WithDefinition(def)(m))

	assert.NotNil(t, m.poolFn)
	assert.Equal(t, pool, m.pool)
	assert.Equal(t, int64(0), config.MsgMaxHops)
	assert.Equal(t, config, m.config)
	assert.NotNil(t, m.ctxFn)
	assert.Equal(t, def, m.def)
}

// mockDynamicEndpoint 记录 DynamicEndpointOption 会调用的 setter，其余由内嵌零值兜底。
type mockDynamicEndpoint struct {
	DynamicEndpoint
	id         string
	config     types.Config
	routerOpts []RouterOption
	onEvent    OnEvent
	restart    bool
	chainCtx   types.ChainCtx
	deferred   bool
	intercepts []Process
	ruleChain  *types.RuleChain
}

func (m *mockDynamicEndpoint) SetId(id string)                         { m.id = id }
func (m *mockDynamicEndpoint) SetConfig(config types.Config)           { m.config = config }
func (m *mockDynamicEndpoint) SetRouterOptions(opts ...RouterOption)   { m.routerOpts = opts }
func (m *mockDynamicEndpoint) SetOnEvent(onEvent OnEvent)              { m.onEvent = onEvent }
func (m *mockDynamicEndpoint) SetRestart(restart bool)                 { m.restart = restart }
func (m *mockDynamicEndpoint) SetChainCtx(chainCtx types.ChainCtx)     { m.chainCtx = chainCtx }
func (m *mockDynamicEndpoint) SetDeferredRouters(deferred bool)        { m.deferred = deferred }
func (m *mockDynamicEndpoint) SetInterceptors(interceptors ...Process) { m.intercepts = interceptors }
func (m *mockDynamicEndpoint) SetRuleChain(ruleChain *types.RuleChain) { m.ruleChain = ruleChain }

// TestDynamicEndpointOptions 各 DynamicEndpointOption 对端点的副作用
func TestDynamicEndpointOptions(t *testing.T) {
	onEvent := func(eventName string, params ...interface{}) {}
	interceptor := func(router Router, exchange *Exchange) bool { return true }
	ruleChain := &types.RuleChain{}
	config := types.NewConfig()

	m := &mockDynamicEndpoint{}
	assert.Nil(t, DynamicEndpointOptions.WithId("ep-1")(m))
	assert.Nil(t, DynamicEndpointOptions.WithConfig(config)(m))
	assert.Nil(t, DynamicEndpointOptions.WithRouterOpts(RouterOptions.WithRuleGo(&stubEnginePool{}))(m))
	assert.Nil(t, DynamicEndpointOptions.WithOnEvent(onEvent)(m))
	assert.Nil(t, DynamicEndpointOptions.WithRestart(true)(m))
	assert.Nil(t, DynamicEndpointOptions.WithChainCtx(nil)(m))
	assert.Nil(t, DynamicEndpointOptions.WithDeferredRouters(true)(m))
	assert.Nil(t, DynamicEndpointOptions.WithInterceptors(interceptor)(m))
	assert.Nil(t, DynamicEndpointOptions.WithRuleChain(ruleChain)(m))

	assert.Equal(t, "ep-1", m.id)
	assert.Equal(t, config, m.config)
	assert.Equal(t, 1, len(m.routerOpts))
	assert.NotNil(t, m.onEvent)
	assert.True(t, m.restart)
	assert.Nil(t, m.chainCtx)
	assert.True(t, m.deferred)
	assert.Equal(t, 1, len(m.intercepts))
	assert.Equal(t, ruleChain, m.ruleChain)
}
