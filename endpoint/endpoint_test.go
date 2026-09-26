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
	"errors"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
)

var testEndpointsFolder = "../testdata/endpoint"
var testRulesFolder = "../testdata/rule"

func TestDynamicEndpoint(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		//assert.Equal(t, "ok", msg.Data)
	})
	msg1 := ctx.NewMsg("TEST_MSG_TYPE_AA", types.NewMetadata(), "{\"name\":\"lala\"}")

	endpointBuf, err := os.ReadFile(testEndpointsFolder + "/http_01.json")
	if err != nil {
		t.Fatal(err)
	}
	endpointStr := strings.Replace(string(endpointBuf), "9090", "9081", -1)

	ruleDsl, err := os.ReadFile(testRulesFolder + "/filter_node.json")

	_, err = engine.New("test01", ruleDsl)
	if err != nil {
		t.Fatal(err)
	}

	ep, err := NewFromDsl([]byte(endpointStr), endpoint.DynamicEndpointOptions.WithConfig(config),
		endpoint.DynamicEndpointOptions.WithRouterOpts(endpoint.RouterOptions.WithContextFunc(func(ctx context.Context, exchange *endpoint.Exchange) context.Context {
			return context.Background()
		})))

	if err != nil {
		t.Fatal(err)
	}

	err = ep.Start()
	time.Sleep(time.Millisecond * 200)

	ep.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		assert.Equal(t, "aa", router.Definition().AdditionalInfo["aa"])
		return true
	})

	var def types.EndpointDsl
	_ = json.Unmarshal([]byte(endpointStr), &def)
	v, _ := json.Marshal(def)
	dsl := strings.Replace(string(v), " ", "", -1)

	assert.Equal(t, dsl, strings.Replace(string(ep.DSL()), " ", "", -1))
	assert.True(t, reflect.DeepEqual(def, ep.Definition()))
	sendMsg(t, "http://127.0.0.1:9081/api/v1/test/test01", "POST", msg1, test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, relationType, types.Success)
	}))
	time.Sleep(time.Millisecond * 2000)

	ep.Destroy()
}

// TestDynamicEndpointStartRetryDestroyRace guards the cancelStartRetry fix:
// while the background Start retry is active, Destroy must wait for the retry
// goroutine to exit before returning, so the endpoint instance is not
// destroyed out from under an in-flight Start().
func TestDynamicEndpointStartRetryDestroyRace(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	// Point at an unreachable port so Start() fails and arms the background retry.
	dsl := `{"id":"ep_retry_race","type":"endpoint/mqtt","configuration":{"server":"127.0.0.1:1","qos":0}}`
	ep, err := NewFromDsl([]byte(dsl), endpoint.DynamicEndpointOptions.WithConfig(config))
	if err != nil {
		t.Fatalf("NewFromDsl: %v", err)
	}

	// Wrapped Start swallows the failure and arms the background retry.
	if err := ep.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Confirm the retry is active.
	if got := ep.ConnectionStatus().Status; got != types.StatusReconnecting {
		t.Fatalf("status=%s, want reconnecting", got)
	}

	// Destroy during active retry must return promptly (not block on the retry).
	destroyDone := make(chan struct{})
	go func() { ep.Destroy(); close(destroyDone) }()
	select {
	case <-destroyDone:
		// returned without blocking
	case <-time.After(5 * time.Second):
		t.Fatal("Destroy blocked for >5s while retry was active")
	}

	// After Destroy, status must no longer report an active retry.
	if got := ep.ConnectionStatus().Status; got == types.StatusReconnecting {
		t.Fatalf("status still reconnecting after Destroy")
	}
}

var deferredStubAddRouterCount int64

type deferredStubEndpoint struct {
	impl.BaseEndpoint
}

func (e *deferredStubEndpoint) Type() string { return "deferredStub" }
func (e *deferredStubEndpoint) Id() string   { return "ep_deferred_reload" }
func (e *deferredStubEndpoint) New() types.Node {
	return &deferredStubEndpoint{}
}
func (e *deferredStubEndpoint) Init(_ types.Config, _ types.Configuration) error { return nil }
func (e *deferredStubEndpoint) AddRouter(_ endpoint.Router, _ ...interface{}) (string, error) {
	atomic.AddInt64(&deferredStubAddRouterCount, 1)
	return "1", nil
}
func (e *deferredStubEndpoint) RemoveRouter(_ string, _ ...interface{}) error { return nil }
func (e *deferredStubEndpoint) Start() error                                  { return nil }

