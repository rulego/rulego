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

package base

import (
	"errors"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestSharedNodeInitFailFast verifies fast-fail within the cooldown, retry after the window, and clearing on success.
func TestSharedNodeInitFailFast(t *testing.T) {
	var calls int32
	dialErr := errors.New("dial: connection refused")
	x := &SharedNode[string]{InitFailRetryInterval: 100 * time.Millisecond}
	_ = x.InitWithClose(types.Config{}, "testNode", "resource1", false, func() (string, error) {
		atomic.AddInt32(&calls, 1)
		return "", dialErr
	}, nil)

	// first call triggers init and fails
	if _, err := x.GetSafely(); err != dialErr {
		t.Fatalf("first call: expected %v, got %v", dialErr, err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls after first failure: got %d, want 1", got)
	}

	// subsequent calls within the window fast-fail without retrying init
	for i := 0; i < 10; i++ {
		if _, err := x.GetSafely(); err != dialErr {
			t.Fatalf("fast-fail call: expected %v, got %v", dialErr, err)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("init retried within cooldown: calls=%d", got)
	}

	// retry is allowed after the window
	time.Sleep(120 * time.Millisecond)
	if _, err := x.GetSafely(); err != dialErr {
		t.Fatalf("cooldown retry: expected %v, got %v", dialErr, err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls after cooldown retry: got %d, want 2", got)
	}

	// a successful retry clears the failure; later calls return the cached instance
	x.Locker.Lock()
	x.InitInstanceFunc = func() (string, error) {
		atomic.AddInt32(&calls, 1)
		return "client-ok", nil
	}
	x.Locker.Unlock()
	time.Sleep(120 * time.Millisecond)
	if v, err := x.GetSafely(); err != nil || v != "client-ok" {
		t.Fatalf("after success: v=%q err=%v", v, err)
	}
	if v, err := x.GetSafely(); err != nil || v != "client-ok" {
		t.Fatalf("cached call: v=%q err=%v", v, err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("calls after success: got %d, want 3", got)
	}
}

// TestSharedNodeInitFailFastConcurrent verifies only one init retry within the cooldown under concurrent callers.
func TestSharedNodeInitFailFastConcurrent(t *testing.T) {
	var calls int32
	x := &SharedNode[string]{InitFailRetryInterval: time.Second}
	_ = x.InitWithClose(types.Config{}, "testNode", "resource2", false, func() (string, error) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(20 * time.Millisecond)
		return "", errors.New("endpoint down")
	}, nil)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = x.GetSafely()
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("init called %d times within cooldown, want 1", got)
	}
}

// TestSharedNodeInitFailClearedByClose verifies Close clears the failure record and allows immediate reinit.
func TestSharedNodeInitFailClearedByClose(t *testing.T) {
	var calls int32
	x := &SharedNode[int]{InitFailRetryInterval: time.Hour}
	_ = x.InitWithClose(types.Config{}, "testNode", "resource3", false, func() (int, error) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return 0, errors.New("first fail")
		}
		return 42, nil
	}, nil)

	if _, err := x.GetSafely(); err == nil {
		t.Fatal("expected first init error")
	}
	// fast-fail expected within the window; Close clears the record and allows an immediate retry
	_ = x.Close()
	v, err := x.GetSafely()
	if err != nil || v != 42 {
		t.Fatalf("after Close: v=%d err=%v", v, err)
	}
}

// TestSharedNodeSetStatusFromInitFunc is a regression guard: GetSafely calls
// InitInstanceFunc while holding x.Locker; if that callback also calls SetStatus
// (as NetNode.initConnect does via setDisconnected), a SetStatus that takes
// x.Locker would self-deadlock. This test catches that with a short timeout.
func TestSharedNodeSetStatusFromInitFunc(t *testing.T) {
	x := &SharedNode[string]{}
	_ = x.InitWithClose(types.Config{}, "testNode", "local-resource", false, func() (string, error) {
		// Sync status from within the init callback, mirroring NetNode.initConnect.
		x.SetStatus(types.StatusConnected, "dial ok")
		return "client", nil
	}, nil)

	done := make(chan struct{})
	go func() {
		// GetSafely holds x.Locker while invoking the InitInstanceFunc callback above.
		_, _ = x.GetSafely()
		close(done)
	}()
	select {
	case <-done:
		// succeeded: no deadlock
	case <-time.After(3 * time.Second):
		t.Fatal("GetSafely deadlocked: SetStatus inside InitInstanceFunc blocked for over 3s")
	}
	if info := x.ConnectionStatus(); info.Status != types.StatusConnected {
		t.Fatalf("status=%s, want connected", info.Status)
	}
}

// stubRegistry is a minimal types.ResourceRegistry mirroring
// engine.resourceRegistry, so tests here do not need to import engine.
type stubRegistry struct {
	mu    sync.RWMutex
	items map[string]any
}

func newStubRegistry() *stubRegistry {
	return &stubRegistry{items: map[string]any{}}
}

func (r *stubRegistry) Lookup(id string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.items[id]
	return v, ok
}

func (r *stubRegistry) Register(id string, resource any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[id] = resource
}

func (r *stubRegistry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, id)
}

// stubChainCtx overrides only Resources/ResourceRegistry/GetNodeById; the nil
// embedded interface panics loudly if any other ChainCtx method is touched.
// nodes is optional: when nil, GetNodeById reports not-found (chain lookup miss).
type stubChainCtx struct {
	types.ChainCtx
	reg   *stubRegistry
	nodes map[string]types.NodeCtx
}

func (s *stubChainCtx) Resources() types.ResourceLookup          { return s.reg }
func (s *stubChainCtx) ResourceRegistry() types.ResourceRegistry { return s.reg }
func (s *stubChainCtx) GetNodeById(id types.RuleNodeId) (types.NodeCtx, bool) {
	n, ok := s.nodes[id.Id]
	return n, ok
}

// stubConn is an identifiable fake connection.
type stubConn struct{ addr string }

// otherConn is an unrelated connection type for the cross-type case.
type otherConn struct{ addr string }

// stubEndpoint mimics an endpoint embedding SharedNode: EndpointAspect registers
// the endpoint instance itself (not a connHolder) into the chain directory, so a
// ref:// borrower must resolve it through the GetInstance fallback.
type stubEndpoint struct {
	SharedNode[*stubConn]
}

func bindChainConfiguration(ctx types.ChainCtx, nodeId string) types.Configuration {
	return types.Configuration{
		types.NodeConfigurationKeyChainCtx:       ctx,
		types.NodeConfigurationKeySelfDefinition: types.RuleNode{Id: nodeId},
	}
}

