package aspect

import (
	"errors"
	"sync"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/test/assert"
)

func TestProcessEndpointDsl(t *testing.T) {
	config := types.NewConfig()
	ruleChain := &types.RuleChain{
		RuleChain: types.RuleChainBaseInfo{
			Configuration: types.Configuration{
				types.Vars: map[string]interface{}{
					"port":      "8080",
					"path":      "/api/test",
					"processor": "log",
					"param":     "GET",
					"target":    "chain:default",
				},
			},
		},
	}

	endpointDsl := &types.EndpointDsl{
		Processors: []string{"${vars.processor}", "auth"},
		Routers: []*types.RouterDsl{
			{
				Params: []interface{}{"${vars.param}", "POST", 200},
				From: types.FromDsl{
					Path: "${vars.path}",
					Configuration: types.Configuration{
						"timeout": "${vars.port}",
					},
					Processors: []string{"${vars.processor}"},
				},
				To: types.ToDsl{
					Path: "${vars.target}",
					Configuration: types.Configuration{
						"retry": "3",
					},
					Processors: []string{"transform"},
				},
			},
		},
	}
	// Set some initial configuration to verify it's processed too
	endpointDsl.Configuration = types.Configuration{
		"server":  "http://localhost:${vars.port}",
		"retries": 3,
	}

	processEndpointDsl(config, ruleChain, endpointDsl)

	if endpointDsl.Processors[0] != "log" {
		t.Errorf("Expected Processors[0] to be 'log', got '%s'", endpointDsl.Processors[0])
	}
	if endpointDsl.Processors[1] != "auth" {
		t.Errorf("Expected Processors[1] to be 'auth', got '%s'", endpointDsl.Processors[1])
	}
	if endpointDsl.Configuration["server"] != "http://localhost:8080" {
		t.Errorf("Expected Configuration['server'] to be 'http://localhost:8080', got '%s'", endpointDsl.Configuration["server"])
	}

	router := endpointDsl.Routers[0]
	if router.Params[0] != "GET" {
		t.Errorf("Expected Params[0] to be 'GET', got '%s'", router.Params[0])
	}
	if router.Params[1] != "POST" {
		t.Errorf("Expected Params[1] to be 'POST', got '%s'", router.Params[1])
	}
	if router.Params[2] != 200 {
		t.Errorf("Expected Params[2] to stay 200, got '%v'", router.Params[2])
	}
	if endpointDsl.Configuration["retries"] != 3 {
		t.Errorf("Expected Configuration['retries'] to stay 3, got '%v'", endpointDsl.Configuration["retries"])
	}

	if router.From.Path != "/api/test" {
		t.Errorf("Expected From.Path to be '/api/test', got '%s'", router.From.Path)
	}
	if router.From.Configuration["timeout"] != "8080" {
		t.Errorf("Expected From.Configuration['timeout'] to be '8080', got '%s'", router.From.Configuration["timeout"])
	}
	if router.From.Processors[0] != "log" {
		t.Errorf("Expected From.Processors[0] to be 'log', got '%s'", router.From.Processors[0])
	}

	if router.To.Path != "chain:default" {
		t.Errorf("Expected To.Path to be 'chain:default', got '%s'", router.To.Path)
	}

	// a nil rule chain leaves the definition untouched
	untouched := &types.EndpointDsl{Routers: []*types.RouterDsl{
		{From: types.FromDsl{Path: "${vars.path}"}},
	}}
	processEndpointDsl(config, nil, untouched)
	if untouched.Routers[0].From.Path != "${vars.path}" {
		t.Errorf("Expected From.Path to stay '${vars.path}', got '%s'", untouched.Routers[0].From.Path)
	}
}

// fakeResourceRegistry is an in-memory types.ResourceRegistry.
type fakeResourceRegistry struct {
	mu   sync.RWMutex
	data map[string]any
}

func newFakeResourceRegistry() *fakeResourceRegistry {
	return &fakeResourceRegistry{data: map[string]any{}}
}

func (r *fakeResourceRegistry) Register(id string, resource any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[id] = resource
}

func (r *fakeResourceRegistry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, id)
}

func (r *fakeResourceRegistry) Lookup(id string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.data[id]
	return v, ok
}

// fakeChainCtx implements the ChainCtx surface used by EndpointAspect.
type fakeChainCtx struct {
	types.ChainCtx
	id     string
	config types.Config
	def    *types.RuleChain
	reg    *fakeResourceRegistry
}