// 链上部署的 endpoint 首次由部署方 ApplyRouters 挂路由；之后走配置变更重启型
// 重载时必须自行把路由挂回去，否则端点以零路由重启。
func TestApplyRoutersThenRestartReloadKeepsRouters(t *testing.T) {
	_ = Registry.Register(&deferredStubEndpoint{})

	def := types.EndpointDsl{
		RuleNode: types.RuleNode{
			Id:            "ep_deferred_reload",
			Type:          "deferredStub",
			Configuration: types.Configuration{"k": "v1"},
		},
		Routers: []*types.RouterDsl{
			{Id: "r1", From: types.FromDsl{Path: "/t"}, To: types.ToDsl{Path: "chainX"}},
		},
	}
	ep, err := NewPool().Factory().NewFromDef(def, endpoint.DynamicEndpointOptions.WithDeferredRouters(true))
	assert.Nil(t, err)

	assert.Nil(t, ep.ApplyRouters())
	before := atomic.LoadInt64(&deferredStubAddRouterCount)
	assert.Equal(t, int64(1), before)

	// 配置变更触发重启型重载，路由应随新实例重建
	def2 := def
	def2.Configuration = types.Configuration{"k": "v2"}
	assert.Nil(t, ep.ReloadFromDef(def2))
	after := atomic.LoadInt64(&deferredStubAddRouterCount)
	assert.Equal(t, before+1, after)

	ep.Destroy()
}

// flakyStart stub: Start fails for the first flakyStartFailFirst calls (shared
// state because Registry.New re-instantiates the component per reload).
var (
	flakyStartCalls     int32
	flakyStartFailFirst int32
	capturedConfig      types.Configuration
)

type flakyStartEndpoint struct {
	impl.BaseEndpoint
}

func (e *flakyStartEndpoint) Type() string { return "flakyStart" }
func (e *flakyStartEndpoint) Id() string   { return "ep_flaky_start" }
func (e *flakyStartEndpoint) New() types.Node {
	return &flakyStartEndpoint{}
}
func (e *flakyStartEndpoint) Init(_ types.Config, configuration types.Configuration) error {
	capturedConfig = configuration
	return nil
}
func (e *flakyStartEndpoint) AddRouter(_ endpoint.Router, _ ...interface{}) (string, error) {
	return "1", nil
}
func (e *flakyStartEndpoint) RemoveRouter(_ string, _ ...interface{}) error { return nil }
func (e *flakyStartEndpoint) Start() error {
	if atomic.AddInt32(&flakyStartCalls, 1) <= atomic.LoadInt32(&flakyStartFailFirst) {
		return errors.New("flaky start failure")
	}
	return nil
}

func setStartRetryIntervals(t *testing.T, interval, max time.Duration) {
	oldInterval, oldMax := StartRetryInterval, StartRetryMaxInterval
	StartRetryInterval, StartRetryMaxInterval = interval, max
	t.Cleanup(func() { StartRetryInterval, StartRetryMaxInterval = oldInterval, oldMax })
}

// The wrapped Start swallows failures and retries in the background; once a
// retry succeeds the endpoint reports Connected via the started flag.
func TestDynamicEndpointStartRetrySucceeds(t *testing.T) {
	_ = Registry.Register(&flakyStartEndpoint{})
	atomic.StoreInt32(&flakyStartCalls, 0)
	atomic.StoreInt32(&flakyStartFailFirst, 1)
	setStartRetryIntervals(t, 20*time.Millisecond, time.Second)

	ep, err := NewFromDsl([]byte(`{"id":"ep_flaky","type":"flakyStart"}`))
	assert.Nil(t, err)
	// First Start fails internally, the wrapped Start arms the retry.
	assert.Nil(t, ep.Start())
	assert.Equal(t, types.StatusReconnecting, ep.ConnectionStatus().Status)
	// Second Start while retrying is idempotent (no second goroutine).
	assert.Nil(t, ep.Start())

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) &&
		ep.ConnectionStatus().Status != types.StatusConnected {
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, types.StatusConnected, ep.ConnectionStatus().Status)
	// Destroy is idempotent.
	ep.Destroy()
	ep.Destroy()
}