// TestUnpackHolderEndpointFallbackHit verifies that a borrower resolves a
// chain-registered endpoint to the endpoint's underlying connection.
func TestUnpackHolderEndpointFallbackHit(t *testing.T) {
	ctx := &stubChainCtx{reg: newStubRegistry()}

	ep := &stubEndpoint{}
	_ = ep.InitWithClose(types.Config{}, "stub/endpoint", "ep-server", false, func() (*stubConn, error) {
		return &stubConn{addr: "ep-server"}, nil
	}, nil)
	ctx.reg.Register("ep1", ep)

	borrower := &SharedNode[*stubConn]{}
	_ = borrower.InitWithClose(types.Config{}, "stub/borrower", "ref://ep1", false, nil, nil)
	borrower.BindChain(bindChainConfiguration(ctx, "borrower"))

	got, err := borrower.GetSafely()
	if err != nil {
		t.Fatalf("borrower GetSafely: %v", err)
	}
	underlying, err := ep.GetSafely()
	if err != nil {
		t.Fatalf("endpoint GetSafely: %v", err)
	}
	if got != underlying {
		t.Fatalf("borrower got %p, want the endpoint's connection %p", got, underlying)
	}
	if got.addr != "ep-server" {
		t.Fatalf("connection addr = %q, want ep-server", got.addr)
	}
}

// TestUnpackHolderEndpointFallbackCrossType verifies that borrowing an endpoint
// whose underlying connection has a different type reports an incompatible error.
func TestUnpackHolderEndpointFallbackCrossType(t *testing.T) {
	ctx := &stubChainCtx{reg: newStubRegistry()}

	ep := &stubEndpoint{}
	_ = ep.InitWithClose(types.Config{}, "stub/endpoint", "ep-server", false, func() (*stubConn, error) {
		return &stubConn{addr: "ep-server"}, nil
	}, nil)
	ctx.reg.Register("ep1", ep)

	borrower := &SharedNode[*otherConn]{}
	_ = borrower.InitWithClose(types.Config{}, "stub/borrower", "ref://ep1", false, nil, nil)
	borrower.BindChain(bindChainConfiguration(ctx, "borrower"))

	if _, err := borrower.GetSafely(); err == nil {
		t.Fatal("cross-type borrow should fail")
	} else if !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("error should mention incompatibility, got: %v", err)
	}
}

// TestUnpackHolderEndpointFallbackCycle verifies that a circular ref:// chain is
// rejected instead of recursing until stack overflow.
func TestUnpackHolderEndpointFallbackCycle(t *testing.T) {
	ctx := &stubChainCtx{reg: newStubRegistry()}

	// An endpoint whose server refs itself and is chain-bound: the only setup
	// that can loop the chain-directory resolution path.
	ep := &stubEndpoint{}
	_ = ep.InitWithClose(types.Config{}, "stub/endpoint", "ref://ep1", false, func() (*stubConn, error) {
		return &stubConn{addr: "unused"}, nil
	}, nil)
	ep.BindChain(bindChainConfiguration(ctx, "ep1"))
	ctx.reg.Register("ep1", ep)

	if _, err := ep.GetSafely(); err == nil {
		t.Fatal("self-referential ref:// should fail")
	} else if !strings.Contains(err.Error(), "circular ref://") {
		t.Fatalf("error should mention circular reference, got: %v", err)
	}
}

// TestSharedNodeConnectionStatus verifies the connection state machine:
// a successful sync init -> Connected; a failed init -> Reconnecting (with message); SetStatus supports external updates.
func TestSharedNodeConnectionStatus(t *testing.T) {
	t.Run("init_now_success_sets_connected", func(t *testing.T) {
		x := &SharedNode[string]{}
		_ = x.InitWithClose(types.Config{}, "testNode", "local-resource", true, func() (string, error) {
			return "client", nil
		}, nil)
		if info := x.ConnectionStatus(); info.Status != types.StatusConnected {
			t.Fatalf("after successful init: status=%s, want connected", info.Status)
		}
	})

	t.Run("init_now_failure_strict_returns_error", func(t *testing.T) {
		// InitWithClose is strict: an initNow failure returns the error as-is (NodeClientInitNow gate).
		x := &SharedNode[string]{InitFailRetryInterval: 100 * time.Millisecond}
		err := x.InitWithClose(types.Config{}, "testNode", "local-resource", true, func() (string, error) {
			return "", errors.New("dial: connection refused")
		}, nil)
		if err == nil || !strings.Contains(err.Error(), "refused") {
			t.Fatalf("strict init: err=%v, want contains 'refused'", err)
		}
		// Status is still set to Reconnecting for diagnostics; the caller decides via the returned error.
		if info := x.ConnectionStatus(); info.Status != types.StatusReconnecting {
			t.Fatalf("after failed strict init: status=%s, want reconnecting", info.Status)
		}
	})

	t.Run("init_now_failure_softfail_sets_reconnecting", func(t *testing.T) {
		// InitWithCloseSoftFail swallows the error and sets Reconnecting, for tolerant endpoint startup.
		x := &SharedNode[string]{InitFailRetryInterval: 100 * time.Millisecond}
		err := x.InitWithCloseSoftFail(types.Config{}, "testNode", "local-resource", true, func() (string, error) {
			return "", errors.New("dial: connection refused")
		}, nil)
		if err != nil {
			t.Fatalf("soft-fail init: err=%v, want nil", err)
		}
		info := x.ConnectionStatus()
		if info.Status != types.StatusReconnecting {
			t.Fatalf("after failed soft-fail init: status=%s, want reconnecting", info.Status)
		}
		if !strings.Contains(info.Message, "refused") {
			t.Fatalf("status message=%q, want contains 'refused'", info.Message)
		}
	})

	t.Run("set_status_then_query", func(t *testing.T) {
		x := &SharedNode[string]{}
		_ = x.InitWithClose(types.Config{}, "testNode", "local-resource", true, func() (string, error) {
			return "client", nil
		}, nil)
		x.SetStatus(types.StatusDisconnected, "graceful stop")
		if info := x.ConnectionStatus(); info.Status != types.StatusDisconnected {
			t.Fatalf("after SetStatus: status=%s, want disconnected", info.Status)
		}
		if info := x.ConnectionStatus(); info.Message != "graceful stop" {
			t.Fatalf("status message=%q, want 'graceful stop'", info.Message)
		}
	})
}

// fakeLookup 实现 types.ResourceLookup，用于测试。
type fakeLookup struct {
	items map[string]any
}

func (f *fakeLookup) Lookup(id string) (any, bool) {
	v, ok := f.items[id]
	return v, ok
}

// TestConnHolder 验证稳定间接层的存取语义：重连只更新内部值，目录条目（holder 指针）不变。
func TestConnHolder(t *testing.T) {
	var h connHolder[*int]
	a, b := 1, 2
	h.store(&a)
	if got := h.load(); got != &a || *got != 1 {
		t.Fatalf("load after store a: got %v", got)
	}
	h.store(&b) // 模拟重连更新
	if got := h.load(); got != &b || *got != 2 {
		t.Fatalf("load after store b: got %v", got)
	}
}