func (c *fakeChainCtx) GetNodeId() types.RuleNodeId {
	return types.RuleNodeId{Id: c.id, Type: types.CHAIN}
}
func (c *fakeChainCtx) Config() types.Config                     { return c.config }
func (c *fakeChainCtx) Definition() *types.RuleChain             { return c.def }
func (c *fakeChainCtx) GetRuleEnginePool() types.RuleEnginePool  { return nil }
func (c *fakeChainCtx) ResourceRegistry() types.ResourceRegistry { return c.reg }

// fakeDynamicEndpoint records lifecycle calls; errors are injected per id by the factory.
type fakeDynamicEndpoint struct {
	id       string
	def      types.EndpointDsl
	starts   int
	destroys int
	startErr error
	applyErr error
	noTarget bool
}

func (e *fakeDynamicEndpoint) Type() string    { return "fake" }
func (e *fakeDynamicEndpoint) New() types.Node { return &fakeDynamicEndpoint{} }
func (e *fakeDynamicEndpoint) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return nil
}
func (e *fakeDynamicEndpoint) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {}
func (e *fakeDynamicEndpoint) Destroy()                                       { e.destroys++ }
func (e *fakeDynamicEndpoint) Id() string                                     { return e.id }
func (e *fakeDynamicEndpoint) SetId(id string)                                { e.id = id }
func (e *fakeDynamicEndpoint) SetOnEvent(onEvent endpoint.OnEvent)            {}
func (e *fakeDynamicEndpoint) Start() error {
	if e.startErr != nil {
		return e.startErr
	}
	e.starts++
	return nil
}
func (e *fakeDynamicEndpoint) AddInterceptors(interceptors ...endpoint.Process) {}
func (e *fakeDynamicEndpoint) AddRouter(router endpoint.Router, params ...interface{}) (string, error) {
	return "", nil
}
func (e *fakeDynamicEndpoint) RemoveRouter(routerId string, params ...interface{}) error { return nil }
func (e *fakeDynamicEndpoint) SetConfig(config types.Config)                             {}
func (e *fakeDynamicEndpoint) SetRouterOptions(opts ...endpoint.RouterOption)            {}
func (e *fakeDynamicEndpoint) SetRestart(restart bool)                                   {}
func (e *fakeDynamicEndpoint) SetInterceptors(interceptors ...endpoint.Process)          {}
func (e *fakeDynamicEndpoint) SetChainCtx(chainCtx types.ChainCtx)                       {}
func (e *fakeDynamicEndpoint) SetDeferredRouters(deferred bool)                          {}
func (e *fakeDynamicEndpoint) Reload(dsl []byte, opts ...endpoint.DynamicEndpointOption) error {
	return nil
}
func (e *fakeDynamicEndpoint) ReloadFromDef(def types.EndpointDsl, opts ...endpoint.DynamicEndpointOption) error {
	return nil
}
func (e *fakeDynamicEndpoint) AddOrReloadRouter(dsl []byte, opts ...endpoint.DynamicEndpointOption) error {
	return nil
}
func (e *fakeDynamicEndpoint) Definition() types.EndpointDsl { return e.def }
func (e *fakeDynamicEndpoint) DSL() []byte                   { return nil }
func (e *fakeDynamicEndpoint) Target() endpoint.Endpoint {
	if e.noTarget {
		return nil
	}
	return e
}
func (e *fakeDynamicEndpoint) ApplyRouters() error                     { return e.applyErr }
func (e *fakeDynamicEndpoint) SetRuleChain(ruleChain *types.RuleChain) {}
func (e *fakeDynamicEndpoint) GetRuleChain() *types.RuleChain          { return nil }

type fakeEndpointFactory struct {
	mu        sync.Mutex
	created   []*fakeDynamicEndpoint
	createErr func(def types.EndpointDsl) error
	applyErr  func(def types.EndpointDsl) error
	startErr  func(def types.EndpointDsl) error
}