// The retry backoff is capped at StartRetryMaxInterval; Destroy cancels the
// retry loop even while every attempt still fails.
func TestDynamicEndpointStartRetryBackoffCapAndCancel(t *testing.T) {
	_ = Registry.Register(&flakyStartEndpoint{})
	atomic.StoreInt32(&flakyStartCalls, 0)
	atomic.StoreInt32(&flakyStartFailFirst, 1000)
	setStartRetryIntervals(t, 10*time.Millisecond, 15*time.Millisecond)

	ep, err := NewFromDsl([]byte(`{"id":"ep_flaky_cap","type":"flakyStart"}`))
	assert.Nil(t, err)
	assert.Nil(t, ep.Start())

	// >=4 calls means the interval was capped at least once (10 -> 15 -> cap).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt32(&flakyStartCalls) < 4 {
		time.Sleep(10 * time.Millisecond)
	}
	assert.True(t, atomic.LoadInt32(&flakyStartCalls) >= 4)
	// startErr is refreshed on every failed retry.
	assert.Equal(t, types.StatusReconnecting, ep.ConnectionStatus().Status)

	ep.Destroy()
	assert.NotEqual(t, types.StatusReconnecting, ep.ConnectionStatus().Status)
	assert.Equal(t, 0, int(ep.ConnectionStatus().Status))
}

// Setters, node/chain accessors and the uninitialized-endpoint error paths.
func TestDynamicEndpointAccessors(t *testing.T) {
	ep := &DynamicEndpoint{}
	// Uninitialized endpoint: Start/ApplyRouters must fail loudly.
	err := ep.Start()
	assert.Equal(t, "endpoint not initialized", err.Error())
	err = ep.ApplyRouters()
	assert.Equal(t, "endpoint not initialized", err.Error())
	// Destroy on an uninitialized endpoint must not panic.
	ep.Destroy()

	ep.SetId("custom_id")
	assert.Equal(t, "custom_id", ep.Id())
	ep.SetInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		return true
	})
	assert.False(t, ep.IsDebugMode())
	nodeId := ep.GetNodeId()
	assert.Equal(t, "custom_id", nodeId.Id)
	assert.Equal(t, types.ENDPOINT, nodeId.Type)
	node, ok := ep.GetNodeById(types.RuleNodeId{Id: "any"})
	assert.False(t, ok)
	assert.Nil(t, node)
	assert.True(t, ep.Target() == nil)

	config := engine.NewConfig(types.WithDefaultPool())
	ep.SetConfig(config)
	assert.Equal(t, config, ep.Config())
	ep.SetRouterOptions(endpoint.RouterOptions.WithRuleConfig(config))
	ep.SetRestart(false)
	ep.SetChainCtx(nil)
	ep.SetRuleChain(nil)
	assert.Nil(t, ep.GetRuleChain())
}

func TestNewFromDslErrors(t *testing.T) {
	_, err := NewFromDsl(nil)
	assert.Equal(t, "def cannot be nil", err.Error())
	_, err = NewFromDsl([]byte("{invalid json"))
	assert.NotNil(t, err)
}

// idEchoEndpoint echoes the router id from AddRouter like real endpoints do;
// used by tests that rely on router ids surviving registration.
type idEchoEndpoint struct {
	impl.BaseEndpoint
}

func (e *idEchoEndpoint) Type() string { return "idEcho" }
func (e *idEchoEndpoint) Id() string   { return "ep_id_echo" }
func (e *idEchoEndpoint) New() types.Node {
	return &idEchoEndpoint{}
}
func (e *idEchoEndpoint) Init(_ types.Config, _ types.Configuration) error { return nil }
func (e *idEchoEndpoint) AddRouter(router endpoint.Router, _ ...interface{}) (string, error) {
	return router.GetId(), nil
}
func (e *idEchoEndpoint) RemoveRouter(_ string, _ ...interface{}) error { return nil }
func (e *idEchoEndpoint) Start() error                                  { return nil }