// TestResolveResource 验证解析顺序：reg 目录优先；reg miss 且 pool=nil 时未命中。
// pool 回退路径（pool.Lookup）由集成测试覆盖（需完整 NodePool）。
func TestResolveResource(t *testing.T) {
	reg := &fakeLookup{items: map[string]any{"k": "v"}}
	if v, ok := ResolveResource(reg, nil, "k"); !ok || v != "v" {
		t.Fatalf("reg hit: got %v ok %v", v, ok)
	}
	if _, ok := ResolveResource(reg, nil, "missing"); ok {
		t.Fatal("expected not found when reg miss and pool nil")
	}
	if _, ok := ResolveResource(nil, nil, "k"); ok {
		t.Fatal("expected not found when reg nil and pool nil")
	}
}

// TestLoadConn 验证连接借用解包：命中 holder 取最新；跨类型 / 未命中 / nil 连接返回 false。
func TestLoadConn(t *testing.T) {
	a := 1
	holder := &connHolder[*int]{}
	holder.store(&a)
	reg := &fakeLookup{items: map[string]any{"k": holder}}
	if v, ok := LoadConn[*int](reg, nil, "k"); !ok || v != &a {
		t.Fatalf("LoadConn hit: got %v ok %v", v, ok)
	}
	// 命中但非 holder（跨类型 ref）→ false
	regWrong := &fakeLookup{items: map[string]any{"k": "not-a-holder"}}
	if _, ok := LoadConn[*int](regWrong, nil, "k"); ok {
		t.Fatal("expected false for non-holder type")
	}
	// 未命中 → false
	if _, ok := LoadConn[*int](reg, nil, "missing"); ok {
		t.Fatal("expected false for miss")
	}
	// holder 存在但连接为 nil（零值）→ false
	nilHolder := &connHolder[*int]{}
	regNil := &fakeLookup{items: map[string]any{"k": nilHolder}}
	if _, ok := LoadConn[*int](regNil, nil, "k"); ok {
		t.Fatal("expected false for nil connection in holder")
	}
}

// TestErrSentinels 确保错误哨兵可被 errors.Is 识别（net_node 兜底分支依赖此）。
func TestErrSentinels(t *testing.T) {
	if !errors.Is(ErrNotTargetSender, ErrNotTargetSender) {
		t.Fatal("ErrNotTargetSender not identifiable")
	}
	if !errors.Is(ErrResourceNotFound, ErrResourceNotFound) {
		t.Fatal("ErrResourceNotFound not identifiable")
	}
}

// closeableConn records Close calls for the Close fallback path that calls
// the client's own Close method when no CloseFunc is set.
type closeableConn struct {
	stubConn
	closed bool
}

func (c *closeableConn) Close() error {
	c.closed = true
	return nil
}

// stubNodePool overrides only the members the ref:// pool fallback touches;
// any other NodePool method panics via the nil embedded interface.
type stubNodePool struct {
	types.NodePool
	items       map[string]any
	ctxs        map[string]types.SharedNodeCtx
	getInstance func(id string) (any, error)
}

func (p *stubNodePool) Lookup(id string) (any, bool) {
	v, ok := p.items[id]
	return v, ok
}

func (p *stubNodePool) Get(id string) (types.SharedNodeCtx, bool) {
	c, ok := p.ctxs[id]
	return c, ok
}

func (p *stubNodePool) GetInstance(id string) (any, error) {
	if p.getInstance != nil {
		return p.getInstance(id)
	}
	if v, ok := p.items[id]; ok {
		return v, nil
	}
	return nil, errors.New("not found")
}

// sharedNodeCtxOf adapts a node to types.SharedNodeCtx for the chain-scoped
// lazy-init path; unimplemented NodeCtx methods panic via the nil embedded
// interface so unexpected calls fail loudly.
type sharedNodeCtxOf struct {
	types.NodeCtx
	node        interface{}
	getInstance func() (interface{}, error)
}

func (s *sharedNodeCtxOf) GetInstance() (interface{}, error) { return s.getInstance() }
func (s *sharedNodeCtxOf) GetNode() interface{}              { return s.node }

// foreignSharedNode implements types.SharedNode WITHOUT embedding
// base.SharedNode, exercising the plain GetInstance fallback in unpackHolder.
type foreignSharedNode struct {
	conn *stubConn
	err  error
}

func (f *foreignSharedNode) New() types.Node                              { return &foreignSharedNode{} }
func (f *foreignSharedNode) Type() string                                 { return "stub/foreign" }
func (f *foreignSharedNode) Init(types.Config, types.Configuration) error { return nil }
func (f *foreignSharedNode) OnMsg(_ types.RuleContext, _ types.RuleMsg)   {}
func (f *foreignSharedNode) Destroy()                                     {}
func (f *foreignSharedNode) GetInstance() (interface{}, error)            { return f.conn, f.err }

func newLocalNode(nodeId string) *SharedNode[*stubConn] {
	return &SharedNode[*stubConn]{}
}

// TestNodeUtilsGetVars: the vars entry is wrapped as {vars: value}; absent key returns nil.
func TestNodeUtilsGetVars(t *testing.T) {
	vars := map[string]interface{}{"k": "v"}
	got := NodeUtils.GetVars(types.Configuration{types.Vars: vars})
	assert.Equal(t, vars, got[types.Vars])
	assert.Nil(t, NodeUtils.GetVars(types.Configuration{}))
}

// TestNodeUtilsGetEvn: GetEvn passes useMetadata=false while GetEvnAndMetadata
// passes true; both return the env produced by ctx.GetEnv.
func TestNodeUtilsGetEvn(t *testing.T) {
	env := map[string]interface{}{"msg": "m"}
	ctx := &stubCtx{env: env}
	msg := types.NewMsg(0, "", types.TEXT, types.NewMetadata(), "")

	assert.Equal(t, env, NodeUtils.GetEvn(ctx, msg))
	assert.False(t, ctx.useMeta[0])

	assert.Equal(t, env, NodeUtils.GetEvnAndMetadata(ctx, msg))
	assert.True(t, ctx.useMeta[1])
}