func (f *fakeEndpointFactory) NewFromDef(def types.EndpointDsl, opts ...endpoint.DynamicEndpointOption) (endpoint.DynamicEndpoint, error) {
	if f.createErr != nil {
		if err := f.createErr(def); err != nil {
			return nil, err
		}
	}
	ep := &fakeDynamicEndpoint{id: def.Id, def: def}
	if f.applyErr != nil {
		ep.applyErr = f.applyErr(def)
	}
	if f.startErr != nil {
		ep.startErr = f.startErr(def)
	}
	for _, opt := range opts {
		if err := opt(ep); err != nil {
			return nil, err
		}
	}
	f.mu.Lock()
	f.created = append(f.created, ep)
	f.mu.Unlock()
	return ep, nil
}

func (f *fakeEndpointFactory) NewFromDsl(dsl []byte, opts ...endpoint.DynamicEndpointOption) (endpoint.DynamicEndpoint, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeEndpointFactory) NewFromType(componentType string, ruleConfig types.Config, configuration interface{}) (endpoint.Endpoint, error) {
	return nil, errors.New("not implemented")
}

type fakeEndpointPool struct {
	factory *fakeEndpointFactory
}

func (p *fakeEndpointPool) New(id string, dsl []byte, opts ...endpoint.DynamicEndpointOption) (endpoint.DynamicEndpoint, error) {
	return nil, nil
}
func (p *fakeEndpointPool) Get(id string) (endpoint.DynamicEndpoint, bool) { return nil, false }
func (p *fakeEndpointPool) Del(id string)                                  {}
func (p *fakeEndpointPool) Stop()                                          {}
func (p *fakeEndpointPool) Reload(opts ...endpoint.DynamicEndpointOption)  {}
func (p *fakeEndpointPool) Range(f func(key, value any) bool)              {}
func (p *fakeEndpointPool) Factory() endpoint.Factory                      { return p.factory }

func newTestChainCtx(id string, endpoints []*types.EndpointDsl, reg *fakeResourceRegistry, config types.Config) *fakeChainCtx {
	return &fakeChainCtx{
		id:     id,
		config: config,
		def: &types.RuleChain{
			RuleChain: types.RuleChainBaseInfo{ID: id, Root: true},
			Metadata:  types.RuleMetadata{Endpoints: endpoints},
		},
		reg: reg,
	}
}

// byId returns the most recently created instance with the given id, matching
// the replace-on-modify semantics of Reload.
func (f *fakeEndpointFactory) byId(id string) *fakeDynamicEndpoint {
	f.mu.Lock()
	defer f.mu.Unlock()
	var found *fakeDynamicEndpoint
	for _, ep := range f.created {
		if ep.id == id {
			found = ep
		}
	}
	return found
}

func TestEndpointAspectBasics(t *testing.T) {
	pool := &fakeEndpointPool{factory: &fakeEndpointFactory{}}
	a := &EndpointAspect{EndpointPool: pool}

	assert.Equal(t, 900, a.Order())
	assert.Equal(t, "endpoint", a.Type())
	assert.True(t, a.PointCut(nil, types.RuleMsg{}, ""))

	clone := a.New().(*EndpointAspect)
	assert.NotNil(t, clone)
	assert.Equal(t, pool, clone.EndpointPool)

	plainNode := &stubNodeCtx{nodeId: types.RuleNodeId{Id: "n1", Type: types.NODE}}

	// non chain context: no-op
	assert.Nil(t, a.OnCreated(plainNode))

	// endpoint module disabled: no endpoint is created
	disabled := types.NewConfig()
	disabled.EndpointEnabled = false
	chainCtx := newTestChainCtx("chain1", []*types.EndpointDsl{{RuleNode: types.RuleNode{Id: "ep1"}}}, newFakeResourceRegistry(), disabled)
	assert.Nil(t, a.OnCreated(chainCtx))
	assert.Nil(t, a.ruleChainEndpoint)

	// reload and destroy without a created endpoint are no-ops
	assert.Nil(t, a.OnReload(plainNode, chainCtx))
	a.OnDestroy(plainNode)
}