// AddRouterFromDef validation: nil def, unknown processors on from/to, and the
// To.Wait flag all take effect without touching a real network endpoint.
func TestAddRouterFromDefErrors(t *testing.T) {
	_ = Registry.Register(&idEchoEndpoint{})
	ep, err := NewFromDef(types.EndpointDsl{
		RuleNode: types.RuleNode{Id: "ep_router_def", Type: "idEcho"},
	})
	assert.Nil(t, err)
	defer ep.Destroy()

	_, err = ep.AddRouterFromDef(nil)
	assert.Equal(t, "routerDsl cannot be nil", err.Error())

	// Unknown from-processor.
	_, err = ep.AddRouterFromDef(&types.RouterDsl{
		Id: "bad_from", From: types.FromDsl{Path: "/t", Processors: []string{"noSuchInProcessor"}},
	})
	assert.Equal(t, "processor not found: noSuchInProcessor", err.Error())

	// Unknown to-processor.
	_, err = ep.AddRouterFromDef(&types.RouterDsl{
		Id:   "bad_to",
		From: types.FromDsl{Path: "/t2"},
		To:   types.ToDsl{Path: "chain:x", Processors: []string{"noSuchOutProcessor"}},
	})
	assert.Equal(t, "processor not found: noSuchOutProcessor", err.Error())

	// Valid from/to processors, to with Wait.
	routerId, err := ep.AddRouterFromDef(&types.RouterDsl{
		Id:   "ok_router",
		From: types.FromDsl{Path: "/ok", Processors: []string{"setJsonDataType"}},
		To:   types.ToDsl{Path: "chain:x", Processors: []string{"responseToBody"}, Wait: true},
	})
	assert.Nil(t, err)
	assert.Equal(t, "ok_router", routerId)
	// definition.Routers must reflect the added router.
	assert.Equal(t, 1, len(ep.Definition().Routers))

	// Removing the router updates the definition.
	assert.Nil(t, ep.RemoveRouter("ok_router"))
	assert.Equal(t, 0, len(ep.Definition().Routers))
}

// newEndpoint injects the rule chain definition and endpoint node identity
// into the component configuration when they are set on the DynamicEndpoint.
func TestNewEndpointConfigInjection(t *testing.T) {
	_ = Registry.Register(&flakyStartEndpoint{})
	atomic.StoreInt32(&flakyStartFailFirst, 0)

	chainDef := &types.RuleChain{RuleChain: types.RuleChainBaseInfo{ID: "chain_inject"}}
	ep := &DynamicEndpoint{}
	ep.SetRuleChain(chainDef)

	ruleDsl, err := os.ReadFile(testRulesFolder + "/filter_node.json")
	assert.Nil(t, err)
	ruleEngine, err := engine.New("inject_ctx", ruleDsl)
	assert.Nil(t, err)
	ep.SetChainCtx(ruleEngine.RootRuleChainCtx())

	assert.Nil(t, ep.ReloadFromDef(types.EndpointDsl{
		RuleNode: types.RuleNode{Id: "ep_inject", Type: "flakyStart"},
	}))
	assert.NotNil(t, capturedConfig[types.NodeConfigurationKeyRuleChainDefinition])
	assert.NotNil(t, capturedConfig[types.NodeConfigurationKeySelfDefinition])
	assert.NotNil(t, capturedConfig[types.NodeConfigurationKeyChainCtx])
	ep.Destroy()
}

// Unknown endpoint-level processors abort endpoint creation.
func TestNewFromDefUnknownProcessor(t *testing.T) {
	_ = Registry.Register(&deferredStubEndpoint{})
	_, err := NewFromDef(types.EndpointDsl{
		RuleNode:   types.RuleNode{Id: "ep_bad_proc", Type: "deferredStub"},
		Processors: []string{"noSuchProcessor"},
	})
	assert.Equal(t, "processor not found: noSuchProcessor", err.Error())
}