// TestNodeUtilsChainAndSelfDefinition: typed hit, missing key and wrong type.
func TestNodeUtilsChainAndSelfDefinition(t *testing.T) {
	chain := &stubChainCtx{reg: newStubRegistry()}
	rn := types.RuleNode{Id: "n1"}
	cfg := types.Configuration{
		types.NodeConfigurationKeyChainCtx:       chain,
		types.NodeConfigurationKeySelfDefinition: rn,
	}
	assert.Equal(t, chain, NodeUtils.GetChainCtx(cfg))
	assert.Equal(t, rn, NodeUtils.GetSelfDefinition(cfg))

	wrong := types.Configuration{
		types.NodeConfigurationKeyChainCtx:       42,
		types.NodeConfigurationKeySelfDefinition: "not-a-node",
	}
	assert.Nil(t, NodeUtils.GetChainCtx(wrong))
	assert.Equal(t, types.RuleNode{}, NodeUtils.GetSelfDefinition(wrong))

	assert.Nil(t, NodeUtils.GetChainCtx(types.Configuration{}))
	assert.Equal(t, types.RuleNode{}, NodeUtils.GetSelfDefinition(types.Configuration{}))
}

// TestNodeUtilsIsInitNetResource: presence of the marker key only.
func TestNodeUtilsIsInitNetResource(t *testing.T) {
	assert.True(t, NodeUtils.IsInitNetResource(types.Config{}, types.Configuration{
		types.NodeConfigurationKeyIsInitNetResource: true,
	}))
	assert.False(t, NodeUtils.IsInitNetResource(types.Config{}, types.Configuration{}))
}

// TestNodeUtilsGetDataByType prepares data per message type: JSON parsed to a
// map, BINARY to bytes (copied when writable), other types as raw string.
func TestNodeUtilsGetDataByType(t *testing.T) {
	tests := []struct {
		name     string
		msg      types.RuleMsg
		readOnly bool
		check    func(t *testing.T, data interface{}, msg types.RuleMsg)
	}{
		{
			name:     "json_readonly",
			msg:      types.NewMsg(0, "", types.JSON, types.NewMetadata(), `{"a":1}`),
			readOnly: true,
			check: func(t *testing.T, data interface{}, _ types.RuleMsg) {
				assert.Equal(t, map[string]interface{}{"a": float64(1)}, data)
			},
		},
		{
			name:     "json_readonly_invalid_falls_back_to_string",
			msg:      types.NewMsg(0, "", types.JSON, types.NewMetadata(), `{bad`),
			readOnly: true,
			check: func(t *testing.T, data interface{}, _ types.RuleMsg) {
				assert.Equal(t, `{bad`, data)
			},
		},
		{
			name:     "json_writable_reparse",
			msg:      types.NewMsg(0, "", types.JSON, types.NewMetadata(), `{"b":2}`),
			readOnly: false,
			check: func(t *testing.T, data interface{}, _ types.RuleMsg) {
				assert.Equal(t, map[string]interface{}{"b": float64(2)}, data)
			},
		},
		{
			name:     "json_writable_invalid_falls_back_to_string",
			msg:      types.NewMsg(0, "", types.JSON, types.NewMetadata(), ``),
			readOnly: false,
			check: func(t *testing.T, data interface{}, _ types.RuleMsg) {
				assert.Equal(t, "", data)
			},
		},
		{
			name:     "binary_readonly_shares_bytes",
			msg:      types.NewMsgFromBytes(0, "", types.BINARY, types.NewMetadata(), []byte{1, 2, 3}),
			readOnly: true,
			check: func(t *testing.T, data interface{}, _ types.RuleMsg) {
				assert.Equal(t, []byte{1, 2, 3}, data)
			},
		},
		{
			name:     "binary_writable_is_a_copy",
			msg:      types.NewMsgFromBytes(0, "", types.BINARY, types.NewMetadata(), []byte{4, 5}),
			readOnly: false,
			check: func(t *testing.T, data interface{}, msg types.RuleMsg) {
				b, ok := data.([]byte)
				if !ok || len(b) != 2 {
					t.Fatalf("writable binary should be []byte, got %T", data)
				}
				b[0] = 99
				if orig := msg.GetBytes(); orig[0] != 4 {
					t.Fatal("mutating the returned copy must not corrupt the message payload")
				}
			},
		},
		{
			name:     "binary_writable_nil_payload",
			msg:      types.RuleMsg{DataType: types.BINARY},
			readOnly: false,
			check: func(t *testing.T, data interface{}, _ types.RuleMsg) {
				assert.Nil(t, data)
			},
		},
		{
			name:     "text_uses_raw_string",
			msg:      types.NewMsg(0, "", types.TEXT, types.NewMetadata(), "hello"),
			readOnly: true,
			check: func(t *testing.T, data interface{}, _ types.RuleMsg) {
				assert.Equal(t, "hello", data)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, NodeUtils.GetDataByType(tc.msg, tc.readOnly), tc.msg)
		})
	}
}

// TestNodeUtilsTrimStrings: string values are trimmed in place, others untouched.
func TestNodeUtilsTrimStrings(t *testing.T) {
	config := types.Configuration{
		"host":  "  127.0.0.1  ",
		"port":  8080,
		"empty": "   ",
	}
	NodeUtils.TrimStrings(config)
	assert.Equal(t, "127.0.0.1", config["host"])
	assert.Equal(t, 8080, config["port"])
	assert.Equal(t, "", config["empty"])
}

// TestSharedNodeInit covers the plain Init wrapper: local init-now, lazy local,
// and the pool form which defers to the NodePool on first use.
func TestSharedNodeInit(t *testing.T) {
	t.Run("local_init_now", func(t *testing.T) {
		x := &SharedNode[string]{}
		assert.Nil(t, x.Init(types.Config{}, "testNode", "srv", true, func() (string, error) {
			return "client", nil
		}))
		assert.True(t, x.IsInit())
		v, ok := x.Instance()
		assert.True(t, ok)
		assert.Equal(t, "client", v)
		assert.Equal(t, types.StatusConnected, x.ConnectionStatus().Status)
	})

	t.Run("local_lazy", func(t *testing.T) {
		x := &SharedNode[string]{}
		assert.Nil(t, x.Init(types.Config{}, "testNode", "srv", false, func() (string, error) {
			return "client", nil
		}))
		assert.True(t, x.IsInit())
		assert.False(t, x.Initialized())
		v, err := x.GetSafely()
		assert.Nil(t, err)
		assert.Equal(t, "client", v)
	})

	t.Run("pool_form_defers_to_pool", func(t *testing.T) {
		x := &SharedNode[string]{}
		assert.Nil(t, x.Init(types.Config{}, "testNode", "ref://pool1", true, func() (string, error) {
			return "client", nil
		}))
		assert.True(t, x.IsFromPool())
		// initNow is ignored for pool nodes: the instance comes from the pool,
		// which is nil here, so GetSafely must report ErrNodePoolNil.
		_, err := x.GetSafely()
		assert.True(t, errors.Is(err, ErrNodePoolNil))
	})
}