func TestEndpointAspectOnCreatedAndDestroy(t *testing.T) {
	factory := &fakeEndpointFactory{}
	pool := &fakeEndpointPool{factory: factory}
	a := &EndpointAspect{EndpointPool: pool}
	reg := newFakeResourceRegistry()

	ep1Def := &types.EndpointDsl{
		RuleNode: types.RuleNode{
			Id:            "ep1",
			Configuration: types.Configuration{"server": "127.0.0.1:0"},
		},
		Routers: []*types.RouterDsl{
			{From: types.FromDsl{Path: "/api/*"}, To: types.ToDsl{}},
		},
	}
	ep2Def := &types.EndpointDsl{RuleNode: types.RuleNode{Id: "ep2"}}
	chainCtx := newTestChainCtx("chain1", []*types.EndpointDsl{ep1Def, ep2Def}, reg, types.NewConfig())

	assert.Nil(t, a.OnCreated(chainCtx))
	assert.NotNil(t, a.ruleChainEndpoint)
	assert.Equal(t, 2, len(factory.created))

	// every endpoint started exactly once and registered into the chain directory
	for _, id := range []string{"ep1", "ep2"} {
		ep := factory.byId(id)
		assert.NotNil(t, ep)
		assert.Equal(t, 1, ep.starts)
		_, ok := reg.Lookup(id)
		assert.True(t, ok, id+" should be registered")
	}

	// router target defaults to the rule chain id
	assert.Equal(t, "chain1", ep1Def.Routers[0].To.Path)

	// destroy tears down all endpoints and clears the registry
	a.OnDestroy(chainCtx)
	assert.Equal(t, 0, len(a.ruleChainEndpoint.GetEndpoints()))
	for _, ep := range factory.created {
		assert.True(t, ep.destroys >= 1)
	}
	_, ok := reg.Lookup("ep1")
	assert.False(t, ok)
	_, ok = reg.Lookup("ep2")
	assert.False(t, ok)
}

func TestEndpointAspectReload(t *testing.T) {
	factory := &fakeEndpointFactory{}
	pool := &fakeEndpointPool{factory: factory}
	a := &EndpointAspect{EndpointPool: pool}
	reg := newFakeResourceRegistry()

	ep1Def := &types.EndpointDsl{RuleNode: types.RuleNode{Id: "ep1", Configuration: types.Configuration{"k": "v1"}}}
	ep2Def := &types.EndpointDsl{RuleNode: types.RuleNode{Id: "ep2"}}
	chainCtx := newTestChainCtx("chain1", []*types.EndpointDsl{ep1Def, ep2Def}, reg, types.NewConfig())
	assert.Nil(t, a.OnCreated(chainCtx))
	ep1Old := factory.byId("ep1")

	// unchanged definitions: nothing recreated or restarted
	assert.Nil(t, a.OnReload(chainCtx, chainCtx))
	assert.Equal(t, 2, len(factory.created))
	assert.Equal(t, 1, ep1Old.starts)

	// ep1 modified, ep2 removed, ep3 added
	ep1Def.Configuration = types.Configuration{"k": "v2"}
	ep3Def := &types.EndpointDsl{RuleNode: types.RuleNode{Id: "ep3"}}
	chainCtx.def.Metadata.Endpoints = []*types.EndpointDsl{ep1Def, ep3Def}
	assert.Nil(t, a.OnReload(chainCtx, chainCtx))

	ep1New := factory.byId("ep1")
	assert.True(t, ep1New != ep1Old, "modified endpoint should be recreated")
	assert.True(t, ep1Old.destroys >= 1)
	assert.Equal(t, 1, ep1New.starts)
	ep2Old := factory.byId("ep2")
	assert.True(t, ep2Old.destroys >= 1)
	_, ok := a.ruleChainEndpoint.GetEndpoint("ep2")
	assert.False(t, ok)
	ep3 := factory.byId("ep3")
	assert.NotNil(t, ep3)
	assert.Equal(t, 1, ep3.starts)
	_, ok = reg.Lookup("ep3")
	assert.True(t, ok)

	// endpoints disabled after reload: everything is destroyed and unregistered
	config := types.NewConfig()
	config.EndpointEnabled = false
	chainCtx.config = config
	assert.Nil(t, a.OnReload(chainCtx, chainCtx))
	assert.Equal(t, 0, len(a.ruleChainEndpoint.GetEndpoints()))
	_, ok = reg.Lookup("ep1")
	assert.False(t, ok)
	_, ok = reg.Lookup("ep3")
	assert.False(t, ok)
}