// ReloadFromDef with a changed def triggers router add/remove/modify diffing.
func TestReloadRouterChanges(t *testing.T) {
	_ = Registry.Register(&idEchoEndpoint{})
	base := func() types.EndpointDsl {
		return types.EndpointDsl{
			RuleNode: types.RuleNode{Id: "ep_reload_diff", Type: "idEcho"},
			Routers: []*types.RouterDsl{
				{Id: "keep", From: types.FromDsl{Path: "/keep"}},
				{Id: "modify", From: types.FromDsl{Path: "/old"}},
				{Id: "remove", From: types.FromDsl{Path: "/remove"}},
			},
		}
	}
	ep, err := NewFromDef(base())
	assert.Nil(t, err)
	defer ep.Destroy()
	assert.Equal(t, 3, len(ep.Definition().Routers))

	changed := base()
	changed.Routers = []*types.RouterDsl{
		{Id: "keep", From: types.FromDsl{Path: "/keep"}},
		{Id: "modify", From: types.FromDsl{Path: "/new"}},
		{Id: "added", From: types.FromDsl{Path: "/added"}},
	}
	assert.Nil(t, ep.ReloadFromDef(changed))
	ids := make(map[string]bool)
	for _, r := range ep.Definition().Routers {
		ids[r.Id] = true
	}
	assert.True(t, ids["keep"])
	assert.True(t, ids["modify"])
	assert.True(t, ids["added"])
	assert.False(t, ids["remove"])

	// Reload with empty DSL bytes keeps the current definition.
	assert.Nil(t, ep.Reload(nil))
}

// AddOrReloadRouter: bad JSON errors; with the restart option the endpoint is
// recreated (router count reflects the new instance).
func TestAddOrReloadRouter(t *testing.T) {
	_ = Registry.Register(&idEchoEndpoint{})
	ep, err := NewFromDef(types.EndpointDsl{
		RuleNode: types.RuleNode{Id: "ep_add_or_reload", Type: "idEcho"},
	})
	assert.Nil(t, err)
	defer ep.Destroy()

	err = ep.AddOrReloadRouter([]byte("{bad json"))
	assert.NotNil(t, err)

	assert.Nil(t, ep.AddOrReloadRouter(
		[]byte(`{"id":"r9","from":{"path":"/r9"},"to":{"path":"chain:x"}}`),
		endpoint.DynamicEndpointOptions.WithRestart(true),
	))

	// Without restart, the router is added to the running endpoint.
	assert.Nil(t, ep.AddOrReloadRouter(
		[]byte(`{"id":"r10","from":{"path":"/r10"},"to":{"path":"chain:x"}}`),
	))
	found := false
	for _, r := range ep.Definition().Routers {
		if r.Id == "r10" {
			found = true
		}
	}
	assert.True(t, found)
}

// needRestart must flag type, configuration and processor changes.
func TestNeedRestart(t *testing.T) {
	old := types.EndpointDsl{
		RuleNode:   types.RuleNode{Type: "t1", Configuration: types.Configuration{"a": 1}},
		Processors: []string{"p1"},
	}
	assert.False(t, needRestart(old, old))
	changedType := old
	changedType.Type = "t2"
	assert.True(t, needRestart(old, changedType))
	changedConfig := old
	changedConfig.Configuration = types.Configuration{"a": 2}
	assert.True(t, needRestart(old, changedConfig))
	changedProcessors := old
	changedProcessors.Processors = []string{"p2"}
	assert.True(t, needRestart(old, changedProcessors))
}

// checkRouterChanges diffing: added, removed and unchanged routers.
func TestCheckRouterChanges(t *testing.T) {
	oldRouters := []*types.RouterDsl{
		{Id: "same", From: types.FromDsl{Path: "/a"}},
		{Id: "gone", From: types.FromDsl{Path: "/b"}},
	}
	newRouters := []*types.RouterDsl{
		{Id: "same", From: types.FromDsl{Path: "/a"}},
		{Id: "new", From: types.FromDsl{Path: "/c"}},
	}
	added, removed, modified := checkRouterChanges(oldRouters, newRouters)
	assert.Equal(t, 1, len(added))
	assert.Equal(t, "new", added[0].Id)
	assert.Equal(t, 1, len(removed))
	assert.Equal(t, "gone", removed[0].Id)
	assert.Equal(t, 0, len(modified))
}