// TestSharedNodeAccessors: IsInit/IsFromPool/RefTarget/IsBorrower/Initialized/Instance
// across the local and ref:// modes of the lifecycle.
func TestSharedNodeAccessors(t *testing.T) {
	t.Run("uninitialized", func(t *testing.T) {
		x := &SharedNode[string]{}
		assert.False(t, x.IsInit())
		assert.False(t, x.IsFromPool())
		assert.False(t, x.Initialized())
		_, ok := x.Instance()
		assert.False(t, ok)
	})

	t.Run("local", func(t *testing.T) {
		x := &SharedNode[string]{}
		_ = x.Init(types.Config{}, "testNode", "srv", false, func() (string, error) {
			return "client", nil
		})
		assert.False(t, x.IsFromPool())
		assert.Equal(t, "", x.RefTarget())
		assert.False(t, x.IsBorrower())
		assert.False(t, x.Initialized())
		v, err := x.GetSafely()
		assert.Nil(t, err)
		assert.Equal(t, "client", v)
		v2, ok := x.Instance()
		assert.True(t, ok)
		assert.Equal(t, "client", v2)
		assert.Nil(t, x.Close())
		_, ok = x.Instance()
		assert.False(t, ok)
	})

	t.Run("borrower", func(t *testing.T) {
		x := &SharedNode[string]{}
		_ = x.Init(types.Config{}, "testNode", "ref://src1", false, nil)
		assert.True(t, x.IsFromPool())
		assert.Equal(t, "src1", x.RefTarget())
		assert.True(t, x.IsBorrower())
	})
}

// TestSharedNodeGetSafelyPoolFallback: with no chain binding the ref:// id is
// resolved from RuleConfig.NodePool; nil pool, pool error and type mismatch
// each surface a distinct error.
func TestSharedNodeGetSafelyPoolFallback(t *testing.T) {
	t.Run("pool_hit", func(t *testing.T) {
		pool := &stubNodePool{items: map[string]any{"p1": "pool-conn"}}
		x := &SharedNode[string]{}
		_ = x.InitWithClose(types.Config{NodePool: pool}, "testNode", "ref://p1", false, nil, nil)
		v, err := x.GetSafely()
		assert.Nil(t, err)
		assert.Equal(t, "pool-conn", v)
		// GetInstance exposes the same value as interface{}
		iv, err := x.GetInstance()
		assert.Nil(t, err)
		assert.Equal(t, "pool-conn", iv)
	})

	t.Run("nil_pool", func(t *testing.T) {
		x := &SharedNode[string]{}
		_ = x.InitWithClose(types.Config{}, "testNode", "ref://p1", false, nil, nil)
		_, err := x.GetSafely()
		assert.True(t, errors.Is(err, ErrNodePoolNil))
	})

	t.Run("pool_error", func(t *testing.T) {
		pool := &stubNodePool{getInstance: func(string) (any, error) {
			return nil, errors.New("gone")
		}}
		x := &SharedNode[string]{}
		_ = x.InitWithClose(types.Config{NodePool: pool}, "testNode", "ref://p1", false, nil, nil)
		_, err := x.GetSafely()
		assert.True(t, strings.Contains(err.Error(), "gone"))
	})

	t.Run("pool_type_mismatch", func(t *testing.T) {
		pool := &stubNodePool{items: map[string]any{"p1": 123}}
		x := &SharedNode[string]{}
		_ = x.InitWithClose(types.Config{NodePool: pool}, "testNode", "ref://p1", false, nil, nil)
		_, err := x.GetSafely()
		assert.True(t, strings.Contains(err.Error(), "incompatible"))
	})
}

// TestSharedNodeGetSafelyNotInit: a local node with no InitInstanceFunc reports ErrClientNotInit.
func TestSharedNodeGetSafelyNotInit(t *testing.T) {
	x := &SharedNode[string]{}
	_ = x.InitWithClose(types.Config{}, "testNode", "srv", false, nil, nil)
	_, err := x.GetSafely()
	assert.True(t, errors.Is(err, ErrClientNotInit))
}

// TestSharedNodeGetSafelyConcurrentSuccess: concurrent callers share one init;
// callers waiting on the write lock return via the double check.
func TestSharedNodeGetSafelyConcurrentSuccess(t *testing.T) {
	var calls int32
	x := &SharedNode[string]{}
	_ = x.InitWithClose(types.Config{}, "testNode", "srv", false, func() (string, error) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(20 * time.Millisecond)
		return "client", nil
	}, nil)

	const n = 10
	results := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, err := x.GetSafely()
			assert.Nil(t, err)
			results[i] = v
		}(i)
	}
	wg.Wait()
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
	for _, v := range results {
		assert.Equal(t, "client", v)
	}
}

// TestSharedNodeInitPartialClientCleanup: when init fails but returns a
// partially initialized client, CloseFunc is invoked on it.
func TestSharedNodeInitPartialClientCleanup(t *testing.T) {
	var closed *stubConn
	x := &SharedNode[*stubConn]{}
	_ = x.InitWithClose(types.Config{}, "testNode", "srv", false, func() (*stubConn, error) {
		return &stubConn{addr: "partial"}, errors.New("handshake failed")
	}, func(c *stubConn) error {
		closed = c
		return nil
	})
	_, err := x.GetSafely()
	assert.NotNil(t, err)
	if closed == nil || closed.addr != "partial" {
		t.Fatalf("CloseFunc should receive the partial client, got %v", closed)
	}
}