func TestEndpointAspectDeployFailures(t *testing.T) {
	// creation failure of the second endpoint rolls back the first one
	factory := &fakeEndpointFactory{createErr: func(def types.EndpointDsl) error {
		if def.Id == "ep2" {
			return errors.New("create failed")
		}
		return nil
	}}
	pool := &fakeEndpointPool{factory: factory}
	a := &EndpointAspect{EndpointPool: pool}
	reg := newFakeResourceRegistry()
	chainCtx := newTestChainCtx("chain1", []*types.EndpointDsl{{RuleNode: types.RuleNode{Id: "ep1"}}, {RuleNode: types.RuleNode{Id: "ep2"}}}, reg, types.NewConfig())

	err := a.OnCreated(chainCtx)
	assert.NotNil(t, err)
	assert.Nil(t, a.ruleChainEndpoint)
	assert.True(t, factory.byId("ep1").destroys >= 1)

	// start failure during deployment destroys all created instances
	factory2 := &fakeEndpointFactory{startErr: func(def types.EndpointDsl) error {
		if def.Id == "ep2" {
			return errors.New("start failed")
		}
		return nil
	}}
	pool2 := &fakeEndpointPool{factory: factory2}
	a2 := &EndpointAspect{EndpointPool: pool2}
	reg2 := newFakeResourceRegistry()
	chainCtx2 := newTestChainCtx("chain1", []*types.EndpointDsl{{RuleNode: types.RuleNode{Id: "ep1"}}, {RuleNode: types.RuleNode{Id: "ep2"}}}, reg2, types.NewConfig())

	err = a2.OnCreated(chainCtx2)
	assert.NotNil(t, err)
	assert.Nil(t, a2.ruleChainEndpoint)
	assert.True(t, factory2.byId("ep1").destroys >= 1)
	assert.True(t, factory2.byId("ep2").destroys >= 1)

	// router attach failure during deployment is also fatal
	factory3 := &fakeEndpointFactory{applyErr: func(def types.EndpointDsl) error {
		if def.Id == "ep2" {
			return errors.New("router failed")
		}
		return nil
	}}
	pool3 := &fakeEndpointPool{factory: factory3}
	a3 := &EndpointAspect{EndpointPool: pool3}
	chainCtx3 := newTestChainCtx("chain1", []*types.EndpointDsl{{RuleNode: types.RuleNode{Id: "ep1"}}, {RuleNode: types.RuleNode{Id: "ep2"}}}, newFakeResourceRegistry(), types.NewConfig())
	err = a3.OnCreated(chainCtx3)
	assert.NotNil(t, err)
	assert.Nil(t, a3.ruleChainEndpoint)

	// failed reload creation leaves the running endpoint untouched
	factory4 := &fakeEndpointFactory{}
	pool4 := &fakeEndpointPool{factory: factory4}
	a4 := &EndpointAspect{EndpointPool: pool4}
	reg4 := newFakeResourceRegistry()
	ep1Def := &types.EndpointDsl{RuleNode: types.RuleNode{Id: "ep1"}}
	chainCtx4 := newTestChainCtx("chain1", []*types.EndpointDsl{ep1Def}, reg4, types.NewConfig())
	assert.Nil(t, a4.OnCreated(chainCtx4))

	factory4.startErr = func(def types.EndpointDsl) error {
		if def.Id == "ep3" {
			return errors.New("start failed")
		}
		return nil
	}
	ep3Def := &types.EndpointDsl{RuleNode: types.RuleNode{Id: "ep3"}}
	chainCtx4.def.Metadata.Endpoints = []*types.EndpointDsl{ep1Def, ep3Def}
	err = a4.OnReload(chainCtx4, chainCtx4)
	assert.NotNil(t, err)
	_, ok := a4.ruleChainEndpoint.GetEndpoint("ep3")
	assert.False(t, ok, "failed endpoint should be rolled back")
	ep1 := factory4.byId("ep1")
	assert.Equal(t, 1, ep1.starts)
	assert.Equal(t, 0, ep1.destroys)

	// creation failure of an added endpoint during reload aborts the reload
	factory5 := &fakeEndpointFactory{}
	pool5 := &fakeEndpointPool{factory: factory5}
	a5 := &EndpointAspect{EndpointPool: pool5}
	ep1Def5 := &types.EndpointDsl{RuleNode: types.RuleNode{Id: "ep1"}}
	chainCtx5 := newTestChainCtx("chain1", []*types.EndpointDsl{ep1Def5}, newFakeResourceRegistry(), types.NewConfig())
	assert.Nil(t, a5.OnCreated(chainCtx5))

	factory5.createErr = func(def types.EndpointDsl) error {
		if def.Id == "ep2" {
			return errors.New("create failed")
		}
		return nil
	}
	chainCtx5.def.Metadata.Endpoints = []*types.EndpointDsl{ep1Def5, {RuleNode: types.RuleNode{Id: "ep2"}}}
	err = a5.OnReload(chainCtx5, chainCtx5)
	assert.NotNil(t, err)
	alive, ok := a5.ruleChainEndpoint.GetEndpoint("ep1")
	assert.True(t, ok)
	assert.Equal(t, 0, alive.(*fakeDynamicEndpoint).destroys)

	// creation failure of a modified endpoint during reload drops the old instance
	factory6 := &fakeEndpointFactory{}
	pool6 := &fakeEndpointPool{factory: factory6}
	a6 := &EndpointAspect{EndpointPool: pool6}
	ep1Def6 := &types.EndpointDsl{RuleNode: types.RuleNode{Id: "ep1", Configuration: types.Configuration{"k": "v1"}}}
	chainCtx6 := newTestChainCtx("chain1", []*types.EndpointDsl{ep1Def6}, newFakeResourceRegistry(), types.NewConfig())
	assert.Nil(t, a6.OnCreated(chainCtx6))

	factory6.createErr = func(def types.EndpointDsl) error {
		if def.Id == "ep1" {
			return errors.New("create failed")
		}
		return nil
	}
	ep1Def6.Configuration = types.Configuration{"k": "v2"}
	err = a6.OnReload(chainCtx6, chainCtx6)
	assert.NotNil(t, err)
	_, ok = a6.ruleChainEndpoint.GetEndpoint("ep1")
	assert.False(t, ok)
	assert.True(t, factory6.byId("ep1").destroys >= 1)
}

func TestEndpointAspectSyncResources(t *testing.T) {
	a := &EndpointAspect{}
	reg := newFakeResourceRegistry()
	chainCtx := newTestChainCtx("chain1", nil, reg, types.NewConfig())

	ep1 := &fakeDynamicEndpoint{id: "ep1"}
	noTarget := &fakeDynamicEndpoint{id: "ep2", noTarget: true}

	// nil entries and endpoints without a target are skipped
	a.syncResources(chainCtx, nil, []endpoint.DynamicEndpoint{ep1, nil, noTarget})
	_, ok := reg.Lookup("ep1")
	assert.True(t, ok)
	_, ok = reg.Lookup("ep2")
	assert.False(t, ok)

	// old entries missing from the new set are unregistered
	ep3 := &fakeDynamicEndpoint{id: "ep3"}
	reg.Register("ep3", ep3)
	a.syncResources(chainCtx, []endpoint.DynamicEndpoint{ep3, nil}, []endpoint.DynamicEndpoint{ep1})
	_, ok = reg.Lookup("ep3")
	assert.False(t, ok)
	_, ok = reg.Lookup("ep1")
	assert.True(t, ok)

	// an endpoint kept in both sets stays registered
	a.syncResources(chainCtx, []endpoint.DynamicEndpoint{ep1}, []endpoint.DynamicEndpoint{ep1})
	_, ok = reg.Lookup("ep1")
	assert.True(t, ok)
}

func TestRuleChainEndpointBasics(t *testing.T) {
	pool := &fakeEndpointPool{factory: &fakeEndpointFactory{}}
	config := types.NewConfig()

	// without a chain context: no resource registration, chainDef is nil
	defs := []*types.EndpointDsl{{RuleNode: types.RuleNode{Id: "ep1"}, Routers: []*types.RouterDsl{{}}}}
	e, err := NewRuleChainEndpoint("chain1", config, pool, nil, nil, defs, nil)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(e.GetEndpoints()))
	assert.Nil(t, e.chainDef())

	_, ok := e.GetEndpoint("ep1")
	assert.True(t, ok)
	_, ok = e.GetEndpoint("missing")
	assert.False(t, ok)

	// AddEndpointAndStart assigns a uuid when the definition has no id
	def2 := &types.EndpointDsl{}
	assert.Nil(t, e.AddEndpointAndStart(def2, true))
	assert.True(t, def2.Id != "")
	added, _ := e.GetEndpoint(def2.Id)
	assert.Equal(t, 1, added.(*fakeDynamicEndpoint).starts)

	// factory failure propagates
	pool.factory.createErr = func(def types.EndpointDsl) error { return errors.New("no") }
	assert.NotNil(t, e.AddEndpointAndStart(&types.EndpointDsl{}, false))
	pool.factory.createErr = nil

	// Start failure propagates
	ep1 := e.byFakeId("ep1")
	ep1.startErr = errors.New("start failed")
	assert.NotNil(t, e.Start())
	ep1.startErr = nil
	assert.Nil(t, e.Start())

	// RemoveEndpoint ignores unknown ids and destroys known ones
	e.RemoveEndpoint("missing")
	e.RemoveEndpoint(def2.Id)
	_, ok = e.GetEndpoint(def2.Id)
	assert.False(t, ok)

	// destroyEndpoints and Destroy are safe to repeat
	e.destroyEndpoints()
	assert.Equal(t, 0, len(e.GetEndpoints()))
	e.Destroy()
	e.Destroy()
}