// TestSharedNodeInitRetryIntervalDefault: with no custom interval the 30s
// default window applies, so immediate retries fast-fail.
func TestSharedNodeInitRetryIntervalDefault(t *testing.T) {
	var calls int32
	x := &SharedNode[string]{}
	_ = x.InitWithClose(types.Config{}, "testNode", "srv", false, func() (string, error) {
		atomic.AddInt32(&calls, 1)
		return "", errors.New("down")
	}, nil)
	_, err := x.GetSafely()
	assert.NotNil(t, err)
	_, err = x.GetSafely()
	assert.NotNil(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

// TestIsZeroValue: invalid (nil interface) and typed zero values are detected
// without panicking on non-comparable types.
func TestIsZeroValue(t *testing.T) {
	assert.True(t, isZeroValue[interface{}](nil))
	assert.True(t, isZeroValue(""))
	assert.True(t, isZeroValue(0))
	assert.True(t, isZeroValue((*stubConn)(nil)))
	assert.True(t, isZeroValue(map[string]int(nil)))
	assert.False(t, isZeroValue("x"))
	assert.False(t, isZeroValue(&stubConn{}))
}

// TestConnHolderStatus: status snapshots are independent of the held connection.
func TestConnHolderStatus(t *testing.T) {
	h := &connHolder[*stubConn]{}
	c := &stubConn{addr: "c"}
	h.store(c)
	h.storeStatus(types.StatusReconnecting, "dialing")
	assert.Equal(t, c, h.load())
	si := h.loadStatus()
	assert.Equal(t, types.StatusReconnecting, si.Status)
	assert.Equal(t, "dialing", si.Message)
}

// TestSharedNodeRefresh: reconnect updates the local cache, the chain-scoped
// holder and the status; borrowers immediately observe the new connection.
func TestSharedNodeRefresh(t *testing.T) {
	chain := &stubChainCtx{reg: newStubRegistry()}
	src := newLocalNode("n1")
	_ = src.InitWithClose(types.Config{}, "testNode", "srv", false, func() (*stubConn, error) {
		return &stubConn{addr: "old"}, nil
	}, nil)
	src.BindChain(bindChainConfiguration(chain, "n1"))
	old, err := src.GetSafely()
	assert.Nil(t, err)

	fresh := &stubConn{addr: "new"}
	src.Refresh(fresh)

	v, ok := src.Instance()
	assert.True(t, ok)
	assert.Equal(t, fresh, v)
	assert.Equal(t, types.StatusConnected, src.ConnectionStatus().Status)
	inst, found := chain.reg.Lookup("n1")
	assert.True(t, found)
	if h, ok := inst.(*connHolder[*stubConn]); !ok || h.load() != fresh {
		t.Fatalf("holder should expose the refreshed connection, got %v", inst)
	}

	borrower := &SharedNode[*stubConn]{}
	_ = borrower.InitWithClose(types.Config{}, "testNode", "ref://n1", false, nil, nil)
	borrower.BindChain(bindChainConfiguration(chain, "borrower"))
	got, err := borrower.GetSafely()
	assert.Nil(t, err)
	assert.Equal(t, fresh, got)
	assert.NotEqual(t, old, got)
}

// TestSharedNodeRefreshAfterFail: Refresh clears the init failure record, so
// the next GetSafely serves the refreshed client instead of fast-failing.
func TestSharedNodeRefreshAfterFail(t *testing.T) {
	var calls int32
	x := &SharedNode[*stubConn]{InitFailRetryInterval: time.Hour}
	_ = x.InitWithClose(types.Config{}, "testNode", "srv", false, func() (*stubConn, error) {
		atomic.AddInt32(&calls, 1)
		return nil, errors.New("down")
	}, nil)
	_, err := x.GetSafely()
	assert.NotNil(t, err)

	fresh := &stubConn{addr: "reconnected"}
	x.Refresh(fresh)
	v, err := x.GetSafely()
	assert.Nil(t, err)
	assert.Equal(t, fresh, v)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

// TestSharedNodeBindChainBackfillRegister: an initNow connection established
// before BindChain is backfilled into the chain directory.
func TestSharedNodeBindChainBackfillRegister(t *testing.T) {
	chain := &stubChainCtx{reg: newStubRegistry()}
	x := newLocalNode("n1")
	_ = x.InitWithClose(types.Config{}, "testNode", "srv", true, func() (*stubConn, error) {
		return &stubConn{addr: "c"}, nil
	}, nil)
	x.BindChain(bindChainConfiguration(chain, "n1"))
	inst, found := chain.reg.Lookup("n1")
	assert.True(t, found)
	if h, ok := inst.(*connHolder[*stubConn]); !ok || h.load().addr != "c" {
		t.Fatalf("BindChain should register the existing client, got %v", inst)
	}
}

// TestSharedNodeBindChainNilPointerCleanup: a non-nil interface wrapping a nil
// ChainCtx pointer must be treated as no chain binding.
func TestSharedNodeBindChainNilPointerCleanup(t *testing.T) {
	var nilChain *stubChainCtx
	x := newLocalNode("n1")
	_ = x.InitWithClose(types.Config{}, "testNode", "srv", false, func() (*stubConn, error) {
		return &stubConn{addr: "c"}, nil
	}, nil)
	x.BindChain(types.Configuration{
		types.NodeConfigurationKeyChainCtx:       nilChain,
		types.NodeConfigurationKeySelfDefinition: types.RuleNode{Id: "n1"},
	})
	assert.True(t, x.chainCtx == nil)
	v, err := x.GetSafely()
	assert.Nil(t, err)
	assert.Equal(t, "c", v.addr)
}

// TestSharedNodeClose covers every Close path: borrower noop, never connected,
// CloseFunc error propagation, client Close fallback, and no cleanup available.
func TestSharedNodeClose(t *testing.T) {
	t.Run("borrower_is_noop", func(t *testing.T) {
		x := &SharedNode[*stubConn]{}
		_ = x.InitWithClose(types.Config{}, "testNode", "ref://src1", false, nil, nil)
		assert.Nil(t, x.Close())
	})

	t.Run("never_connected_clears_failure", func(t *testing.T) {
		var calls int32
		x := &SharedNode[string]{InitFailRetryInterval: time.Hour}
		_ = x.InitWithClose(types.Config{}, "testNode", "srv", false, func() (string, error) {
			if atomic.AddInt32(&calls, 1) == 1 {
				return "", errors.New("down")
			}
			return "ok", nil
		}, nil)
		_, _ = x.GetSafely()
		assert.Nil(t, x.Close())
		assert.Equal(t, types.StatusDisconnected, x.ConnectionStatus().Status)
		// the failure record is cleared: init retried immediately on next use
		v, err := x.GetSafely()
		assert.Nil(t, err)
		assert.Equal(t, "ok", v)
	})

	t.Run("close_func_error_propagates", func(t *testing.T) {
		closeErr := errors.New("close failed")
		x := &SharedNode[*stubConn]{}
		_ = x.InitWithClose(types.Config{}, "testNode", "srv", false, func() (*stubConn, error) {
			return &stubConn{addr: "c"}, nil
		}, func(*stubConn) error { return closeErr })
		_, _ = x.GetSafely()
		assert.True(t, errors.Is(x.Close(), closeErr))
		_, ok := x.Instance()
		assert.False(t, ok)
		assert.Equal(t, types.StatusDisconnected, x.ConnectionStatus().Status)
	})

	t.Run("client_close_fallback", func(t *testing.T) {
		cc := &closeableConn{stubConn: stubConn{addr: "c"}}
		x := &SharedNode[*closeableConn]{}
		x.NodeType = "testNode"
		x.InitInstanceFunc = func() (*closeableConn, error) { return cc, nil }
		v, err := x.GetSafely()
		assert.Nil(t, err)
		assert.Equal(t, cc, v)
		assert.Nil(t, x.Close())
		assert.True(t, cc.closed)
	})

	t.Run("no_cleanup_available", func(t *testing.T) {
		x := &SharedNode[*stubConn]{}
		x.NodeType = "testNode"
		x.InitInstanceFunc = func() (*stubConn, error) { return &stubConn{addr: "c"}, nil }
		_, _ = x.GetSafely()
		assert.Nil(t, x.Close())
		_, ok := x.Instance()
		assert.False(t, ok)
	})
}

// TestSharedNodeCloseUnregistersChainEntry: Close removes the node's own
// holder from the chain directory before closing the connection.
func TestSharedNodeCloseUnregistersChainEntry(t *testing.T) {
	chain := &stubChainCtx{reg: newStubRegistry()}
	x := newLocalNode("n1")
	_ = x.InitWithClose(types.Config{}, "testNode", "srv", false, func() (*stubConn, error) {
		return &stubConn{addr: "c"}, nil
	}, nil)
	x.BindChain(bindChainConfiguration(chain, "n1"))
	_, _ = x.GetSafely()
	_, found := chain.reg.Lookup("n1")
	assert.True(t, found)

	assert.Nil(t, x.Close())
	_, found = chain.reg.Lookup("n1")
	assert.False(t, found)
	assert.False(t, x.isRegistered)
}

// TestSharedNodeCloseForeignEntryPreserved: when another holder took over the
// node's directory id, Close must not unregister the foreign entry (soft CAS).
func TestSharedNodeCloseForeignEntryPreserved(t *testing.T) {
	chain := &stubChainCtx{reg: newStubRegistry()}
	x := newLocalNode("n1")
	_ = x.InitWithClose(types.Config{}, "testNode", "srv", false, func() (*stubConn, error) {
		return &stubConn{addr: "c"}, nil
	}, nil)
	x.BindChain(bindChainConfiguration(chain, "n1"))
	_, _ = x.GetSafely()

	foreign := &connHolder[*stubConn]{}
	foreign.store(&stubConn{addr: "other"})
	chain.reg.Register("n1", foreign)

	assert.Nil(t, x.Close())
	inst, found := chain.reg.Lookup("n1")
	assert.True(t, found)
	if h, ok := inst.(*connHolder[*stubConn]); !ok || h != foreign {
		t.Fatal("Close must leave a foreign holder registered under a colliding id")
	}
}

// TestUnpackHolderNilConnection: a directory hit on an empty holder reports a
// nil-connection error instead of returning the zero value silently.
func TestUnpackHolderNilConnection(t *testing.T) {
	chain := &stubChainCtx{reg: newStubRegistry()}
	chain.reg.Register("src", &connHolder[*stubConn]{})
	borrower := &SharedNode[*stubConn]{}
	_ = borrower.InitWithClose(types.Config{}, "testNode", "ref://src", false, nil, nil)
	borrower.BindChain(bindChainConfiguration(chain, "borrower"))
	_, err := borrower.GetSafely()
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "connection is nil"))
}

// TestUnpackHolderForeignSharedNode: directory entries implementing
// types.SharedNode without embedding base.SharedNode resolve via GetInstance.
func TestUnpackHolderForeignSharedNode(t *testing.T) {
	t.Run("resolves_via_get_instance", func(t *testing.T) {
		chain := &stubChainCtx{reg: newStubRegistry()}
		want := &stubConn{addr: "foreign"}
		chain.reg.Register("src", &foreignSharedNode{conn: want})
		borrower := &SharedNode[*stubConn]{}
		_ = borrower.InitWithClose(types.Config{}, "testNode", "ref://src", false, nil, nil)
		borrower.BindChain(bindChainConfiguration(chain, "borrower"))
		got, err := borrower.GetSafely()
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("get_instance_error_propagates", func(t *testing.T) {
		chain := &stubChainCtx{reg: newStubRegistry()}
		chain.reg.Register("src", &foreignSharedNode{err: errors.New("boom")})
		borrower := &SharedNode[*stubConn]{}
		_ = borrower.InitWithClose(types.Config{}, "testNode", "ref://src", false, nil, nil)
		borrower.BindChain(bindChainConfiguration(chain, "borrower"))
		_, err := borrower.GetSafely()
		assert.True(t, strings.Contains(err.Error(), "boom"))
	})

	t.Run("typed_nil_connection", func(t *testing.T) {
		chain := &stubChainCtx{reg: newStubRegistry()}
		chain.reg.Register("src", &foreignSharedNode{conn: nil})
		borrower := &SharedNode[*stubConn]{}
		_ = borrower.InitWithClose(types.Config{}, "testNode", "ref://src", false, nil, nil)
		borrower.BindChain(bindChainConfiguration(chain, "borrower"))
		_, err := borrower.GetSafely()
		assert.True(t, strings.Contains(err.Error(), "connection is nil"))
	})
}

// TestUnpackHolderIncompatibleType: a directory entry of an unexpected type is
// reported instead of panicking on the assertion.
func TestUnpackHolderIncompatibleType(t *testing.T) {
	chain := &stubChainCtx{reg: newStubRegistry()}
	chain.reg.Register("src", "plain-string")
	borrower := &SharedNode[*stubConn]{}
	_ = borrower.InitWithClose(types.Config{}, "testNode", "ref://src", false, nil, nil)
	borrower.BindChain(bindChainConfiguration(chain, "borrower"))
	_, err := borrower.GetSafely()
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "incompatible"))
}

// TestSharedNodeCircularRef: mutually referencing ref:// endpoints are rejected
// instead of recursing until stack overflow.
func TestSharedNodeCircularRef(t *testing.T) {
	chain := &stubChainCtx{reg: newStubRegistry()}
	newEp := func(id, server string) *stubEndpoint {
		ep := &stubEndpoint{}
		_ = ep.InitWithClose(types.Config{}, "stub/endpoint", server, false, func() (*stubConn, error) {
			return &stubConn{addr: "unused"}, nil
		}, nil)
		ep.BindChain(bindChainConfiguration(chain, id))
		chain.reg.Register(id, ep)
		return ep
	}
	newEp("a", "ref://b")
	newEp("b", "ref://a")

	borrower := &SharedNode[*stubConn]{}
	_ = borrower.InitWithClose(types.Config{}, "testNode", "ref://a", false, nil, nil)
	borrower.BindChain(bindChainConfiguration(chain, "borrower"))
	_, err := borrower.GetSafely()
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "circular ref://"))
}