func TestRuleChainEndpointChanges(t *testing.T) {
	e := &RuleChainEndpoint{}

	oldA := &types.EndpointDsl{RuleNode: types.RuleNode{Id: "a"}}
	oldB := &types.EndpointDsl{RuleNode: types.RuleNode{Id: "b"}}
	sameA := &types.EndpointDsl{RuleNode: types.RuleNode{Id: "a"}}
	modA := &types.EndpointDsl{RuleNode: types.RuleNode{Id: "a", Configuration: types.Configuration{"k": "v"}}}
	newC := &types.EndpointDsl{RuleNode: types.RuleNode{Id: "c"}}

	// unchanged endpoint
	added, removed, modified := e.checkEndpointChanges([]*types.EndpointDsl{oldA}, []*types.EndpointDsl{sameA})
	assert.Nil(t, added)
	assert.Nil(t, removed)
	assert.Nil(t, modified)

	// modified endpoint
	added, removed, modified = e.checkEndpointChanges([]*types.EndpointDsl{oldA}, []*types.EndpointDsl{modA})
	assert.Nil(t, added)
	assert.Nil(t, removed)
	assert.Equal(t, []*types.EndpointDsl{modA}, modified)

	// added and removed endpoints
	added, removed, modified = e.checkEndpointChanges(
		[]*types.EndpointDsl{oldA, oldB},
		[]*types.EndpointDsl{sameA, newC})
	assert.Equal(t, []*types.EndpointDsl{newC}, added)
	assert.Equal(t, []*types.EndpointDsl{oldB}, removed)
	assert.Nil(t, modified)

	// empty inputs
	added, removed, modified = e.checkEndpointChanges(nil, nil)
	assert.Nil(t, added)
	assert.Nil(t, removed)
	assert.Nil(t, modified)

	assert.False(t, e.isEndpointModified(oldA, sameA))
	assert.True(t, e.isEndpointModified(oldA, modA))

	// bindTo fills empty router targets with the rule engine id
	def := &types.EndpointDsl{Routers: []*types.RouterDsl{
		{To: types.ToDsl{}},
		{To: types.ToDsl{Path: "kept"}},
	}}
	e.bindTo(def, "chain1")
	assert.Equal(t, "chain1", def.Routers[0].To.Path)
	assert.Equal(t, "kept", def.Routers[1].To.Path)

	// registerResources without a chain context is a no-op
	e.registerResources(nil, nil)

	// nil entries and target-less endpoints are skipped during registration
	reg := newFakeResourceRegistry()
	e.chainCtx = newTestChainCtx("chain1", nil, reg, types.NewConfig())
	noTarget := &fakeDynamicEndpoint{id: "ep1", noTarget: true}
	e.registerResources(nil, []endpoint.DynamicEndpoint{nil, noTarget})
	_, ok := reg.Lookup("ep1")
	assert.False(t, ok)

	// entries only present in the old set are unregistered
	old := &fakeDynamicEndpoint{id: "ep2"}
	fresh := &fakeDynamicEndpoint{id: "ep3"}
	e.registerResources([]endpoint.DynamicEndpoint{old, nil}, []endpoint.DynamicEndpoint{fresh})
	_, ok = reg.Lookup("ep2")
	assert.False(t, ok)
	_, ok = reg.Lookup("ep3")
	assert.True(t, ok)
}

func (e *RuleChainEndpoint) byFakeId(id string) *fakeDynamicEndpoint {
	if ep, ok := e.GetEndpoint(id); ok {
		return ep.(*fakeDynamicEndpoint)
	}
	return nil
}