// TestSharedNodeLazyChainTargetInit: a chain-directory miss falls back to the
// chain node table; a non-borrower target is initialized on demand and its
// connection becomes visible for a second lookup.
func TestSharedNodeLazyChainTargetInit(t *testing.T) {
	t.Run("triggers_target_init", func(t *testing.T) {
		chain := &stubChainCtx{reg: newStubRegistry()}
		target := newLocalNode("src1")
		_ = target.InitWithClose(types.Config{}, "testNode", "srv", false, func() (*stubConn, error) {
			return &stubConn{addr: "lazy"}, nil
		}, nil)
		target.BindChain(bindChainConfiguration(chain, "src1"))
		chain.nodes = map[string]types.NodeCtx{
			"src1": &sharedNodeCtxOf{node: target, getInstance: target.GetInstance},
		}

		borrower := &SharedNode[*stubConn]{}
		_ = borrower.InitWithClose(types.Config{}, "testNode", "ref://src1", false, nil, nil)
		borrower.BindChain(bindChainConfiguration(chain, "borrower"))
		got, err := borrower.GetSafely()
		assert.Nil(t, err)
		assert.Equal(t, "lazy", got.addr)
	})

	t.Run("target_init_error_wrapped", func(t *testing.T) {
		chain := &stubChainCtx{reg: newStubRegistry()}
		chain.nodes = map[string]types.NodeCtx{
			"src1": &sharedNodeCtxOf{
				node: newLocalNode("src1"),
				getInstance: func() (interface{}, error) {
					return nil, errors.New("boom")
				},
			},
		}
		borrower := &SharedNode[*stubConn]{}
		_ = borrower.InitWithClose(types.Config{}, "testNode", "ref://src1", false, nil, nil)
		borrower.BindChain(bindChainConfiguration(chain, "borrower"))
		_, err := borrower.GetSafely()
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "chain node src1 init"))
		assert.True(t, strings.Contains(err.Error(), "boom"))
	})

	t.Run("borrower_target_not_initialized", func(t *testing.T) {
		chain := &stubChainCtx{reg: newStubRegistry()}
		// the target is itself a ref:// borrower: lazy init must not be chased
		borrowerTarget := &SharedNode[*stubConn]{}
		_ = borrowerTarget.InitWithClose(types.Config{}, "testNode", "ref://upstream", false, nil, nil)
		called := false
		chain.nodes = map[string]types.NodeCtx{
			"src1": &sharedNodeCtxOf{
				node: borrowerTarget,
				getInstance: func() (interface{}, error) {
					called = true
					return nil, errors.New("must not be called")
				},
			},
		}
		borrower := &SharedNode[*stubConn]{}
		_ = borrower.InitWithClose(types.Config{}, "testNode", "ref://src1", false, nil, nil)
		borrower.BindChain(bindChainConfiguration(chain, "borrower"))
		_, err := borrower.GetSafely()
		assert.True(t, errors.Is(err, ErrNodePoolNil))
		assert.False(t, called)
	})

	t.Run("nil_target_node_skips_init", func(t *testing.T) {
		chain := &stubChainCtx{reg: newStubRegistry()}
		called := false
		chain.nodes = map[string]types.NodeCtx{
			"src1": &sharedNodeCtxOf{
				node: nil,
				getInstance: func() (interface{}, error) {
					called = true
					return nil, errors.New("must not be called")
				},
			},
		}
		borrower := &SharedNode[*stubConn]{}
		_ = borrower.InitWithClose(types.Config{}, "testNode", "ref://src1", false, nil, nil)
		borrower.BindChain(bindChainConfiguration(chain, "borrower"))
		_, err := borrower.GetSafely()
		assert.True(t, errors.Is(err, ErrNodePoolNil))
		assert.False(t, called)
	})

	t.Run("node_table_miss_falls_to_pool", func(t *testing.T) {
		chain := &stubChainCtx{reg: newStubRegistry()}
		borrower := &SharedNode[*stubConn]{}
		_ = borrower.InitWithClose(types.Config{}, "testNode", "ref://src1", false, nil, nil)
		borrower.BindChain(bindChainConfiguration(chain, "borrower"))
		_, err := borrower.GetSafely()
		assert.True(t, errors.Is(err, ErrNodePoolNil))
	})
}

// TestSharedNodeConnectionStatusDelegation: a borrower's status resolves from
// the chain holder first, then the pool source node, then its local snapshot.
func TestSharedNodeConnectionStatusDelegation(t *testing.T) {
	t.Run("reads_chain_holder", func(t *testing.T) {
		chain := &stubChainCtx{reg: newStubRegistry()}
		src := newLocalNode("n1")
		_ = src.InitWithClose(types.Config{}, "testNode", "srv", false, func() (*stubConn, error) {
			return &stubConn{addr: "c"}, nil
		}, nil)
		src.BindChain(bindChainConfiguration(chain, "n1"))
		_, _ = src.GetSafely()
		src.SetStatus(types.StatusReconnecting, "flaky")

		borrower := &SharedNode[*stubConn]{}
		_ = borrower.InitWithClose(types.Config{}, "testNode", "ref://n1", false, nil, nil)
		borrower.BindChain(bindChainConfiguration(chain, "borrower"))
		info := borrower.ConnectionStatus()
		assert.Equal(t, types.StatusReconnecting, info.Status)
		assert.Equal(t, "flaky", info.Message)
	})

	t.Run("chain_entry_not_holder_falls_through", func(t *testing.T) {
		chain := &stubChainCtx{reg: newStubRegistry()}
		chain.reg.Register("x", "not-a-holder")
		borrower := &SharedNode[*stubConn]{}
		_ = borrower.InitWithClose(types.Config{}, "testNode", "ref://x", false, nil, nil)
		borrower.BindChain(bindChainConfiguration(chain, "borrower"))
		assert.Equal(t, types.StatusNone, borrower.ConnectionStatus().Status)
	})

	t.Run("delegates_to_pool_source", func(t *testing.T) {
		src := &SharedNode[string]{}
		_ = src.InitWithClose(types.Config{}, "testNode", "srv", true, func() (string, error) {
			return "c", nil
		}, nil)
		src.SetStatus(types.StatusConnected, "ok")
		pool := &stubNodePool{ctxs: map[string]types.SharedNodeCtx{
			"p1": &sharedNodeCtxOf{node: src},
		}}
		borrower := &SharedNode[string]{}
		_ = borrower.InitWithClose(types.Config{NodePool: pool}, "testNode", "ref://p1", false, nil, nil)
		info := borrower.ConnectionStatus()
		assert.Equal(t, types.StatusConnected, info.Status)
		assert.Equal(t, "ok", info.Message)
	})

	t.Run("pool_source_without_status_falls_to_local", func(t *testing.T) {
		pool := &stubNodePool{ctxs: map[string]types.SharedNodeCtx{
			"p1": &sharedNodeCtxOf{node: "plain-node"},
		}}
		borrower := &SharedNode[string]{}
		_ = borrower.InitWithClose(types.Config{NodePool: pool}, "testNode", "ref://p1", false, nil, nil)
		borrower.SetStatus(types.StatusDisconnected, "local view")
		info := borrower.ConnectionStatus()
		assert.Equal(t, types.StatusDisconnected, info.Status)
		assert.Equal(t, "local view", info.Message)
	})

	t.Run("pool_miss_reads_local", func(t *testing.T) {
		pool := &stubNodePool{}
		borrower := &SharedNode[string]{}
		_ = borrower.InitWithClose(types.Config{NodePool: pool}, "testNode", "ref://p1", false, nil, nil)
		assert.Equal(t, types.StatusNone, borrower.ConnectionStatus().Status)
	})
}
